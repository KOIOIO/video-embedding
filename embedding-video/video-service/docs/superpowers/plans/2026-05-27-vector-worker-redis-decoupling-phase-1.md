# Vector Worker Redis Decoupling Phase 1 Implementation Plan

> Status: Historical. This phase-1 plan describes the reusable queue/state foundation and references an older fine-grained follow-up direction. Current vector-worker stage implementation is based on `docs/superpowers/plans/2026-06-06-vector-worker-full-redis-stage-decoupling.md` and intentionally uses only four Redis stage queues.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Phase 1 foundation for Redis-decoupled vector processing by fixing vector queue ACK safety, adding a reusable Redis Stream queue abstraction, and adding durable vector stage state primitives.

**Architecture:** Keep the existing monolithic `handleVectorizeTask` execution path intact for now. Introduce small reusable infrastructure beneath it: explicit ACK queue messages, generic stage queues, and a PostgreSQL-backed stage state model that later phases can use for fan-out/fan-in processing.

**Tech Stack:** Go, go-redis v8, GORM, PostgreSQL, Redis Streams, existing `videoapp` task types.

---

## Scope

This plan implements only Phase 1 from `docs/superpowers/specs/2026-05-27-vector-worker-redis-decoupling-design.md`.

Included:

1. Generic Redis Stream queue abstraction with explicit ACK.
2. Safe vector top-level queue consumption with ACK after task processing.
3. Vector stage task envelope and queue key constants.
4. PostgreSQL model and repository for vector stage state.
5. Focused unit tests.

Excluded:

1. Splitting `handleVectorizeTask` into stage workers.
2. Implementing `vector.prepare`, `vector.coarse.clip`, or other stage handlers.
3. Public API changes.
4. Git staging or commits.

## File Structure

Create:

- `internal/infrastructure/redis/stream_queue.go`
  - Generic Redis Stream queue with explicit `Ack`, `Requeue`, and `MoveToDeadLetter`.
- `internal/infrastructure/redis/stream_queue_test.go`
  - Unit tests using `miniredis` for queue semantics.
- `internal/worker/vectorworker/stage_task.go`
  - Stage names, queue keys, and `VectorStageTask` envelope.
- `internal/worker/vectorworker/stage_task_test.go`
  - JSON and validation tests for stage task envelope.
- `internal/infrastructure/persistence/vector_stage_repository.go`
  - GORM repository for stage state upsert, complete, fail, and readiness checks.
- `internal/infrastructure/persistence/vector_stage_repository_test.go`
  - SQLite-backed repository unit tests.

Modify:

- `go.mod`
  - Add test dependencies for Redis Stream and GORM repository tests.
- `go.sum`
  - Updated by `go get`/`go mod tidy` as needed.
- `internal/model/video.go`
  - Add `EduVideoVectorStage` model and table name.
- `internal/model/video_test.go`
  - Add table name test.
- `internal/infrastructure/redis/transcode.go`
  - Reuse the generic queue internally where practical.
  - Change `VectorizeQueue.Dequeue` to return message ID and stop ACKing immediately.
  - Add `Ack`, `Requeue`, and `MoveToDeadLetter` methods for vector queue.
- `internal/application/videoapp/types.go`
  - Add `VectorizeQueueMessage`.
- `internal/worker/vectorworker/app.go`
  - ACK vector messages only after `handleVectorizeTask` succeeds or terminal failure handling is complete.
- `internal/http/app/app.go`
  - Include `EduVideoVectorStage` in `AutoMigrate`.
- `internal/worker/vectorworker/app.go`
  - Include `EduVideoVectorStage` in `AutoMigrate`.
- `internal/worker/transcodeworker/app.go`
  - Include `EduVideoVectorStage` in `AutoMigrate` only if this worker continues migrating shared video tables.

## Task 1: Test Dependencies

**Files:**

