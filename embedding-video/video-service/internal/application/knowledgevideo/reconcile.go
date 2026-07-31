package knowledgevideo

import (
	"context"
	"fmt"
	"time"
)

const (
	reconcilePageSize = 100
	reconcileStaleAge = 5 * time.Minute
)

type ReconcileRepository interface {
	ListStalePending(ctx context.Context, cutoff time.Time, limit int) ([]Video, error)
	SetEnqueueTime(ctx context.Context, id uint64, enqueueTime time.Time) (bool, error)
}

type ReconcileQueue interface {
	Enqueue(ctx context.Context, task TranscodeTask) error
}

type Reconciler struct {
	Repository ReconcileRepository
	Queue      ReconcileQueue
	Now        func() time.Time
}

func (r Reconciler) RunOnce(ctx context.Context) error {
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	videos, err := r.Repository.ListStalePending(ctx, now.Add(-reconcileStaleAge), reconcilePageSize)
	if err != nil {
		return err
	}
	for _, video := range videos {
		task := TranscodeTask{
			KnowledgeVideoID: video.ID,
			SourceObjectKey:  video.SourceObjectKey,
			HLSObjectPrefix:  video.HLSObjectPrefix,
			TaskID:           fmt.Sprintf("knowledge-video-%d", video.ID),
		}
		if err := r.Queue.Enqueue(ctx, task); err != nil {
			return err
		}
		if _, err := r.Repository.SetEnqueueTime(ctx, video.ID, now); err != nil {
			return err
		}
	}
	return nil
}
