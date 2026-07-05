package profile

import (
	"math"
	"testing"
	"time"
)

func TestBuildUserVideoProfileWeightsEventsWithDecayAndNormalizes(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	result := BuildUserVideoProfile([]WeightedEvent{
		{
			SourceType:   SourceSegmentReaction,
			ReactionType: ReactionDoubleLike,
			Vector:       []float32{1, 0},
			EventTime:    now.AddDate(0, 0, -3),
		},
		{
			SourceType:   SourceSegmentReaction,
			ReactionType: ReactionLike,
			Vector:       []float32{0, 1},
			EventTime:    now.AddDate(0, 0, -20),
		},
	}, now)

	if !result.Valid {
		t.Fatal("expected profile to be valid")
	}
	if result.PositiveCount != 2 || result.NegativeCount != 0 || result.SourceEventCount != 2 {
		t.Fatalf("counts = %+v", result)
	}
	if !result.LastEventTime.Equal(now.AddDate(0, 0, -3)) {
		t.Fatalf("last event time = %v", result.LastEventTime)
	}

	// double_like gets weight 3.0 with no decay; like gets weight 2.0 * 0.7.
	wantX := 3.0 / math.Sqrt(3.0*3.0+1.4*1.4)
	wantY := 1.4 / math.Sqrt(3.0*3.0+1.4*1.4)
	assertFloat32Near(t, result.Vector[0], float32(wantX), 0.0001)
	assertFloat32Near(t, result.Vector[1], float32(wantY), 0.0001)
}

func TestBuildUserVideoProfileTracksNegativeButDoesNotEnableNegativeOnlyProfile(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	result := BuildUserVideoProfile([]WeightedEvent{
		{
			SourceType:   SourceSegmentReaction,
			ReactionType: ReactionDislike,
			Vector:       []float32{1, 0},
			EventTime:    now,
		},
	}, now)

	if result.Valid {
		t.Fatal("expected negative-only profile to be invalid")
	}
	if result.PositiveCount != 0 || result.NegativeCount != 1 || result.SourceEventCount != 1 {
		t.Fatalf("counts = %+v", result)
	}
}

func TestBuildUserVideoProfileUsesWatchRatioWeights(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	result := BuildUserVideoProfile([]WeightedEvent{
		{
			SourceType:      SourceWatch,
			Vector:          []float32{1, 0},
			WatchDuration:   80,
			SegmentDuration: 100,
			EventTime:       now,
		},
		{
			SourceType:      SourceWatch,
			Vector:          []float32{0, 1},
			WatchDuration:   20,
			SegmentDuration: 100,
			EventTime:       now,
		},
	}, now)

	if !result.Valid {
		t.Fatal("expected watch profile to be valid")
	}
	if result.WatchCount != 2 || result.PositiveCount != 2 {
		t.Fatalf("counts = %+v", result)
	}
	wantX := 1.2 / math.Sqrt(1.2*1.2+0.3*0.3)
	wantY := 0.3 / math.Sqrt(1.2*1.2+0.3*0.3)
	assertFloat32Near(t, result.Vector[0], float32(wantX), 0.0001)
	assertFloat32Near(t, result.Vector[1], float32(wantY), 0.0001)
}

func TestBuildUserVideoProfileSkipsEmptyVectorsAndZeroWeightEvents(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	result := BuildUserVideoProfile([]WeightedEvent{
		{
			SourceType:   SourceSegmentReaction,
			ReactionType: ReactionLike,
			Vector:       nil,
			EventTime:    now,
		},
		{
			SourceType:      SourceWatch,
			Vector:          []float32{1, 0},
			WatchDuration:   0,
			SegmentDuration: 100,
			EventTime:       now,
		},
	}, now)

	if result.Valid {
		t.Fatal("expected profile to be invalid")
	}
	if result.SourceEventCount != 0 {
		t.Fatalf("source event count = %d, want 0", result.SourceEventCount)
	}
}

func assertFloat32Near(t *testing.T, got float32, want float32, tolerance float32) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		t.Fatalf("got %v, want %v", got, want)
	}
}