- Modify: `go.mod`
- Modify: `go.sum`
- Verify: `go test ./internal/infrastructure/redis ./internal/infrastructure/persistence`

- [ ] **Step 1: Add focused test dependencies**

Run:

```bash
go get github.com/alicebob/miniredis/v2 gorm.io/driver/sqlite
```

Expected:

```text
go: added github.com/alicebob/miniredis/v2 ...
go: added gorm.io/driver/sqlite ...
```

The exact transitive dependency lines may differ by module cache state.

- [ ] **Step 2: Verify dependencies resolve**

Run:

```bash
go test ./internal/infrastructure/redis ./internal/infrastructure/persistence
```

Expected before later tasks are implemented:

```text
?   	embedding-video/http/internal/infrastructure/redis	[no test files]
?   	embedding-video/http/internal/infrastructure/persistence	[no test files]
```

If existing tests are added before this task runs, the packages may fail for missing implementation symbols. That is acceptable only if the dependency resolution errors are gone.

- [ ] **Step 3: Inspect dependency-only diff**

Run:

```bash
git diff -- go.mod go.sum
```

Expected: only dependency changes related to `github.com/alicebob/miniredis/v2`, `gorm.io/driver/sqlite`, and their transitive dependencies. Preserve any pre-existing user edits in `go.mod`.

## Task 2: Generic Redis Stream Queue

**Files:**

- Create: `internal/infrastructure/redis/stream_queue.go`
- Create: `internal/infrastructure/redis/stream_queue_test.go`
- Verify: `go test ./internal/infrastructure/redis`

- [ ] **Step 1: Add failing tests for explicit ACK behavior**

Create `internal/infrastructure/redis/stream_queue_test.go` with these tests:

```go
package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/alicebob/miniredis/v2"
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
```

- [ ] **Step 2: Run the tests and verify they fail**

Run:

```bash
go test ./internal/infrastructure/redis
```

Expected:

```text
undefined: StreamQueue
undefined: NewStreamQueue
undefined: StreamQueueOptions
```

- [ ] **Step 3: Add the generic queue implementation**

Create `internal/infrastructure/redis/stream_queue.go`:

```go
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
			Values: map[string]interface{}{
				"payload": string(b),
			},
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
```

- [ ] **Step 4: Add `isBusyGroup` helper**

Modify `internal/infrastructure/redis/transcode.go` by adding this helper near `streamGroupName`:

```go
func isBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}
```

Then update both existing `ensureGroup` methods in `transcode.go` to use it:

```go
if err != nil && !isBusyGroup(err) {
	q.onceErr = err
}
```

- [ ] **Step 5: Run Redis package tests**

Run:

```bash
go test ./internal/infrastructure/redis
```

Expected:

```text
ok  	embedding-video/http/internal/infrastructure/redis
```

- [ ] **Step 6: Check diff**

Run:

```bash
git diff -- internal/infrastructure/redis
```

Expected: only `stream_queue.go`, `stream_queue_test.go`, and the small helper usage in `transcode.go` are changed.

## Task 3: Safe Vector Queue ACK Semantics

**Files:**

- Modify: `internal/application/videoapp/types.go`
- Modify: `internal/infrastructure/redis/transcode.go`
- Modify: `internal/worker/vectorworker/app.go`
- Test: `internal/infrastructure/redis/stream_queue_test.go`
- Verify: `go test ./internal/infrastructure/redis ./internal/worker/vectorworker`

- [ ] **Step 1: Add vector message type**

Modify `internal/application/videoapp/types.go` near `VectorizeTask`:

```go
type VectorizeQueueMessage struct {
	MessageID string
	Task      VectorizeTask
}
```

- [ ] **Step 2: Change `VectorizeQueue.Dequeue` signature and behavior**

Modify `internal/infrastructure/redis/transcode.go`.

Replace:

```go
func (q *VectorizeQueue) Dequeue(ctx context.Context) (videoapp.VectorizeTask, error)
```

