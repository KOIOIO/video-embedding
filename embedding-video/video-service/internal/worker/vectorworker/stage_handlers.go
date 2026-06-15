package vectorworker

import (
	"context"
	"errors"
	"strings"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/infrastructure/persistence"
)

var errNonHierarchicalStageAdapter = errors.New("stage adapter only handles hierarchical mode")

type stagePendingRepository interface {
	UpsertPending(context.Context, persistence.VectorStageRecord) error
}

type stageEnqueuer interface {
	Enqueue(context.Context, VectorStageTask) error
}

type topLevelVectorStageAdapter struct {
	mode  string
	repo  stagePendingRepository
	queue stageEnqueuer
}

func newTopLevelVectorStageAdapter(mode string, repo stagePendingRepository, queue stageEnqueuer) *topLevelVectorStageAdapter {
	return &topLevelVectorStageAdapter{
		mode:  strings.ToLower(strings.TrimSpace(mode)),
		repo:  repo,
		queue: queue,
	}
}

func (a *topLevelVectorStageAdapter) Handle(ctx context.Context, task videoapp.VectorizeTask) error {
	if a.mode != "hierarchical" {
		return errNonHierarchicalStageAdapter
	}
	if task.VideoID == 0 || strings.TrimSpace(task.RawKey) == "" {
		return errors.New("invalid task")
	}
	prepare := VectorStageTask{
		TaskID:    task.TaskID,
		VideoID:   task.VideoID,
		RawKey:    task.RawKey,
		Stage:     VectorStagePrepare,
		ObjectKey: task.RawKey,
	}
	if err := a.repo.UpsertPending(ctx, persistence.VectorStageRecord{
		TaskID:    prepare.TaskID,
		VideoID:   prepare.VideoID,
		Stage:     prepare.Stage,
		ObjectKey: prepare.ObjectKey,
	}); err != nil {
		return err
	}
	return a.queue.Enqueue(ctx, prepare)
}
