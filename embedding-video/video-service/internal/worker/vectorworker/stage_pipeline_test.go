package vectorworker

import (
	"context"
	"testing"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/infrastructure/persistence"
)

type recordingStageQueue struct {
	enqueued []VectorStageTask
}

func (q *recordingStageQueue) Enqueue(_ context.Context, task VectorStageTask) error {
	q.enqueued = append(q.enqueued, task)
	return nil
}

type recordingStageRepo struct {
	pending []persistence.VectorStageRecord
}

func (r *recordingStageRepo) UpsertPending(_ context.Context, rec persistence.VectorStageRecord) error {
	r.pending = append(r.pending, rec)
	return nil
}

func TestTopLevelVectorTaskEnqueuesPrepareForHierarchicalMode(t *testing.T) {
	repo := &recordingStageRepo{}
	queue := &recordingStageQueue{}
	adapter := newTopLevelVectorStageAdapter("hierarchical", repo, queue)

	err := adapter.Handle(context.Background(), videoapp.VectorizeTask{
		VideoID: 42,
		TaskID:  "task-1",
		RawKey:  "raw/video.mp4",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(repo.pending) != 1 {
		t.Fatalf("pending count = %d, want 1", len(repo.pending))
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued count = %d, want 1", len(queue.enqueued))
	}
	got := queue.enqueued[0]
	if got.Stage != VectorStagePrepare || got.VideoID != 42 || got.TaskID != "task-1" || got.RawKey != "raw/video.mp4" {
		t.Fatalf("unexpected prepare task: %+v", got)
	}
}

func TestTopLevelVectorTaskRejectsInvalidHierarchicalTask(t *testing.T) {
	repo := &recordingStageRepo{}
	queue := &recordingStageQueue{}
	adapter := newTopLevelVectorStageAdapter("hierarchical", repo, queue)

	if err := adapter.Handle(context.Background(), videoapp.VectorizeTask{}); err == nil {
		t.Fatal("expected invalid task error")
	}
	if len(repo.pending) != 0 || len(queue.enqueued) != 0 {
		t.Fatalf("invalid task should not be persisted or enqueued: pending=%+v enqueued=%+v", repo.pending, queue.enqueued)
	}
}
