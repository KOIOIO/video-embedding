package vectorworker

import (
	"context"
	"testing"

	"nlp-video-analysis/internal/infrastructure/persistence"
)

type prepareRepo struct {
	foundVideo bool
	pending    []persistence.VectorStageRecord
	complete   []persistence.VectorStageRecord
}

func (r *prepareRepo) VideoExists(_ context.Context, _ uint64) (bool, error) {
	return r.foundVideo, nil
}

func (r *prepareRepo) HasExistingSegments(_ context.Context, _ uint64) (bool, error) {
	return false, nil
}

func (r *prepareRepo) UpsertPending(_ context.Context, rec persistence.VectorStageRecord) error {
	r.pending = append(r.pending, rec)
	return nil
}

func (r *prepareRepo) MarkComplete(_ context.Context, rec persistence.VectorStageRecord) error {
	r.complete = append(r.complete, rec)
	return nil
}

type prepareProbe struct {
	duration int
}

func (p prepareProbe) Probe(_ context.Context, _ string) (int, error) {
	return p.duration, nil
}

func TestPrepareStageCreatesCoarsePlanAndEnqueuesCoarse(t *testing.T) {
	repo := &prepareRepo{foundVideo: true}
	queue := &recordingStageQueue{}
	handler := newPrepareStageHandler(repo, prepareProbe{duration: 95}, queue, 40)

	err := handler.Handle(context.Background(), VectorStageTask{
		TaskID:  "task-1",
		VideoID: 7,
		RawKey:  "raw/video.mp4",
		Stage:   VectorStagePrepare,
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(repo.pending) != 3 {
		t.Fatalf("coarse segment plan rows = %d, want 3", len(repo.pending))
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Stage != VectorStageCoarse {
		t.Fatalf("coarse task not enqueued: %+v", queue.enqueued)
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStagePrepare {
		t.Fatalf("prepare not complete: %+v", repo.complete)
	}
}

func TestPrepareStageShortVideoSkipsCoarseAndEnqueuesRefine(t *testing.T) {
	repo := &prepareRepo{foundVideo: true}
	coarseQueue := &recordingStageQueue{}
	refineQueue := &recordingStageQueue{}
	handler := newPrepareStageHandlerWithRefine(repo, prepareProbe{duration: 194}, coarseQueue, refineQueue, 40)

	err := handler.Handle(context.Background(), VectorStageTask{
		TaskID:  "task-short",
		VideoID: 7,
		RawKey:  "raw/short.mp4",
		Stage:   VectorStagePrepare,
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(repo.pending) != 0 {
		t.Fatalf("coarse segment plan rows = %d, want 0", len(repo.pending))
	}
	if len(coarseQueue.enqueued) != 0 {
		t.Fatalf("coarse task enqueued for short video: %+v", coarseQueue.enqueued)
	}
	if len(refineQueue.enqueued) != 1 || refineQueue.enqueued[0].Stage != VectorStageRefine {
		t.Fatalf("refine task not enqueued: %+v", refineQueue.enqueued)
	}
	if refineQueue.enqueued[0].EndSec != 194 {
		t.Fatalf("refine EndSec = %d, want 194", refineQueue.enqueued[0].EndSec)
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStagePrepare {
		t.Fatalf("prepare not complete: %+v", repo.complete)
	}
}
