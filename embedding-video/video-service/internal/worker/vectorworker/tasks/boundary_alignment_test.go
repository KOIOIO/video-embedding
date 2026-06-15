package tasks

import "testing"

func TestBuildBoundaryWindows(t *testing.T) {
	startMin, startMax, endMin, endMax := buildBoundaryWindows(30, 70, 120)
	if startMin != 27 || startMax != 32 || endMin != 68 || endMax != 74 {
		t.Fatalf("unexpected windows: %d %d %d %d", startMin, startMax, endMin, endMax)
	}
}

func TestAlignSegmentBoundariesPrefersNaturalSentenceAndCapsOverlap(t *testing.T) {
	current := LLMSegment{SegmentIndex: 0, StartTimeSec: 30, EndTimeSec: 70, StartAnchorText: "下面先看定义", EndAnchorText: "这就是它的定义", BoundaryConfidence: "high"}
	next := &LLMSegment{SegmentIndex: 1, StartTimeSec: 70, EndTimeSec: 100, StartAnchorText: "接下来我们看例题"}

	aligned := alignSegmentBoundaries(current, next, boundaryAlignmentSnapshot{
		StartCandidates: []boundaryCandidate{{Sec: 29, Text: "所以", Score: -2}, {Sec: 30, Text: "下面先看定义", Score: 5}},
		EndCandidates:   []boundaryCandidate{{Sec: 71, Text: "这就是它的定义", Score: 4}, {Sec: 74, Text: "这就是它的定义。", Score: 8}},
	})

	if aligned.StartTimeSec != 30 {
		t.Fatalf("StartTimeSec = %d, want 30", aligned.StartTimeSec)
	}
	if aligned.EndTimeSec != 74 {
		t.Fatalf("EndTimeSec = %d, want 74", aligned.EndTimeSec)
	}
	if next.StartTimeSec != 71 {
		t.Fatalf("next.StartTimeSec = %d, want 71 to cap overlap at 3s", next.StartTimeSec)
	}
}

func TestScoreBoundaryCandidateUsesAnchorAndConfidence(t *testing.T) {
	seg := LLMSegment{StartAnchorText: "下面先看定义", BoundaryConfidence: "high"}
	score := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 30, Text: "下面先看定义"}, true)
	if score <= 0 {
		t.Fatalf("score = %d, want positive anchor score", score)
	}
}

func TestScoreBoundaryCandidatePrefersSemanticSignalsOverPhraseOnly(t *testing.T) {
	seg := LLMSegment{StartAnchorText: "新的知识点", BoundaryConfidence: "high"}
	phraseOnly := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 10, Text: "下面先看", Score: 0}, true)
	anchored := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 10, Text: "新的知识点从这里开始", Score: 0}, true)
	if phraseOnly >= anchored {
		t.Fatalf("phraseOnly score = %d, anchored score = %d", phraseOnly, anchored)
	}
}

func TestScoreBoundaryCandidatePenalizesSentenceFragmentStart(t *testing.T) {
	seg := LLMSegment{}
	fragment := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 10, Text: "所以"}, true)
	natural := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 10, Text: "下面先看定义"}, true)
	if fragment >= natural {
		t.Fatalf("fragment score = %d, natural score = %d", fragment, natural)
	}
}

func TestScoreBoundaryCandidateWithoutAnchorPrefersCompleteStartOverPhraseHit(t *testing.T) {
	seg := LLMSegment{}
	phraseOnly := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 10, Text: "下面先看"}, true)
	complete := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 10, Text: "这是新的知识点定义"}, true)
	if phraseOnly >= complete {
		t.Fatalf("phraseOnly score = %d, complete score = %d", phraseOnly, complete)
	}
}

func TestAlignSegmentBoundariesWithoutAnchorKeepsLegalRange(t *testing.T) {
	current := LLMSegment{SegmentIndex: 0, StartTimeSec: 10, EndTimeSec: 20}
	aligned := alignSegmentBoundaries(current, nil, boundaryAlignmentSnapshot{
		StartCandidates: []boundaryCandidate{{Sec: 10, Text: "因为", Score: 0}},
		EndCandidates:   []boundaryCandidate{{Sec: 20, Text: "然后", Score: 0}},
	})
	if aligned.StartTimeSec < 0 || aligned.EndTimeSec <= aligned.StartTimeSec {
		t.Fatalf("invalid aligned segment: %+v", aligned)
	}
}

func TestAlignSegmentBoundariesCapsOverlapAtThreeSeconds(t *testing.T) {
	current := LLMSegment{SegmentIndex: 0, StartTimeSec: 30, EndTimeSec: 70}
	next := &LLMSegment{SegmentIndex: 1, StartTimeSec: 68, EndTimeSec: 100}
	aligned := alignSegmentBoundaries(current, next, boundaryAlignmentSnapshot{
		StartCandidates: []boundaryCandidate{{Sec: 30, Text: "下面先看定义", Score: 5}},
		EndCandidates:   []boundaryCandidate{{Sec: 74, Text: "这就是它的定义。", Score: 8}},
	})
	if aligned.EndTimeSec-next.StartTimeSec > 3 {
		t.Fatalf("overlap = %d, want <= 3", aligned.EndTimeSec-next.StartTimeSec)
	}
}
