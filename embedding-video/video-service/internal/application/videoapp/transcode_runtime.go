package videoapp

import (
	"context"
	"time"

	workerapp "nlp-video-analysis/internal/application/videoapp/worker"
)

const maxRetryAttempts = workerapp.MaxRetryAttempts

type TranscodeQueueMessage struct {
	MessageID string
	Task      TranscodeTask
}

type TranscodeTaskConsumer interface {
	Dequeue(ctx context.Context) (TranscodeQueueMessage, error)
	Ack(ctx context.Context, messageID string) error
	Requeue(ctx context.Context, msg TranscodeQueueMessage, delay time.Duration, reason string) error
	MoveToDeadLetter(ctx context.Context, msg TranscodeQueueMessage, reason string) error
}

type RetryDecision = workerapp.RetryDecision

type RetryPolicy func(err error, retries int) RetryDecision

type Lease = workerapp.Lease

type LeaseStore interface {
	Acquire(ctx context.Context, lease Lease, ttl time.Duration) error
	Renew(ctx context.Context, lease Lease, ttl time.Duration) error
	Release(ctx context.Context, taskID string) error
}

func DefaultRetryPolicy(err error, retries int) RetryDecision {
	return workerapp.DefaultRetryPolicy(err, retries)
}
