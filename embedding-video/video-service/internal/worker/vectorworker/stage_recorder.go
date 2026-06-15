package vectorworker

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"nlp-video-analysis/internal/infrastructure/persistence"
	"nlp-video-analysis/internal/worker/vectorworker/tasks"
)

type vectorStageRepository interface {
	UpsertPending(context.Context, persistence.VectorStageRecord) error
	MarkComplete(context.Context, persistence.VectorStageRecord) error
	MarkFailed(context.Context, persistence.VectorStageRecord) error
}

type vectorStageRecorder struct {
	repo vectorStageRepository
}

func newVectorStageRecorder(repo vectorStageRepository) *vectorStageRecorder {
	return &vectorStageRecorder{repo: repo}
}

func (r *vectorStageRecorder) Pending(ctx context.Context, task VectorStageTask) {
	if r == nil || r.repo == nil {
		return
	}
	if err := r.repo.UpsertPending(ctx, vectorStageRecord(task, persistence.VectorStageStatusPending, "", "")); err != nil {
		zap.L().Warn("vector_stage_record_failed", zap.String("stage", task.Stage), zap.String("op", "pending"), zap.Error(err))
	}
}

func (r *vectorStageRecorder) Complete(ctx context.Context, task VectorStageTask, text string) {
	if r == nil || r.repo == nil {
		return
	}
	rec := vectorStageRecord(task, persistence.VectorStageStatusComplete, text, "")
	if err := r.repo.UpsertPending(ctx, rec); err != nil {
		zap.L().Warn("vector_stage_record_failed", zap.String("stage", task.Stage), zap.String("op", "ensure_pending"), zap.Error(err))
		return
	}
	if err := r.repo.MarkComplete(ctx, rec); err != nil {
		zap.L().Warn("vector_stage_record_failed", zap.String("stage", task.Stage), zap.String("op", "complete"), zap.Error(err))
	}
}

func (r *vectorStageRecorder) Fail(ctx context.Context, task VectorStageTask, stageErr error) {
	if r == nil || r.repo == nil || stageErr == nil {
		return
	}
	rec := vectorStageRecord(task, persistence.VectorStageStatusFailed, "", stageErr.Error())
	if err := r.repo.UpsertPending(ctx, rec); err != nil {
		zap.L().Warn("vector_stage_record_failed", zap.String("stage", task.Stage), zap.String("op", "ensure_pending"), zap.Error(err))
		return
	}
	if err := r.repo.MarkFailed(ctx, rec); err != nil {
		zap.L().Warn("vector_stage_record_failed", zap.String("stage", task.Stage), zap.String("op", "failed"), zap.Error(err))
	}
}

func (r *vectorStageRecorder) PendingTask(ctx context.Context, task tasks.StageRecord) {
	r.Pending(ctx, vectorStageTaskFromRecord(task))
}

func (r *vectorStageRecorder) CompleteTask(ctx context.Context, task tasks.StageRecord) {
	r.Complete(ctx, vectorStageTaskFromRecord(task), task.Text)
}

func (r *vectorStageRecorder) FailTask(ctx context.Context, task tasks.StageRecord, stageErr error) {
	r.Fail(ctx, vectorStageTaskFromRecord(task), stageErr)
}

func vectorStageRecord(task VectorStageTask, status int16, text string, errMsg string) persistence.VectorStageRecord {
	return persistence.VectorStageRecord{
		TaskID:       strings.TrimSpace(task.TaskID),
		VideoID:      task.VideoID,
		Stage:        strings.TrimSpace(task.Stage),
		SegmentIndex: task.SegmentIndex,
		SegmentID:    task.SegmentID,
		Status:       status,
		ObjectKey:    strings.TrimSpace(task.ObjectKey),
		Text:         strings.TrimSpace(text),
		ErrorMessage: strings.TrimSpace(errMsg),
		RetryCount:   task.RetryCount,
		StartSec:     task.StartSec,
		EndSec:       task.EndSec,
	}
}

func vectorStageTaskFromRecord(rec tasks.StageRecord) VectorStageTask {
	return VectorStageTask{
		TaskID:       rec.TaskID,
		VideoID:      rec.VideoID,
		Stage:        rec.Stage,
		SegmentIndex: rec.SegmentIndex,
		SegmentID:    rec.SegmentID,
		ObjectKey:    rec.ObjectKey,
		RetryCount:   rec.RetryCount,
		StartSec:     rec.StartSec,
		EndSec:       rec.EndSec,
	}
}
