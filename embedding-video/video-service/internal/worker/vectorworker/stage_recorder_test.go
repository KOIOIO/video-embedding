package vectorworker

import (
	"context"
	"errors"
	"testing"

	"nlp-video-analysis/internal/infrastructure/persistence"
)

type recordingStageRepository struct {
	pending  []persistence.VectorStageRecord
	complete []persistence.VectorStageRecord
	failed   []persistence.VectorStageRecord
}

func (r *recordingStageRepository) UpsertPending(_ context.Context, rec persistence.VectorStageRecord) error {
	r.pending = append(r.pending, rec)
	return nil
}

func (r *recordingStageRepository) MarkComplete(_ context.Context, rec persistence.VectorStageRecord) error {
	rec.Status = persistence.VectorStageStatusComplete
	r.complete = append(r.complete, rec)
	return nil
}

func (r *recordingStageRepository) MarkFailed(_ context.Context, rec persistence.VectorStageRecord) error {
	rec.Status = persistence.VectorStageStatusFailed
	r.failed = append(r.failed, rec)
	return nil
}

func TestVectorStageRecorderRecordsCompleteStage(t *testing.T) {
	repo := &recordingStageRepository{}
	recorder := newVectorStageRecorder(repo)

	recorder.Complete(context.Background(), VectorStageTask{
		TaskID:       "task-1",
		VideoID:      42,
		Stage:        VectorStageCoarseASR,
		SegmentIndex: 3,
		SegmentID:    99,
		ObjectKey:    "segments/coarse/video_42/task-1/seg_003.mp4",
		StartSec:     120,
		EndSec:       160,
		RetryCount:   2,
	}, "讲解函数单调性")

	if len(repo.pending) != 1 {
		t.Fatalf("pending records = %d, want 1", len(repo.pending))
	}
	if len(repo.complete) != 1 {
		t.Fatalf("complete records = %d, want 1", len(repo.complete))
	}
	got := repo.complete[0]
	if got.TaskID != "task-1" || got.VideoID != 42 || got.Stage != VectorStageCoarseASR {
		t.Fatalf("unexpected record identity: %+v", got)
	}
	if got.SegmentIndex != 3 || got.SegmentID != 99 || got.RetryCount != 2 {
		t.Fatalf("unexpected segment metadata: %+v", got)
	}
	if got.ObjectKey != "segments/coarse/video_42/task-1/seg_003.mp4" || got.Text != "讲解函数单调性" {
		t.Fatalf("unexpected payload: %+v", got)
	}
	if got.StartSec != 120 || got.EndSec != 160 || got.Status != persistence.VectorStageStatusComplete {
		t.Fatalf("unexpected status/timing: %+v", got)
	}
}

func TestVectorStageRecorderRecordsFailureWithoutReturningError(t *testing.T) {
	repo := &recordingStageRepository{}
	recorder := newVectorStageRecorder(repo)

	recorder.Fail(context.Background(), VectorStageTask{
		TaskID:       "task-1",
		VideoID:      42,
		Stage:        VectorStageRefineASR,
		SegmentIndex: 4,
		SegmentID:    100,
		StartSec:     160,
		EndSec:       200,
	}, errors.New("asr failed"))

	if len(repo.pending) != 1 {
		t.Fatalf("pending records = %d, want 1", len(repo.pending))
	}
	if len(repo.failed) != 1 {
		t.Fatalf("failed records = %d, want 1", len(repo.failed))
	}
	got := repo.failed[0]
	if got.Status != persistence.VectorStageStatusFailed || got.ErrorMessage != "asr failed" {
		t.Fatalf("unexpected failed record: %+v", got)
	}
}
