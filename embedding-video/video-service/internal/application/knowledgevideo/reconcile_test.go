package knowledgevideo

import (
	"context"
	"testing"
	"time"
)

func TestReconcilerEnqueuesStalePendingAndUpdatesEnqueueTime(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	repo := &reconcileTestRepository{videos: []Video{{ID: 8, SourceObjectKey: "raw/8/source.mp4", HLSObjectPrefix: "hls/8", Status: VideoPending}}}
	queue := &reconcileTestQueue{}
	reconciler := Reconciler{Repository: repo, Queue: queue, Now: func() time.Time { return now }}
	if err := reconciler.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := TranscodeTask{KnowledgeVideoID: 8, SourceObjectKey: "raw/8/source.mp4", HLSObjectPrefix: "hls/8", TaskID: "knowledge-video-8"}
	if len(queue.tasks) != 1 || queue.tasks[0] != want {
		t.Fatalf("tasks=%+v", queue.tasks)
	}
	if !repo.enqueueTime.Equal(now) || !repo.cutoff.Equal(now.Add(-5*time.Minute)) || repo.limit != 100 {
		t.Fatalf("repo=%+v", repo)
	}
}

type reconcileTestRepository struct {
	videos      []Video
	cutoff      time.Time
	limit       int
	enqueueTime time.Time
}

func (r *reconcileTestRepository) ListStalePending(_ context.Context, cutoff time.Time, limit int) ([]Video, error) {
	r.cutoff, r.limit = cutoff, limit
	return r.videos, nil
}
func (r *reconcileTestRepository) SetEnqueueTime(_ context.Context, _ uint64, value time.Time) (bool, error) {
	r.enqueueTime = value
	return true, nil
}

type reconcileTestQueue struct{ tasks []TranscodeTask }

func (q *reconcileTestQueue) Enqueue(_ context.Context, task TranscodeTask) error {
	q.tasks = append(q.tasks, task)
	return nil
}
