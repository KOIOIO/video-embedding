package redis

import (
	"context"
	"time"

	goredis "github.com/go-redis/redis/v8"

	"video-service/internal/application/knowledgevideo"
)

type KnowledgeVideoTranscodeQueue struct {
	*StreamQueue[knowledgevideo.TranscodeTask]
	block time.Duration
}

func NewKnowledgeVideoTranscodeQueue(rdb *goredis.Client, key string) *KnowledgeVideoTranscodeQueue {
	return &KnowledgeVideoTranscodeQueue{
		StreamQueue: NewStreamQueue[knowledgevideo.TranscodeTask](rdb, StreamQueueOptions{
			Key:      key,
			Group:    streamGroupName(key),
			Consumer: streamConsumerName("knowledge_video_transcode"),
		}),
		block: blockingQueuePollInterval,
	}
}

func (q *KnowledgeVideoTranscodeQueue) SetPendingMinIdle(minIdle time.Duration) {
	if minIdle >= 0 {
		q.pendingMinIdle = minIdle
	}
}

func (q *KnowledgeVideoTranscodeQueue) Dequeue(ctx context.Context) (knowledgevideo.QueueMessage, error) {
	message, err := q.StreamQueue.Dequeue(ctx, q.block)
	if err != nil {
		return knowledgevideo.QueueMessage{}, err
	}
	return knowledgevideo.QueueMessage{MessageID: message.ID, Task: message.Payload}, nil
}

func (q *KnowledgeVideoTranscodeQueue) Requeue(ctx context.Context, message knowledgevideo.QueueMessage, delay time.Duration, reason string) error {
	return q.StreamQueue.Requeue(ctx, StreamMessage[knowledgevideo.TranscodeTask]{ID: message.MessageID, Payload: message.Task}, delay, reason)
}

func (q *KnowledgeVideoTranscodeQueue) MoveToDeadLetter(ctx context.Context, message knowledgevideo.QueueMessage, reason string) error {
	return q.StreamQueue.MoveToDeadLetter(ctx, StreamMessage[knowledgevideo.TranscodeTask]{ID: message.MessageID, Payload: message.Task}, reason)
}

var _ knowledgevideo.TranscodeQueue = (*KnowledgeVideoTranscodeQueue)(nil)
