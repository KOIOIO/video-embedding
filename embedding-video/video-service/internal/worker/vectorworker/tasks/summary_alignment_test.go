package tasks

import "testing"

func TestDetectSummaryContentMismatchFlagsShortSegmentWithBroadSummary(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   0,
		EndTimeSec:     18,
		ContentSummary: "完整讲解二次函数顶点式的定义和适用条件",
	}
	text := "先看二次函数。"
	if !detectSummaryContentMismatch(seg, text, "") {
		t.Fatal("expected mismatch for short segment with oversized summary")
	}
}

func TestDetectSummaryContentMismatchFlagsHalfSentenceContinuation(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   30,
		EndTimeSec:     55,
		ContentSummary: "定义讲解完成",
	}
	text := "然后继续说明这个定义在题目里的用法"
	if !detectSummaryContentMismatch(seg, text, "接下来分析例题") {
		t.Fatal("expected mismatch for continuation-like incomplete text")
	}
}

func TestShouldMergeMismatchedSegmentWithNextWhenTopicContinues(t *testing.T) {
	current := LLMSegment{SegmentIndex: 0, StartTimeSec: 0, EndTimeSec: 25, ContentSummary: "定义说明"}
	next := LLMSegment{SegmentIndex: 1, StartTimeSec: 25, EndTimeSec: 50, ContentSummary: "继续说明定义在题目中的用法"}
	if !shouldMergeMismatchedSegment(current, next, "然后继续说明定义的用法") {
		t.Fatal("expected merge decision for continued topic")
	}
}

func TestRepairMismatchedSegmentsMergesContinuationWithNext(t *testing.T) {
	segs := []LLMSegment{
		{SegmentIndex: 0, StartTimeSec: 0, EndTimeSec: 20, ContentSummary: "定义说明"},
		{SegmentIndex: 1, StartTimeSec: 20, EndTimeSec: 50, ContentSummary: "继续说明定义的适用条件"},
	}
	coarse := []CoarseItem{{StartSec: 0, EndSec: 50, Text: "下面先看定义，然后继续说明定义的适用条件。"}}
	repaired := repairMismatchedSegments(segs, coarse)
	if len(repaired) != 1 {
		t.Fatalf("len(repaired) = %d, want 1", len(repaired))
	}
	if repaired[0].StartTimeSec != 0 || repaired[0].EndTimeSec != 50 {
		t.Fatalf("merged segment = [%d, %d]", repaired[0].StartTimeSec, repaired[0].EndTimeSec)
	}
}

func TestSummaryNeedsRewriteWhenTextIsStableButSummaryOverreaches(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   0,
		EndTimeSec:     45,
		ContentSummary: "完整讲解定义与全部适用条件",
	}
	text := "下面先看定义，这就是它的定义。"
	if !summaryNeedsRewrite(seg, text) {
		t.Fatal("expected stable text with oversized summary to require rewrite")
	}
}

func TestSummaryNeedsRewriteWhenSummaryIsUnsupportedByText(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   8,
		StartTimeSec:   720,
		EndTimeSec:     790,
		ContentSummary: "圆锥曲线离心率范围",
	}
	text := "我们现在看函数单调性，先求导数，再判断导函数的正负。"
	if !summaryNeedsRewrite(seg, text) {
		t.Fatal("expected unsupported summary to require rewrite")
	}
}

func TestSummaryNeedsRewriteWhenSummaryHasOnlyWeakTextSupport(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   9,
		StartTimeSec:   790,
		EndTimeSec:     860,
		ContentSummary: "函数零点存在性",
	}
	text := "我们现在看函数单调性，先求导数，再判断导函数的正负。"
	if !summaryNeedsRewrite(seg, text) {
		t.Fatal("expected weakly supported summary to require rewrite")
	}
}

func TestRewriteSummaryFromTextBuildsShortTitleFromStableText(t *testing.T) {
	got := rewriteSummaryFromText("下面先看定义，这就是它的定义。")
	if got != "下面先看定义" {
		t.Fatalf("rewriteSummaryFromText() = %q, want %q", got, "下面先看定义")
	}
}

func TestShouldUseLLMSummaryRewriteSkipsLLMForSimpleStableText(t *testing.T) {
	text := "下面先看定义，这就是它的定义。"
	if shouldUseLLMSummaryRewrite(text) {
		t.Fatal("did not expect simple stable text to require LLM rewrite")
	}
}

func TestAlignSummaryToStableTextRewritesSummaryWhenNeeded(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   0,
		EndTimeSec:     45,
		ContentSummary: "完整讲解定义与全部适用条件",
	}
	rewritten, changed := alignSummaryToStableText(seg, "下面先看定义，这就是它的定义。")
	if !changed {
		t.Fatal("expected summary rewrite")
	}
	if rewritten.ContentSummary != "下面先看定义" {
		t.Fatalf("rewritten summary = %q", rewritten.ContentSummary)
	}
}

func TestAlignSummaryToStableTextKeepsAlignedSummary(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   0,
		EndTimeSec:     45,
		ContentSummary: "下面先看定义",
	}
	rewritten, changed := alignSummaryToStableText(seg, "下面先看定义，这就是它的定义。")
	if changed {
		t.Fatal("did not expect summary rewrite")
	}
	if rewritten.ContentSummary != "下面先看定义" {
		t.Fatalf("summary = %q", rewritten.ContentSummary)
	}
}

func TestShouldUseLLMSummaryRewriteUsesLLMForComplexStableText(t *testing.T) {
	text := "先定义概念。然后解释适用条件。最后补充题目中的使用方式。"
	if !shouldUseLLMSummaryRewrite(text) {
		t.Fatal("expected complex stable text to require LLM summary rewrite")
	}
}
