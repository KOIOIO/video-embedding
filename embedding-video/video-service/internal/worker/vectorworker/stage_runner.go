package vectorworker

import (
	"context"
	"errors"
	"time"

	infraredis "nlp-video-analysis/internal/infrastructure/redis"

	goredis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

const maxVectorStageRetryCount = 3

type stageRetryDecision struct {
	Retry  bool
	Delay  time.Duration
	Reason string
}

type vectorStageHandler interface {
	Handle(context.Context, VectorStageTask) error
}

func decideStageRetry(task VectorStageTask, err error) stageRetryDecision {
	if err == nil {
		return stageRetryDecision{Retry: false, Reason: "success"}
	}
	if errors.Is(err, context.Canceled) {
		return stageRetryDecision{Retry: false, Reason: "context_canceled"}
	}
	if task.RetryCount >= maxVectorStageRetryCount {
		return stageRetryDecision{Retry: false, Reason: "max_retries_exceeded"}
	}
	delay := time.Duration(task.RetryCount+1) * 5 * time.Second
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return stageRetryDecision{Retry: true, Delay: delay, Reason: "retryable_error"}
}

func nextRetryTask(task VectorStageTask) VectorStageTask {
	task.RetryCount++
	return task
}

func runVectorStageWorker(ctx context.Context, stage string, queue *infraredis.StreamQueue[VectorStageTask], handler vectorStageHandler) error {
	for {
		msg, err := queue.Dequeue(ctx, time.Second)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, goredis.Nil) {
				continue
			}
			zap.L().Error("vector_stage_dequeue_failed", zap.String("stage", stage), zap.Error(err))
			continue
		}
		task := msg.Payload
		if task.Stage == "" {
			task.Stage = stage
		}
		if err := handler.Handle(ctx, task); err != nil {
			decision := decideStageRetry(task, err)
			if decision.Retry {
				retryMsg := infraredis.StreamMessage[VectorStageTask]{
					ID:      msg.ID,
					Payload: nextRetryTask(task),
				}
				if requeueErr := queue.Requeue(ctx, retryMsg, decision.Delay, decision.Reason); requeueErr != nil {
					zap.L().Error("vector_stage_requeue_failed", zap.String("stage", stage), zap.Error(requeueErr))
					return requeueErr
				}
				continue
			}
			if dlqErr := queue.MoveToDeadLetter(ctx, msg, err.Error()); dlqErr != nil {
				zap.L().Error("vector_stage_dlq_failed", zap.String("stage", stage), zap.Error(dlqErr))
				return dlqErr
			}
			continue
		}
		if err := queue.Ack(ctx, msg.ID); err != nil {
			zap.L().Error("vector_stage_ack_failed", zap.String("stage", stage), zap.Error(err))
			return err
		}
	}
}
