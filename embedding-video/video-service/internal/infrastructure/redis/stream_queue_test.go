package redis

import (
	"context"
	"encoding/json"
	"errors"
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

func TestStreamQueueDequeueReclaimsPendingMessage(t *testing.T) {
	ctx := context.Background()
	q, _, cleanup := newTestStreamQueue(t)
	defer cleanup()
	q.pendingMinIdle = 0

	if err := q.Enqueue(ctx, streamQueueTestPayload{ID: "1"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	first, err := q.Dequeue(ctx, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("first dequeue: %v", err)
	}

	replacement := NewStreamQueue[streamQueueTestPayload](q.rdb, StreamQueueOptions{
		Key:      q.key,
		Group:    q.group,
		Consumer: "replacement_consumer",
	})
	replacement.pendingMinIdle = 0
	reclaimed, err := replacement.Dequeue(ctx, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("reclaim dequeue: %v", err)
	}
	if reclaimed.ID != first.ID {
		t.Fatalf("reclaimed id = %q, want %q", reclaimed.ID, first.ID)
	}
	if reclaimed.Payload.ID != "1" {
		t.Fatalf("reclaimed payload = %+v, want ID 1", reclaimed.Payload)
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

func TestStreamQueueRequeueDelayHidesMessageUntilDue(t *testing.T) {
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
	if err := q.Requeue(ctx, msg, 50*time.Millisecond, "retry"); err != nil {
		t.Fatalf("requeue: %v", err)
	}

	if got := rdb.XLen(ctx, "test:stream").Val(); got != 0 {
		t.Fatalf("stream len before delay = %d, want 0", got)
	}
	if _, err := q.Dequeue(ctx, 5*time.Millisecond); !errors.Is(err, goredis.Nil) {
		t.Fatalf("immediate dequeue err = %v, want redis nil", err)
	}

	time.Sleep(70 * time.Millisecond)
	requeued, err := q.Dequeue(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("delayed dequeue: %v", err)
	}
	if requeued.Payload.ID != "2" {
		t.Fatalf("delayed payload ID = %q, want 2", requeued.Payload.ID)
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

func TestListDeadLettersReturnsEntriesWithPayloadAndReason(t *testing.T) {
	ctx := context.Background()
	q, _, cleanup := newTestStreamQueue(t)
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

	entries, err := ListDeadLetters(ctx, q.rdb, "test:stream", 10)
	if err != nil {
		t.Fatalf("list dlq: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].ID == "" {
		t.Fatal("entry id is empty")
	}
	if entries[0].Reason != "terminal" {
		t.Fatalf("reason = %q, want terminal", entries[0].Reason)
	}
	var decoded streamQueueTestPayload
	if err := json.Unmarshal([]byte(entries[0].Payload), &decoded); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if decoded.ID != "1" {
		t.Fatalf("payload id = %q, want 1", decoded.ID)
	}
}

func TestReplayDeadLetterRequeuesPayloadAndDeletesDLQByDefault(t *testing.T) {
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
	entries, err := ListDeadLetters(ctx, q.rdb, "test:stream", 10)
	if err != nil {
		t.Fatalf("list dlq: %v", err)
	}

	replayed, err := ReplayDeadLetter(ctx, q.rdb, "test:stream", entries[0].ID, ReplayDeadLetterOptions{})
	if err != nil {
		t.Fatalf("replay dlq: %v", err)
	}
	if !replayed {
		t.Fatal("expected replay to report success")
	}
	if got := rdb.XLen(ctx, "test:stream").Val(); got != 1 {
		t.Fatalf("stream len = %d, want 1", got)
	}
	if got := rdb.XLen(ctx, "test:stream:dlq").Val(); got != 0 {
		t.Fatalf("dlq len = %d, want 0", got)
	}

	requeued, err := q.Dequeue(ctx, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("dequeue replayed: %v", err)
	}
	if requeued.Payload.ID != "1" {
		t.Fatalf("replayed payload id = %q, want 1", requeued.Payload.ID)
	}
}

func TestReplayDeadLetterCanKeepOriginalDLQEntry(t *testing.T) {
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
	entries, err := ListDeadLetters(ctx, q.rdb, "test:stream", 10)
	if err != nil {
		t.Fatalf("list dlq: %v", err)
	}

	replayed, err := ReplayDeadLetter(ctx, q.rdb, "test:stream", entries[0].ID, ReplayDeadLetterOptions{KeepDeadLetter: true})
	if err != nil {
		t.Fatalf("replay dlq: %v", err)
	}
	if !replayed {
		t.Fatal("expected replay to report success")
	}
	if got := rdb.XLen(ctx, "test:stream").Val(); got != 1 {
		t.Fatalf("stream len = %d, want 1", got)
	}
	if got := rdb.XLen(ctx, "test:stream:dlq").Val(); got != 1 {
		t.Fatalf("dlq len = %d, want 1", got)
	}
}

func TestTranscodeQueueDequeueReclaimsPendingMessage(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	q := NewTranscodeQueue(rdb, "video:transcode:test")
	q.consumer = "first_consumer"
	q.pendingMinIdle = 0
	if err := q.Enqueue(ctx, videoapp.TranscodeTask{VideoID: 10, TaskID: "10", RawKey: "raw.mp4"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	first, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("first dequeue: %v", err)
	}

	replacement := NewTranscodeQueue(rdb, "video:transcode:test")
	replacement.consumer = "replacement_consumer"
	replacement.pendingMinIdle = 0
	reclaimCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	reclaimed, err := replacement.Dequeue(reclaimCtx)
	if err != nil {
		t.Fatalf("reclaim dequeue: %v", err)
	}
	if reclaimed.MessageID != first.MessageID {
		t.Fatalf("reclaimed message id = %q, want %q", reclaimed.MessageID, first.MessageID)
	}
	if reclaimed.Task.VideoID != 10 || reclaimed.Task.TaskID != "10" {
		t.Fatalf("unexpected reclaimed task: %+v", reclaimed.Task)
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
	dequeueCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	msg, err := q.Dequeue(dequeueCtx)
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
