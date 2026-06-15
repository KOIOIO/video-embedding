package tasks

import (
	"strings"
	"testing"
)

func TestNormalizeLLMSegmentsKeepsOptionalBoundaryFields(t *testing.T) {
	llmOut := `{
	  "segments": [
	    {
	      "segment_index": 0,
	      "start_time": 0,
	      "end_time": 42,
	      "content_summary": "定义概念",
	      "knowledge_tags": ["定义"],
	      "boundary_reason": "从题目背景进入定义讲解",
	      "start_anchor_text": "下面先看定义",
	      "end_anchor_text": "这就是它的定义",
	      "boundary_confidence": "high",
	      "ignored_field": "ignored"
	    }
	  ]
	}`

	segs, err := NormalizeLLMSegments(llmOut, 120, 20, 180)
	if err != nil {
		t.Fatalf("NormalizeLLMSegments error = %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("len(segs) = %d, want 1", len(segs))
	}
	if segs[0].BoundaryReason != "从题目背景进入定义讲解" {
		t.Fatalf("BoundaryReason = %q", segs[0].BoundaryReason)
	}
	if segs[0].StartAnchorText != "下面先看定义" {
		t.Fatalf("StartAnchorText = %q", segs[0].StartAnchorText)
	}
	if segs[0].EndAnchorText != "这就是它的定义" {
		t.Fatalf("EndAnchorText = %q", segs[0].EndAnchorText)
	}
	if segs[0].BoundaryConfidence != "high" {
		t.Fatalf("BoundaryConfidence = %q", segs[0].BoundaryConfidence)
	}
}

func TestNormalizeLLMSegmentsAllowsMissingOptionalBoundaryFields(t *testing.T) {
	llmOut := `{
	  "segments": [
	    {
	      "segment_index": 0,
	      "start_time": 0,
	      "end_time": 42,
	      "content_summary": "定义概念",
	      "knowledge_tags": ["定义"]
	    }
	  ]
	}`

	segs, err := NormalizeLLMSegments(llmOut, 120, 20, 180)
	if err != nil {
		t.Fatalf("NormalizeLLMSegments error = %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("len(segs) = %d, want 1", len(segs))
	}
	if segs[0].BoundaryReason != "" || segs[0].StartAnchorText != "" || segs[0].EndAnchorText != "" || segs[0].BoundaryConfidence != "" {
		t.Fatalf("optional fields should stay empty: %+v", segs[0])
	}
}

func TestBuildHierarchicalSegmentationPromptMentionsBoundaryFields(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "下面先看定义，这就是它的定义。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationPrompt error = %v", err)
	}
	for _, needle := range []string{"boundary_reason", "start_anchor_text", "end_anchor_text", "boundary_confidence"} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q", needle)
		}
	}
}

func TestNormalizeLLMSegmentsMergesLowConfidenceContinuation(t *testing.T) {
	llmOut := `{
	  "segments": [
	    {
	      "segment_index": 0,
	      "start_time": 0,
	      "end_time": 40,
	      "content_summary": "先定义概念，再解释它的适用条件",
	      "knowledge_tags": ["定义"],
	      "boundary_reason": "先给出定义",
	      "start_anchor_text": "下面先看定义",
	      "end_anchor_text": "它的适用条件",
	      "boundary_confidence": "low"
	    },
	    {
	      "segment_index": 1,
	      "start_time": 40,
	      "end_time": 70,
	      "content_summary": "然后继续说明这个定义在题目中的用法",
	      "knowledge_tags": ["定义应用"],
	      "boundary_reason": "继续解释",
	      "start_anchor_text": "然后继续",
	      "end_anchor_text": "在题目中的用法",
	      "boundary_confidence": "low"
	    }
	  ]
	}`

	segs, err := NormalizeLLMSegments(llmOut, 120, 20, 180)
	if err != nil {
		t.Fatalf("NormalizeLLMSegments error = %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("len(segs) = %d, want 1", len(segs))
	}
	if segs[0].StartTimeSec != 0 || segs[0].EndTimeSec != 70 {
		t.Fatalf("merged segment = %d-%d, want 0-70", segs[0].StartTimeSec, segs[0].EndTimeSec)
	}
	if !strings.Contains(segs[0].ContentSummary, "继续说明") {
		t.Fatalf("merged content summary missing continuation: %q", segs[0].ContentSummary)
	}
	if segs[0].SegmentIndex != 0 {
		t.Fatalf("SegmentIndex = %d, want 0", segs[0].SegmentIndex)
	}
	if len(segs[0].KnowledgeTags) != 2 {
		t.Fatalf("KnowledgeTags len = %d, want 2", len(segs[0].KnowledgeTags))
	}
}

func TestNormalizeLLMSegmentsRecutsAmbiguousBoundary(t *testing.T) {
	llmOut := `{
	  "segments": [
	    {
	      "segment_index": 0,
	      "start_time": 0,
	      "end_time": 40,
	      "content_summary": "先介绍这个公式的基本形式",
	      "knowledge_tags": ["公式"],
	      "boundary_reason": "概念引入",
	      "start_anchor_text": "我们先看这个公式",
	      "end_anchor_text": "公式的基本形式",
	      "boundary_confidence": "medium"
	    },
	    {
	      "segment_index": 1,
	      "start_time": 35,
	      "end_time": 70,
	      "content_summary": "这里继续推一下，但也开始进入下一步推导",
	      "knowledge_tags": ["推导"],
	      "boundary_reason": "这里可能有阶段切换，但边界不够稳定",
	      "start_anchor_text": "这里继续推",
	      "end_anchor_text": "开始进入下一步",
	      "boundary_confidence": "low"
	    }
	  ]
	}`

	segs, err := NormalizeLLMSegments(llmOut, 120, 20, 180)
	if err != nil {
		t.Fatalf("NormalizeLLMSegments error = %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("len(segs) = %d, want 2", len(segs))
	}
	if segs[0].EndTimeSec != segs[1].StartTimeSec {
		t.Fatalf("expected recut boundary to align, got prev_end=%d curr_start=%d", segs[0].EndTimeSec, segs[1].StartTimeSec)
	}
	if segs[0].EndTimeSec >= 40 || segs[0].EndTimeSec <= 35 {
		t.Fatalf("expected recut boundary to move inside overlap range, got %d", segs[0].EndTimeSec)
	}
}

func TestBuildHierarchicalSegmentationPromptMentionsKnowledgeUnitRules(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "下面先看定义，这就是它的定义。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationPrompt error = %v", err)
	}
	for _, needle := range []string{"完整的知识单元", "不要把“定义”和它紧随其后的关键解释强行拆开", "先保证知识单元完整"} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q", needle)
		}
	}
}

