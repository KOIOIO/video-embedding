package tasks

import (
	"context"
	"testing"

	"github.com/pgvector/pgvector-go"

	"nlp-video-analysis/internal/model"
)

func TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow(t *testing.T) {
	called := false
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           60,
		EndSec:             120,
		NextStartSec:       0,
		Summary:            "定义",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 60, EndSec: 120, Text: "这一段在讲定义。"}}, 180, func(context.Context, int, int) (string, error) {
		called = true
		return "", nil
	}, nil)
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if called {
		t.Fatal("expected no refine ASR call")
	}
	if result.StartSec != 60 || result.EndSec != 120 {
		t.Fatalf("window = (%d, %d), want (60, 120)", result.StartSec, result.EndSec)
	}
	if result.Summary != "定义" {
		t.Fatalf("summary = %q, want %q", result.Summary, "定义")
	}
	if result.SummaryMode != "original" {
		t.Fatalf("summary mode = %q, want original", result.SummaryMode)
	}
	if result.Input != "定义\n这一段在讲定义。" {
		t.Fatalf("input = %q", result.Input)
	}
}

func TestBuildRefineSegmentInputUsesSingleShotASRForLowConfidence(t *testing.T) {
	var calls int
	var gotStart int
	var gotEnd int
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           60,
		EndSec:             120,
		NextStartSec:       123,
		Summary:            "例题",
		BoundaryConfidence: "low",
	}, []CoarseItem{{StartSec: 60, EndSec: 120, Text: "coarse text"}}, 200, func(_ context.Context, startSec int, endSec int) (string, error) {
		calls++
		gotStart = startSec
		gotEnd = endSec
		return "低置信度段补识别文本。", nil
	}, nil)
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("ASR calls = %d, want 1", calls)
	}
	if gotStart != 57 || gotEnd != 123 {
		t.Fatalf("ASR window = (%d, %d), want (57, 123)", gotStart, gotEnd)
	}
	if result.StartSec != 57 || result.EndSec != 123 {
		t.Fatalf("returned window = (%d, %d), want (57, 123)", result.StartSec, result.EndSec)
	}
	if result.Summary != "例题" {
		t.Fatalf("summary = %q, want %q", result.Summary, "例题")
	}
	if result.Input != "例题\n低置信度段补识别文本。" {
		t.Fatalf("input = %q", result.Input)
	}
}

func TestBuildRefineSegmentInputFallsBackToCoarseTextWhenLowConfidenceASRFails(t *testing.T) {
	var calls int
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           30,
		EndSec:             70,
		NextStartSec:       0,
		Summary:            "总结",
		BoundaryConfidence: "low",
	}, []CoarseItem{{StartSec: 0, EndSec: 90, Text: "这部分是总结内容。"}}, 100, func(context.Context, int, int) (string, error) {
		calls++
		return "", context.DeadlineExceeded
	}, nil)
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("ASR calls = %d, want 1", calls)
	}
	if result.StartSec != 30 || result.EndSec != 70 {
		t.Fatalf("returned window = (%d, %d), want original (30, 70)", result.StartSec, result.EndSec)
	}
	if result.Summary != "总结" {
		t.Fatalf("summary = %q, want %q", result.Summary, "总结")
	}
	if result.Input != "总结\n这部分是总结内容。" {
		t.Fatalf("input = %q", result.Input)
	}
}

func TestBuildRefineSegmentInputUsesContentSummaryOnlyWhenCoarseTextIsEmpty(t *testing.T) {
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           10,
		EndSec:             20,
		Summary:            "标题",
		BoundaryConfidence: "high",
	}, nil, 100, func(context.Context, int, int) (string, error) {
		return "", nil
	}, nil)
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if result.StartSec != 10 || result.EndSec != 20 {
		t.Fatalf("window = (%d, %d), want (10, 20)", result.StartSec, result.EndSec)
	}
	if result.Summary != "标题" {
		t.Fatalf("summary = %q, want %q", result.Summary, "标题")
	}
	if result.Input != "标题" {
		t.Fatalf("input = %q, want %q", result.Input, "标题")
	}
}

