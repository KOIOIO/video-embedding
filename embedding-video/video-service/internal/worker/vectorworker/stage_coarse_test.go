package vectorworker

import (
	"context"
	"testing"

	"nlp-video-analysis/internal/infrastructure/persistence"
)

type coarseRepo struct {
	segments []persistence.VectorStageRecord
	complete []persistence.VectorStageRecord
}

func (r *coarseRepo) ListStage(_ context.Context, _ string, _ string) ([]persistence.VectorStageRecord, error) {
	return r.segments, nil
}

func (r *coarseRepo) MarkComplete(_ context.Context, rec persistence.VectorStageRecord) error {
	r.complete = append(r.complete, rec)
	return nil
}

type fakeCoarseProcessor struct {
	called bool
}

func (p *fakeCoarseProcessor) ProcessCoarse(_ context.Context, _ VectorStageTask, _ []persistence.VectorStageRecord) error {
	p.called = true
	return nil
}

func TestCoarseStageProcessesPlanAndEnqueuesRefine(t *testing.T) {
	repo := &coarseRepo{segments: []persistence.VectorStageRecord{
		{TaskID: "task-1", Stage: vectorStageCoarseSegment, SegmentIndex: 0, StartSec: 0, EndSec: 40},
	}}
	nextQueue := &recordingStageQueue{}
	processor := &fakeCoarseProcessor{}
	handler := newCoarseStageHandler(repo, processor, nextQueue)

	err := handler.Handle(context.Background(), VectorStageTask{TaskID: "task-1", VideoID: 9, RawKey: "raw/video.mp4", Stage: VectorStageCoarse})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !processor.called {
		t.Fatal("expected processor to be called")
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStageCoarse {
		t.Fatalf("coarse not complete: %+v", repo.complete)
	}
	if len(nextQueue.enqueued) != 1 || nextQueue.enqueued[0].Stage != VectorStageRefine {
		t.Fatalf("refine not enqueued: %+v", nextQueue.enqueued)
	}
}
