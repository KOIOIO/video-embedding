package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

type StreamQueueOptions struct {
	Key      string
	Group    string
	Consumer string
}

type StreamMessage[T any] struct {
	ID      string
	Payload T
}

type StreamQueue[T any] struct {
	rdb      *goredis.Client
	key      string
	group    string
	consumer string
}

func NewStreamQueue[T any](rdb *goredis.Client, opts StreamQueueOptions) *StreamQueue[T] {
	group := opts.Group
	if group == "" {
		group = streamGroupName(opts.Key)
	}
	consumer := opts.Consumer
	if consumer == "" {
		consumer = streamConsumerName("stream")
	}
	return &StreamQueue[T]{
		rdb:      rdb,
		key:      opts.Key,
		group:    group,
		consumer: consumer,
	}
}

func (q *StreamQueue[T]) Enqueue(ctx context.Context, payload T) error {
	if err := q.ensureGroup(ctx); err != nil {
		return err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return withRetry(ctx, func() error {
		_, err := q.rdb.XAdd(ctx, &goredis.XAddArgs{
			Stream: q.key,
			Values: map[string]interface{}{"payload": string(b)},
		}).Result()
		return err
	})
}

func (q *StreamQueue[T]) Dequeue(ctx context.Context, block time.Duration) (StreamMessage[T], error) {
	if err := q.ensureGroup(ctx); err != nil {
		return StreamMessage[T]{}, err
	}
	streams, err := q.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumer,
		Streams:  []string{q.key, ">"},
		Count:    1,
		Block:    block,
	}).Result()
	if err != nil {
		return StreamMessage[T]{}, err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return StreamMessage[T]{}, errors.New("empty stream message")
	}
	raw := streams[0].Messages[0]
	payload, _ := raw.Values["payload"].(string)
	if payload == "" {
		_ = q.ackAndDelete(ctx, raw.ID)
		return StreamMessage[T]{}, errors.New("stream payload missing")
	}
	var decoded T
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		_ = q.ackAndDelete(ctx, raw.ID)
		return StreamMessage[T]{}, err
	}
	return StreamMessage[T]{ID: raw.ID, Payload: decoded}, nil
}

func (q *StreamQueue[T]) Ack(ctx context.Context, id string) error {
	return q.ackAndDelete(ctx, id)
}

func (q *StreamQueue[T]) Requeue(ctx context.Context, msg StreamMessage[T], delay time.Duration, reason string) error {
	if err := q.ensureGroup(ctx); err != nil {
		return err
	}
	b, err := json.Marshal(msg.Payload)
	if err != nil {
		return err
	}
	values := map[string]interface{}{
		"payload":      string(b),
		"retry_reason": reason,
	}
	if delay > 0 {
		values["visible_at"] = time.Now().Add(delay).Unix()
	}
	if err := withRetry(ctx, func() error {
		_, err := q.rdb.XAdd(ctx, &goredis.XAddArgs{Stream: q.key, Values: values}).Result()
		return err
	}); err != nil {
		return err
	}
	return q.ackAndDelete(ctx, msg.ID)
}

func (q *StreamQueue[T]) MoveToDeadLetter(ctx context.Context, msg StreamMessage[T], reason string) error {
	b, err := json.Marshal(msg.Payload)
	if err != nil {
		return err
	}
	if err := withRetry(ctx, func() error {
		_, err := q.rdb.XAdd(ctx, &goredis.XAddArgs{
			Stream: q.key + ":dlq",
			Values: map[string]interface{}{
				"payload": string(b),
				"reason":  reason,
			},
		}).Result()
		return err
	}); err != nil {
		return err
	}
	return q.ackAndDelete(ctx, msg.ID)
}

func (q *StreamQueue[T]) ensureGroup(ctx context.Context) error {
	_, err := q.rdb.XGroupCreateMkStream(ctx, q.key, q.group, "$").Result()
	if err != nil && !isBusyGroup(err) {
		return err
	}
	return nil
}

func (q *StreamQueue[T]) ackAndDelete(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	if err := withRetry(ctx, func() error {
		return q.rdb.XAck(ctx, q.key, q.group, id).Err()
	}); err != nil {
		return err
	}
	_ = q.rdb.XDel(ctx, q.key, id).Err()
	return nil
}
