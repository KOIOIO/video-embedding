package vectorworker

import (
	"context"
	"errors"

	"nlp-video-analysis/internal/infrastructure/persistence"
)

type finalizeStageRepository interface {
	MarkComplete(context.Context, persistence.VectorStageRecord) error
}

type finalizeStageHandler struct {
	repo finalizeStageRepository
}

func newFinalizeStageHandler(repo finalizeStageRepository) *finalizeStageHandler {
	return &finalizeStageHandler{repo: repo}
}

func (h *finalizeStageHandler) Handle(ctx context.Context, task VectorStageTask) error {
	if task.TaskID == "" || task.VideoID == 0 {
		return errors.New("invalid finalize task")
	}
	return h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		Stage:   VectorStageFinalize,
		EndSec:  task.EndSec,
	})
}
