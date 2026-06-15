package tasks

const (
	boundaryStartLookBackSec  = 3
	boundaryStartLookAheadSec = 2
	boundaryEndLookBackSec    = 2
	boundaryEndLookAheadSec   = 4
	maxRecommendedOverlapSec  = 3
)

type boundaryCandidate struct {
	Sec   int
	Text  string
	Score int
}

type boundaryAlignmentSnapshot struct {
	StartCandidates []boundaryCandidate
	EndCandidates   []boundaryCandidate
}

func buildBoundaryWindows(startSec int, endSec int, durationSec int) (int, int, int, int) {
	startMin := startSec - boundaryStartLookBackSec
	if startMin < 0 {
		startMin = 0
	}
	startMax := startSec + boundaryStartLookAheadSec
	if durationSec > 0 && startMax > durationSec {
		startMax = durationSec
	}
	endMin := endSec - boundaryEndLookBackSec
	if endMin < 0 {
		endMin = 0
	}
	endMax := endSec + boundaryEndLookAheadSec
	if durationSec > 0 && endMax > durationSec {
		endMax = durationSec
	}
	return startMin, startMax, endMin, endMax
}

func scoreBoundaryCandidate(seg LLMSegment, candidate boundaryCandidate, isStart bool) int {
	score := float64(candidate.Score)
	tmp := seg
	if isStart {
		tmp.StartAnchorText = candidate.Text
		startScore := EvaluateStartBoundary(tmp)
		score += startScore.Score
	} else {
		tmp.EndAnchorText = candidate.Text
		endScore := EvaluateEndBoundary(tmp)
		score += endScore.Score
	}
	if seg.BoundaryConfidence == "high" {
		score += 0.5
	}
	return int(score * 10)
}

func alignSegmentBoundaries(current LLMSegment, next *LLMSegment, snap boundaryAlignmentSnapshot) LLMSegment {
	bestStart := current.StartTimeSec
	bestStartScore := -1 << 30
	for _, c := range snap.StartCandidates {
		score := scoreBoundaryCandidate(current, c, true)
		if score > bestStartScore {
			bestStartScore = score
			bestStart = c.Sec
		}
	}
	bestEnd := current.EndTimeSec
	bestEndScore := -1 << 30
	for _, c := range snap.EndCandidates {
		score := scoreBoundaryCandidate(current, c, false)
		if score > bestEndScore {
			bestEndScore = score
			bestEnd = c.Sec
		}
	}
	current.StartTimeSec = bestStart
	current.EndTimeSec = bestEnd
	if current.EndTimeSec <= current.StartTimeSec {
		current.EndTimeSec = current.StartTimeSec + 1
	}
	if next != nil && current.EndTimeSec-next.StartTimeSec > maxRecommendedOverlapSec {
		next.StartTimeSec = current.EndTimeSec - maxRecommendedOverlapSec
	}
	return current
}
