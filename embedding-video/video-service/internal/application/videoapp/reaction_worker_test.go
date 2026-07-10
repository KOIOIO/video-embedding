package videoapp

import (
	"context"
	"testing"
	"time"
)

func TestVideoReactionWorkerDoesNotRebuildProfileAfterPersistingEvent(t *testing.T) {
	queue := &reactionWorkerTestQueue{
		msg: VideoReactionQueueMessage{
			MessageID: "1-0",
			Event: VideoReactionEvent{
				VideoID:      11,
				UserID:       7,
				ReactionType: VideoReactionLike,
				Active:       true,
			},
		},
	}
	repo := &reactionWorkerVideoRepo{found: true}
	worker := NewVideoReactionWorker(queue, repo)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if repo.videoID != 11 || repo.userID != 7 || repo.reactionType != VideoReactionLike || !repo.active {
		t.Fatalf("repo call = %+v", repo)
	}
	if queue.ackedID != "1-0" {
		t.Fatalf("ackedID = %q, want 1-0", queue.ackedID)
	}
}

func TestSegmentReactionWorkerDoesNotRebuildProfileAfterPersistingEvent(t *testing.T) {
	queue := &reactionWorkerTestQueue{
		msg: VideoReactionQueueMessage{
			MessageID: "2-0",
			Event: VideoReactionEvent{
				VideoID:      22,
				UserID:       8,
				ReactionType: VideoReactionDoubleLike,
				Active:       true,
			},
		},
	}
	repo := &reactionWorkerSegmentRepo{found: true}
	worker := NewSegmentReactionWorker(queue, repo)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if repo.segmentID != 22 || repo.userID != 8 || repo.reactionType != VideoReactionDoubleLike || !repo.active {
		t.Fatalf("repo call = %+v", repo)
	}
	if queue.ackedID != "2-0" {
		t.Fatalf("ackedID = %q, want 2-0", queue.ackedID)
	}
}

type reactionWorkerTestQueue struct {
	msg     VideoReactionQueueMessage
	ackedID string
}

func (q *reactionWorkerTestQueue) Enqueue(context.Context, VideoReactionEvent) error {
	panic("unexpected call")
}

func (q *reactionWorkerTestQueue) Dequeue(context.Context) (VideoReactionQueueMessage, error) {
	return q.msg, nil
}

func (q *reactionWorkerTestQueue) Ack(_ context.Context, id string) error {
	q.ackedID = id
	return nil
}

func (*reactionWorkerTestQueue) Requeue(context.Context, VideoReactionQueueMessage, time.Duration, string) error {
	panic("unexpected call")
}

func (*reactionWorkerTestQueue) MoveToDeadLetter(context.Context, VideoReactionQueueMessage, string) error {
	panic("unexpected call")
}

type reactionWorkerVideoRepo struct {
	found        bool
	videoID      uint64
	userID       uint64
	reactionType VideoReactionType
	active       bool
}

func (r *reactionWorkerVideoRepo) ApplyVideoReactionState(_ context.Context, videoID uint64, userID uint64, reactionType VideoReactionType, active bool) (bool, error) {
	r.videoID = videoID
	r.userID = userID
	r.reactionType = reactionType
	r.active = active
	return r.found, nil
}

type reactionWorkerSegmentRepo struct {
	found        bool
	segmentID    uint64
	userID       uint64
	reactionType VideoReactionType
	active       bool
}

func (r *reactionWorkerSegmentRepo) ApplySegmentReactionState(_ context.Context, segmentID uint64, userID uint64, reactionType VideoReactionType, active bool) (bool, error) {
	r.segmentID = segmentID
	r.userID = userID
	r.reactionType = reactionType
	r.active = active
	return r.found, nil
}
