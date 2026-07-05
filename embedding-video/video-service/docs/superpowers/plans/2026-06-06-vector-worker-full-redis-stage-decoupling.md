# Vector Worker Coarse Redis Stage Decoupling Implementation Plan

> Status: Current implementation plan. This plan is the source of truth for the implemented Redis stage split: `prepare`, `coarse`, `refine`, and `finalize`. It deliberately avoids the older fine-grained 11-queue direction.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split hierarchical vectorization into four Redis-backed stages: prepare, coarse, refine, and finalize.

**Architecture:** Keep the current public API and upload flow unchanged. In hierarchical mode, the top-level vector queue only starts the Redis stage pipeline; `full` and `sample` continue using the existing monolithic path. Coarse and refine stages keep their current internal goroutine/ants-pool concurrency, so the architecture gains durability and stage-level visibility without adding excessive queues.

**Tech Stack:** Go, Redis Streams via go-redis v8, GORM/PostgreSQL, pgvector, existing object storage, FFmpeg, ASR, LLM, and embedding abstractions.

---

## Scope

Design source:

- `docs/superpowers/specs/2026-06-06-vector-worker-full-redis-stage-decoupling-design.md`

Included:

1. Queue config for:
   - `video:vector:prepare`
   - `video:vector:coarse`
   - `video:vector:refine`
   - `video:vector:finalize`
2. `VectorStageWorkers` config for four stage consumers.
3. Generic stage runner for `VectorStageTask`.
4. Top-level hierarchical adapter from `video:vectorize:queue` to `vector.prepare`.
5. Coarse-grained stage handlers:
   - `vector.prepare`
   - `vector.coarse`
   - `vector.refine`
   - `vector.finalize`
6. Tests for config, queue routing, stage idempotency, and stage handoff.
7. README and parameter docs update.

Excluded:

1. Splitting coarse clip and coarse ASR into separate Redis queues.
2. Splitting LLM, refine ASR, and embedding into separate Redis queues.
3. Public HTTP API changes.
4. Provider prompt/model behavior changes.
5. Removing `full` and `sample` mode monolithic processing.

## File Structure

Create:

- `internal/worker/vectorworker/stage_config.go`
  - Four-stage queue config helpers and worker count helpers.
- `internal/worker/vectorworker/stage_queue.go`
  - Typed `VectorStageTask` queue construction.
- `internal/worker/vectorworker/stage_runner.go`
  - Generic Redis stage worker loop.
- `internal/worker/vectorworker/stage_handlers.go`
  - Shared handler interfaces and top-level adapter.
- `internal/worker/vectorworker/stage_prepare.go`
  - Prepare stage handler.
- `internal/worker/vectorworker/stage_coarse.go`
  - Coarse stage handler using existing coarse clip/upload/ASR logic internally.
- `internal/worker/vectorworker/stage_refine.go`
  - Refine stage handler using existing LLM/refine ASR/embedding logic internally.
- `internal/worker/vectorworker/stage_finalize.go`
  - Finalize stage handler.
- Focused tests next to each file.

Modify:

- `internal/config/types.go`
  - Add four queue keys and `VectorStageWorkersConfig`.
- `internal/config/defaults.go`
  - Add default queue key helpers.
- `configs/video.yml`
  - Add stage queue and worker config.
- `configs/video_prod.yml`
  - Add stage queue and worker config.
- `internal/infrastructure/persistence/vector_stage_repository.go`
  - Add read/list helpers needed by stage idempotency.
- `internal/worker/vectorworker/app.go`
  - Register four stage queues and workers.
- `internal/worker/vectorworker/stage_task.go`
  - Reduce active queue key mapping to the four coarse-grained stages while keeping existing constants available if useful.
- `README.md`
  - Document four-stage vector pipeline.
- `PROJECT_PARAMETERS.md`
  - Document queue keys and worker counts.
- `PROJECT_PARAMETERS_EN.md`
  - Same documentation in English.

## Task 1: Add Four-Stage Config

**Files:**

- Modify: `internal/config/types.go`
- Modify: `internal/config/defaults.go`
- Modify: `configs/video.yml`
- Modify: `configs/video_prod.yml`
- Create: `internal/worker/vectorworker/stage_config.go`
- Create: `internal/worker/vectorworker/stage_config_test.go`

- [ ] **Step 1: Write failing config tests**

Create `internal/worker/vectorworker/stage_config_test.go`:

