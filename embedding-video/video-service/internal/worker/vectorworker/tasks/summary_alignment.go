package tasks

import (
	"strings"
	"unicode"
)

func detectSummaryContentMismatch(seg LLMSegment, text string, nextSummary string) bool {
	text = strings.TrimSpace(text)
	summary := strings.TrimSpace(seg.ContentSummary)
	if text == "" {
		return false
	}
	if seg.EndTimeSec-seg.StartTimeSec < 20 && len([]rune(summary)) > 12 {
		return true
	}
	if len([]rune(summary)) >= 10 && len([]rune(text)) > 0 && len([]rune(summary)) >= len([]rune(text))*2 {
		return true
	}
	for _, token := range []string{"全部", "完整讲解", "适用条件", "总结", "结论"} {
		if strings.Contains(summary, token) && !strings.Contains(text, token) {
			return true
		}
	}
	if summaryUnsupportedByText(summary, text) {
		return true
	}
	if looksLikeContinuationText(text) {
		return true
	}
	if sharesTopic(summary, nextSummary) && !LooksLikeSentenceEnd(text) {
		return true
	}
	if strings.TrimSpace(nextSummary) != "" && sharesTopic(summary, nextSummary) && !LooksLikeSentenceEnd(text) {
		return true
	}
	return false
}

func summaryUnsupportedByText(summary string, text string) bool {
	summary = compactMeaningfulText(summary)
	text = compactMeaningfulText(text)
	if summary == "" || text == "" {
		return false
	}
	if len([]rune(summary)) < 4 || len([]rune(text)) < 12 {
		return false
	}
	if strings.Contains(text, summary) {
		return false
	}
	matches := 0
	tokens := meaningfulBigrams(summary)
	for _, token := range tokens {
		if strings.Contains(text, token) {
			matches++
		}
	}
	if matches == 0 {
		return len(tokens) >= 2
	}
	return len(tokens) >= 4 && matches*4 < len(tokens)
}

func compactMeaningfulText(s string) string {
	s = NormalizeText(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func meaningfulBigrams(s string) []string {
	runes := []rune(s)
	if len(runes) < 2 {
		return nil
	}
	out := make([]string, 0, len(runes)-1)
	seen := make(map[string]struct{}, len(runes)-1)
	for i := 0; i+1 < len(runes); i++ {
		token := string(runes[i : i+2])
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func shouldMergeMismatchedSegment(current LLMSegment, next LLMSegment, currentText string) bool {
	if strings.TrimSpace(next.ContentSummary) == "" {
		return false
	}
	nextSummary := strings.TrimSpace(next.ContentSummary)
	if current.EndTimeSec-current.StartTimeSec <= 20 && looksLikeContinuationText(nextSummary) {
		return true
	}
	if !detectSummaryContentMismatch(current, currentText, next.ContentSummary) {
		return false
	}
	return sharesTopic(current.ContentSummary, nextSummary) || looksLikeContinuationText(currentText) || looksLikeContinuationText(nextSummary)
}

func repairMismatchedSegments(segs []LLMSegment, coarseItems []CoarseItem) []LLMSegment {
	if len(segs) < 2 {
		return segs
	}
	out := make([]LLMSegment, 0, len(segs))
	for i := 0; i < len(segs); i++ {
		current := segs[i]
		if i+1 < len(segs) {
			currentText := buildTranscriptFromCoarseItems(coarseItems, current.StartTimeSec, current.EndTimeSec)
			next := segs[i+1]
			if shouldMergeMismatchedSegment(current, next, currentText) {
				current.EndTimeSec = next.EndTimeSec
				current.ContentSummary = strings.TrimSpace(current.ContentSummary + "\n" + next.ContentSummary)
				current.KnowledgeTags = MergeTags(current.KnowledgeTags, next.KnowledgeTags)
				out = append(out, current)
				i++
				continue
			}
		}
		out = append(out, current)
	}
	for i := range out {
		out[i].SegmentIndex = i
	}
	return out
}

func RepairMismatchedSegments(segs []LLMSegment, coarseItems []CoarseItem) []LLMSegment {
	return repairMismatchedSegments(segs, coarseItems)
}

func summaryNeedsRewrite(seg LLMSegment, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return detectSummaryContentMismatch(seg, text, "")
}

func rewriteSummaryFromText(text string) string {
	text = NormalizeText(text)
	if text == "" {
		return ""
	}
	for _, sep := range []string{"，", "。", "\n"} {
		if idx := strings.Index(text, sep); idx > 0 {
			return normalizeSegmentTitle(text[:idx])
		}
	}
	return normalizeSegmentTitle(text)
}

func shouldUseLLMSummaryRewrite(text string) bool {
	text = NormalizeText(text)
	return len([]rune(text)) > 60 || strings.Count(text, "。") >= 2 || strings.Count(text, "\n") >= 2
}

func alignSummaryToStableText(seg LLMSegment, text string) (LLMSegment, bool) {
	if !summaryNeedsRewrite(seg, text) {
		return seg, false
	}
	rewritten := seg
	rewritten.ContentSummary = rewriteSummaryFromText(text)
	return rewritten, rewritten.ContentSummary != seg.ContentSummary
}

func alignSummaryToStableTextWithMode(seg LLMSegment, text string) (LLMSegment, string) {
	aligned, changed := alignSummaryToStableText(seg, text)
	if !changed {
		return aligned, "original"
	}
	return aligned, "rule_rewrite"
}
