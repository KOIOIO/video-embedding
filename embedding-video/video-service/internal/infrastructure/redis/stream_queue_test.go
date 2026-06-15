package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"nlp-video-analysis/internal/application/videoapp"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
)

type streamQueueTestPayload struct {
	ID string `json:"id"`
}

func newTestStreamQueue(t *testing.T) (*StreamQueue[streamQueueTestPayload], *goredis.Client, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	q := NewStreamQueue[streamQueueTestPayload](rdb, StreamQueueOptions{
		Key:      "test:stream",
		Group:    "test_group",
		Consumer: "test_consumer",
	})
	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}
	return q, rdb, cleanup
}

func TestStreamQueueDequeueDoesNotAck(t *testing.T) {
	ctx := context.Background()
	q, rdb, cleanup := newTestStreamQueue(t)
	defer cleanup()

	if err := q.Enqueue(ctx, streamQueueTestPayload{ID: "1"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	msg, err := q.Dequeue(ctx, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if msg.ID == "" || msg.Payload.ID != "1" {
		t.Fatalf("unexpected message: %+v", msg)
	}

	pending, err := rdb.XPending(ctx, "test:stream", "test_group").Result()
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if pending.Count != 1 {
		t.Fatalf("pending count = %d, want 1", pending.Count)
	}
}

func TestStreamQueueAckDeletesMessage(t *testing.T) {
	ctx := context.Background()
	q, rdb, cleanup := newTestStreamQueue(t)
	defer cleanup()

	if err := q.Enqueue(ctx, streamQueueTestPayload{ID: "1"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	msg, err := q.Dequeue(ctx, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if err := q.Ack(ctx, msg.ID); err != nil {
		t.Fatalf("ack: %v", err)
	}

	pending, err := rdb.XPending(ctx, "test:stream", "test_group").Result()
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("pending count = %d, want 0", pending.Count)
	}
	if got := rdb.XLen(ctx, "test:stream").Val(); got != 0 {
		t.Fatalf("stream len = %d, want 0", got)
	}
}

func TestStreamQueueRequeueAddsNewMessageAndAcksOld(t *testing.T) {
	ctx := context.Background()
	q, rdb, cleanup := newTestStreamQueue(t)
	defer cleanup()

	if err := q.Enqueue(ctx, streamQueueTestPayload{ID: "1"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	msg, err := q.Dequeue(ctx, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	msg.Payload.ID = "2"
	if err := q.Requeue(ctx, msg, 0, "retry"); err != nil {
		t.Fatalf("requeue: %v", err)
	}

	pending, err := rdb.XPending(ctx, "test:stream", "test_group").Result()
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("pending count = %d, want 0", pending.Count)
	}
	if got := rdb.XLen(ctx, "test:stream").Val(); got != 1 {
		t.Fatalf("stream len = %d, want 1", got)
	}

	entries, err := rdb.XRange(ctx, "test:stream", "-", "+").Result()
	if err != nil {
		t.Fatalf("xrange: %v", err)
	}
	payload, _ := entries[0].Values["payload"].(string)
	var decoded streamQueueTestPayload
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded.ID != "2" {
		t.Fatalf("requeued payload ID = %q, want 2", decoded.ID)
	}
}

func TestStreamQueueMoveToDeadLetterAddsDLQAndAcksOld(t *testing.T) {
	ctx := context.Background()
	q, rdb, cleanup := newTestStreamQueue(t)
	defer cleanup()

	if err := q.Enqueue(ctx, streamQueueTestPayload{ID: "1"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	msg, err := q.Dequeue(ctx, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if err := q.MoveToDeadLetter(ctx, msg, "terminal"); err != nil {
		t.Fatalf("dlq: %v", err)
	}

	pending, err := rdb.XPending(ctx, "test:stream", "test_group").Result()
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("pending count = %d, want 0", pending.Count)
	}
	if got := rdb.XLen(ctx, "test:stream:dlq").Val(); got != 1 {
		t.Fatalf("dlq len = %d, want 1", got)
	}
}

func TestVectorizeQueueDequeueDoesNotAck(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	q := NewVectorizeQueue(rdb, "video:vectorize:test")
	if err := q.Enqueue(ctx, videoapp.VectorizeTask{VideoID: 10, TaskID: "10", RawKey: "raw.mp4"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	msg, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if msg.MessageID == "" || msg.Task.VideoID != 10 {
		t.Fatalf("unexpected message: %+v", msg)
	}

	pending, err := rdb.XPending(ctx, "video:vectorize:test", streamGroupName("video:vectorize:test")).Result()
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if pending.Count != 1 {
		t.Fatalf("pending count = %d, want 1", pending.Count)
	}
}
