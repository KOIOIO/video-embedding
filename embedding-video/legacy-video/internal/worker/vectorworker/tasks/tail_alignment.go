package tasks

import "strings"

type TailAlignmentConfig struct {
	Enabled       bool
	MaxExtendSec  int
	ProbeStepSec  int
	MaxOverlapSec int
}

var sentenceEndTokens = []string{"。", "！", "？", ".", "!", "?"}

var sentenceEndPhrases = []string{
	"讲到这里",
	"到这里",
	"总结一下",
	"就是这样",
	"讲完了",
}

var trailingConnectors = []string{
	"所以",
	"然后",
	"但是",
	"因为",
	"如果",
	"接下来",
	"我们来看",
	"也就是说",
}

var sentenceStartPhrases = []string{
	"下面先看",
	"我们先看",
	"第一步",
	"接下来我们看",
	"先来看",
}

func NormalizeTailAlignmentConfig(cfg TailAlignmentConfig) TailAlignmentConfig {
	if cfg.MaxExtendSec <= 0 {
		cfg.MaxExtendSec = 3
	}
	if cfg.ProbeStepSec <= 0 {
		cfg.ProbeStepSec = 1
	}
	if cfg.MaxOverlapSec <= 0 {
		cfg.MaxOverlapSec = 6
	}
	return cfg
}

func LooksLikeSentenceEnd(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, token := range sentenceEndTokens {
		if strings.HasSuffix(text, token) {
			return true
		}
	}
	for _, phrase := range sentenceEndPhrases {
		if strings.HasSuffix(text, phrase) {
			return true
		}
	}
	for _, connector := range trailingConnectors {
		if strings.HasSuffix(text, connector) {
			return false
		}
	}
	return false
}

func LooksLikeSentenceStart(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, phrase := range trailingConnectors {
		if text == phrase {
			return false
		}
	}
	for _, phrase := range sentenceStartPhrases {
		if strings.HasPrefix(text, phrase) {
			return true
		}
	}
	if strings.HasPrefix(text, "因为") || strings.HasPrefix(text, "所以") || strings.HasPrefix(text, "然后") {
		return false
	}
	return len([]rune(text)) >= 4
}

func NormalizeBoundaryConfidence(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "high", "medium", "low":
		return s
	default:
		return ""
	}
}

func NeedsTailExtension(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return !LooksLikeSentenceEnd(text)
}

func NextAlignedEndSec(currentEndSec int, originalEndSec int, nextSegmentStartSec int, videoDurationSec int, cfg TailAlignmentConfig) int {
	cfg = NormalizeTailAlignmentConfig(cfg)
	limit := originalEndSec + cfg.MaxExtendSec
	if videoDurationSec > 0 && videoDurationSec < limit {
		limit = videoDurationSec
	}
	if nextSegmentStartSec > 0 {
		overlapLimit := nextSegmentStartSec + cfg.MaxOverlapSec
		if overlapLimit < limit {
			limit = overlapLimit
		}
	}
	next := currentEndSec + cfg.ProbeStepSec
	if next > limit {
		next = limit
	}
	if next < currentEndSec {
		return currentEndSec
	}
	return next
}
