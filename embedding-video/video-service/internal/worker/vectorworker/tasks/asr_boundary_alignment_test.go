package tasks

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestNormalizeRefineASRWorkersDefaultsToFour(t *testing.T) {
	if got := normalizeRefineASRWorkers(0); got != 4 {
		t.Fatalf("normalizeRefineASRWorkers(0) = %d, want 4", got)
	}
}

func TestNormalizeRefineASRWorkersCapsAtTwenty(t *testing.T) {
	if got := normalizeRefineASRWorkers(60); got != 20 {
		t.Fatalf("normalizeRefineASRWorkers(60) = %d, want 20", got)
	}
}

func TestAlignSegmentForRefinePrefersBoundaryAlignmentResult(t *testing.T) {
	seg := LLMSegment{SegmentIndex: 0, StartTimeSec: 30, EndTimeSec: 70, StartAnchorText: "下面先看定义", EndAnchorText: "这就是它的定义", BoundaryConfidence: "high"}

	start, end, text, usedBoundaryAlignment, usedTailFallback, err := alignSegmentForRefine(context.Background(), seg, nil, 120,
		func(startSec int, endSec int) (string, error) {
			if endSec == 74 {
				return "这就是它的定义。", nil
			}
			if startSec == 30 {
				return "下面先看定义", nil
			}
			return "", nil
		},
		func(context.Context, TailAlignmentConfig, int, int, int, int, func(int) (string, error)) (int, string, error) {
			return 70, "fallback", nil
		},
	)
	if err != nil {
		t.Fatalf("alignSegmentForRefine error = %v", err)
	}
	if start != 30 || end != 74 {
		t.Fatalf("got start=%d end=%d", start, end)
	}
	if text == "fallback" {
		t.Fatal("should prefer boundary alignment result")
	}
	if !usedBoundaryAlignment {
		t.Fatal("expected boundary alignment to be used")
	}
	if usedTailFallback {
		t.Fatal("did not expect tail fallback")
	}
}

func TestAlignSegmentForRefineFallsBackToTailAlignment(t *testing.T) {
	seg := LLMSegment{SegmentIndex: 0, StartTimeSec: 30, EndTimeSec: 70}
	_, end, text, usedBoundaryAlignment, usedTailFallback, err := alignSegmentForRefine(context.Background(), seg, nil, 120,
		func(int, int) (string, error) {
			return "", errors.New("probe failed")
		},
		func(context.Context, TailAlignmentConfig, int, int, int, int, func(int) (string, error)) (int, string, error) {
			return 72, "fallback text。", nil
		},
	)
	if err != nil {
		t.Fatalf("alignSegmentForRefine error = %v", err)
	}
	if end != 72 || text != "fallback text。" {
		t.Fatalf("fallback result = %d %q", end, text)
	}
	if usedBoundaryAlignment {
		t.Fatal("did not expect boundary alignment to succeed")
	}
	if !usedTailFallback {
		t.Fatal("expected tail fallback to be used")
	}
}

func TestAlignSegmentForRefineUsesShortBoundaryProbes(t *testing.T) {
	seg := LLMSegment{SegmentIndex: 0, StartTimeSec: 30, EndTimeSec: 70, StartAnchorText: "下面先看定义", EndAnchorText: "这就是它的定义", BoundaryConfidence: "high"}
	var calls []string

	start, end, text, usedBoundaryAlignment, usedTailFallback, err := alignSegmentForRefine(context.Background(), seg, nil, 120,
		func(startSec int, endSec int) (string, error) {
			calls = append(calls, fmt.Sprintf("%d-%d", startSec, endSec))
			if endSec <= startSec {
				return "", fmt.Errorf("invalid probe range %d-%d", startSec, endSec)
			}
			if startSec >= 27 && endSec <= 32 {
				if startSec == 30 {
					return "下面先看定义", nil
				}
				return "所以", nil
			}
			if startSec >= 68 && endSec <= 74 {
				if endSec == 74 {
					return "这就是它的定义。", nil
				}
				return "这就是它的定义", nil
			}
			if startSec == 30 && endSec == 74 {
				return "下面先看定义\n这就是它的定义。", nil
			}
			return "", fmt.Errorf("unexpected long probe %d-%d", startSec, endSec)
		},
		func(context.Context, TailAlignmentConfig, int, int, int, int, func(int) (string, error)) (int, string, error) {
			return 72, "fallback", nil
		},
	)
	if err != nil {
		t.Fatalf("alignSegmentForRefine error = %v, calls=%v", err, calls)
	}
	if start != 30 || end != 74 {
		t.Fatalf("got start=%d end=%d", start, end)
	}
	if text == "fallback" {
		t.Fatal("should prefer boundary alignment result")
	}
	if !usedBoundaryAlignment || usedTailFallback {
		t.Fatalf("usedBoundaryAlignment=%v usedTailFallback=%v", usedBoundaryAlignment, usedTailFallback)
	}
	for _, call := range calls {
		if call == "27-27" || call == "28-28" || call == "29-29" || call == "30-30" || call == "31-31" || call == "32-32" {
			t.Fatalf("unexpected empty-range probe: %v", calls)
		}
	}
}
