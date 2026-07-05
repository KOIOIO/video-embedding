package profile

import (
	"math"
	"time"
)

type SourceType string

const (
	SourceSegmentReaction SourceType = "segment_reaction"
	SourceVideoReaction   SourceType = "video_reaction"
	SourceWatch           SourceType = "watch"
)

type ReactionType string

const (
	ReactionLike       ReactionType = "like"
	ReactionDoubleLike ReactionType = "double_like"
	ReactionDislike    ReactionType = "dislike"
)

type WeightedEvent struct {
	SourceType      SourceType
	ReactionType    ReactionType
	Vector          []float32
	WatchDuration   int
	SegmentDuration int
	EventTime       time.Time
}

type UserVideoProfile struct {
	Vector           []float32
	Valid            bool
	PositiveCount    int
	NegativeCount    int
	WatchCount       int
	SourceEventCount int
	LastEventTime    time.Time
}

func BuildUserVideoProfile(events []WeightedEvent, now time.Time) UserVideoProfile {
	var out UserVideoProfile
	var sum []float64
	var denominator float64

	for _, event := range events {
		normalized, ok := normalize(event.Vector)
		if !ok {
			continue
		}

		weight := eventWeight(event)
		if weight == 0 {
			continue
		}
		weight *= timeDecay(now.Sub(event.EventTime))
		if weight == 0 {
			continue
		}

		if sum == nil {
			sum = make([]float64, len(normalized))
		}
		if len(normalized) != len(sum) {
			continue
		}
		for i, value := range normalized {
			sum[i] += float64(value) * weight
		}
		denominator += math.Abs(weight)
		out.SourceEventCount++
		if weight > 0 {
			out.PositiveCount++
		}
		if weight < 0 {
			out.NegativeCount++
		}
		if event.SourceType == SourceWatch {
			out.WatchCount++
		}
		if out.LastEventTime.IsZero() || event.EventTime.After(out.LastEventTime) {
			out.LastEventTime = event.EventTime
		}
	}

	if denominator == 0 || len(sum) == 0 || out.PositiveCount == 0 {
		return out
	}

	vector := make([]float32, len(sum))
	for i, value := range sum {
		vector[i] = float32(value / denominator)
	}
	normalized, ok := normalize(vector)
	if !ok {
		return out
	}
	out.Vector = normalized
	out.Valid = true
	return out
}

func eventWeight(event WeightedEvent) float64 {
	switch event.SourceType {
	case SourceSegmentReaction:
		return segmentReactionWeight(event.ReactionType)
	case SourceVideoReaction:
		return videoReactionWeight(event.ReactionType)
	case SourceWatch:
		return watchWeight(event.WatchDuration, event.SegmentDuration)
	default:
		return 0
	}
}

func segmentReactionWeight(reaction ReactionType) float64 {
	switch reaction {
	case ReactionDoubleLike:
		return 3
	case ReactionLike:
		return 2
	case ReactionDislike:
		return -2
	default:
		return 0
	}
}

func videoReactionWeight(reaction ReactionType) float64 {
	switch reaction {
	case ReactionDoubleLike:
		return 1.5
	case ReactionLike:
		return 1
	case ReactionDislike:
		return -1
	default:
		return 0
	}
}

func watchWeight(watchDuration int, segmentDuration int) float64 {
	if watchDuration <= 0 {
		return 0
	}
	if segmentDuration <= 0 {
		segmentDuration = 1
	}
	ratio := float64(watchDuration) / float64(segmentDuration)
	if ratio > 1 {
		ratio = 1
	}
	switch {
	case ratio >= 0.8:
		return 1.2
	case ratio >= 0.4:
		return 0.7
	case ratio > 0:
		return 0.3
	default:
		return 0
	}
}

func timeDecay(age time.Duration) float64 {
	if age < 0 {
		return 1
	}
	days := age.Hours() / 24
	switch {
	case days <= 7:
		return 1
	case days <= 30:
		return 0.7
	case days <= 90:
		return 0.4
	default:
		return 0.2
	}
}

func normalize(v []float32) ([]float32, bool) {
	if len(v) == 0 {
		return nil, false
	}
	var sumSquares float64
	for _, value := range v {
		sumSquares += float64(value) * float64(value)
	}
	if sumSquares == 0 {
		return nil, false
	}
	norm := math.Sqrt(sumSquares)
	out := make([]float32, len(v))
	for i, value := range v {
		out[i] = float32(float64(value) / norm)
	}
	return out, true
}