with:

```go
func (q *VectorizeQueue) Dequeue(ctx context.Context) (videoapp.VectorizeQueueMessage, error)
```

The body should parse the payload and return:

```go
return videoapp.VectorizeQueueMessage{MessageID: msg.ID, Task: task}, nil
```

Do not call `q.ackAndDelete(ctx, msg.ID)` after successful JSON parsing.

For missing payload or invalid JSON, keep ACK-and-delete behavior because the message cannot be recovered.

- [ ] **Step 3: Add explicit vector queue methods**

Add these methods to `internal/infrastructure/redis/transcode.go`:

```go
func (q *VectorizeQueue) Ack(ctx context.Context, id string) error {
	return q.ackAndDelete(ctx, id)
}

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
```

- [ ] **Step 4: Update vector worker app call site**

Modify `internal/worker/vectorworker/app.go`.

Replace:

```go
task, err := queue.Dequeue(ctx)
```

with:

```go
msg, err := queue.Dequeue(ctx)
```

Then set:

```go
task := msg.Task
```

After the retry loop, add explicit handling:

```go
if lastErr == nil {
	if err := queue.Ack(ctx, msg.MessageID); err != nil {
		zap.L().Error("vector_task_ack_failed",
			zap.Int("worker_id", id),
			zap.Uint64("video_id", task.VideoID),
			zap.String("task_id", task.TaskID),
			zap.String("message_id", msg.MessageID),
			zap.Error(err))
	}
} else {
	_ = queue.MoveToDeadLetter(ctx, msg, lastErr.Error())
}
```

Keep this simple for Phase 1. Do not add delayed requeue here yet, because the existing in-process retry loop already retries AI failures up to three times. DLQ after exhaustion is safer than silently ACKing failures.

- [ ] **Step 5: Add Redis test proving vector queue does not ACK on dequeue**

Append to `internal/infrastructure/redis/stream_queue_test.go`:

```go
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
```

Add the missing import to `stream_queue_test.go`:

```go
"embedding-video/http/internal/application/videoapp"
```

- [ ] **Step 6: Run focused tests**

Run:

```bash
go test ./internal/infrastructure/redis ./internal/worker/vectorworker
```

Expected:

```text
ok  	embedding-video/http/internal/infrastructure/redis
ok  	embedding-video/http/internal/worker/vectorworker
```

- [ ] **Step 7: Check diff**

Run:

```bash
git diff -- internal/application/videoapp/types.go internal/infrastructure/redis/transcode.go internal/worker/vectorworker/app.go internal/infrastructure/redis/stream_queue_test.go
```

Expected: vector queue now returns a message ID and worker ACKs after processing.

## Task 4: Vector Stage Task Envelope

**Files:**

- Create: `internal/worker/vectorworker/stage_task.go`
- Create: `internal/worker/vectorworker/stage_task_test.go`
- Verify: `go test ./internal/worker/vectorworker`

- [ ] **Step 1: Add failing tests for stage task constants and JSON**

Create `internal/worker/vectorworker/stage_task_test.go`:

```go
package vectorworker

import (
	"encoding/json"
	"testing"
)

func TestVectorStageTaskJSONRoundTrip(t *testing.T) {
	in := VectorStageTask{
		TaskID:        "42",
		VideoID:       42,
		RawKey:        "raw/video.mp4",
		Stage:         VectorStageCoarseClip,
		SegmentIndex:  3,
		SegmentID:     99,
		StartSec:      120,
		EndSec:        160,
		NextStartSec:  161,
		ObjectKey:     "segments/coarse/video_42/42/seg_003_120_160.mp4",
		RetryCount:    2,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out VectorStageTask
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Fatalf("round trip = %+v, want %+v", out, in)
	}
}

func TestVectorStageQueueKey(t *testing.T) {
	cases := map[string]string{
		VectorStagePrepare:    "video:vector:prepare",
		VectorStageCoarseClip: "video:vector:coarse:clip",
		VectorStageCoarseASR:  "video:vector:coarse:asr",
		VectorStageSegmentLLM: "video:vector:segment:llm",
		VectorStageRefineASR:  "video:vector:refine:asr",
		VectorStageEmbedding:  "video:vector:embedding",
		VectorStageFinalize:   "video:vector:finalize",
	}
	for stage, want := range cases {
		if got := VectorStageQueueKey(stage); got != want {
			t.Fatalf("VectorStageQueueKey(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestVectorStageQueueKeyUnknown(t *testing.T) {
	if got := VectorStageQueueKey("unknown"); got != "" {
		t.Fatalf("VectorStageQueueKey(unknown) = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/worker/vectorworker
```

