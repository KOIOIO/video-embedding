package videoapp

import (
	"context"
	"errors"
	"time"
)

const maxCommentLikeRetries = 3

// CommentLikeWorker 消费评论互动事件流，把 Redis 缓冲中的互动状态异步落到数据库。
type CommentLikeWorker struct {
	Queue CommentLikeQueue
	Repo  CommentReactionStateRepository
}

func NewCommentLikeWorker(queue CommentLikeQueue, repo CommentReactionStateRepository) *CommentLikeWorker {
	return &CommentLikeWorker{Queue: queue, Repo: repo}
}

func (w *CommentLikeWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.Queue == nil || w.Repo == nil {
		return errors.New("comment reaction worker dependencies are required")
	}
	msg, err := w.Queue.Dequeue(ctx)
	if err != nil {
		return err
	}
	found, err := w.Repo.ApplyCommentReactionState(ctx, msg.Event.CommentID, msg.Event.UserID, msg.Event.ReactionType, msg.Event.Active)
	if err == nil && found {
		return w.Queue.Ack(ctx, msg.MessageID)
	}
	if err == nil && !found {
		return w.Queue.MoveToDeadLetter(ctx, msg, "comment_not_found")
	}
	if msg.Event.RetryCount() >= maxCommentLikeRetries {
		return w.Queue.MoveToDeadLetter(ctx, msg, err.Error())
	}
	next := msg
	next.Event.Retry = msg.Event.Retry + 1
	return w.Queue.Requeue(ctx, next, time.Second*time.Duration(next.Event.Retry), err.Error())
}
