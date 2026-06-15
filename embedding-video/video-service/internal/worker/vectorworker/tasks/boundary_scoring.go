package tasks

import "strings"

type BoundaryScore struct {
	Score      float64
	Confidence string
	Reasons    []string
}

type SegmentBoundaryDecision struct {
	Action     string
	Score      float64
	Confidence string
	Reasons    []string
}

func EvaluateStartBoundary(curr LLMSegment) BoundaryScore {
	score := 0.0
	reasons := make([]string, 0, 4)
	anchor := strings.TrimSpace(curr.StartAnchorText)
	summary := strings.TrimSpace(curr.ContentSummary)

	if anchor != "" && LooksLikeSentenceStart(anchor) {
		score += 0.6
		reasons = append(reasons, "start_anchor_looks_complete")
	}
	if anchor != "" && len([]rune(anchor)) >= 6 && !looksLikeContinuationText(anchor) {
		score += 0.5
		reasons = append(reasons, "start_anchor_has_substantial_content")
	}
	if summary != "" && !looksLikeContinuationText(summary) {
		score += 0.8
		reasons = append(reasons, "summary_not_continuation_like")
	}
	if summary != "" && len([]rune(summary)) >= 8 {
		score += 0.5
		reasons = append(reasons, "summary_has_complete_unit_signal")
	}
	if anchor == "" && summary == "" {
		score -= 0.5
		reasons = append(reasons, "missing_start_signals")
	}
	if looksLikeContinuationText(anchor) || looksLikeContinuationText(summary) {
		score -= 1.2
		reasons = append(reasons, "continuation_like_start")
	}

	return BoundaryScore{Score: score, Confidence: curr.BoundaryConfidence, Reasons: reasons}
}

func EvaluateEndBoundary(curr LLMSegment) BoundaryScore {
	score := 0.0
	reasons := make([]string, 0, 4)
	anchor := strings.TrimSpace(curr.EndAnchorText)
	summary := strings.TrimSpace(curr.ContentSummary)

	if anchor != "" && LooksLikeSentenceEnd(anchor) {
		score += 1.0
		reasons = append(reasons, "end_anchor_looks_closed")
	}
	if summary != "" && LooksLikeSentenceEnd(summary) {
		score += 0.6
		reasons = append(reasons, "summary_looks_closed")
	}
	if looksLikeContinuationText(anchor) {
		score -= 0.8
		reasons = append(reasons, "continuation_like_end")
	}
	if anchor == "" && summary == "" {
		score -= 0.5
		reasons = append(reasons, "missing_end_signals")
	}

	return BoundaryScore{Score: score, Confidence: curr.BoundaryConfidence, Reasons: reasons}
}

func EvaluateContinuation(prev LLMSegment, curr LLMSegment) BoundaryScore {
	score := 0.0
	reasons := make([]string, 0, 6)
	if prev.BoundaryConfidence == "low" || curr.BoundaryConfidence == "low" {
		score += 0.6
		reasons = append(reasons, "low_boundary_confidence")
	}
	if looksLikeContinuationText(curr.StartAnchorText) {
		score += 1.0
		reasons = append(reasons, "continuation_like_start_anchor")
	}
	if looksLikeContinuationText(curr.ContentSummary) {
		score += 0.8
		reasons = append(reasons, "continuation_like_summary")
	}
	if sharesTopic(prev.ContentSummary, curr.ContentSummary) {
		score += 0.5
		reasons = append(reasons, "shared_topic_signals")
	}
	if strings.Contains(curr.BoundaryReason, "继续") || strings.Contains(curr.BoundaryReason, "补充") {
		score += 0.6
		reasons = append(reasons, "boundary_reason_indicates_continuation")
	}

	return BoundaryScore{Score: score, Confidence: curr.BoundaryConfidence, Reasons: reasons}
}

func EvaluateSeparation(prev LLMSegment, curr LLMSegment) BoundaryScore {
	score := 0.0
	reasons := make([]string, 0, 6)
	if prev.BoundaryConfidence == "high" && curr.BoundaryConfidence == "high" {
		score += 0.8
		reasons = append(reasons, "high_boundary_confidence")
	}
	if strings.Contains(curr.BoundaryReason, "新的") || strings.Contains(curr.BoundaryReason, "开始") || strings.Contains(curr.BoundaryReason, "进入") {
		score += 1.0
		reasons = append(reasons, "boundary_reason_indicates_new_unit")
	}
	if !sharesTopic(prev.ContentSummary, curr.ContentSummary) {
		score += 0.6
		reasons = append(reasons, "topic_shift_detected")
	}
	if !looksLikeContinuationText(curr.StartAnchorText) && !looksLikeContinuationText(curr.ContentSummary) {
		score += 0.4
		reasons = append(reasons, "not_continuation_like")
	}

	return BoundaryScore{Score: score, Confidence: curr.BoundaryConfidence, Reasons: reasons}
}

func EvaluateSegmentBoundary(prev LLMSegment, curr LLMSegment) SegmentBoundaryDecision {
	cont := EvaluateContinuation(prev, curr)
	sep := EvaluateSeparation(prev, curr)
	start := EvaluateStartBoundary(curr)
	score := cont.Score - sep.Score - start.Score*0.3
	reasons := append([]string{}, cont.Reasons...)
	reasons = append(reasons, sep.Reasons...)
	reasons = append(reasons, start.Reasons...)
	ambiguous := (len(cont.Reasons) > 0 && len(sep.Reasons) > 0)
	if score >= 0.5 {
		return SegmentBoundaryDecision{Action: "merge", Score: score, Confidence: curr.BoundaryConfidence, Reasons: reasons}
	}
	if (curr.BoundaryConfidence == "low" || curr.BoundaryConfidence == "medium") && ambiguous && score > -1.2 && score < 0.8 {
		reasons = append(reasons, "ambiguous_boundary_needs_recut")
		return SegmentBoundaryDecision{Action: "recut", Score: score, Confidence: curr.BoundaryConfidence, Reasons: reasons}
	}
	return SegmentBoundaryDecision{Action: "keep", Score: score, Confidence: curr.BoundaryConfidence, Reasons: reasons}
}

func looksLikeContinuationText(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, prefix := range continuationPrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func sharesTopic(prev string, curr string) bool {
	prev = strings.TrimSpace(prev)
	curr = strings.TrimSpace(curr)
	if prev == "" || curr == "" {
		return false
	}
	for _, token := range []string{"定义", "步骤", "例题", "结论", "公式", "条件"} {
		if strings.Contains(prev, token) && strings.Contains(curr, token) {
			return true
		}
	}
	return false
}