```go
package vectorworker

import (
	"testing"

	"embedding-video/http/internal/config"
)

func TestCoarseStageQueueKeyFromConfigUsesDefaults(t *testing.T) {
	cfg := config.Config{}
	cases := map[string]string{
		VectorStagePrepare:  "video:vector:prepare",
		VectorStageCoarse:   "video:vector:coarse",
		VectorStageRefine:   "video:vector:refine",
		VectorStageFinalize: "video:vector:finalize",
	}
	for stage, want := range cases {
		if got := vectorStageQueueKeyFromConfig(cfg, stage); got != want {
			t.Fatalf("vectorStageQueueKeyFromConfig(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestCoarseStageQueueKeyFromConfigUsesOverrides(t *testing.T) {
	cfg := config.Config{
		RedisKeys: config.RedisKeysConfig{
			VectorPrepareQueue: "custom:prepare",
			VectorCoarseQueue:  "custom:coarse",
			VectorRefineQueue:  "custom:refine",
			VectorFinalizeQueue:"custom:finalize",
		},
	}
	cases := map[string]string{
		VectorStagePrepare:  "custom:prepare",
		VectorStageCoarse:   "custom:coarse",
		VectorStageRefine:   "custom:refine",
		VectorStageFinalize: "custom:finalize",
	}
	for stage, want := range cases {
		if got := vectorStageQueueKeyFromConfig(cfg, stage); got != want {
			t.Fatalf("vectorStageQueueKeyFromConfig(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestCoarseStageWorkerCountFromConfig(t *testing.T) {
	cfg := config.Config{
		VectorStageWorkers: config.VectorStageWorkersConfig{
			Prepare:  1,
			Coarse:   2,
			Refine:   3,
			Finalize: 4,
		},
	}
	cases := map[string]int{
		VectorStagePrepare:  1,
		VectorStageCoarse:   2,
		VectorStageRefine:   3,
		VectorStageFinalize: 4,
	}
	for stage, want := range cases {
		if got := vectorStageWorkerCountFromConfig(cfg, stage); got != want {
			t.Fatalf("vectorStageWorkerCountFromConfig(%q) = %d, want %d", stage, got, want)
		}
	}
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run:

```bash
go test ./internal/worker/vectorworker -run 'TestCoarseStageQueueKeyFromConfig|TestCoarseStageWorkerCountFromConfig'
```

Expected:

```text
undefined: VectorStageCoarse
undefined: vectorStageQueueKeyFromConfig
undefined: config.VectorStageWorkersConfig
```

- [ ] **Step 3: Add four-stage constants**

Modify `internal/worker/vectorworker/stage_task.go`.

Keep existing constants if they are still used by `stage_recorder`, but add:

```go
const (
	VectorStageCoarse = "vector.coarse"
	VectorStageRefine = "vector.refine"
)
```

Update `VectorStageQueueKey` to return only the four Redis-backed stage queues. Existing fine-grained constants such as `VectorStageCoarseClip`, `VectorStageCoarseASR`, `VectorStageSegmentLLM`, `VectorStageRefineASR`, and `VectorStageEmbedding` may remain for stage recorder rows, but they must not map to Redis queue keys in this implementation.

```go
func VectorStageQueueKey(stage string) string {
	switch stage {
	case VectorStagePrepare:
		return "video:vector:prepare"
	case VectorStageCoarse:
		return "video:vector:coarse"
	case VectorStageRefine:
		return "video:vector:refine"
	case VectorStageFinalize:
		return "video:vector:finalize"
	default:
		return ""
	}
}
```

- [ ] **Step 4: Add config fields**

Modify `internal/config/types.go`.

Extend `Config`:

```go
VectorStageWorkers VectorStageWorkersConfig `yaml:"VectorStageWorkers"`
```

Extend `RedisKeysConfig`:

```go
VectorPrepareQueue  string `yaml:"VectorPrepareQueue"`
VectorCoarseQueue   string `yaml:"VectorCoarseQueue"`
VectorRefineQueue   string `yaml:"VectorRefineQueue"`
VectorFinalizeQueue string `yaml:"VectorFinalizeQueue"`
```

Add:

```go
type VectorStageWorkersConfig struct {
	Prepare  int `yaml:"Prepare"`
	Coarse   int `yaml:"Coarse"`
	Refine   int `yaml:"Refine"`
	Finalize int `yaml:"Finalize"`
}
```

- [ ] **Step 5: Add default helpers**

Modify `internal/config/defaults.go`.

Add constants:

```go
defaultVectorPrepareQueueKey  = "video:vector:prepare"
defaultVectorCoarseQueueKey   = "video:vector:coarse"
defaultVectorRefineQueueKey   = "video:vector:refine"
defaultVectorFinalizeQueueKey = "video:vector:finalize"
```

Add helpers:

```go
func VectorPrepareQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VectorPrepareQueue, defaultVectorPrepareQueueKey)
}

func VectorCoarseQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VectorCoarseQueue, defaultVectorCoarseQueueKey)
}

func VectorRefineQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VectorRefineQueue, defaultVectorRefineQueueKey)
}

func VectorFinalizeQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VectorFinalizeQueue, defaultVectorFinalizeQueueKey)
}
```

- [ ] **Step 6: Add stage config helper**

Create `internal/worker/vectorworker/stage_config.go`:

```go
package vectorworker

import "embedding-video/http/internal/config"

var vectorStageOrder = []string{
	VectorStagePrepare,
	VectorStageCoarse,
	VectorStageRefine,
	VectorStageFinalize,
}

func vectorStageQueueKeyFromConfig(cfg config.Config, stage string) string {
	switch stage {
	case VectorStagePrepare:
		return config.VectorPrepareQueueKey(cfg)
	case VectorStageCoarse:
		return config.VectorCoarseQueueKey(cfg)
	case VectorStageRefine:
		return config.VectorRefineQueueKey(cfg)
	case VectorStageFinalize:
		return config.VectorFinalizeQueueKey(cfg)
	default:
		return ""
	}
}

func vectorStageWorkerCountFromConfig(cfg config.Config, stage string) int {
	var n int
	switch stage {
	case VectorStagePrepare:
		n = cfg.VectorStageWorkers.Prepare
	case VectorStageCoarse:
		n = cfg.VectorStageWorkers.Coarse
	case VectorStageRefine:
		n = cfg.VectorStageWorkers.Refine
	case VectorStageFinalize:
		n = cfg.VectorStageWorkers.Finalize
	}
	if n > 0 {
		return n
	}
	switch stage {
	case VectorStagePrepare, VectorStageFinalize:
		return 1
	case VectorStageCoarse, VectorStageRefine:
		return 2
	default:
		return 1
	}
}
```

- [ ] **Step 7: Add YAML entries**

In both `configs/video.yml` and `configs/video_prod.yml`, extend `RedisKeys`:

```yaml
  VectorPrepareQueue: "video:vector:prepare"
  VectorCoarseQueue: "video:vector:coarse"
  VectorRefineQueue: "video:vector:refine"
  VectorFinalizeQueue: "video:vector:finalize"
```

Add:

```yaml
VectorStageWorkers:
  Prepare: 1
  Coarse: 2
  Refine: 2
  Finalize: 1
```

- [ ] **Step 8: Run focused tests**

Run:

```bash
go test ./internal/config ./internal/worker/vectorworker -run 'TestCoarseStageQueueKeyFromConfig|TestCoarseStageWorkerCountFromConfig|TestConfig'
```

Expected: tests pass.

## Task 2: Add Stage Queue Runner

**Files:**

- Create: `internal/worker/vectorworker/stage_queue.go`
- Create: `internal/worker/vectorworker/stage_runner.go`
- Create: `internal/worker/vectorworker/stage_runner_test.go`

- [ ] **Step 1: Write failing runner tests**

Create `internal/worker/vectorworker/stage_runner_test.go`:

```go
package vectorworker

import (
	"errors"
	"testing"
	"time"
)

func TestStageRetryDecisionRetriesBeforeLimit(t *testing.T) {
	decision := decideStageRetry(VectorStageTask{RetryCount: 0}, errors.New("temporary"))
	if !decision.Retry {
		t.Fatal("expected retry")
	}
	if decision.Delay <= 0 {
		t.Fatalf("delay = %v, want positive", decision.Delay)
	}
}

