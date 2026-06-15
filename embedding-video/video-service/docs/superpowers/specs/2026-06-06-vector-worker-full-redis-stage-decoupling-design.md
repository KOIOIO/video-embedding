# Vector Worker Redis Stage Decoupling Design

> Status: Current implementation design. As of 2026-06-06, the code follows this four-stage Redis pipeline. Older documents that mention per-node queues such as `vector.coarse.clip`, `vector.coarse.asr`, `vector.segment.llm`, `vector.refine.asr`, or `vector.embedding` are historical references only.

## Summary

The vector worker should be decoupled with Redis Streams, but not every internal function needs its own queue. The right first step is a coarse-grained stage pipeline that separates the long-running hierarchical vectorization flow into four durable queues:

```text
video:vectorize:queue
  -> video:vector:prepare
  -> video:vector:coarse
  -> video:vector:refine
  -> video:vector:finalize
```

This keeps the architecture lighter while still solving the main operational problems: long task ownership, crash recovery, stage-level visibility, and independent scaling of the biggest processing blocks.

## Why Not Split Every Node

Splitting every node into separate queues, for example `coarse.clip`, `coarse.asr`, `segment.llm`, `refine.asr`, and `embedding`, is possible but too heavy for the first implementation.

Risks of over-splitting:

1. Too many queues, consumer groups, DLQs, and metrics to operate.
2. More fan-in/fan-out coordination logic.
3. More idempotency surfaces and half-success states.
4. More Redis scheduling overhead without guaranteed throughput gain.
5. Larger code surface before production bottlenecks are measured.

Most runtime cost comes from FFmpeg, ASR, LLM, embedding, database writes, and object storage. Redis queue granularity should follow actual scaling bottlenecks, not every helper boundary.

## Existing Foundation

The project already has useful primitives:

- `internal/infrastructure/redis/stream_queue.go`
  - Generic Redis Stream queue with explicit ACK, requeue, and DLQ support.
- `internal/infrastructure/redis/transcode.go`
  - Top-level vector queue returns message IDs and supports ACK after processing.
- `internal/worker/vectorworker/stage_task.go`
  - Existing stage task envelope and some stage constants.
- `internal/infrastructure/persistence/vector_stage_repository.go`
  - Stage status upsert, complete, fail, and count primitives.
- `internal/worker/vectorworker/stage_recorder.go`
  - Stage progress recorder.
- `model.EduVideoVectorStage`
  - Durable stage status table.

The implementation should reuse these primitives and reduce the planned queue set.

## Goals

1. Split hierarchical vectorization into four Redis-backed stages.
2. Keep Redis messages small and durable.
3. Store real state in PostgreSQL and object storage.
4. ACK only after stage output and next-stage enqueue succeed.
5. Keep each stage idempotent.
6. Preserve current hierarchical behavior.
7. Keep `full` and `sample` modes on the existing monolithic path.
8. Avoid adding unnecessary queues before bottlenecks are proven.

## Non-Goals

1. Do not redesign ASR, LLM, embedding, or prompts.
2. Do not change public HTTP APIs.
3. Do not add another queue technology.
4. Do not store audio, video, transcript arrays, or embeddings in Redis.
5. Do not immediately split coarse clip, coarse ASR, LLM, refine ASR, and embedding into separate queues.

## Queue Model

### Top-Level Queue

Queue: `video:vectorize:queue`

Existing upload flow keeps publishing top-level vectorization tasks here.

In hierarchical mode, this queue becomes an adapter:

1. Consume `VectorizeTask`.
2. Upsert `vector.prepare` pending state.
3. Enqueue `video:vector:prepare`.
4. ACK the top-level message.

For `full` and `sample`, keep the existing monolithic execution path.

### `vector.prepare`

Queue: `video:vector:prepare`

Responsibilities:

1. Validate the video exists and is not deleted.
2. Download/probe the raw video as needed.
3. Persist duration when available.
4. Compute coarse segment plan.
5. Persist coarse plan rows in `edu_video_vector_stage`.
6. Enqueue one `vector.coarse` task.

This stage is intentionally not one message per coarse segment. It creates the durable plan, then hands the full coarse block to the next stage.

### `vector.coarse`

Queue: `video:vector:coarse`

Responsibilities:

1. Load the coarse segment plan.
2. Use the existing internal worker pools to:
   - cut coarse clips,
   - upload coarse clips,
   - run coarse ASR.
3. Persist coarse transcript text and object keys per segment in `edu_video_vector_stage`.
4. Enqueue one `vector.refine` task.

This keeps the current in-process parallelism where it is useful, while still making the whole coarse block durable and restartable as a stage.

### `vector.refine`

Queue: `video:vector:refine`

Responsibilities:

1. Load completed coarse transcripts.
2. Run the existing LLM segmentation logic.
3. Persist draft `edu_video_segment` rows.
4. Run refine ASR, tail alignment, boundary alignment, and embedding using existing internal concurrency.
5. Persist final segment rows and embeddings.
6. Enqueue one `vector.finalize` task.