Expected:

```text
undefined: VectorStageTask
undefined: VectorStageCoarseClip
```

- [ ] **Step 3: Add stage task implementation**

Create `internal/worker/vectorworker/stage_task.go`:

```go
package vectorworker

const (
	VectorStagePrepare    = "vector.prepare"
	VectorStageCoarseClip = "vector.coarse.clip"
	VectorStageCoarseASR  = "vector.coarse.asr"
	VectorStageSegmentLLM = "vector.segment.llm"
	VectorStageRefineASR  = "vector.refine.asr"
	VectorStageEmbedding  = "vector.embedding"
	VectorStageFinalize   = "vector.finalize"
)

type VectorStageTask struct {
	TaskID        string `json:"task_id"`
	VideoID       uint64 `json:"video_id"`
	RawKey        string `json:"raw_key,omitempty"`
	Stage         string `json:"stage"`
	SegmentIndex  int    `json:"segment_index,omitempty"`
	SegmentID     uint64 `json:"segment_id,omitempty"`
	StartSec      int    `json:"start_sec,omitempty"`
	EndSec        int    `json:"end_sec,omitempty"`
	NextStartSec  int    `json:"next_start_sec,omitempty"`
	ObjectKey     string `json:"object_key,omitempty"`
	RetryCount    int    `json:"retry_count,omitempty"`
}

func VectorStageQueueKey(stage string) string {
	switch stage {
	case VectorStagePrepare:
		return "video:vector:prepare"
	case VectorStageCoarseClip:
		return "video:vector:coarse:clip"
	case VectorStageCoarseASR:
		return "video:vector:coarse:asr"
	case VectorStageSegmentLLM:
		return "video:vector:segment:llm"
	case VectorStageRefineASR:
		return "video:vector:refine:asr"
	case VectorStageEmbedding:
		return "video:vector:embedding"
	case VectorStageFinalize:
		return "video:vector:finalize"
	default:
		return ""
	}
}
```

- [ ] **Step 4: Run vectorworker tests**

Run:

```bash
go test ./internal/worker/vectorworker
```

Expected:

```text
ok  	embedding-video/http/internal/worker/vectorworker
```

## Task 5: Vector Stage State Model

**Files:**

- Modify: `internal/model/video.go`
- Modify: `internal/model/video_test.go`
- Modify: `internal/http/app/app.go`
- Modify: `internal/worker/vectorworker/app.go`
- Modify: `internal/worker/transcodeworker/app.go`
- Verify: `go test ./internal/model ./cmd/httpapi ./cmd/worker`

- [ ] **Step 1: Add failing table name test**

Modify `internal/model/video_test.go` by adding:

```go
func TestEduVideoVectorStageTableName(t *testing.T) {
	if got := (EduVideoVectorStage{}).TableName(); got != "edu_video_vector_stage" {
		t.Fatalf("EduVideoVectorStage.TableName() = %q", got)
	}
}
```

- [ ] **Step 2: Run model tests and verify failure**

Run:

```bash
go test ./internal/model
```

Expected:

```text
undefined: EduVideoVectorStage
```