func TestStageRetryDecisionStopsAfterLimit(t *testing.T) {
	decision := decideStageRetry(VectorStageTask{RetryCount: 3}, errors.New("temporary"))
	if decision.Retry {
		t.Fatal("did not expect retry after max retries")
	}
}

func TestNextRetryTaskIncrementsRetryCount(t *testing.T) {
	task := VectorStageTask{TaskID: "task-1", Stage: VectorStageCoarse, RetryCount: 1}
	got := nextRetryTask(task)
	if got.RetryCount != 2 {
		t.Fatalf("RetryCount = %d, want 2", got.RetryCount)
	}
	if got.TaskID != task.TaskID || got.Stage != task.Stage {
		t.Fatalf("identity changed: %+v", got)
	}
}

func TestStageRetryBackoffIncreases(t *testing.T) {
	first := decideStageRetry(VectorStageTask{RetryCount: 0}, errors.New("temporary"))
	second := decideStageRetry(VectorStageTask{RetryCount: 1}, errors.New("temporary"))
	if second.Delay <= first.Delay {
		t.Fatalf("second delay = %v, first = %v", second.Delay, first.Delay)
	}
	if second.Delay > 30*time.Second {
		t.Fatalf("delay too large: %v", second.Delay)
	}
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run:

```bash
go test ./internal/worker/vectorworker -run 'TestStageRetry|TestNextRetryTask'
```

Expected:

```text
undefined: decideStageRetry
undefined: nextRetryTask
```

- [ ] **Step 3: Add stage queue factory**

Create `internal/worker/vectorworker/stage_queue.go`:

```go
package vectorworker

import (
	"embedding-video/http/internal/config"
	infraredis "embedding-video/http/internal/infrastructure/redis"

	goredis "github.com/go-redis/redis/v8"
)

type vectorStageQueues map[string]*infraredis.StreamQueue[VectorStageTask]

func newVectorStageQueues(rdb *goredis.Client, cfg config.Config) vectorStageQueues {
	queues := make(vectorStageQueues, len(vectorStageOrder))
	for _, stage := range vectorStageOrder {
		key := vectorStageQueueKeyFromConfig(cfg, stage)
		queues[stage] = infraredis.NewStreamQueue[VectorStageTask](rdb, infraredis.StreamQueueOptions{
			Key:      key,
			Group:    key + ":group",
			Consumer: "vector-stage",
		})
	}
	return queues
}

func (qs vectorStageQueues) queue(stage string) *infraredis.StreamQueue[VectorStageTask] {
	if qs == nil {
		return nil
	}
	return qs[stage]
}
```

- [ ] **Step 4: Add runner**

Create `internal/worker/vectorworker/stage_runner.go`:

```go
package vectorworker

import (
	"context"
	"errors"
	"time"

	infraredis "embedding-video/http/internal/infrastructure/redis"

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
```

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/worker/vectorworker -run 'TestStageRetry|TestNextRetryTask'
```

Expected: tests pass.

## Task 3: Add Stage Repository Read Helpers

**Files:**

- Modify: `internal/infrastructure/persistence/vector_stage_repository.go`
- Modify: `internal/infrastructure/persistence/vector_stage_repository_test.go`

- [ ] **Step 1: Add failing read-helper tests**

Append to `internal/infrastructure/persistence/vector_stage_repository_test.go`:

```go
func TestVectorStageRepositoryFindStage(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVectorStageTestRepo(t)

	if err := repo.UpsertPending(ctx, VectorStageRecord{
		TaskID:       "task-1",
		VideoID:      10,
		Stage:        "vector.coarse.segment",
		SegmentIndex: 2,
		ObjectKey:    "segments/coarse/video_10/task-1/seg_002.mp4",
		StartSec:     40,
		EndSec:       80,
	}); err != nil {
		t.Fatalf("UpsertPending: %v", err)
	}

	rec, found, err := repo.FindStage(ctx, "task-1", "vector.coarse.segment", 2, 0)
	if err != nil {
		t.Fatalf("FindStage: %v", err)
	}
	if !found {
		t.Fatal("expected stage to be found")
	}
	if rec.ObjectKey != "segments/coarse/video_10/task-1/seg_002.mp4" || rec.StartSec != 40 || rec.EndSec != 80 {
		t.Fatalf("unexpected record: %+v", rec)
	}
}

func TestVectorStageRepositoryListStageOrdered(t *testing.T) {
	ctx := context.Background()
	repo, _ := newVectorStageTestRepo(t)

	for _, idx := range []int{2, 0, 1} {
		if err := repo.UpsertPending(ctx, VectorStageRecord{
			TaskID:       "task-1",
			VideoID:      10,
			Stage:        "vector.coarse.segment",
			SegmentIndex: idx,
		}); err != nil {
			t.Fatalf("UpsertPending: %v", err)
		}
	}

	recs, err := repo.ListStage(ctx, "task-1", "vector.coarse.segment")
	if err != nil {
		t.Fatalf("ListStage: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("len = %d, want 3", len(recs))
	}
	for i, rec := range recs {
		if rec.SegmentIndex != i {
			t.Fatalf("record %d SegmentIndex = %d, want %d", i, rec.SegmentIndex, i)
		}
	}
}
```

- [ ] **Step 2: Run tests and confirm they fail**

Run:

```bash
go test ./internal/infrastructure/persistence -run 'TestVectorStageRepositoryFindStage|TestVectorStageRepositoryListStageOrdered'
```

Expected:

```text
repo.FindStage undefined
repo.ListStage undefined
```

- [ ] **Step 3: Implement read helpers**

Modify `internal/infrastructure/persistence/vector_stage_repository.go`:

```go
func (r *VectorStageRepository) FindStage(ctx context.Context, taskID string, stage string, segmentIndex int, segmentID uint64) (VectorStageRecord, bool, error) {
	var row model.EduVideoVectorStage
	err := r.db.WithContext(ctx).
		Where("task_id = ? AND stage = ? AND segment_index = ? AND segment_id = ?", taskID, stage, segmentIndex, segmentID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return VectorStageRecord{}, false, nil
	}
	if err != nil {
		return VectorStageRecord{}, false, err
	}
	return vectorStageRecordFromModel(row), true, nil
}

func (r *VectorStageRepository) ListStage(ctx context.Context, taskID string, stage string) ([]VectorStageRecord, error) {
	var rows []model.EduVideoVectorStage
	if err := r.db.WithContext(ctx).
		Where("task_id = ? AND stage = ?", taskID, stage).
		Order("segment_index ASC, segment_id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]VectorStageRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, vectorStageRecordFromModel(row))
	}
	return out, nil
}

func vectorStageRecordFromModel(row model.EduVideoVectorStage) VectorStageRecord {
	return VectorStageRecord{
		TaskID:       row.TaskID,
		VideoID:      row.VideoID,
		Stage:        row.Stage,
		SegmentIndex: row.SegmentIndex,
		SegmentID:    row.SegmentID,
		Status:       row.Status,
		ObjectKey:    row.ObjectKey,
		Text:         row.Text,
		ErrorMessage: row.ErrorMessage,
		RetryCount:   row.RetryCount,
		StartSec:     row.StartTimeSec,
		EndSec:       row.EndTimeSec,
	}
}
```

Add `errors` to imports.

- [ ] **Step 4: Run repository tests**

Run:

```bash
go test ./internal/infrastructure/persistence -run TestVectorStageRepository
```

Expected: tests pass.

## Task 4: Top-Level Adapter And Stage Registration

**Files:**

- Create: `internal/worker/vectorworker/stage_handlers.go`
- Create: `internal/worker/vectorworker/stage_pipeline_test.go`
- Modify: `internal/worker/vectorworker/app.go`

- [ ] **Step 1: Add failing adapter test**

Create `internal/worker/vectorworker/stage_pipeline_test.go`:

```go
package vectorworker

import (
	"context"
	"testing"

	"embedding-video/http/internal/application/videoapp"
	"embedding-video/http/internal/infrastructure/persistence"
)

type recordingStageQueue struct {
	enqueued []VectorStageTask
}

func (q *recordingStageQueue) Enqueue(_ context.Context, task VectorStageTask) error {
	q.enqueued = append(q.enqueued, task)
	return nil
}

type recordingStageRepo struct {
	pending []persistence.VectorStageRecord
}

func (r *recordingStageRepo) UpsertPending(_ context.Context, rec persistence.VectorStageRecord) error {
	r.pending = append(r.pending, rec)
	return nil
}

func TestTopLevelVectorTaskEnqueuesPrepareForHierarchicalMode(t *testing.T) {
	repo := &recordingStageRepo{}
	queue := &recordingStageQueue{}
	adapter := newTopLevelVectorStageAdapter("hierarchical", repo, queue)

	err := adapter.Handle(context.Background(), videoapp.VectorizeTask{
		VideoID: 42,
		TaskID:  "task-1",
		RawKey:  "raw/video.mp4",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(repo.pending) != 1 {
		t.Fatalf("pending count = %d, want 1", len(repo.pending))
	}
	if len(queue.enqueued) != 1 {
		t.Fatalf("enqueued count = %d, want 1", len(queue.enqueued))
	}
	got := queue.enqueued[0]
	if got.Stage != VectorStagePrepare || got.VideoID != 42 || got.TaskID != "task-1" || got.RawKey != "raw/video.mp4" {
		t.Fatalf("unexpected prepare task: %+v", got)
	}
}
```

- [ ] **Step 2: Run test and confirm it fails**

Run:

```bash
go test ./internal/worker/vectorworker -run TestTopLevelVectorTaskEnqueuesPrepareForHierarchicalMode
```

Expected:

```text
undefined: newTopLevelVectorStageAdapter
```

- [ ] **Step 3: Implement adapter**

Create `internal/worker/vectorworker/stage_handlers.go`:

```go
package vectorworker

import (
	"context"
	"errors"
	"strings"

	"embedding-video/http/internal/application/videoapp"
	"embedding-video/http/internal/infrastructure/persistence"
)

var errNonHierarchicalStageAdapter = errors.New("stage adapter only handles hierarchical mode")

type stagePendingRepository interface {
	UpsertPending(context.Context, persistence.VectorStageRecord) error
}

type stageEnqueuer interface {
	Enqueue(context.Context, VectorStageTask) error
}

type topLevelVectorStageAdapter struct {
	mode  string
	repo  stagePendingRepository
	queue stageEnqueuer
}

func newTopLevelVectorStageAdapter(mode string, repo stagePendingRepository, queue stageEnqueuer) *topLevelVectorStageAdapter {
	return &topLevelVectorStageAdapter{mode: strings.ToLower(strings.TrimSpace(mode)), repo: repo, queue: queue}
}

func (a *topLevelVectorStageAdapter) Handle(ctx context.Context, task videoapp.VectorizeTask) error {
	if a.mode != "hierarchical" {
		return errNonHierarchicalStageAdapter
	}
	prepare := VectorStageTask{
		TaskID:    task.TaskID,
		VideoID:   task.VideoID,
		RawKey:    task.RawKey,
		Stage:     VectorStagePrepare,
		ObjectKey: task.RawKey,
	}
	if err := a.repo.UpsertPending(ctx, persistence.VectorStageRecord{
		TaskID:    prepare.TaskID,
		VideoID:   prepare.VideoID,
		Stage:     prepare.Stage,
		ObjectKey: prepare.ObjectKey,
	}); err != nil {
		return err
	}
	return a.queue.Enqueue(ctx, prepare)
}
```

- [ ] **Step 4: Register stage queues in app**

Modify `internal/worker/vectorworker/app.go`:

1. Build stage repo and queues after DB and Redis initialization:

```go
stageRepo := persistence.NewVectorStageRepository(db)
stageRecorder := newVectorStageRecorder(stageRepo)
stageQueues := newVectorStageQueues(rdb, cfg)
```

2. Replace duplicate `stageRecorder := ...` with the shared variable.

3. In the top-level vector dequeue loop, for hierarchical mode call adapter instead of `handleVectorizeTask`:

```go
if strings.EqualFold(mode, "hierarchical") {
	adapter := newTopLevelVectorStageAdapter(mode, stageRepo, stageQueues.queue(VectorStagePrepare))
	handledErr = adapter.Handle(taskCtx, task)
} else {
	handledErr = handleVectorizeTask(...)
}
```

4. Keep existing ACK/retry/DLQ logic.

- [ ] **Step 5: Register four stage runners**

In `app.go`, after stage handler creation in later tasks, each stage should use this shape:

```go
for i := 0; i < vectorStageWorkerCountFromConfig(cfg, VectorStagePrepare); i++ {
	app.Go(func(ctx context.Context) error {
		return runVectorStageWorker(ctx, VectorStagePrepare, stageQueues.queue(VectorStagePrepare), prepareHandler)
	})
}
```

At this task, only wire the queues and leave handler creation for Tasks 5 to 8.

- [ ] **Step 6: Run focused tests**

Run:

```bash
go test ./internal/worker/vectorworker -run TestTopLevelVectorTaskEnqueuesPrepareForHierarchicalMode
```

Expected: test passes.

## Task 5: Implement Prepare Stage

**Files:**

- Create: `internal/worker/vectorworker/stage_prepare.go`
- Create: `internal/worker/vectorworker/stage_prepare_test.go`
- Modify: `internal/worker/vectorworker/app.go`

- [ ] **Step 1: Add failing prepare test**

Create `internal/worker/vectorworker/stage_prepare_test.go`:

```go
package vectorworker

import (
	"context"
	"testing"

	"embedding-video/http/internal/infrastructure/persistence"
)

type prepareRepo struct {
	foundVideo bool
	pending []persistence.VectorStageRecord
	complete []persistence.VectorStageRecord
}

func (r *prepareRepo) VideoExists(_ context.Context, videoID uint64) (bool, error) {
	return r.foundVideo, nil
}

func (r *prepareRepo) UpsertPending(_ context.Context, rec persistence.VectorStageRecord) error {
	r.pending = append(r.pending, rec)
	return nil
}

func (r *prepareRepo) MarkComplete(_ context.Context, rec persistence.VectorStageRecord) error {
	r.complete = append(r.complete, rec)
	return nil
}

type prepareProbe struct {
	duration int
}

func (p prepareProbe) Probe(_ context.Context, rawKey string) (int, error) {
	return p.duration, nil
}

func TestPrepareStageCreatesCoarsePlanAndEnqueuesCoarse(t *testing.T) {
	repo := &prepareRepo{foundVideo: true}
	queue := &recordingStageQueue{}
	handler := newPrepareStageHandler(repo, prepareProbe{duration: 95}, queue, 40)

	err := handler.Handle(context.Background(), VectorStageTask{
		TaskID: "task-1",
		VideoID: 7,
		RawKey: "raw/video.mp4",
		Stage: VectorStagePrepare,
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(repo.pending) != 3 {
		t.Fatalf("coarse segment plan rows = %d, want 3", len(repo.pending))
	}
	if len(queue.enqueued) != 1 || queue.enqueued[0].Stage != VectorStageCoarse {
		t.Fatalf("coarse task not enqueued: %+v", queue.enqueued)
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStagePrepare {
		t.Fatalf("prepare not complete: %+v", repo.complete)
	}
}
```

- [ ] **Step 2: Run test and confirm it fails**

Run:

```bash
go test ./internal/worker/vectorworker -run TestPrepareStageCreatesCoarsePlanAndEnqueuesCoarse
```

Expected:

```text
undefined: newPrepareStageHandler
```

- [ ] **Step 3: Implement prepare handler**

Create `internal/worker/vectorworker/stage_prepare.go`:

```go
package vectorworker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"embedding-video/http/internal/infrastructure/persistence"
)

const vectorStageCoarseSegment = "vector.coarse.segment"

type prepareRepository interface {
	VideoExists(context.Context, uint64) (bool, error)
	UpsertPending(context.Context, persistence.VectorStageRecord) error
	MarkComplete(context.Context, persistence.VectorStageRecord) error
}

type rawVideoProber interface {
	Probe(context.Context, string) (int, error)
}

type prepareStageHandler struct {
	repo prepareRepository
	prober rawVideoProber
	coarseQueue stageEnqueuer
	coarseSegmentSec int
}

func newPrepareStageHandler(repo prepareRepository, prober rawVideoProber, coarseQueue stageEnqueuer, coarseSegmentSec int) *prepareStageHandler {
	if coarseSegmentSec <= 0 {
		coarseSegmentSec = 60
	}
	return &prepareStageHandler{repo: repo, prober: prober, coarseQueue: coarseQueue, coarseSegmentSec: coarseSegmentSec}
}

func (h *prepareStageHandler) Handle(ctx context.Context, task VectorStageTask) error {
	if task.TaskID == "" || task.VideoID == 0 || strings.TrimSpace(task.RawKey) == "" {
		return fmt.Errorf("invalid prepare task")
	}
	exists, err := h.repo.VideoExists(ctx, task.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	durationSec, err := h.prober.Probe(ctx, task.RawKey)
	if err != nil {
		return err
	}
	if durationSec <= 0 {
		return fmt.Errorf("invalid duration for hierarchical mode")
	}
	prefix := fmt.Sprintf("segments/coarse/video_%d/%s", task.VideoID, strings.TrimSpace(task.TaskID))
	segIdx := 0
	for startSec := 0; startSec < durationSec; startSec += h.coarseSegmentSec {
		endSec := startSec + h.coarseSegmentSec
		if endSec > durationSec {
			endSec = durationSec
		}
		if endSec <= startSec {
			continue
		}
		key := filepath.ToSlash(filepath.Join(prefix, fmt.Sprintf("seg_%03d_%d_%d.mp4", segIdx, startSec, endSec)))
		if err := h.repo.UpsertPending(ctx, persistence.VectorStageRecord{
			TaskID:       task.TaskID,
			VideoID:      task.VideoID,
			Stage:        vectorStageCoarseSegment,
			SegmentIndex: segIdx,
			StartSec:     startSec,
			EndSec:       endSec,
			ObjectKey:    key,
		}); err != nil {
			return err
		}
		segIdx++
	}
	if segIdx == 0 {
		return fmt.Errorf("no coarse segments")
	}
	if err := h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		Stage:   VectorStagePrepare,
		EndSec:  durationSec,
	}); err != nil {
		return err
	}
	return h.coarseQueue.Enqueue(ctx, VectorStageTask{
		TaskID: task.TaskID,
		VideoID: task.VideoID,
		RawKey: task.RawKey,
		Stage: VectorStageCoarse,
		EndSec: durationSec,
	})
}
```

- [ ] **Step 4: Add production repository/prober adapters**

In `stage_prepare.go`, add:

- GORM repository wrapper:
  - `VideoExists`
  - `UpsertPending`
  - `MarkComplete`
- Prober wrapper:
  - download raw video from object storage to temp file,
  - call `ff.ProbeDurationSeconds`,
  - update `EduVideoResource.duration`,
  - delete temp file.

- [ ] **Step 5: Register prepare handler**

Modify `app.go`:

```go
prepareHandler := newPrepareStageHandler(
	newGormPrepareRepository(db, stageRepo),
	newObjectStorageRawVideoProber(store, ff, tmpRoot, db),
	stageQueues.queue(VectorStageCoarse),
	coarseSegmentSec,
)
for i := 0; i < vectorStageWorkerCountFromConfig(cfg, VectorStagePrepare); i++ {
	app.Go(func(ctx context.Context) error {
		return runVectorStageWorker(ctx, VectorStagePrepare, stageQueues.queue(VectorStagePrepare), prepareHandler)
	})
}
```

- [ ] **Step 6: Run focused tests**

Run:

```bash
go test ./internal/worker/vectorworker -run TestPrepareStage
```

Expected: tests pass.

## Task 6: Implement Coarse Stage

**Files:**

- Create: `internal/worker/vectorworker/stage_coarse.go`
- Create: `internal/worker/vectorworker/stage_coarse_test.go`
- Modify: `internal/worker/vectorworker/task.go`
- Modify: `internal/worker/vectorworker/app.go`

- [ ] **Step 1: Add failing coarse stage test**

Create `internal/worker/vectorworker/stage_coarse_test.go`:

```go
package vectorworker

import (
	"context"
	"testing"

	"embedding-video/http/internal/infrastructure/persistence"
)

type coarseRepo struct {
	segments []persistence.VectorStageRecord
	complete []persistence.VectorStageRecord
}

func (r *coarseRepo) ListStage(_ context.Context, taskID string, stage string) ([]persistence.VectorStageRecord, error) {
	return r.segments, nil
}

func (r *coarseRepo) MarkComplete(_ context.Context, rec persistence.VectorStageRecord) error {
	r.complete = append(r.complete, rec)
	return nil
}

type fakeCoarseProcessor struct {
	called bool
}

func (p *fakeCoarseProcessor) ProcessCoarse(_ context.Context, task VectorStageTask, plan []persistence.VectorStageRecord) error {
	p.called = true
	return nil
}

func TestCoarseStageProcessesPlanAndEnqueuesRefine(t *testing.T) {
	repo := &coarseRepo{segments: []persistence.VectorStageRecord{
		{TaskID: "task-1", Stage: vectorStageCoarseSegment, SegmentIndex: 0, StartSec: 0, EndSec: 40},
	}}
	nextQueue := &recordingStageQueue{}
	processor := &fakeCoarseProcessor{}
	handler := newCoarseStageHandler(repo, processor, nextQueue)

	err := handler.Handle(context.Background(), VectorStageTask{TaskID: "task-1", VideoID: 9, RawKey: "raw/video.mp4", Stage: VectorStageCoarse})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !processor.called {
		t.Fatal("expected processor to be called")
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStageCoarse {
		t.Fatalf("coarse not complete: %+v", repo.complete)
	}
	if len(nextQueue.enqueued) != 1 || nextQueue.enqueued[0].Stage != VectorStageRefine {
		t.Fatalf("refine not enqueued: %+v", nextQueue.enqueued)
	}
}
```

- [ ] **Step 2: Run test and confirm it fails**

Run:

```bash
go test ./internal/worker/vectorworker -run TestCoarseStageProcessesPlanAndEnqueuesRefine
```

Expected:

```text
undefined: newCoarseStageHandler
```

- [ ] **Step 3: Extract existing coarse logic**

Modify `internal/worker/vectorworker/task.go`.

Extract the hierarchical coarse block into a helper:

```go
func processHierarchicalCoarseSegments(ctx context.Context, input hierarchicalCoarseInput) ([]tasks.CoarseItem, error)
```

The helper should contain the existing logic for:

1. clip jobs,
2. clip worker pool,
3. upload worker pool,
4. coarse ASR worker pool,
5. stage recorder complete/fail for `VectorStageCoarseClip` and `VectorStageCoarseASR`,
6. returning ordered `[]tasks.CoarseItem`.

Keep behavior unchanged.

- [ ] **Step 4: Implement coarse stage handler**

Create `internal/worker/vectorworker/stage_coarse.go`:

```go
package vectorworker

import (
	"context"
	"fmt"

	"embedding-video/http/internal/infrastructure/persistence"
)

type coarseStageRepository interface {
	ListStage(context.Context, string, string) ([]persistence.VectorStageRecord, error)
	MarkComplete(context.Context, persistence.VectorStageRecord) error
}

type coarseStageProcessor interface {
	ProcessCoarse(context.Context, VectorStageTask, []persistence.VectorStageRecord) error
}

type coarseStageHandler struct {
	repo coarseStageRepository
	processor coarseStageProcessor
	refineQueue stageEnqueuer
}

func newCoarseStageHandler(repo coarseStageRepository, processor coarseStageProcessor, refineQueue stageEnqueuer) *coarseStageHandler {
	return &coarseStageHandler{repo: repo, processor: processor, refineQueue: refineQueue}
}

func (h *coarseStageHandler) Handle(ctx context.Context, task VectorStageTask) error {
	if task.TaskID == "" || task.VideoID == 0 || task.RawKey == "" {
		return fmt.Errorf("invalid coarse task")
	}
	plan, err := h.repo.ListStage(ctx, task.TaskID, vectorStageCoarseSegment)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		return fmt.Errorf("coarse segment plan is empty")
	}
	if err := h.processor.ProcessCoarse(ctx, task, plan); err != nil {
		return err
	}
	if err := h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		Stage:   VectorStageCoarse,
		EndSec:  task.EndSec,
	}); err != nil {
		return err
	}
	return h.refineQueue.Enqueue(ctx, VectorStageTask{
		TaskID: task.TaskID,
		VideoID: task.VideoID,
		RawKey: task.RawKey,
		Stage: VectorStageRefine,
		EndSec: task.EndSec,
	})
}
```

- [ ] **Step 5: Add production coarse processor**

Production processor should:

1. Download raw video to a local temp file.
2. Convert persisted coarse plan rows to the helper input.
3. Call `processHierarchicalCoarseSegments`.
4. Persist coarse ASR text and object keys through the existing stage recorder/repository.
5. Delete local raw video.

- [ ] **Step 6: Register coarse handler**

Modify `app.go`:

```go
coarseHandler := newCoarseStageHandler(
	stageRepo,
	newProductionCoarseStageProcessor(db, store, ff, client, tmpRoot, asrWorkers, coarseWorkers, stageRecorder),
	stageQueues.queue(VectorStageRefine),
)
for i := 0; i < vectorStageWorkerCountFromConfig(cfg, VectorStageCoarse); i++ {
	app.Go(func(ctx context.Context) error {
		return runVectorStageWorker(ctx, VectorStageCoarse, stageQueues.queue(VectorStageCoarse), coarseHandler)
	})
}
```

- [ ] **Step 7: Run focused tests**

Run:

```bash
go test ./internal/worker/vectorworker -run 'TestCoarseStage|TestCalcCoarse'
```

Expected: tests pass.

## Task 7: Implement Refine Stage

**Files:**

- Create: `internal/worker/vectorworker/stage_refine.go`
- Create: `internal/worker/vectorworker/stage_refine_test.go`
- Modify: `internal/worker/vectorworker/task.go`
- Modify: `internal/worker/vectorworker/app.go`

- [ ] **Step 1: Add failing refine stage test**

Create `internal/worker/vectorworker/stage_refine_test.go`:

```go
package vectorworker

import (
	"context"
	"testing"

	"embedding-video/http/internal/infrastructure/persistence"
)

type refineRepo struct {
	coarse []persistence.VectorStageRecord
	complete []persistence.VectorStageRecord
}

func (r *refineRepo) ListStage(_ context.Context, taskID string, stage string) ([]persistence.VectorStageRecord, error) {
	return r.coarse, nil
}

func (r *refineRepo) MarkComplete(_ context.Context, rec persistence.VectorStageRecord) error {
	r.complete = append(r.complete, rec)
	return nil
}

type fakeRefineProcessor struct {
	called bool
}

func (p *fakeRefineProcessor) ProcessRefine(_ context.Context, task VectorStageTask, coarse []persistence.VectorStageRecord) error {
	p.called = true
	return nil
}

func TestRefineStageProcessesCoarseAndEnqueuesFinalize(t *testing.T) {
	repo := &refineRepo{coarse: []persistence.VectorStageRecord{
		{TaskID: "task-1", Stage: vectorStageCoarseSegment, SegmentIndex: 0, Text: "coarse text"},
	}}
	nextQueue := &recordingStageQueue{}
	processor := &fakeRefineProcessor{}
	handler := newRefineStageHandler(repo, processor, nextQueue)

	err := handler.Handle(context.Background(), VectorStageTask{TaskID: "task-1", VideoID: 9, RawKey: "raw/video.mp4", Stage: VectorStageRefine})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !processor.called {
		t.Fatal("expected processor to be called")
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStageRefine {
		t.Fatalf("refine not complete: %+v", repo.complete)
	}
	if len(nextQueue.enqueued) != 1 || nextQueue.enqueued[0].Stage != VectorStageFinalize {
		t.Fatalf("finalize not enqueued: %+v", nextQueue.enqueued)
	}
}
```

- [ ] **Step 2: Run test and confirm it fails**

Run:

```bash
go test ./internal/worker/vectorworker -run TestRefineStageProcessesCoarseAndEnqueuesFinalize
```

Expected:

```text
undefined: newRefineStageHandler
```

- [ ] **Step 3: Extract existing refine logic**

Modify `internal/worker/vectorworker/task.go`.

Extract the hierarchical LLM + refine ASR + embedding block into:

```go
func processHierarchicalRefine(ctx context.Context, input hierarchicalRefineInput) error
```

The helper should contain existing behavior for:

1. building LLM prompt,
2. calling LLM,
3. normalizing/repairing LLM segments,
4. upserting hierarchical segments,
5. calling `tasks.RefineSegmentsASRAndEmbed`,
6. stage recorder updates for existing sub-stages.

Keep behavior unchanged.

- [ ] **Step 4: Implement refine stage handler**

Create `internal/worker/vectorworker/stage_refine.go`:

```go
package vectorworker

import (
	"context"
	"fmt"

	"embedding-video/http/internal/infrastructure/persistence"
)

type refineStageRepository interface {
	ListStage(context.Context, string, string) ([]persistence.VectorStageRecord, error)
	MarkComplete(context.Context, persistence.VectorStageRecord) error
}

type refineStageProcessor interface {
	ProcessRefine(context.Context, VectorStageTask, []persistence.VectorStageRecord) error
}

type refineStageHandler struct {
	repo refineStageRepository
	processor refineStageProcessor
	finalizeQueue stageEnqueuer
}

func newRefineStageHandler(repo refineStageRepository, processor refineStageProcessor, finalizeQueue stageEnqueuer) *refineStageHandler {
	return &refineStageHandler{repo: repo, processor: processor, finalizeQueue: finalizeQueue}
}

func (h *refineStageHandler) Handle(ctx context.Context, task VectorStageTask) error {
	if task.TaskID == "" || task.VideoID == 0 || task.RawKey == "" {
		return fmt.Errorf("invalid refine task")
	}
	coarse, err := h.repo.ListStage(ctx, task.TaskID, vectorStageCoarseSegment)
	if err != nil {
		return err
	}
	if len(coarse) == 0 {
		return fmt.Errorf("coarse transcript list is empty")
	}
	if err := h.processor.ProcessRefine(ctx, task, coarse); err != nil {
		return err
	}
	if err := h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		Stage:   VectorStageRefine,
		EndSec:  task.EndSec,
	}); err != nil {
		return err
	}
	return h.finalizeQueue.Enqueue(ctx, VectorStageTask{
		TaskID: task.TaskID,
		VideoID: task.VideoID,
		RawKey: task.RawKey,
		Stage: VectorStageFinalize,
		EndSec: task.EndSec,
	})
}
```

- [ ] **Step 5: Add production refine processor**

Production processor should:

1. Download raw video to local temp file.
2. Convert coarse rows to `[]tasks.CoarseItem`.
3. Call `processHierarchicalRefine`.
4. Delete local raw video.

- [ ] **Step 6: Register refine handler**

Modify `app.go`:

```go
refineHandler := newRefineStageHandler(
	stageRepo,
	newProductionRefineStageProcessor(db, store, ff, client, tmpRoot, coarseSegmentSec, refineMinSegmentSec, refineMaxSegmentSec, llmModel, llmTimeoutMinutes, asrWorkers, embedBatch, embeddingDim, tailCfg, stageRecorder),
	stageQueues.queue(VectorStageFinalize),
)
for i := 0; i < vectorStageWorkerCountFromConfig(cfg, VectorStageRefine); i++ {
	app.Go(func(ctx context.Context) error {
		return runVectorStageWorker(ctx, VectorStageRefine, stageQueues.queue(VectorStageRefine), refineHandler)
	})
}
```

- [ ] **Step 7: Run focused tests**

Run:

```bash
go test ./internal/worker/vectorworker -run 'TestRefineStage|Test.*Hierarchical'
```

Expected: tests pass.

## Task 8: Implement Finalize Stage

**Files:**

- Create: `internal/worker/vectorworker/stage_finalize.go`
- Create: `internal/worker/vectorworker/stage_finalize_test.go`
- Modify: `internal/worker/vectorworker/app.go`

- [ ] **Step 1: Add failing finalize test**

Create `internal/worker/vectorworker/stage_finalize_test.go`:

```go
package vectorworker

import (
	"context"
	"testing"

	"embedding-video/http/internal/infrastructure/persistence"
)

type finalizeRepo struct {
	complete []persistence.VectorStageRecord
}

func (r *finalizeRepo) MarkComplete(_ context.Context, rec persistence.VectorStageRecord) error {
	r.complete = append(r.complete, rec)
	return nil
}

func TestFinalizeStageMarksComplete(t *testing.T) {
	repo := &finalizeRepo{}
	handler := newFinalizeStageHandler(repo)

	err := handler.Handle(context.Background(), VectorStageTask{TaskID: "task-1", VideoID: 9, Stage: VectorStageFinalize})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(repo.complete) != 1 || repo.complete[0].Stage != VectorStageFinalize {
		t.Fatalf("finalize not complete: %+v", repo.complete)
	}
}
```

- [ ] **Step 2: Run test and confirm it fails**

Run:

```bash
go test ./internal/worker/vectorworker -run TestFinalizeStageMarksComplete
```

Expected:

```text
undefined: newFinalizeStageHandler
```

- [ ] **Step 3: Implement finalize stage**

Create `internal/worker/vectorworker/stage_finalize.go`:

```go
package vectorworker

import (
	"context"
	"fmt"

	"embedding-video/http/internal/infrastructure/persistence"
)

type finalizeStageRepository interface {
	MarkComplete(context.Context, persistence.VectorStageRecord) error
}

type finalizeStageHandler struct {
	repo finalizeStageRepository
}

func newFinalizeStageHandler(repo finalizeStageRepository) *finalizeStageHandler {
	return &finalizeStageHandler{repo: repo}
}

func (h *finalizeStageHandler) Handle(ctx context.Context, task VectorStageTask) error {
	if task.TaskID == "" || task.VideoID == 0 {
		return fmt.Errorf("invalid finalize task")
	}
	return h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		Stage:   VectorStageFinalize,
		EndSec:  task.EndSec,
	})
}
```

- [ ] **Step 4: Register finalize handler**

Modify `app.go`:

```go
finalizeHandler := newFinalizeStageHandler(stageRepo)
for i := 0; i < vectorStageWorkerCountFromConfig(cfg, VectorStageFinalize); i++ {
	app.Go(func(ctx context.Context) error {
		return runVectorStageWorker(ctx, VectorStageFinalize, stageQueues.queue(VectorStageFinalize), finalizeHandler)
	})
}
```

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/worker/vectorworker -run TestFinalizeStage
```