This combines LLM, refine ASR, and embedding in one Redis stage because those steps share segment context and currently have intertwined helper logic. If production metrics later show embedding or refine ASR as independent bottlenecks, this stage can be split further.

### `vector.finalize`

Queue: `video:vector:finalize`

Responsibilities:

1. Verify required stage output exists.
2. Mark final vectorization stage complete.
3. Update runtime counters and final logs.
4. Leave task in a state that recommendation/search can use.

## Message Contract

Use `VectorStageTask` with the reduced stage set:

```go
type VectorStageTask struct {
    TaskID       string `json:"task_id"`
    VideoID      uint64 `json:"video_id"`
    RawKey       string `json:"raw_key,omitempty"`
    Stage        string `json:"stage"`
    SegmentIndex int    `json:"segment_index,omitempty"`
    SegmentID    uint64 `json:"segment_id,omitempty"`
    StartSec     int    `json:"start_sec,omitempty"`
    EndSec       int    `json:"end_sec,omitempty"`
    NextStartSec int    `json:"next_start_sec,omitempty"`
    ObjectKey    string `json:"object_key,omitempty"`
    RetryCount   int    `json:"retry_count,omitempty"`
}
```

For coarse-grained stages, most messages only need:

- `task_id`
- `video_id`
- `raw_key`
- `stage`
- `retry_count`

Segment-level fields remain useful for durable stage rows and future finer splitting.

## Durable State

`edu_video_vector_stage` stores stage state for both coarse stages and per-segment progress:

- `vector.prepare`
- `vector.coarse`
- `vector.coarse.segment`
- `vector.refine`
- `vector.finalize`

Recommended usage:

1. `vector.prepare`: one row for the prepare stage.
2. `vector.coarse.segment`: one row per coarse segment, storing `segment_index`, `start_time`, `end_time`, `object_key`, and coarse transcript text.
3. `vector.coarse`: one aggregate row for the coarse block.
4. `vector.refine`: one aggregate row for LLM/refine/embedding.
5. `vector.finalize`: one aggregate row.

`edu_video_segment` remains the durable source for final segment data and embeddings.

## Configuration

Add queue keys:

```yaml
RedisKeys:
  VectorPrepareQueue: "video:vector:prepare"
  VectorCoarseQueue: "video:vector:coarse"
  VectorRefineQueue: "video:vector:refine"
  VectorFinalizeQueue: "video:vector:finalize"
```

Add worker counts:

```yaml
VectorStageWorkers:
  Prepare: 1
  Coarse: 2
  Refine: 2
  Finalize: 1
```

Keep `WorkerPools` for internal ants pools used inside coarse and refine stages.

## Retry And DLQ

Each queue uses:

1. Explicit ACK.
2. Requeue for retryable errors.
3. DLQ stream with `:dlq` suffix for terminal errors.
4. `retry_count` in the message.
5. stage status updates in `edu_video_vector_stage`.

ACK rule:

```text
ACK only after current stage output is durable and the next stage message is enqueued.
```

## Migration Plan

1. Add coarse-grained queue config and worker counts.
2. Add queue factory and generic stage runner.
3. Change top-level hierarchical vector queue handling to enqueue `vector.prepare`.
4. Implement `vector.prepare`.
5. Implement `vector.coarse` using existing coarse clip/upload/ASR logic internally.
6. Implement `vector.refine` using existing LLM/refine ASR/embedding logic internally.
7. Implement `vector.finalize`.
8. Keep `full` and `sample` monolithic.
9. Add metrics/logging per stage.
10. Only split additional queues if production metrics show clear bottlenecks.

## Future Split Points

If metrics justify it later:

1. Split `vector.coarse` into `vector.coarse.clip` and `vector.coarse.asr`.
2. Split `vector.refine` into `vector.segment.llm`, `vector.refine.asr`, and `vector.embedding`.
3. Add batching for embedding stage.

These should be follow-up changes, not first implementation scope.

## Testing Strategy

Unit tests:

- Stage queue key defaults and overrides.
- `VectorStageWorkers` defaults and overrides.
- Top-level adapter enqueues prepare.
- Prepare stage writes coarse plan and enqueues coarse.
- Coarse stage processes a fake plan and enqueues refine.
- Refine stage persists fake final segments and enqueues finalize.
- Finalize stage marks completion.

Integration-style tests:

- Fake Redis queues.
- Fake object storage, FFmpeg, ASR, LLM, and embedding providers.
- Drive one hierarchical task through:

```text
prepare -> coarse -> refine -> finalize
```

Verification commands:

```bash
go test ./internal/worker/vectorworker ./internal/infrastructure/redis ./internal/infrastructure/persistence ./internal/config
go test ./...
```

## Recommendation

Use the four-queue design first. It is the best balance between resilience, visibility, scalability, and implementation weight for the current project.
