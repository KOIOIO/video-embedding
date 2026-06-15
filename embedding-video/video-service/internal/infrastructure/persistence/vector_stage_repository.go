package persistence

import (
	"context"

	"nlp-video-analysis/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	VectorStageStatusPending    int16 = 0
	VectorStageStatusProcessing int16 = 1
	VectorStageStatusComplete   int16 = 2
	VectorStageStatusFailed     int16 = 3
	VectorStageStatusSkipped    int16 = 4
)

type VectorStageRecord struct {
	TaskID       string
	VideoID      uint64
	Stage        string
	SegmentIndex int
	SegmentID    uint64
	Status       int16
	ObjectKey    string
	Text         string
	ErrorMessage string
	RetryCount   int
	StartSec     int
	EndSec       int
}

type VectorStageRepository struct {
	db *gorm.DB
}

func NewVectorStageRepository(db *gorm.DB) *VectorStageRepository {
	return &VectorStageRepository{db: db}
}

func (r *VectorStageRepository) UpsertPending(ctx context.Context, rec VectorStageRecord) error {
	row := vectorStageModel(rec)
	if row.Status == 0 {
		row.Status = VectorStageStatusPending
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "task_id"},
			{Name: "stage"},
			{Name: "segment_index"},
			{Name: "segment_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"video_id",
			"status",
			"object_key",
			"text",
			"error_message",
			"retry_count",
			"start_time",
			"end_time",
			"update_time",
		}),
	}).Create(&row).Error
}

func (r *VectorStageRepository) MarkComplete(ctx context.Context, rec VectorStageRecord) error {
	current, found, err := r.FindStage(ctx, rec.TaskID, rec.Stage, rec.SegmentIndex, rec.SegmentID)
	if err != nil {
		return err
	}
	if !found {
		pending := rec
		pending.Status = VectorStageStatusPending
		if err := r.UpsertPending(ctx, pending); err != nil {
			return err
		}
	} else {
		rec = mergeVectorStageRecord(current, rec)
	}
	updates := map[string]any{
		"status":        VectorStageStatusComplete,
		"object_key":    rec.ObjectKey,
		"text":          rec.Text,
		"error_message": "",
		"retry_count":   rec.RetryCount,
		"start_time":    rec.StartSec,
		"end_time":      rec.EndSec,
	}
	return r.db.WithContext(ctx).Model(&model.EduVideoVectorStage{}).
		Where("task_id = ? AND stage = ? AND segment_index = ? AND segment_id = ?", rec.TaskID, rec.Stage, rec.SegmentIndex, rec.SegmentID).
		Updates(updates).Error
}

func (r *VectorStageRepository) MarkFailed(ctx context.Context, rec VectorStageRecord) error {
	updates := map[string]any{
		"status":        VectorStageStatusFailed,
		"error_message": rec.ErrorMessage,
		"retry_count":   rec.RetryCount,
	}
	return r.db.WithContext(ctx).Model(&model.EduVideoVectorStage{}).
		Where("task_id = ? AND stage = ? AND segment_index = ? AND segment_id = ?", rec.TaskID, rec.Stage, rec.SegmentIndex, rec.SegmentID).
		Updates(updates).Error
}

func (r *VectorStageRepository) CountStage(ctx context.Context, taskID string, stage string) (int64, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.EduVideoVectorStage{}).
		Where("task_id = ? AND stage = ?", taskID, stage).
		Count(&total).Error; err != nil {
		return 0, 0, err
	}
	var complete int64
	if err := r.db.WithContext(ctx).Model(&model.EduVideoVectorStage{}).
		Where("task_id = ? AND stage = ? AND status = ?", taskID, stage, VectorStageStatusComplete).
		Count(&complete).Error; err != nil {
		return 0, 0, err
	}
	return total, complete, nil
}

func (r *VectorStageRepository) FindStage(ctx context.Context, taskID string, stage string, segmentIndex int, segmentID uint64) (VectorStageRecord, bool, error) {
	var row model.EduVideoVectorStage
	res := r.db.WithContext(ctx).
		Where("task_id = ? AND stage = ? AND segment_index = ? AND segment_id = ?", taskID, stage, segmentIndex, segmentID).
		Limit(1).
		Find(&row)
	if res.Error != nil {
		return VectorStageRecord{}, false, res.Error
	}
	if res.RowsAffected == 0 {
		return VectorStageRecord{}, false, nil
	}
	return vectorStageRecordFromModel(row), true, nil
}

func (r *VectorStageRepository) ListStage(ctx context.Context, taskID string, stage string) ([]VectorStageRecord, error) {
	var rows []model.EduVideoVectorStage
	if err := r.db.WithContext(ctx).
		Where("task_id = ? AND stage = ?", taskID, stage).
		Order("segment_index ASC, segment_id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]VectorStageRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, vectorStageRecordFromModel(row))
	}
	return out, nil
}

func vectorStageModel(rec VectorStageRecord) model.EduVideoVectorStage {
	return model.EduVideoVectorStage{
		TaskID:       rec.TaskID,
		VideoID:      rec.VideoID,
		Stage:        rec.Stage,
		SegmentIndex: rec.SegmentIndex,
		SegmentID:    rec.SegmentID,
		Status:       rec.Status,
		ObjectKey:    rec.ObjectKey,
		Text:         rec.Text,
		ErrorMessage: rec.ErrorMessage,
		RetryCount:   rec.RetryCount,
		StartTimeSec: rec.StartSec,
		EndTimeSec:   rec.EndSec,
	}
}

func mergeVectorStageRecord(current VectorStageRecord, next VectorStageRecord) VectorStageRecord {
	if next.VideoID == 0 {
		next.VideoID = current.VideoID
	}
	if next.ObjectKey == "" {
		next.ObjectKey = current.ObjectKey
	}
	if next.Text == "" {
		next.Text = current.Text
	}
	if next.StartSec == 0 {
		next.StartSec = current.StartSec
	}
	if next.EndSec == 0 {
		next.EndSec = current.EndSec
	}
	if next.RetryCount == 0 {
		next.RetryCount = current.RetryCount
	}
	return next
}

func vectorStageRecordFromModel(row model.EduVideoVectorStage) VectorStageRecord {
	return VectorStageRecord{
		TaskID:       row.TaskID,
		VideoID:      row.VideoID,
		Stage:        row.Stage,
		SegmentIndex: row.SegmentIndex,
		SegmentID:    row.SegmentID,
		Status:       row.Status,
		ObjectKey:    row.ObjectKey,
		Text:         row.Text,
		ErrorMessage: row.ErrorMessage,
		RetryCount:   row.RetryCount,
		StartSec:     row.StartTimeSec,
		EndSec:       row.EndTimeSec,
	}
}
