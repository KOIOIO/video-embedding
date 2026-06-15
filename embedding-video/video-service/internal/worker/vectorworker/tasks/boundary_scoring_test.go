package tasks

import "testing"

func TestEvaluateContinuation_PrefersMergeForLowConfidenceContinuation(t *testing.T) {
	prev := LLMSegment{
		ContentSummary:     "定义函数，并解释它的适用条件",
		BoundaryReason:     "先给出定义",
		EndAnchorText:      "它的适用条件",
		BoundaryConfidence: "low",
	}
	curr := LLMSegment{
		ContentSummary:     "然后继续说明这个定义在题目中的用法",
		BoundaryReason:     "继续解释",
		StartAnchorText:    "然后继续",
		BoundaryConfidence: "low",
	}

	score := EvaluateContinuation(prev, curr)
	if score.Score <= 0 {
		t.Fatalf("expected positive continuation score, got %v", score.Score)
	}
	if len(score.Reasons) == 0 {
		t.Fatal("expected continuation reasons")
	}

	decision := EvaluateSegmentBoundary(prev, curr)
	if decision.Action != "merge" {
		t.Fatalf("decision.Action = %q, want merge", decision.Action)
	}
}

func TestEvaluateContinuation_PrefersKeepForNewTopic(t *testing.T) {
	prev := LLMSegment{
		ContentSummary:     "先总结这个定义的基本形式",
		BoundaryReason:     "定义部分结束",
		EndAnchorText:      "这就是定义",
		BoundaryConfidence: "high",
	}
	curr := LLMSegment{
		ContentSummary:     "接下来进入新的例题，开始分析题意",
		BoundaryReason:     "开始新的例题阶段",
		StartAnchorText:    "下面看例题",
		BoundaryConfidence: "high",
	}

	decision := EvaluateSegmentBoundary(prev, curr)
	if decision.Action != "keep" {
		t.Fatalf("decision.Action = %q, want keep", decision.Action)
	}
	if decision.Score >= 0 {
		t.Fatalf("decision.Score = %v, want negative score for separation", decision.Score)
	}
}

func TestEvaluateStartBoundary_PenalizesFragmentLikeStart(t *testing.T) {
	seg := LLMSegment{StartAnchorText: "然后", ContentSummary: "然后继续说明上一段的结论"}
	score := EvaluateStartBoundary(seg)
	if score.Score >= 0 {
		t.Fatalf("start score = %v, want negative for fragment-like start", score.Score)
	}
}

func TestEvaluateEndBoundary_RecognizesSentenceClosure(t *testing.T) {
	seg := LLMSegment{EndAnchorText: "这就是它的定义。", ContentSummary: "定义讲解完成。"}
	score := EvaluateEndBoundary(seg)
	if score.Score <= 0 {
		t.Fatalf("end score = %v, want positive for closed ending", score.Score)
	}
}

func TestEvaluateSegmentBoundary_UsesRecutForAmbiguousBoundary(t *testing.T) {
	prev := LLMSegment{
		ContentSummary:     "先介绍这个公式的基本形式",
		BoundaryReason:     "概念引入",
		EndAnchorText:      "公式的基本形式",
		BoundaryConfidence: "medium",
	}
	curr := LLMSegment{
		ContentSummary:     "这里继续推一下，但也开始进入下一步推导",
		BoundaryReason:     "这里可能有阶段切换，但边界不够稳定",
		StartAnchorText:    "这里继续推",
		BoundaryConfidence: "low",
	}

	decision := EvaluateSegmentBoundary(prev, curr)
	t.Logf("decision score=%v reasons=%v", decision.Score, decision.Reasons)
	if decision.Action != "recut" {
		t.Fatalf("decision.Action = %q, want recut", decision.Action)
	}
	if len(decision.Reasons) == 0 {
		t.Fatal("expected recut reasons")
	}
}
