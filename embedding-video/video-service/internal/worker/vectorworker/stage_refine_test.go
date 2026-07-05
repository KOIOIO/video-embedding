package vectorworker

import (
	"context"
	"testing"

	"nlp-video-analysis/internal/infrastructure/persistence"
)

type refineRepo struct {
	coarse   []persistence.VectorStageRecord
	existing bool
	complete []persistence.VectorStageRecord
}

func (r *refineRepo) ListStage(_ context.Context, _ string, _ string) ([]persistence.VectorStageRecord, error) {
	return r.coarse, nil
}

func (r *refineRepo) MarkComplete(_ context.Context, rec persistence.VectorStageRecord) error {
	r.complete = append(r.complete, rec)
	return nil
}

func (r *refineRepo) HasExistingSegments(_ context.Context, _ uint64) (bool, error) {
	return r.existing, nil
}

type fakeRefineProcessor struct {
	called bool
	task   VectorStageTask
}

func (p *fakeRefineProcessor) ProcessRefine(_ context.Context, task VectorStageTask, _ []persistence.VectorStageRecord) error {
	p.called = true
	p.task = task
	return nil
}

func TestRefineStageProcessesCoarseAndEnqueuesFinalize(t *testing.T) {
	repo := &refineRepo{coarse: []persistence.VectorStageRecord{
		{TaskID: "task-1", Stage: vectorStageCoarseSegment, SegmentIndex: 0, Text: "coarse text"},
	}}
	nextQueue := &recordingStageQueue{}
	processor := &fakeRefineProcessor{}
	handler := newRefineStageHandler(repo, processor, nextQueue)

	err := handler.Handle(context.Background(), VectorStageTask{TaskID: "task-1", VideoID: 9, RawKey: "raw/video.mp4", Stage: VectorStageRefine})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !processor.called {
		t.Fatal("expected processor to be called")
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStageRefine {
		t.Fatalf("refine not complete: %+v", repo.complete)
	}
	if len(nextQueue.enqueued) != 1 || nextQueue.enqueued[0].Stage != VectorStageFinalize {
		t.Fatalf("finalize not enqueued: %+v", nextQueue.enqueued)
	}
}

func TestRefineStageRejectsEmptyCoarseWithoutExistingSegments(t *testing.T) {
	repo := &refineRepo{}
	nextQueue := &recordingStageQueue{}
	processor := &fakeRefineProcessor{}
	handler := newRefineStageHandler(repo, processor, nextQueue)

	err := handler.Handle(context.Background(), VectorStageTask{TaskID: "task-1", VideoID: 9, RawKey: "raw/video.mp4", Stage: VectorStageRefine})
	if err == nil {
		t.Fatal("expected empty coarse error")
	}
	if processor.called {
		t.Fatal("processor should not be called")
	}
}

func TestRefineStageAllowsEmptyCoarseForShortVideo(t *testing.T) {
	repo := &refineRepo{}
	nextQueue := &recordingStageQueue{}
	processor := &fakeRefineProcessor{}
	handler := newRefineStageHandler(repo, processor, nextQueue)

	err := handler.Handle(context.Background(), VectorStageTask{
		TaskID:  "task-short",
		VideoID: 9,
		RawKey:  "raw/short.mp4",
		Stage:   VectorStageRefine,
		EndSec:  194,
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !processor.called {
		t.Fatal("expected processor to be called")
	}
	if processor.task.EndSec != 194 {
		t.Fatalf("processor EndSec = %d, want 194", processor.task.EndSec)
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStageRefine {
		t.Fatalf("refine not complete: %+v", repo.complete)
	}
	if len(nextQueue.enqueued) != 1 || nextQueue.enqueued[0].Stage != VectorStageFinalize {
		t.Fatalf("finalize not enqueued: %+v", nextQueue.enqueued)
	}
}
