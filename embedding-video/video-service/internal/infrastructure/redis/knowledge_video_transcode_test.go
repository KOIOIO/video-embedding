package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"

	"video-service/internal/application/knowledgevideo"
)

func TestKnowledgeVideoQueueRoundTripAndAck(t *testing.T) {
	q, rdb := newKnowledgeVideoTestQueue(t)
	ctx := context.Background()
	want := knowledgevideo.TranscodeTask{KnowledgeVideoID: 8, SourceObjectKey: "raw/8/source.mp4", HLSObjectPrefix: "hls/8", TaskID: "knowledge-video-8"}
	if err := q.Enqueue(ctx, want); err != nil {
		t.Fatal(err)
	}
	msg, err := q.Dequeue(ctx)
	if err != nil || msg.Task != want {
		t.Fatalf("msg=%+v err=%v", msg, err)
	}
	if err := q.Ack(ctx, msg.MessageID); err != nil {
		t.Fatal(err)
	}
	if got := rdb.XLen(ctx, q.key).Val(); got != 0 {
		t.Fatalf("stream len = %d", got)
	}
}

func TestKnowledgeVideoQueueReclaimsPending(t *testing.T) {
	q, _ := newKnowledgeVideoTestQueue(t)
	ctx := context.Background()
	q.pendingMinIdle = 0
	if err := q.Enqueue(ctx, knowledgevideo.TranscodeTask{KnowledgeVideoID: 8}); err != nil {
		t.Fatal(err)
	}
	first, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	replacement := NewKnowledgeVideoTranscodeQueue(q.rdb, q.key)
	replacement.consumer = "replacement"
	replacement.pendingMinIdle = 0
	reclaimed, err := replacement.Dequeue(ctx)
	if err != nil || reclaimed.MessageID != first.MessageID {
		t.Fatalf("reclaimed=%+v err=%v", reclaimed, err)
	}
}

func TestKnowledgeVideoQueueDelayedRequeue(t *testing.T) {
	q, _ := newKnowledgeVideoTestQueue(t)
	ctx := context.Background()
	if err := q.Enqueue(ctx, knowledgevideo.TranscodeTask{KnowledgeVideoID: 8}); err != nil {
		t.Fatal(err)
	}
	msg, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	msg.Task.RetryCount = 1
	if err := q.Requeue(ctx, msg, 20*time.Millisecond, "retry"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	got, err := q.Dequeue(ctx)
	if err != nil || got.Task.RetryCount != 1 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestKnowledgeVideoQueueMalformedPayloadIsRemoved(t *testing.T) {
	q, rdb := newKnowledgeVideoTestQueue(t)
	ctx := context.Background()
	if err := q.ensureGroup(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := rdb.XAdd(ctx, &goredis.XAddArgs{Stream: q.key, Values: map[string]interface{}{"payload": "{"}}).Result(); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Dequeue(ctx); err == nil {
		t.Fatal("Dequeue() error = nil")
	}
	if got := rdb.XLen(ctx, q.key).Val(); got != 0 {
		t.Fatalf("stream len = %d", got)
	}
}

func TestKnowledgeVideoQueueMovesTerminalFailureToDLQ(t *testing.T) {
	q, rdb := newKnowledgeVideoTestQueue(t)
	ctx := context.Background()
	if err := q.Enqueue(ctx, knowledgevideo.TranscodeTask{KnowledgeVideoID: 8}); err != nil {
		t.Fatal(err)
	}
	msg, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.MoveToDeadLetter(ctx, msg, "terminal"); err != nil {
		t.Fatal(err)
	}
	if got := rdb.XLen(ctx, q.key+":dlq").Val(); got != 1 {
		t.Fatalf("dlq len = %d", got)
	}
	if got := rdb.XLen(ctx, q.key).Val(); got != 0 {
		t.Fatalf("stream len = %d", got)
	}
}

func newKnowledgeVideoTestQueue(t *testing.T) (*KnowledgeVideoTranscodeQueue, *goredis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		mr.Close()
	})
	q := NewKnowledgeVideoTranscodeQueue(rdb, "knowledge_video:transcode:test")
	q.block = 10 * time.Millisecond
	return q, rdb
}
