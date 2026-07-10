package recommendation

import (
	"math"
	"sort"
)

type ProfileCandidate struct {
	Candidate
	ProfileDistance    float64
	LikeCount          int
	DoubleLikeCount    int
	UserDisliked       bool
	UserVideoDisliked  bool
	UserWatched        bool
	repeatedVideoCount int
}

func RerankProfileCandidates(candidates []ProfileCandidate, limit int) []ResultItem {
	if limit <= 0 {
		limit = len(candidates)
	}
	videoCounts := make(map[uint64]int, len(candidates))
	scored := make([]struct {
		candidate ProfileCandidate
		score     float64
	}, 0, len(candidates))

	for _, candidate := range candidates {
		candidate.repeatedVideoCount = videoCounts[candidate.VideoID]
		videoCounts[candidate.VideoID]++
		score := profileRerankScore(candidate)
		scored = append(scored, struct {
			candidate ProfileCandidate
			score     float64
		}{candidate: candidate, score: score})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].candidate.Distance < scored[j].candidate.Distance
		}
		return scored[i].score > scored[j].score
	})

	if limit > len(scored) {
		limit = len(scored)
	}
	items := make([]ResultItem, 0, limit)
	for i := 0; i < limit; i++ {
		row := scored[i]
		items = append(items, buildResultItem(0, row.candidate.Candidate, row.score, row.candidate.UserWatched, 0))
	}
	return items
}

func BuildQuestionRankedItems(candidates []Candidate, limit int) []ResultItem {
	if limit <= 0 {
		limit = len(candidates)
	}
	sorted := append([]Candidate(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Distance < sorted[j].Distance
	})
	if limit > len(sorted) {
		limit = len(sorted)
	}
	items := make([]ResultItem, 0, limit)
	for i := 0; i < limit; i++ {
		score := distanceToScore(sorted[i].Distance)
		items = append(items, buildResultItem(0, sorted[i], score, false, 0))
	}
	return items
}

func BuildRecBoleRankedItems(candidates []RecBoleCandidate, limit int) []ResultItem {
	if limit <= 0 {
		limit = len(candidates)
	}
	sorted := append([]RecBoleCandidate(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Distance < sorted[j].Distance
	})
	if limit > len(sorted) {
		limit = len(sorted)
	}
	items := make([]ResultItem, 0, limit)
	for i := 0; i < limit; i++ {
		score := distanceToScore(sorted[i].Distance)
		items = append(items, buildResultItem(0, sorted[i].Candidate, score, false, 0))
	}
	return items
}

func profileRerankScore(candidate ProfileCandidate) float64 {
	return 0.65*distanceToScore(candidate.Distance) +
		0.25*distanceToScore(candidate.ProfileDistance) +
		0.10*popularityScore(candidate.LikeCount, candidate.DoubleLikeCount, candidate.ViewCount) -
		penaltyScore(candidate)
}

func distanceToScore(distance float64) float64 {
	if distance < 0 {
		return 0
	}
	return 1 / (1 + distance)
}

func popularityScore(likeCount int, doubleLikeCount int, viewCount int) float64 {
	raw := likeCount + 2*doubleLikeCount + viewCount
	if raw <= 0 {
		return 0
	}
	score := math.Log1p(float64(raw)) / 10
	if score > 1 {
		return 1
	}
	return score
}

func penaltyScore(candidate ProfileCandidate) float64 {
	score := 0.0
	if candidate.UserDisliked {
		score += 1
	}
	if candidate.UserVideoDisliked {
		score += 0.5
	}
	if candidate.UserWatched {
		score += 0.2
	}
	if candidate.repeatedVideoCount > 0 {
		score += float64(candidate.repeatedVideoCount) * 0.1
	}
	return score
}