func TestBuildHierarchicalSegmentationPromptRequiresTitleStyleContentSummary(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "下面先看定义，这就是它的定义。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationPrompt error = %v", err)
	}
	for _, needle := range []string{"短标题", "不要写成长段内容简介", "content_summary 字段存的是该视频段标题"} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q", needle)
		}
	}
}

func TestBuildHierarchicalSegmentationPromptRequiresSummaryToBeFullySupported(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "先讲定义，再解释它的适用条件。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationPrompt error = %v", err)
	}
	for _, needle := range []string{
		"content_summary 必须能被该分段内的实际讲解内容完整支撑",
		"如果某个知识点在本段没有讲完，不要把它总结成已经完整讲完",
		"优先保证知识单元完整，不要为了时长均匀硬切",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q", needle)
		}
	}
}

func TestBuildHierarchicalSegmentationRetryPromptMentionsBoundaryFields(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationRetryPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "下面先看定义，这就是它的定义。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationRetryPrompt error = %v", err)
	}
	for _, needle := range []string{"boundary_reason", "start_anchor_text", "end_anchor_text", "boundary_confidence"} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("retry prompt missing %q", needle)
		}
	}
}

func TestBuildHierarchicalSegmentationRetryPromptRequiresTitleStyleContentSummary(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationRetryPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "下面先看定义，这就是它的定义。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationRetryPrompt error = %v", err)
	}
	for _, needle := range []string{"短标题", "不要写成长段内容简介", "content_summary 字段存的是该视频段标题"} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("retry prompt missing %q", needle)
		}
	}
}

func TestBuildHierarchicalSegmentationRetryPromptRequiresSummaryToBeFullySupported(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationRetryPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "先讲定义，再解释它的适用条件。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationRetryPrompt error = %v", err)
	}
	for _, needle := range []string{
		"content_summary 必须能被该分段内的实际讲解内容完整支撑",
		"如果某个知识点在本段没有讲完，不要把它总结成已经完整讲完",
		"优先保证知识单元完整，不要为了时长均匀硬切",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("retry prompt missing %q", needle)
		}
	}
}

func TestBuildHierarchicalSegmentationRetryPromptMentionsContentBoundaryChecks(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationRetryPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "下面先看定义，这就是它的定义。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationRetryPrompt error = %v", err)
	}
	for _, needle := range []string{"同一个知识点被拆成多个过碎小段", "不合格分段", "先保证知识单元完整"} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("retry prompt missing %q", needle)
		}
	}
}

func TestNormalizeSegmentTitleCollapsesMultilineSummary(t *testing.T) {
	got := normalizeSegmentTitle("  定义概念\n这是对定义的展开解释\n继续补充  ")
	if got != "定义概念 这是对定义的展开解释 继续补充" {
		t.Fatalf("normalizeSegmentTitle() = %q", got)
	}
}

func TestNormalizeSegmentTitleExtractsTitleFromJSONObject(t *testing.T) {
	got := normalizeSegmentTitle(`{"title":"正确认识自己"}`)
	if got != "正确认识自己" {
		t.Fatalf("normalizeSegmentTitle() = %q", got)
	}
}

func TestNormalizeSegmentTitleExtractsJSONString(t *testing.T) {
	got := normalizeSegmentTitle(`"正确认识自己"`)
	if got != "正确认识自己" {
		t.Fatalf("normalizeSegmentTitle() = %q", got)
	}
}

func TestBuildSummaryRewritePromptRestrictsOutputToSupportedTitle(t *testing.T) {
	prompt := BuildSummaryRewritePrompt("下面先看定义，这就是它的定义。")
	for _, needle := range []string{
		"只根据提供的正文生成标题",
		"不要引入正文里没有出现的概念",
		"输出短标题",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("summary rewrite prompt missing %q", needle)
		}
	}
}
