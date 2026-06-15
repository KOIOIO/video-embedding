package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"nlp-video-analysis/internal/infrastructure/persistence/sqlqueries"
	"nlp-video-analysis/internal/model"
)

// LLMSegment 表示 LLM 生成的视频分段
type LLMSegment struct {
	SegmentIndex       int      `json:"segment_index"`
	StartTimeSec       int      `json:"start_time"`
	EndTimeSec         int      `json:"end_time"`
	ContentSummary     string   `json:"content_summary"`
	KnowledgeTags      []string `json:"knowledge_tags"`
	BoundaryReason     string   `json:"boundary_reason"`
	StartAnchorText    string   `json:"start_anchor_text"`
	EndAnchorText      string   `json:"end_anchor_text"`
	BoundaryConfidence string   `json:"boundary_confidence"`
}

// CoarseItem 表示粗分段的项
type CoarseItem struct {
	Index     int
	StartSec  int
	EndSec    int
	Text      string
	ObjectKey string
}

// 为了向后兼容，保留旧的小写类型名
type llmSegment = LLMSegment
type coarseItem = CoarseItem

func normalizeSegmentTitle(raw string) string {
	title := strings.TrimSpace(raw)
	if title == "" {
		return ""
	}
	if parsed, ok := extractSegmentTitleFromJSON(title); ok {
		title = parsed
	}
	title = strings.ReplaceAll(title, "\r", " ")
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Join(strings.Fields(title), " ")
	const maxRunes = 60
	runes := []rune(title)
	if len(runes) > maxRunes {
		title = strings.TrimSpace(string(runes[:maxRunes]))
	}
	return title
}

func extractSegmentTitleFromJSON(raw string) (string, bool) {
	var str string
	if err := json.Unmarshal([]byte(raw), &str); err == nil {
		return strings.TrimSpace(str), true
	}
	var obj struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil && strings.TrimSpace(obj.Title) != "" {
		return strings.TrimSpace(obj.Title), true
	}
	return "", false
}

// UpsertHierarchicalSegments 写入 LLM 生成的细分段草稿。
// 已完成 embedding 的老分段会保留，未完成的旧草稿会被软删除后重新插入。
func UpsertHierarchicalSegments(ctx context.Context, db *gorm.DB, videoID uint64, segs []llmSegment) error {
	if videoID == 0 {
		return errors.New("videoID is required")
	}
	if len(segs) == 0 {
		return errors.New("segments is empty")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.EduVideoSegment{}).
			Where("video_id = ? AND deleted = 0 AND status = 0", videoID).
			Update("deleted", 1).Error; err != nil {
			return err
		}

		var existingSegments []model.EduVideoSegment
		if err := tx.Model(&model.EduVideoSegment{}).
			Where("video_id = ? AND deleted = 0 AND status = 1", videoID).
			Find(&existingSegments).Error; err != nil {
			return err
		}
		existingIndices := make(map[int]bool)
		for _, s := range existingSegments {
			existingIndices[s.SegmentIndex] = true
		}

		placeholders := make([]string, 0, len(segs))
		args := make([]any, 0, len(segs)*9)
		now := time.Now()
		for _, s := range segs {
			if existingIndices[s.SegmentIndex] {
				continue
			}
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			args = append(args,
				videoID,
				s.SegmentIndex,
				s.StartTimeSec,
				s.EndTimeSec,
				normalizeSegmentTitle(s.ContentSummary),
				model.TextArray(normalizeTags(s.KnowledgeTags)),
				int16(0),
				int16(0),
				now,
			)
		}

		if len(placeholders) == 0 {
			return nil
		}

		query := sqlqueries.UpsertHierarchicalSegmentsQueryPrefix + strings.Join(placeholders, ",")
		return tx.Exec(query, args...).Error
	})
}
