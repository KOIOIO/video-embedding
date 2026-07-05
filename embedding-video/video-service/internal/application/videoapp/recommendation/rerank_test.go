package recommendation

import (
	"testing"
)

func TestRerankProfileCandidatesUsesProfileScoreWhenQuestionScoresAreClose(t *testing.T) {
	items := RerankProfileCandidates([]ProfileCandidate{
		{
			Candidate:       Candidate{VideoSegmentID: 1, VideoID: 10, Distance: 0.20},
			ProfileDistance: 0.80,
		},
		{
			Candidate:       Candidate{VideoSegmentID: 2, VideoID: 20, Distance: 0.24},
			ProfileDistance: 0.05,
		},
	}, 2)

	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].VideoSegmentID != 2 {
		t.Fatalf("first segment = %d, want profile-matched segment 2", items[0].VideoSegmentID)
	}
	if items[0].RecommendScore <= items[1].RecommendScore {
		t.Fatalf("scores not descending: %+v", items)
	}
}

func TestRerankProfileCandidatesKeepsStrongQuestionMatchAhead(t *testing.T) {
	items := RerankProfileCandidates([]ProfileCandidate{
		{
			Candidate:       Candidate{VideoSegmentID: 1, VideoID: 10, Distance: 0.01},
			ProfileDistance: 1.0,
		},
		{
			Candidate:       Candidate{VideoSegmentID: 2, VideoID: 20, Distance: 1.0},
			ProfileDistance: 0.01,
		},
	}, 2)

	if items[0].VideoSegmentID != 1 {
		t.Fatalf("first segment = %d, want strong question match segment 1", items[0].VideoSegmentID)
	}
}

func TestRerankProfileCandidatesPenalizesDislikedAndWatchedSegments(t *testing.T) {
	items := RerankProfileCandidates([]ProfileCandidate{
		{
			Candidate:       Candidate{VideoSegmentID: 1, VideoID: 10, Distance: 0.1},
			ProfileDistance: 0.1,
			UserDisliked:    true,
		},
		{
			Candidate:       Candidate{VideoSegmentID: 2, VideoID: 20, Distance: 0.1},
			ProfileDistance: 0.1,
			UserWatched:     true,
		},
		{
			Candidate:       Candidate{VideoSegmentID: 3, VideoID: 30, Distance: 0.1},
			ProfileDistance: 0.1,
		},
	}, 3)

	if items[0].VideoSegmentID != 3 {
		t.Fatalf("first segment = %d, want unpenalized segment 3", items[0].VideoSegmentID)
	}
	if items[2].VideoSegmentID != 1 {
		t.Fatalf("last segment = %d, want disliked segment 1", items[2].VideoSegmentID)
	}
}

func TestRerankProfileCandidatesPenalizesRepeatedVideoSegments(t *testing.T) {
	items := RerankProfileCandidates([]ProfileCandidate{
		{
			Candidate:       Candidate{VideoSegmentID: 1, VideoID: 10, Distance: 0.1},
			ProfileDistance: 0.1,
		},
		{
			Candidate:       Candidate{VideoSegmentID: 2, VideoID: 10, Distance: 0.1},
			ProfileDistance: 0.1,
		},
		{
			Candidate:       Candidate{VideoSegmentID: 3, VideoID: 30, Distance: 0.1},
			ProfileDistance: 0.1,
		},
	}, 3)

	if items[1].VideoID == 10 {
		t.Fatalf("second result should prefer a different video: %+v", items)
	}
}

func TestBuildQuestionRankedItemsPreservesQuestionDistanceOrder(t *testing.T) {
	items := BuildQuestionRankedItems([]Candidate{
		{VideoSegmentID: 1, VideoID: 10, Distance: 0.5},
		{VideoSegmentID: 2, VideoID: 20, Distance: 0.1},
	}, 2)

	if items[0].VideoSegmentID != 2 || items[1].VideoSegmentID != 1 {
		t.Fatalf("items = %+v, want sorted by question distance", items)
	}
}