Expected: tests pass.

## Task 9: Documentation And Verification

**Files:**

- Modify: `README.md`
- Modify: `PROJECT_PARAMETERS.md`
- Modify: `PROJECT_PARAMETERS_EN.md`

- [ ] **Step 1: Update README**

Add a concise section:

```markdown
### Vector Worker Stage Queues

In `hierarchical` mode, vectorization uses four Redis Stream stages:

- `video:vector:prepare`
- `video:vector:coarse`
- `video:vector:refine`
- `video:vector:finalize`

The coarse and refine stages keep internal concurrency with existing worker pools. Queue names are configured under `RedisKeys`, and consumer counts are configured under `VectorStageWorkers`.
```

- [ ] **Step 2: Update parameter documents**

Update both parameter docs:

- Add `RedisKeys.VectorPrepareQueue`.
- Add `RedisKeys.VectorCoarseQueue`.
- Add `RedisKeys.VectorRefineQueue`.
- Add `RedisKeys.VectorFinalizeQueue`.
- Add `VectorStageWorkers.Prepare`.
- Add `VectorStageWorkers.Coarse`.
- Add `VectorStageWorkers.Refine`.
- Add `VectorStageWorkers.Finalize`.
- Clarify that `WorkerPools` remains for internal ants pool concurrency.

- [ ] **Step 3: Run focused package tests**

Run:

```bash
go test ./internal/worker/vectorworker ./internal/infrastructure/redis ./internal/infrastructure/persistence ./internal/config
```

Expected: tests pass.

- [ ] **Step 4: Run full test suite**

Run:

```bash
go test ./...
```

Expected: tests pass.

- [ ] **Step 5: Check diffs**

Run:

```bash
git diff --check
```

Expected: no output.

## Final Verification

Run:

```bash
go test ./...
git diff --check
```

Expected:

- All Go tests pass.
- No whitespace errors.

## Implementation Notes

The critical design choice is to avoid queue explosion. Keep these boundaries:

1. Redis queues represent durable large stages.
2. Existing ants pools represent internal parallelism within a stage.
3. Only split a finer queue later if metrics show that one internal node is a standalone scaling bottleneck.

Do not add `coarse.clip`, `coarse.asr`, `segment.llm`, `refine.asr`, or `embedding` queues in this implementation.
