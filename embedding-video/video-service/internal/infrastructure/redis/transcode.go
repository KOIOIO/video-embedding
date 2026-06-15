package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	goredis "github.com/go-redis/redis/v8"

	"nlp-video-analysis/internal/application/videoapp"
	domainvideo "nlp-video-analysis/internal/domain/video"
)

const (
	maxRedisRetries = 3
	retryDelayBase  = time.Millisecond * 500
)

// withRetry 为 Redis 短暂性错误提供简单重试包装。
func withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for retry := 0; retry < maxRedisRetries; retry++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fn(); err != nil {
			lastErr = err
			if retry < maxRedisRetries-1 {
				select {
				case <-time.After(retryDelayBase * time.Duration(retry+1)):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		} else {
			return nil
		}
	}
	return lastErr
}

// TranscodeQueue 基于 Redis Streams 的转码任务队列实现。
type TranscodeQueue struct {
	rdb      *goredis.Client
	key      string
	group    string
	consumer string
	once     sync.Once
	onceErr  error
}

// NewTranscodeQueue 创建转码任务队列。
func NewTranscodeQueue(rdb *goredis.Client, key string) *TranscodeQueue {
	return &TranscodeQueue{
		rdb:      rdb,
		key:      key,
		group:    streamGroupName(key),
		consumer: streamConsumerName("transcode"),
	}
}

// Enqueue 向 Redis Stream 写入一条转码任务消息。
func (q *TranscodeQueue) Enqueue(ctx context.Context, task videoapp.TranscodeTask) error {
	if err := q.ensureGroup(ctx); err != nil {
		return err
	}
	b, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return withRetry(ctx, func() error {
		_, err := q.rdb.XAdd(ctx, &goredis.XAddArgs{
			Stream: q.key,
			Values: map[string]interface{}{
				"payload": string(b),
			},
		}).Result()
		return err
	})
}

