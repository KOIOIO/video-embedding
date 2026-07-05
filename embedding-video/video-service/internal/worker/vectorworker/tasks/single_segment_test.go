package tasks

import (
	"strings"
	"testing"
)

func TestNormalizeSingleSegmentSummaryPreservesKnowledgeTags(t *testing.T) {
	got, err := NormalizeSingleSegmentSummary(`{
		"content_summary": "一次函数定义",
		"knowledge_tags": ["一次函数", "定义"]
	}`, 194)
	if err != nil {
		t.Fatalf("NormalizeSingleSegmentSummary returned error: %v", err)
	}
	if got.SegmentIndex != 0 {
		t.Fatalf("SegmentIndex = %d, want 0", got.SegmentIndex)
	}
	if got.StartTimeSec != 0 || got.EndTimeSec != 194 {
		t.Fatalf("range = %d-%d, want 0-194", got.StartTimeSec, got.EndTimeSec)
	}
	if got.ContentSummary != "一次函数定义" {
		t.Fatalf("ContentSummary = %q, want 一次函数定义", got.ContentSummary)
	}
	if len(got.KnowledgeTags) != 2 || got.KnowledgeTags[0] != "一次函数" || got.KnowledgeTags[1] != "定义" {
		t.Fatalf("KnowledgeTags = %v, want [一次函数 定义]", got.KnowledgeTags)
	}
}

func TestBuildSingleSegmentSummaryPromptRequestsSummaryAndTags(t *testing.T) {
	got, err := BuildSingleSegmentSummaryPrompt(194, "这节课讲一次函数定义和图像。")
	if err != nil {
		t.Fatalf("BuildSingleSegmentSummaryPrompt returned error: %v", err)
	}
	for _, want := range []string{"content_summary", "knowledge_tags", "194", "这节课讲一次函数定义和图像"} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt = %q, want to contain %q", got, want)
		}
	}
}
