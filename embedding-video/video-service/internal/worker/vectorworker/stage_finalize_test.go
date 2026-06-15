package vectorworker

import (
	"context"
	"testing"

	"nlp-video-analysis/internal/infrastructure/persistence"
)

type finalizeRepo struct {
	complete []persistence.VectorStageRecord
}

func (r *finalizeRepo) MarkComplete(_ context.Context, rec persistence.VectorStageRecord) error {
	r.complete = append(r.complete, rec)
	return nil
}

func TestFinalizeStageMarksComplete(t *testing.T) {
	repo := &finalizeRepo{}
	handler := newFinalizeStageHandler(repo)

	err := handler.Handle(context.Background(), VectorStageTask{TaskID: "task-1", VideoID: 9, Stage: VectorStageFinalize})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStageFinalize {
		t.Fatalf("finalize not complete: %+v", repo.complete)
	}
}
