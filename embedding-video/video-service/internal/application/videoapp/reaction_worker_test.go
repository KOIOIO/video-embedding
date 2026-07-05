package videoapp

import (
	"context"
	"testing"
	"time"
)

func TestVideoReactionWorkerRebuildsProfileAfterPersistingEvent(t *testing.T) {
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
	updater := &reactionWorkerProfileUpdater{}
	worker := NewVideoReactionWorker(queue, repo)
	worker.ProfileUpdater = updater

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if repo.videoID != 11 || repo.userID != 7 || repo.reactionType != VideoReactionLike || !repo.active {
		t.Fatalf("repo call = %+v", repo)
	}
	if updater.calls != 1 || updater.lastUserID != 7 {
		t.Fatalf("profile updater calls=%d userID=%d, want one call for user 7", updater.calls, updater.lastUserID)
	}
	if updater.towerCalls != 1 || updater.lastTowerUserID != 7 || updater.lastTowerVersion != "two_tower_v2" {
		t.Fatalf("tower updater calls=%d userID=%d version=%q, want one call for user 7 version two_tower_v2", updater.towerCalls, updater.lastTowerUserID, updater.lastTowerVersion)
	}
	if queue.ackedID != "1-0" {
		t.Fatalf("ackedID = %q, want 1-0", queue.ackedID)
	}
}

func TestSegmentReactionWorkerRebuildsProfileAfterPersistingEvent(t *testing.T) {
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
	updater := &reactionWorkerProfileUpdater{}
	worker := NewSegmentReactionWorker(queue, repo)
	worker.ProfileUpdater = updater

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if repo.segmentID != 22 || repo.userID != 8 || repo.reactionType != VideoReactionDoubleLike || !repo.active {
		t.Fatalf("repo call = %+v", repo)
	}
	if updater.calls != 1 || updater.lastUserID != 8 {
		t.Fatalf("profile updater calls=%d userID=%d, want one call for user 8", updater.calls, updater.lastUserID)
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

type reactionWorkerProfileUpdater struct {
	calls            int
	lastUserID       uint64
	towerCalls       int
	lastTowerUserID  uint64
	lastTowerVersion string
}

func (u *reactionWorkerProfileUpdater) RebuildUserVideoProfile(_ context.Context, userID uint64, _ string, _ time.Time) error {
	u.calls++
	u.lastUserID = userID
	return nil
}

func (*reactionWorkerProfileUpdater) GetActiveTwoTowerModelVersion(context.Context) (string, bool, error) {
	return "two_tower_v2", true, nil
}

func (u *reactionWorkerProfileUpdater) RebuildUserTowerEmbedding(_ context.Context, userID uint64, modelVersion string, _ time.Time) error {
	u.towerCalls++
	u.lastTowerUserID = userID
	u.lastTowerVersion = modelVersion
	return nil
}