// Dequeue 从消费者组中取出一条转码任务，成功解析后交由调用方决定何时 ACK。
func (q *TranscodeQueue) Dequeue(ctx context.Context) (videoapp.TranscodeQueueMessage, error) {
	if err := q.ensureGroup(ctx); err != nil {
		return videoapp.TranscodeQueueMessage{}, err
	}
	streams, err := q.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumer,
		Streams:  []string{q.key, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	if err != nil {
		return videoapp.TranscodeQueueMessage{}, err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return videoapp.TranscodeQueueMessage{}, errors.New("empty stream message")
	}
	msg := streams[0].Messages[0]
	task, err := q.decodeTranscodeTask(ctx, msg)
	if err != nil {
		return videoapp.TranscodeQueueMessage{}, err
	}
	return videoapp.TranscodeQueueMessage{MessageID: msg.ID, Task: task}, nil
}

// Ack 在消息成功处理后执行 ACK，并尽力从流中删除该消息。
func (q *TranscodeQueue) Ack(ctx context.Context, id string) error {
	return q.ackAndDelete(ctx, id)
}

// Requeue 重新投递一条消息，并在成功入队后 ACK 原消息。
func (q *TranscodeQueue) Requeue(ctx context.Context, msg videoapp.TranscodeQueueMessage, delay time.Duration, reason string) error {
	if err := q.ensureGroup(ctx); err != nil {
		return err
	}
	b, err := json.Marshal(msg.Task)
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
	return q.ackAndDelete(ctx, msg.MessageID)
}

// MoveToDeadLetter 把终态失败消息写入死信流，并在成功后 ACK 原消息。
func (q *TranscodeQueue) MoveToDeadLetter(ctx context.Context, msg videoapp.TranscodeQueueMessage, reason string) error {
	b, err := json.Marshal(msg.Task)
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
	return q.ackAndDelete(ctx, msg.MessageID)
}

// VectorizeQueue 基于 Redis Streams 的向量化任务队列实现。
type VectorizeQueue struct {
	rdb      *goredis.Client
	key      string
	group    string
	consumer string
	once     sync.Once
	onceErr  error
}

// NewVectorizeQueue 创建向量化任务队列。
func NewVectorizeQueue(rdb *goredis.Client, key string) *VectorizeQueue {
	return &VectorizeQueue{
		rdb:      rdb,
		key:      key,
		group:    streamGroupName(key),
		consumer: streamConsumerName("vectorize"),
	}
}

// Enqueue 向 Redis Stream 写入一条向量化任务消息。
func (q *VectorizeQueue) Enqueue(ctx context.Context, task videoapp.VectorizeTask) error {
	if err := q.ensureGroup(ctx); err != nil {
		return err
	}
	b, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return withRetry(ctx, func() error {
		_, err := q.rdb.XAdd(ctx, &goredis.XAddArgs{
			Stream: q.key,
			Values: map[string]interface{}{
				"payload": string(b),
			},
		}).Result()
		return err
	})
}

// Dequeue 从消费者组中取出一条向量化任务，成功解析后交由调用方决定何时 ACK。
func (q *VectorizeQueue) Dequeue(ctx context.Context) (videoapp.VectorizeQueueMessage, error) {
	if err := q.ensureGroup(ctx); err != nil {
		return videoapp.VectorizeQueueMessage{}, err
	}
	streams, err := q.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumer,
		Streams:  []string{q.key, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	if err != nil {
		return videoapp.VectorizeQueueMessage{}, err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return videoapp.VectorizeQueueMessage{}, errors.New("empty stream message")
	}
	msg := streams[0].Messages[0]
	payload, _ := msg.Values["payload"].(string)
	if payload == "" {
		_ = q.ackAndDelete(ctx, msg.ID)
		return videoapp.VectorizeQueueMessage{}, errors.New("stream payload missing")
	}

	var task videoapp.VectorizeTask
	if err := json.Unmarshal([]byte(payload), &task); err != nil {
		_ = q.ackAndDelete(ctx, msg.ID)
		return videoapp.VectorizeQueueMessage{}, err
	}
	return videoapp.VectorizeQueueMessage{MessageID: msg.ID, Task: task}, nil
}

// Ack 在消息成功处理后执行 ACK，并尽力从流中删除该消息。
func (q *VectorizeQueue) Ack(ctx context.Context, id string) error {
	return q.ackAndDelete(ctx, id)
}

// Requeue 重新投递一条消息，并在成功入队后 ACK 原消息。
func (q *VectorizeQueue) Requeue(ctx context.Context, msg videoapp.VectorizeQueueMessage, delay time.Duration, reason string) error {
	if err := q.ensureGroup(ctx); err != nil {
		return err
	}
	b, err := json.Marshal(msg.Task)
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
	return q.ackAndDelete(ctx, msg.MessageID)
}

// MoveToDeadLetter 把终态失败消息写入死信流，并在成功后 ACK 原消息。
func (q *VectorizeQueue) MoveToDeadLetter(ctx context.Context, msg videoapp.VectorizeQueueMessage, reason string) error {
	b, err := json.Marshal(msg.Task)
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
	return q.ackAndDelete(ctx, msg.MessageID)
}

func (q *TranscodeQueue) decodeTranscodeTask(ctx context.Context, msg goredis.XMessage) (videoapp.TranscodeTask, error) {
	payload, _ := msg.Values["payload"].(string)
	if payload == "" {
		_ = q.ackAndDelete(ctx, msg.ID)
		return videoapp.TranscodeTask{}, errors.New("stream payload missing")
	}
	var task videoapp.TranscodeTask
	if err := json.Unmarshal([]byte(payload), &task); err != nil {
		_ = q.ackAndDelete(ctx, msg.ID)
		return videoapp.TranscodeTask{}, err
	}
	return task, nil
}

// ensureGroup 确保转码队列对应的消费者组只被创建一次。
func (q *TranscodeQueue) ensureGroup(ctx context.Context) error {
	q.once.Do(func() {
		_, err := q.rdb.XGroupCreateMkStream(ctx, q.key, q.group, "$").Result()
		if err != nil && !isBusyGroup(err) {
			q.onceErr = err
		}
	})
	return q.onceErr
}

// ensureGroup 确保向量化队列对应的消费者组只被创建一次。
func (q *VectorizeQueue) ensureGroup(ctx context.Context) error {
	q.once.Do(func() {
		_, err := q.rdb.XGroupCreateMkStream(ctx, q.key, q.group, "$").Result()
		if err != nil && !isBusyGroup(err) {
			q.onceErr = err
		}
	})
	return q.onceErr
}

// ackAndDelete 在消息成功处理后执行 ACK，并尽力从流中删除该消息。
func (q *TranscodeQueue) ackAndDelete(ctx context.Context, id string) error {
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

// ackAndDelete 在消息成功处理后执行 ACK，并尽力从流中删除该消息。
func (q *VectorizeQueue) ackAndDelete(ctx context.Context, id string) error {
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

// streamGroupName 根据 stream key 生成稳定的消费者组名。
func streamGroupName(key string) string {
	k := strings.ReplaceAll(strings.TrimSpace(key), ":", "_")
	if k == "" {
		k = "queue"
	}
	return fmt.Sprintf("%s_group", k)
}

func isBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}

// streamConsumerName 生成带主机名与进程信息的消费者名，便于多实例并发消费。
func streamConsumerName(prefix string) string {
	host, _ := os.Hostname()
	if strings.TrimSpace(host) == "" {
		host = "worker"
	}
	rd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%s_%s_%d_%d", prefix, host, os.Getpid(), rd.Intn(100000))
}

// TranscodeStatusStore 基于 Redis KV 保存转码状态。
type TranscodeStatusStore struct {
	rdb    *goredis.Client
	prefix string
}

// NewTranscodeStatusStore 创建转码状态存储。
func NewTranscodeStatusStore(rdb *goredis.Client, prefix string) *TranscodeStatusStore {
	return &TranscodeStatusStore{
		rdb:    rdb,
		prefix: prefix,
	}
}

// Set 写入任务状态及对应的 HLS URL。
func (s *TranscodeStatusStore) Set(ctx context.Context, taskID string, status domainvideo.Status, hlsURL string, ttl time.Duration) error {
	info := videoapp.TranscodeStatus{
		Status: status,
		HLSURL: hlsURL,
	}
	b, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, s.prefix+taskID, b, ttl).Err()
}

// Get 读取任务状态，不存在时返回 ok=false。
func (s *TranscodeStatusStore) Get(ctx context.Context, taskID string) (videoapp.TranscodeStatus, bool, error) {
	val, err := s.rdb.Get(ctx, s.prefix+taskID).Result()
	if err == goredis.Nil {
		return videoapp.TranscodeStatus{}, false, nil
	}
	if err != nil {
		return videoapp.TranscodeStatus{}, false, err
	}

	var info videoapp.TranscodeStatus
	if err := json.Unmarshal([]byte(val), &info); err != nil {
		return videoapp.TranscodeStatus{}, false, err
	}
	return info, true, nil
}