func TestBuildRefineJobsUsesFullSegmentTimelineForNextStart(t *testing.T) {
	all := []model.EduVideoSegment{
		{ID: 1, SegmentIndex: 0, StartTimeSec: 0, EndTimeSec: 60, ContentSummary: "一"},
		{ID: 2, SegmentIndex: 1, StartTimeSec: 60, EndTimeSec: 120, ContentSummary: "二"},
		{ID: 3, SegmentIndex: 2, StartTimeSec: 120, EndTimeSec: 180, ContentSummary: "三"},
		{ID: 4, SegmentIndex: 3, StartTimeSec: 180, EndTimeSec: 240, ContentSummary: "四"},
	}
	pending := []model.EduVideoSegment{
		all[1],
		all[3],
	}

	jobs := buildRefineJobs(pending, all, map[int]LLMSegment{1: {BoundaryConfidence: "low"}})

	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(jobs))
	}
	if jobs[0].SegmentIndex != 1 || jobs[0].NextStartSec != 120 {
		t.Fatalf("first job = %+v, want segment 1 next start 120", jobs[0])
	}
	if jobs[1].SegmentIndex != 3 || jobs[1].NextStartSec != 0 {
		t.Fatalf("second job = %+v, want segment 3 next start 0", jobs[1])
	}
	if jobs[0].BoundaryConfidence != "low" {
		t.Fatalf("boundary confidence = %q, want low", jobs[0].BoundaryConfidence)
	}
}

func TestBuildRefineSegmentInputUsesRefineASRWhenSummaryContentMismatchIsDetected(t *testing.T) {
	var calls int
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           40,
		EndSec:             60,
		NextStartSec:       80,
		Summary:            "完整讲解定义和适用条件",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 40, EndSec: 60, Text: "先看定义"}}, 120, func(context.Context, int, int) (string, error) {
		calls++
		return "完整讲解定义和适用条件。", nil
	}, nil)
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("ASR calls = %d, want 1", calls)
	}
	if result.Summary != "完整讲解定义和适用条件" {
		t.Fatalf("summary = %q, want %q", result.Summary, "完整讲解定义和适用条件")
	}
	if result.Input != "完整讲解定义和适用条件\n完整讲解定义和适用条件。" {
		t.Fatalf("input = %q", result.Input)
	}
}

func TestBuildRefineSegmentInputSkipsRefineForHealthyHighConfidenceSegment(t *testing.T) {
	var calls int
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           0,
		EndSec:             45,
		NextStartSec:       0,
		Summary:            "定义讲解",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 0, EndSec: 45, Text: "下面先看定义，这就是它的定义。"}}, 120, func(context.Context, int, int) (string, error) {
		calls++
		return "", nil
	}, nil)
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("ASR calls = %d, want 0", calls)
	}
	if result.Summary != "定义讲解" {
		t.Fatalf("summary = %q, want %q", result.Summary, "定义讲解")
	}
	if result.Input != "定义讲解\n下面先看定义，这就是它的定义。" {
		t.Fatalf("input = %q", result.Input)
	}
}

func TestBuildRefineSegmentInputRewritesSummaryFromStableCoarseText(t *testing.T) {
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           0,
		EndSec:             45,
		NextStartSec:       0,
		Summary:            "完整讲解定义与全部适用条件",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 0, EndSec: 45, Text: "下面先看定义，这就是它的定义。"}}, 120, func(context.Context, int, int) (string, error) {
		return "", nil
	}, nil)
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if result.Summary != "下面先看定义" {
		t.Fatalf("summary = %q, want %q", result.Summary, "下面先看定义")
	}
	if result.Input != "下面先看定义\n下面先看定义，这就是它的定义。" {
		t.Fatalf("input = %q", result.Input)
	}
}