- [ ] **Step 3: Add model**

Modify `internal/model/video.go` after `EduVideoSegment`:

```go
type EduVideoVectorStage struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	TaskID       string `gorm:"column:task_id;type:text;not null;index:idx_video_vector_stage_unique,unique,priority:1" json:"task_id"`
	VideoID      uint64 `gorm:"column:video_id;not null;index" json:"video_id"`
	Stage        string `gorm:"column:stage;type:text;not null;index:idx_video_vector_stage_unique,unique,priority:2" json:"stage"`
	SegmentIndex int    `gorm:"column:segment_index;not null;default:0;index:idx_video_vector_stage_unique,unique,priority:3" json:"segment_index"`
	SegmentID    uint64 `gorm:"column:segment_id;not null;default:0;index:idx_video_vector_stage_unique,unique,priority:4" json:"segment_id"`

	Status       int16  `gorm:"column:status;not null;default:0;index" json:"status"`
	ObjectKey    string `gorm:"column:object_key;type:text;not null;default:''" json:"object_key"`
	Text         string `gorm:"column:text;type:text;not null;default:''" json:"text"`
	ErrorMessage string `gorm:"column:error_message;type:text;not null;default:''" json:"error_message"`
	RetryCount   int    `gorm:"column:retry_count;not null;default:0" json:"retry_count"`
	StartTimeSec int    `gorm:"column:start_time;not null;default:0" json:"start_time"`
	EndTimeSec   int    `gorm:"column:end_time;not null;default:0" json:"end_time"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

func (EduVideoVectorStage) TableName() string { return "edu_video_vector_stage" }
```

- [ ] **Step 4: Include model in AutoMigrate**

Modify these `AutoMigrate` calls to include `&model.EduVideoVectorStage{}`:

```go
db.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoSegment{}, &model.EduUserVideoRecommend{}, &model.EduVideoVectorStage{})
```

Files:

- `internal/http/app/app.go`
- `internal/worker/vectorworker/app.go`
- `internal/worker/transcodeworker/app.go`

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/model ./cmd/httpapi ./cmd/worker
```

Expected:

```text
ok  	embedding-video/http/internal/model
ok  	embedding-video/http/cmd/httpapi
ok  	embedding-video/http/cmd/worker
```

## Task 6: Vector Stage Repository

**Files:**

- Create: `internal/infrastructure/persistence/vector_stage_repository.go`
- Create: `internal/infrastructure/persistence/vector_stage_repository_test.go`
- Verify: `go test ./internal/infrastructure/persistence`

- [ ] **Step 1: Add repository tests**

Create `internal/infrastructure/persistence/vector_stage_repository_test.go`:

```go
package persistence

import (
	"context"
	"testing"

	"embedding-video/http/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newVectorStageTestRepo(t *testing.T) (*VectorStageRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduVideoVectorStage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewVectorStageRepository(db), db
}

func TestVectorStageRepositoryUpsertIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo, db := newVectorStageTestRepo(t)

	in := VectorStageRecord{
		TaskID:       "42",
		VideoID:      42,
		Stage:        "vector.coarse.asr",
		SegmentIndex: 3,
		StartSec:     120,
		EndSec:       160,
		ObjectKey:    "obj-a",
	}
	if err := repo.UpsertPending(ctx, in); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	in.ObjectKey = "obj-b"
	if err := repo.UpsertPending(ctx, in); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var rows []model.EduVideoVectorStage
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].ObjectKey != "obj-b" {
		t.Fatalf("object key = %q, want obj-b", rows[0].ObjectKey)
	}
}

func TestVectorStageRepositoryMarkCompleteAndCount(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVectorStageTestRepo(t)

	for i := 0; i < 3; i++ {
		if err := repo.UpsertPending(ctx, VectorStageRecord{
			TaskID:       "42",
			VideoID:      42,
			Stage:        "vector.coarse.asr",
			SegmentIndex: i,
		}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	if err := repo.MarkComplete(ctx, VectorStageRecord{
		TaskID:       "42",
		Stage:        "vector.coarse.asr",
		SegmentIndex: 0,
		Text:         "first",
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}

	total, complete, err := repo.CountStage(ctx, "42", "vector.coarse.asr")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 3 || complete != 1 {
		t.Fatalf("count = total %d complete %d, want 3/1", total, complete)
	}
}

func TestVectorStageRepositoryMarkFailed(t *testing.T) {
	ctx := context.Background()
	repo, db := newVectorStageTestRepo(t)

	if err := repo.UpsertPending(ctx, VectorStageRecord{
		TaskID:       "42",
		VideoID:      42,
		Stage:        "vector.coarse.asr",
		SegmentIndex: 1,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.MarkFailed(ctx, VectorStageRecord{
		TaskID:       "42",
		Stage:        "vector.coarse.asr",
		SegmentIndex: 1,
		ErrorMessage: "asr failed",
		RetryCount:   2,
	}); err != nil {
		t.Fatalf("failed: %v", err)
	}

	var row model.EduVideoVectorStage
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if row.Status != VectorStageStatusFailed || row.ErrorMessage != "asr failed" || row.RetryCount != 2 {
		t.Fatalf("unexpected row: %+v", row)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/infrastructure/persistence
```

Expected:

```text
undefined: VectorStageRepository
undefined: VectorStageRecord
```

- [ ] **Step 3: Add repository implementation**

Create `internal/infrastructure/persistence/vector_stage_repository.go`:

```go
package persistence

import (
	"context"

	"embedding-video/http/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	VectorStageStatusPending    int16 = 0
	VectorStageStatusProcessing int16 = 1
	VectorStageStatusComplete   int16 = 2
	VectorStageStatusFailed     int16 = 3
	VectorStageStatusSkipped    int16 = 4
)

type VectorStageRecord struct {
	TaskID       string
	VideoID      uint64
	Stage        string
	SegmentIndex int
	SegmentID    uint64
	Status       int16
	ObjectKey    string
	Text         string
	ErrorMessage string
	RetryCount   int
	StartSec     int
	EndSec       int
}

type VectorStageRepository struct {
	db *gorm.DB
}

func NewVectorStageRepository(db *gorm.DB) *VectorStageRepository {
	return &VectorStageRepository{db: db}
}

func (r *VectorStageRepository) UpsertPending(ctx context.Context, rec VectorStageRecord) error {
	row := vectorStageModel(rec)
	if row.Status == 0 {
		row.Status = VectorStageStatusPending
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "task_id"},
			{Name: "stage"},
			{Name: "segment_index"},
			{Name: "segment_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"video_id",
			"status",
			"object_key",
			"text",
			"error_message",
			"retry_count",
			"start_time",
			"end_time",
			"update_time",
		}),
	}).Create(&row).Error
}

func (r *VectorStageRepository) MarkComplete(ctx context.Context, rec VectorStageRecord) error {
	updates := map[string]any{
		"status":        VectorStageStatusComplete,
		"object_key":    rec.ObjectKey,
		"text":          rec.Text,
		"error_message": "",
		"retry_count":   rec.RetryCount,
		"start_time":    rec.StartSec,
		"end_time":      rec.EndSec,
	}
	return r.db.WithContext(ctx).Model(&model.EduVideoVectorStage{}).
		Where("task_id = ? AND stage = ? AND segment_index = ? AND segment_id = ?", rec.TaskID, rec.Stage, rec.SegmentIndex, rec.SegmentID).
		Updates(updates).Error
}

func (r *VectorStageRepository) MarkFailed(ctx context.Context, rec VectorStageRecord) error {
	updates := map[string]any{
		"status":        VectorStageStatusFailed,
		"error_message": rec.ErrorMessage,
		"retry_count":   rec.RetryCount,
	}
	return r.db.WithContext(ctx).Model(&model.EduVideoVectorStage{}).
		Where("task_id = ? AND stage = ? AND segment_index = ? AND segment_id = ?", rec.TaskID, rec.Stage, rec.SegmentIndex, rec.SegmentID).
		Updates(updates).Error
}

func (r *VectorStageRepository) CountStage(ctx context.Context, taskID string, stage string) (int64, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.EduVideoVectorStage{}).
		Where("task_id = ? AND stage = ?", taskID, stage).
		Count(&total).Error; err != nil {
		return 0, 0, err
	}
	var complete int64
	if err := r.db.WithContext(ctx).Model(&model.EduVideoVectorStage{}).
		Where("task_id = ? AND stage = ? AND status = ?", taskID, stage, VectorStageStatusComplete).
		Count(&complete).Error; err != nil {
		return 0, 0, err
	}
	return total, complete, nil
}

func vectorStageModel(rec VectorStageRecord) model.EduVideoVectorStage {
	return model.EduVideoVectorStage{
		TaskID:       rec.TaskID,
		VideoID:      rec.VideoID,
		Stage:        rec.Stage,
		SegmentIndex: rec.SegmentIndex,
		SegmentID:    rec.SegmentID,
		Status:       rec.Status,
		ObjectKey:    rec.ObjectKey,
		Text:         rec.Text,
		ErrorMessage: rec.ErrorMessage,
		RetryCount:   rec.RetryCount,
		StartTimeSec: rec.StartSec,
		EndTimeSec:   rec.EndSec,
	}
}
```

