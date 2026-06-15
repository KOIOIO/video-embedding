package tasks

import "testing"

func TestBuildTranscriptFromCoarseItemsConcatenatesOverlapsInOrder(t *testing.T) {
	items := []CoarseItem{
		{Index: 0, StartSec: 0, EndSec: 60, Text: "第一段。"},
		{Index: 1, StartSec: 60, EndSec: 120, Text: "第二段。"},
		{Index: 2, StartSec: 120, EndSec: 180, Text: "第三段。"},
	}

	got := buildTranscriptFromCoarseItems(items, 30, 130)
	want := "第一段。\n第二段。\n第三段。"
	if got != want {
		t.Fatalf("buildTranscriptFromCoarseItems() = %q, want %q", got, want)
	}
}

func TestShouldUseRefineASRFallbackOnlyForLowConfidence(t *testing.T) {
	if !shouldUseRefineASRFallback("low") {
		t.Fatal("expected low confidence to trigger fallback")
	}
	for _, confidence := range []string{"", "medium", "high"} {
		if shouldUseRefineASRFallback(confidence) {
			t.Fatalf("confidence %q should not trigger fallback", confidence)
		}
	}
}

func TestBuildExpandedFallbackWindowClampsToBoundsAndNextSegment(t *testing.T) {
	start, end := buildExpandedFallbackWindow(2, 20, 22, 100, 4)
	if start != 0 || end != 22 {
		t.Fatalf("buildExpandedFallbackWindow() = (%d, %d), want (0, 22)", start, end)
	}
}