func TestBuildRefineSegmentInputRewritesUnsupportedLateSummaryFromCoarseText(t *testing.T) {
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           720,
		EndSec:             790,
		NextStartSec:       0,
		Summary:            "圆锥曲线离心率范围",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 720, EndSec: 790, Text: "我们现在看函数单调性，先求导数，再判断导函数的正负。"}}, 900, func(context.Context, int, int) (string, error) {
		return "", nil
	}, nil)
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if result.Summary != "我们现在看函数单调性" {
		t.Fatalf("summary = %q, want %q", result.Summary, "我们现在看函数单调性")
	}
	if result.Input != "我们现在看函数单调性\n我们现在看函数单调性，先求导数，再判断导函数的正负。" {
		t.Fatalf("input = %q", result.Input)
	}
}

func TestBuildRefineSegmentInputUsesLLMRewriteForComplexStableText(t *testing.T) {
	var llmCalls int
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           0,
		EndSec:             90,
		NextStartSec:       0,
		Summary:            "完整讲解定义与适用条件",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 0, EndSec: 90, Text: "先定义概念。然后解释适用条件。最后补充题目中的使用方式。"}}, 120, func(context.Context, int, int) (string, error) {
		return "", nil
	}, func(context.Context, string) (string, error) {
		llmCalls++
		return "定义及适用条件", nil
	})
	if err != nil {
		t.Fatalf("buildRefineSegmentInputWithSummaryRewrite error = %v", err)
	}
	if llmCalls != 1 {
		t.Fatalf("LLM calls = %d, want 1", llmCalls)
	}
	if result.Summary != "定义及适用条件" {
		t.Fatalf("summary = %q, want %q", result.Summary, "定义及适用条件")
	}
	if result.Input != "定义及适用条件\n先定义概念。然后解释适用条件。最后补充题目中的使用方式。" {
		t.Fatalf("input = %q", result.Input)
	}
}

func TestBuildRefineSegmentInputSkipsLLMRewriteForSimpleStableText(t *testing.T) {
	var llmCalls int
	result, err := buildRefineSegmentInputWithSummaryRewrite(context.Background(), refineInputJob{
		StartSec:           0,
		EndSec:             45,
		NextStartSec:       0,
		Summary:            "完整讲解定义与全部适用条件",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 0, EndSec: 45, Text: "下面先看定义，这就是它的定义。"}}, 120, func(context.Context, int, int) (string, error) {
		return "", nil
	}, func(context.Context, string) (string, error) {
		llmCalls++
		return "定义", nil
	})
	if err != nil {
		t.Fatalf("buildRefineSegmentInputWithSummaryRewrite error = %v", err)
	}
	if llmCalls != 0 {
		t.Fatalf("LLM calls = %d, want 0", llmCalls)
	}
	if result.Summary != "下面先看定义" {
		t.Fatalf("summary = %q, want %q", result.Summary, "下面先看定义")
	}
	if result.Input != "下面先看定义\n下面先看定义，这就是它的定义。" {
		t.Fatalf("input = %q", result.Input)
	}
}

func TestBuildSegmentEmbeddingUpdateValuesIncludesContentSummary(t *testing.T) {
	values := buildSegmentEmbeddingUpdateValues(10, 45, "  下面先看定义  ", pgvector.NewVector([]float32{0.5}))

	if values["start_time"] != 10 {
		t.Fatalf("start_time = %v, want 10", values["start_time"])
	}
	if values["end_time"] != 45 {
		t.Fatalf("end_time = %v, want 45", values["end_time"])
	}
	if values["content_summary"] != "下面先看定义" {
		t.Fatalf("content_summary = %v, want 下面先看定义", values["content_summary"])
	}
	if values["status"] != int16(1) {
		t.Fatalf("status = %v, want %v", values["status"], int16(1))
	}
	if _, ok := values["embedding"]; !ok {
		t.Fatal("expected embedding key")
	}
}
