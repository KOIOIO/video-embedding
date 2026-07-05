package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// BuildSingleSegmentSummaryPrompt asks the LLM for metadata for a short video
// that should be stored as one edu_video_segment row.
func BuildSingleSegmentSummaryPrompt(durationSec int, transcript string) (string, error) {
	transcript = strings.TrimSpace(transcript)
	if durationSec <= 0 {
		return "", errors.New("durationSec must be > 0")
	}
	if transcript == "" {
		return "", errors.New("transcript is required")
	}
	var b strings.Builder
	b.WriteString("请根据短视频的完整转写内容，生成一个用于视频检索的单段信息。\n")
	b.WriteString(fmt.Sprintf("- 视频总时长（秒）：%d\n", durationSec))
	b.WriteString("- 输出必须是 JSON 对象，不要输出 Markdown。\n")
	b.WriteString("- content_summary 是短标题，不是长段内容简介，建议 8-24 个汉字。\n")
	b.WriteString("- knowledge_tags 是知识点标签数组，保留 1-6 个最重要标签。\n")
	b.WriteString("输出格式：\n")
	b.WriteString("{\"content_summary\":\"...\",\"knowledge_tags\":[\"tag1\",\"tag2\"]}\n")
	b.WriteString("完整转写内容：\n")
	b.WriteString(transcript)
	return b.String(), nil
}

// NormalizeSingleSegmentSummary normalizes the LLM JSON output into one full-video segment.
func NormalizeSingleSegmentSummary(llmOut string, durationSec int) (LLMSegment, error) {
	if durationSec <= 0 {
		return LLMSegment{}, errors.New("durationSec must be > 0")
	}
	raw := strings.TrimSpace(llmOut)
	if obj, ok := ExtractFirstJSONObject(raw); ok {
		raw = obj
	}
	var parsed struct {
		ContentSummary string   `json:"content_summary"`
		KnowledgeTags  []string `json:"knowledge_tags"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return LLMSegment{}, err
	}
	summary := normalizeSegmentTitle(parsed.ContentSummary)
	if summary == "" {
		return LLMSegment{}, errors.New("content_summary is required")
	}
	return LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   0,
		EndTimeSec:     durationSec,
		ContentSummary: summary,
		KnowledgeTags:  NormalizeTags(parsed.KnowledgeTags),
	}, nil
}
