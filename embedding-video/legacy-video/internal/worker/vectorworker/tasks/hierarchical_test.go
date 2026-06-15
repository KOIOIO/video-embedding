package tasks

import (
	"strings"
	"testing"
)

func TestNormalizeLLMSegments_WithOptionalBoundaryFields(t *testing.T) {
	llmOut := `{"segments":[{"segment_index":9,"start_time":5,"end_time":35,"content_summary":"  第一段  ","knowledge_tags":["函数", "导数"],"boundary_reason":"  从定义转入例题  ","start_anchor_text":"  我们先看定义  ","end_anchor_text":"  接下来做一道题  ","boundary_confidence":"  HIGH  "}]}`

	got, err := NormalizeLLMSegments(llmOut, 120, 20, 180)
	if err != nil {
		t.Fatalf("NormalizeLLMSegments() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(segments) = %d, want 1", len(got))
	}
	seg := got[0]
	if seg.BoundaryReason != "从定义转入例题" {
		t.Fatalf("BoundaryReason = %q, want %q", seg.BoundaryReason, "从定义转入例题")
	}
	if seg.StartAnchorText != "我们先看定义" {
		t.Fatalf("StartAnchorText = %q, want %q", seg.StartAnchorText, "我们先看定义")
	}
	if seg.EndAnchorText != "接下来做一道题" {
		t.Fatalf("EndAnchorText = %q, want %q", seg.EndAnchorText, "接下来做一道题")
	}
	if seg.BoundaryConfidence != "high" {
		t.Fatalf("BoundaryConfidence = %q, want %q", seg.BoundaryConfidence, "high")
	}
}

func TestNormalizeLLMSegments_WithoutOptionalBoundaryFields(t *testing.T) {
	llmOut := `{"segments":[{"segment_index":0,"start_time":0,"end_time":30,"content_summary":"第一段","knowledge_tags":["集合"]}]}`

	got, err := NormalizeLLMSegments(llmOut, 90, 20, 180)
	if err != nil {
		t.Fatalf("NormalizeLLMSegments() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(segments) = %d, want 1", len(got))
	}
	seg := got[0]
	if seg.BoundaryReason != "" {
		t.Fatalf("BoundaryReason = %q, want empty", seg.BoundaryReason)
	}
	if seg.StartAnchorText != "" {
		t.Fatalf("StartAnchorText = %q, want empty", seg.StartAnchorText)
	}
	if seg.EndAnchorText != "" {
		t.Fatalf("EndAnchorText = %q, want empty", seg.EndAnchorText)
	}
	if seg.BoundaryConfidence != "" {
		t.Fatalf("BoundaryConfidence = %q, want empty", seg.BoundaryConfidence)
	}
}

func TestBuildHierarchicalSegmentationPrompt_MentionsBoundaryFields(t *testing.T) {
	coarseItems := []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "定义部分"}}

	prompt, err := BuildHierarchicalSegmentationPrompt(120, 60, 20, 180, coarseItems)
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationPrompt() error = %v", err)
	}
	retryPrompt, err := BuildHierarchicalSegmentationRetryPrompt(120, 60, 20, 180, coarseItems)
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationRetryPrompt() error = %v", err)
	}

	for _, field := range []string{"boundary_reason", "start_anchor_text", "end_anchor_text", "boundary_confidence"} {
		if !strings.Contains(prompt, field) {
			t.Fatalf("prompt missing field %q", field)
		}
		if !strings.Contains(retryPrompt, field) {
			t.Fatalf("retry prompt missing field %q", field)
		}
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
