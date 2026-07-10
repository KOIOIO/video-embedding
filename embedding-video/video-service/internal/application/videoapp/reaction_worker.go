package videoapp

import (
	"context"
	"errors"
	"time"
)

const maxVideoReactionRetries = 3

type VideoReactionWorker struct {
	Queue VideoReactionQueue
	Repo  VideoReactionStateRepository
}

type SegmentReactionWorker struct {
	Queue VideoReactionQueue
	Repo  SegmentReactionStateRepository
}

func NewVideoReactionWorker(queue VideoReactionQueue, repo VideoReactionStateRepository) *VideoReactionWorker {
	return &VideoReactionWorker{Queue: queue, Repo: repo}
}

func NewSegmentReactionWorker(queue VideoReactionQueue, repo SegmentReactionStateRepository) *SegmentReactionWorker {
	return &SegmentReactionWorker{Queue: queue, Repo: repo}
}

func (w *VideoReactionWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.Queue == nil || w.Repo == nil {
		return errors.New("video reaction worker dependencies are required")
	}
	msg, err := w.Queue.Dequeue(ctx)
	if err != nil {
		return err
	}
	found, err := w.Repo.ApplyVideoReactionState(ctx, msg.Event.VideoID, msg.Event.UserID, msg.Event.ReactionType, msg.Event.Active)
	if err == nil && found {
		return w.Queue.Ack(ctx, msg.MessageID)
	}
	if err == nil && !found {
		return w.Queue.MoveToDeadLetter(ctx, msg, "video_not_found")
	}
	if msg.Event.RetryCount() >= maxVideoReactionRetries {
		return w.Queue.MoveToDeadLetter(ctx, msg, err.Error())
	}
	next := msg
	next.Event.Retry = msg.Event.Retry + 1
	return w.Queue.Requeue(ctx, next, time.Second*time.Duration(next.Event.Retry), err.Error())
}

func (w *SegmentReactionWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.Queue == nil || w.Repo == nil {
		return errors.New("segment reaction worker dependencies are required")
	}
	msg, err := w.Queue.Dequeue(ctx)
	if err != nil {
		return err
	}
	found, err := w.Repo.ApplySegmentReactionState(ctx, msg.Event.VideoID, msg.Event.UserID, msg.Event.ReactionType, msg.Event.Active)
	if err == nil && found {
		return w.Queue.Ack(ctx, msg.MessageID)
	}
	if err == nil && !found {
		return w.Queue.MoveToDeadLetter(ctx, msg, "segment_not_found")
	}
	if msg.Event.RetryCount() >= maxVideoReactionRetries {
		return w.Queue.MoveToDeadLetter(ctx, msg, err.Error())
	}
	next := msg
	next.Event.Retry = msg.Event.Retry + 1
	return w.Queue.Requeue(ctx, next, time.Second*time.Duration(next.Event.Retry), err.Error())
}

func (e VideoReactionEvent) RetryCount() int {
	if e.Retry < 0 {
		return 0
	}
	return e.Retry
}
