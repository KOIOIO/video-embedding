package tasks

import (
	"context"
	"fmt"
	"testing"
)

func TestAlignSegmentTailStopsAtNaturalSentenceEnd(t *testing.T) {
	cfg := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       true,
		MaxExtendSec:  3,
		ProbeStepSec:  1,
		MaxOverlapSec: 6,
	})

	responses := map[int]string{
		10: "所以这里我们可以得到",
		11: "所以这里我们可以得到这个结论",
		12: "所以这里我们可以得到这个结论。",
	}

	called := make([]int, 0, 3)
	probe := func(endSec int) (string, error) {
		called = append(called, endSec)
		text, ok := responses[endSec]
		if !ok {
			return "", fmt.Errorf("unexpected endSec=%d", endSec)
		}
		return text, nil
	}

	gotEnd, gotText, err := alignSegmentTail(context.Background(), cfg, 0, 10, 20, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 12 {
		t.Fatalf("gotEnd = %d, want 12", gotEnd)
	}
	if gotText != responses[12] {
		t.Fatalf("gotText = %q, want %q", gotText, responses[12])
	}
	if len(called) != 3 {
		t.Fatalf("probe calls = %v, want [10 11 12]", called)
	}
}

func TestAlignSegmentTailSkipsWhenDisabled(t *testing.T) {
	probeCount := 0
	probe := func(endSec int) (string, error) {
		probeCount++
		return "接下来我们来看", nil
	}

	gotEnd, _, err := alignSegmentTail(context.Background(), TailAlignmentConfig{Enabled: false, MaxExtendSec: 3, ProbeStepSec: 1, MaxOverlapSec: 6}, 0, 10, 20, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 10 {
		t.Fatalf("gotEnd = %d, want 10", gotEnd)
	}
	if probeCount != 1 {
		t.Fatalf("probeCount = %d, want 1", probeCount)
	}
}

func TestAlignSegmentTailHonorsOverlapLimit(t *testing.T) {
	cfg := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       true,
		MaxExtendSec:  3,
		ProbeStepSec:  1,
		MaxOverlapSec: 1,
	})

	probe := func(endSec int) (string, error) {
		return "接下来我们来看", nil
	}

	gotEnd, _, err := alignSegmentTail(context.Background(), cfg, 0, 10, 10, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 11 {
		t.Fatalf("gotEnd = %d, want 11", gotEnd)
	}
}

func TestAlignSegmentTailStopsAtMaxExtendWhenNoSentenceEnd(t *testing.T) {
	cfg := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       true,
		MaxExtendSec:  2,
		ProbeStepSec:  1,
		MaxOverlapSec: 6,
	})

	called := make([]int, 0, 3)
	probe := func(endSec int) (string, error) {
		called = append(called, endSec)
		return "接下来我们来看", nil
	}

	gotEnd, _, err := alignSegmentTail(context.Background(), cfg, 0, 10, 30, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 12 {
		t.Fatalf("gotEnd = %d, want 12", gotEnd)
	}
	if len(called) != 3 {
		t.Fatalf("probe calls = %v, want [10 11 12]", called)
	}
}

func TestAlignSegmentTailKeepsCompleteSentenceUnchanged(t *testing.T) {
	probeCount := 0
	probe := func(endSec int) (string, error) {
		probeCount++
		return "这一题我们就讲到这里。", nil
	}

	gotEnd, gotText, err := alignSegmentTail(context.Background(), NormalizeTailAlignmentConfig(TailAlignmentConfig{Enabled: true}), 0, 10, 30, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 10 {
		t.Fatalf("gotEnd = %d, want 10", gotEnd)
	}
	if gotText != "这一题我们就讲到这里。" {
		t.Fatalf("gotText = %q", gotText)
	}
	if probeCount != 1 {
		t.Fatalf("probeCount = %d, want 1", probeCount)
	}
}