- [ ] **Step 4: Run persistence tests**

Run:

```bash
go test ./internal/infrastructure/persistence
```

Expected:

```text
ok  	embedding-video/http/internal/infrastructure/persistence
```

## Task 7: Final Focused Verification

**Files:**

- Verify only.

- [ ] **Step 1: Run focused package tests**

Run:

```bash
go test ./internal/infrastructure/redis ./internal/infrastructure/persistence ./internal/model ./internal/worker/vectorworker ./cmd/httpapi ./cmd/worker
```

Expected:

```text
ok  	embedding-video/http/internal/infrastructure/redis
ok  	embedding-video/http/internal/infrastructure/persistence
ok  	embedding-video/http/internal/model
ok  	embedding-video/http/internal/worker/vectorworker
ok  	embedding-video/http/cmd/httpapi
ok  	embedding-video/http/cmd/worker
```

- [ ] **Step 2: Run full Go test suite**

Run:

```bash
go test ./...
```

Expected:

```text
ok  	... 
```

If any package requires unavailable external services, record the exact package and failure message, then rerun the focused package tests above as the accepted local verification.

- [ ] **Step 3: Inspect final diff without touching git index**

Run:

```bash
git diff --stat
git diff -- internal/application/videoapp/types.go internal/infrastructure/redis internal/infrastructure/persistence internal/model internal/worker/vectorworker internal/http/app/app.go internal/worker/transcodeworker/app.go
```

Expected:

1. No unrelated formatting churn.
2. No generated swagger changes.
3. Existing `go.mod` changes, if any, are not modified unless dependency additions require it.
4. The implementation still runs the monolithic vector handler; stage worker execution is reserved for later phases.

## Self-Review Notes

Spec coverage:

1. Explicit ACK queue semantics are covered by Tasks 1 and 2.
2. Stage task envelope and queue keys are covered by Task 3.
3. Durable PostgreSQL stage state is covered by Tasks 4 and 5.
4. Incremental migration without splitting the worker is preserved by excluding stage handler implementation.

Intentional deferrals:

1. Coarse pipeline fan-out starts in Phase 2.
2. LLM/refine/embedding stage workers start in later phases.
3. Public vector status APIs are not introduced.

No git commands that modify the index are included because the user requested not to touch git.
