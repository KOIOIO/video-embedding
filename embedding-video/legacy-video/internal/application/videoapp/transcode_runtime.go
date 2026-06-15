package videoapp

import (
	"context"
	"errors"
	"strings"
	"time"
)

const maxRetryAttempts = 5

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

type RetryDecision struct {
	Retry  bool
	Delay  time.Duration
	Reason string
}

type RetryPolicy func(err error, retries int) RetryDecision

type Lease struct {
	TaskID    string
	MessageID string
	WorkerID  string
	Stage     string
	ExpiresAt time.Time
}

type LeaseStore interface {
	Acquire(ctx context.Context, lease Lease, ttl time.Duration) error
	Renew(ctx context.Context, lease Lease, ttl time.Duration) error
	Release(ctx context.Context, taskID string) error
}

func DefaultRetryPolicy(err error, retries int) RetryDecision {
	if err == nil {
		return RetryDecision{}
	}
	if retries >= maxRetryAttempts {
		return RetryDecision{Reason: "retry_exhausted"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * time.Minute, Reason: "timeout"}
	}
	if isTemporaryStorageError(err) {
		return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * 30 * time.Second, Reason: "temporary_storage_error"}
	}
	return RetryDecision{Reason: "terminal"}
}

func isTemporaryStorageError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, token := range []string{"timeout", "connection reset", "tempor", "unavailable", "eof"} {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}
