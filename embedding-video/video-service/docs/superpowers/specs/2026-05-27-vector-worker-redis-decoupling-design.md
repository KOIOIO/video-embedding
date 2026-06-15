# Vector Worker Redis Decoupling Design

> Status: Superseded. This document records an earlier fine-grained Redis decoupling proposal. The current implementation follows `docs/superpowers/specs/2026-06-06-vector-worker-full-redis-stage-decoupling-design.md`, which uses four coarse Redis stages: `vector.prepare`, `vector.coarse`, `vector.refine`, and `vector.finalize`. Do not use the fine-grained queue examples in this document as current implementation guidance.

## Summary

The current `vector_worker` already receives top-level vectorization tasks from Redis Streams, but the internal processing path is still a single in-process pipeline. In hierarchical mode, one worker function downloads the video, probes duration, creates coarse clips, uploads them, runs coarse ASR, calls the LLM for fine segmentation, writes segment drafts, refines ASR, runs embedding, and updates the final database rows.

This design decouples that path with Redis Streams stage queues. Redis should coordinate work and retries. Durable intermediate state should live in PostgreSQL and object storage, not only in Redis messages.

The first implementation should be incremental. Start by fixing queue reliability and splitting the coarse pipeline, then split refine ASR and embedding after the stage model is proven.

## Problem

The current vector worker has two different levels of queueing:

1. A Redis Stream queue for top-level vectorize tasks.
2. In-process channels and ants pools for internal nodes such as coarse clip, upload, coarse ASR, refine ASR, and embedding.

This creates several operational problems:

1. A long video occupies one worker for the whole pipeline.
2. Coarse and refine sub-steps cannot scale independently across worker processes.
3. A process crash loses in-memory stage progress.
4. Retries replay more work than necessary.
5. Internal concurrency is limited to the lifecycle of one top-level task.
6. Queue visibility is shallow: Redis shows only top-level pending vector tasks, not stage-level backlog.

There is also a reliability issue in the current top-level vector queue: `VectorizeQueue.Dequeue` ACKs and deletes a message immediately after JSON parsing. If the worker exits after parsing but before processing completes, the task is lost.

## Goals

1. Decouple vector worker processing nodes with Redis Streams.
2. Allow coarse clip/upload/ASR, LLM segmentation, refine ASR, embedding, and finalization to scale independently.
3. Make each stage retryable and idempotent.
4. Preserve the existing hierarchical segmentation behavior.
5. Keep large intermediate data out of Redis messages.
6. Make stage progress inspectable by task, video, stage, and index.
7. Fix vector queue ACK semantics so messages are acknowledged only after successful processing or explicit terminal handling.
8. Support incremental migration without requiring a full rewrite in one change.

## Non-Goals

1. Do not redesign the LLM segmentation prompt.
2. Do not change ASR or embedding provider behavior.
3. Do not change the public upload or recommendation APIs.
4. Do not require Redis to store video, audio, transcript, or embedding payloads.
5. Do not add a second queue technology.
6. Do not split every helper function into a separate worker process in the first implementation.
7. Do not remove existing `full` or `sample` mode behavior in this design.

## Existing Context

Relevant files:

- `internal/worker/vectorworker/app.go`
  - Registers the vector worker.
  - Creates the top-level Redis queue.
  - Starts workers that call `handleVectorizeTask`.
- `internal/worker/vectorworker/task.go`
  - Contains the main hierarchical path.
  - Uses channels and ants pools for coarse clip, upload, and ASR.
  - Calls LLM segmentation and segment draft persistence.
  - Calls refine ASR and embedding.
- `internal/worker/vectorworker/tasks/asr.go`
  - Runs refine ASR, summary alignment, embedding, and final segment updates.
- `internal/infrastructure/redis/transcode.go`
  - Contains the current Redis Stream queue implementations.
  - `TranscodeQueue` supports explicit ACK, retry, and DLQ.
  - `VectorizeQueue` currently ACKs immediately after parsing.
- `configs/video.yml` and `configs/video_prod.yml`
  - Configure worker pool sizes.

The existing database table `edu_video_segment` already acts as durable state for LLM draft and final segment rows. Object storage already stores coarse segment clips under deterministic keys.

## Proposed Architecture

Introduce a stage-based vector pipeline coordinated by Redis Streams:

```text
video:vectorize:queue
  -> vector.prepare
  -> vector.coarse.clip
  -> vector.coarse.asr
  -> vector.segment.llm
  -> vector.refine.asr
  -> vector.embedding
  -> vector.finalize
```

Each stage worker should:

1. Consume one stage message.
2. Check durable state to decide whether the work is already complete.
3. Execute only its own stage.
4. Persist its output.
5. Enqueue the next required stage message or mark aggregate readiness.
6. ACK the input message only after durable output and next-stage enqueue succeed.

## Stage Responsibilities

### `vector.prepare`

Input:

- `task_id`
- `video_id`
- `raw_key`

Responsibilities:

1. Validate the video still exists and is not deleted.
2. Download or probe the raw video as needed.
3. Persist video duration to `edu_video_resource`.
4. Create deterministic coarse segment metadata.
5. Enqueue one `vector.coarse.clip` message per coarse segment.

Durable output:

- Video duration in PostgreSQL.
- Stage state rows for each coarse segment job.

### `vector.coarse.clip`

Input:

- `task_id`
- `video_id`
- `raw_key`
- `segment_index`
- `start_sec`
- `end_sec`
- deterministic coarse object key

Responsibilities:

1. Extract the coarse video clip and audio.
2. Upload the coarse video clip to object storage.
3. Persist the coarse object key and status.
4. Enqueue `vector.coarse.asr`.

Durable output:

- Coarse clip object key.
- Coarse clip stage status.

The audio file should not be passed through Redis. It can be a local temporary file used within this stage, or it can be uploaded if the ASR stage must run on a different process without re-extracting audio. The first implementation should prefer re-extracting audio from the deterministic coarse clip or raw video to avoid storing extra audio artifacts.

### `vector.coarse.asr`

Input:

- `task_id`
- `video_id`
- `segment_index`
- `start_sec`
- `end_sec`
- coarse object key or raw key

Responsibilities:

1. Produce transcript text for one coarse segment.
2. Persist normalized transcript text.
3. Mark the coarse ASR segment complete.
4. If all coarse ASR segments are complete for the task, enqueue one `vector.segment.llm` message.

Durable output:

- Coarse transcript by `task_id + segment_index`.
- Coarse ASR stage status.

### `vector.segment.llm`

Input:

- `task_id`
- `video_id`
- `raw_key`
- `duration_sec`

Responsibilities:

1. Load all complete coarse transcripts in index order.
2. Build the existing hierarchical segmentation prompt.
3. Call LLM segmentation.
4. Normalize and repair LLM segments with the existing logic.
5. Persist segment drafts to `edu_video_segment`.
6. Enqueue one `vector.refine.asr` message per unfinished segment.

Durable output:

- Draft rows in `edu_video_segment`.
- Optional stage metadata containing LLM raw output preview or normalized segment count.

This stage should remain a single aggregate stage because LLM segmentation needs the full ordered coarse transcript set.

### `vector.refine.asr`

Input:

- `task_id`
- `video_id`
- `raw_key`
- `segment_id`
- `segment_index`
- `start_sec`
- `end_sec`
- `next_start_sec`
- `boundary_confidence`

Responsibilities:

1. Build refine input using existing selective refine behavior.
2. Run refine ASR only when the current logic requires it.
3. Persist final embedding input, final summary, and final time boundaries for the segment.
4. Mark the segment refine stage complete.
5. If enough refined inputs are ready, enqueue `vector.embedding`.

Durable output:

- Refined embedding input.
- Final summary.
- Final start/end boundaries.
- Refine stage status.

This stage should not update final `embedding` or `status = 1`. That remains the embedding stage's responsibility.

### `vector.embedding`

Input:

- `task_id`
- `video_id`
- optional batch cursor or segment ID list

Responsibilities:

1. Load refined inputs that are ready and not embedded.
2. Call embedding in batches using existing `EmbedBatch`.
3. Update segment summary, final boundaries, embedding, and `status = 1` in one transaction per batch.
4. Enqueue `vector.finalize` when all required segments are embedded.

Durable output:

- Final `edu_video_segment` rows with embedding and status.

Embedding should stay batched. Splitting this into one Redis message per segment would reduce provider efficiency and increase cost.

### `vector.finalize`

Input:

- `task_id`
- `video_id`

Responsibilities:

1. Verify all expected segments are complete.
2. Mark task-level vectorization status complete.
3. Clean temporary stage keys or short-lived Redis metadata if needed.
4. Emit final logs and metrics.

Durable output:

- Task-level completion state.

## Message Model

Use one shared stage message envelope:

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

Redis messages should stay small. They should contain references and routing data, not transcript bodies, embeddings, or binary paths that only exist on one worker's local filesystem.

## Queue Model

Use Redis Streams with one stream per stage:

- `video:vector:prepare`
- `video:vector:coarse:clip`
- `video:vector:coarse:asr`
- `video:vector:segment:llm`
- `video:vector:refine:asr`
- `video:vector:embedding`
- `video:vector:finalize`

Each stream should have a stable consumer group derived from the key, matching the existing queue naming style.

The queue abstraction should support:

1. `Enqueue`
2. `Dequeue` returning message ID plus payload
3. `Ack`
4. `Requeue`
5. `MoveToDeadLetter`

The new queue abstraction can be shared by vector stages and eventually replace the duplicated transcode/vector queue code.

## Durable State Model

The pipeline needs durable stage state. Redis Streams alone are not enough because aggregate stages need to know whether all indexed child jobs are done.

Preferred first implementation:

1. Add a small PostgreSQL stage table for vector task state.
2. Keep `edu_video_segment` as the durable store for draft and final segment rows.
3. Keep object storage as the durable store for coarse clips.

Suggested table:

```sql
CREATE TABLE edu_video_vector_stage (
	id BIGSERIAL PRIMARY KEY,
	task_id TEXT NOT NULL,
	video_id BIGINT NOT NULL,
	stage TEXT NOT NULL,
	segment_index INT NOT NULL DEFAULT 0,
	segment_id BIGINT NOT NULL DEFAULT 0,
	status SMALLINT NOT NULL DEFAULT 0,
	object_key TEXT NOT NULL DEFAULT '',
	text TEXT NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	retry_count INT NOT NULL DEFAULT 0,
	start_time INT NOT NULL DEFAULT 0,
	end_time INT NOT NULL DEFAULT 0,
	create_time TIMESTAMP NOT NULL DEFAULT now(),
	update_time TIMESTAMP NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_video_vector_stage_unique
ON edu_video_vector_stage(task_id, stage, segment_index, segment_id);
```

Status values:

- `0`: pending
- `1`: processing
- `2`: complete
- `3`: failed
- `4`: skipped

If adding a table is too much for the first patch, Redis hashes can be used temporarily for stage metadata. That is less durable and harder to query, so it should not be the long-term design.

## ACK, Retry, and DLQ Semantics

All vector queues should follow this rule:

1. Do not ACK after parse.
2. ACK only after the stage's durable output is written and next-stage enqueue succeeds.
3. On retryable errors, enqueue a retry message with incremented `RetryCount`, then ACK the original message.
4. On terminal errors, write stage failure state, move the message to the stage DLQ, then ACK the original message.
5. Invalid payloads can be ACKed and DLQed immediately because no valid task can be recovered.

The first implementation must fix the existing `VectorizeQueue.Dequeue` behavior before adding more stage queues.

Retry classification should reuse `DecideVectorAIRetry` where applicable. Stage-specific non-AI errors, such as missing video, invalid duration, and malformed payloads, should be classified separately.

## Idempotency

Every stage must be safe to process more than once.

Required idempotency rules:

1. `vector.prepare` should recreate the same child jobs for the same `task_id`.
2. `vector.coarse.clip` should use deterministic object keys and treat existing successful output as complete.
3. `vector.coarse.asr` should skip ASR when a complete transcript already exists.
4. `vector.segment.llm` should reuse existing unfinished or finished segment drafts where current resume logic already does so.
5. `vector.refine.asr` should skip segments whose refine input is already complete.
6. `vector.embedding` should update only rows still missing embedding or `status = 0`.
7. `vector.finalize` should be repeatable.

These rules are more important than avoiding duplicate Redis messages. The design should assume duplicate delivery can happen.

## Concurrency and Aggregation

Fan-out stages:

- `vector.coarse.clip`
- `vector.coarse.asr`
- `vector.refine.asr`

Aggregate stages:

- `vector.segment.llm`
- `vector.embedding`
- `vector.finalize`

Readiness checks should use PostgreSQL counts or transactional updates, not in-memory counters. For example, after a coarse ASR segment completes, the worker can check whether all `vector.coarse.asr` rows for the task are complete. If yes, it enqueues `vector.segment.llm`.

To avoid duplicate aggregate enqueue, use an idempotent stage state row for the aggregate stage. Insert or update it transactionally before enqueueing.

## Configuration

Keep the existing `WorkerPools` names initially:

- `vector.coarse`
- `vector.sample_asr`
- `vector.refine_asr`

Add stage-specific names only when independent scaling is implemented:

- `vector.prepare`
- `vector.coarse_clip`
- `vector.coarse_asr`
- `vector.segment_llm`
- `vector.embedding`
- `vector.finalize`

Default sizes should preserve current behavior unless explicitly configured.

## Migration Plan

### Phase 1: Queue Reliability and Stage Infrastructure

1. Introduce a generic Redis Stream queue abstraction with explicit ACK.
2. Change vector top-level queue to return message IDs and ACK after task handling.
3. Add `VectorStageTask` and stage queue keys.
4. Add durable stage state repository.
5. Add tests for ACK-after-success, retry, and DLQ behavior.

This phase can run the current monolithic handler unchanged after the ACK fix.

### Phase 2: Coarse Pipeline Decoupling

1. Split prepare and coarse child job creation out of `handleVectorizeTask`.
2. Implement `vector.coarse.clip`.
3. Implement `vector.coarse.asr`.
4. Persist coarse transcripts.
5. Trigger `vector.segment.llm` only when all coarse ASR rows are complete.

This phase gives the largest scaling benefit while leaving LLM/refine/embedding behavior mostly unchanged.

### Phase 3: LLM and Refine Decoupling

1. Move LLM segmentation into `vector.segment.llm`.
2. Persist enough hints for selective refine.
3. Implement `vector.refine.asr` per segment.
4. Persist final refine inputs and summaries.

This phase removes the biggest remaining long-running in-process stage.

### Phase 4: Embedding and Finalization

1. Implement batched `vector.embedding`.
2. Update final segment rows in transactions.
3. Implement `vector.finalize`.
4. Add metrics for stage backlog, stage latency, retries, and DLQ counts.

## Error Handling

Stage errors should be recorded with:

- task ID
- video ID
- stage
- segment index or segment ID
- retry count
- short error message

For a failed child stage, the task should not proceed to aggregate stages. Terminal child failure should mark the task-level pipeline failed or degraded according to a policy chosen during implementation.

Initial policy:

1. Missing or deleted video: skip task as terminal success.
2. Invalid duration: terminal failure.
3. ASR/LLM/embedding upstream outage: retry with backoff.
4. Malformed LLM output: retry according to current logic, then terminal failure.
5. Object storage upload/download failure: retry with backoff.

## Testing Plan

### Unit Tests

1. Generic Redis queue does not ACK on dequeue.
2. Generic Redis queue ACKs only when `Ack` is called.
3. Requeue writes a new message and ACKs the old message.
4. DLQ writes failure payload and ACKs the old message.
5. Stage task JSON round-trips.
6. Stage readiness checks enqueue aggregate stages once.
7. Idempotent stage handlers skip already complete work.

### Focused Package Tests

Run focused tests after each phase:

```bash
go test ./internal/infrastructure/redis
go test ./internal/worker/vectorworker
go test ./internal/worker/vectorworker/tasks
```

If database repository code is added, include its package tests.

### Full Verification

When all phases are implemented:

```bash
go test ./...
```

Runtime smoke tests that boot the worker may require PostgreSQL, Redis, object storage, FFmpeg, and provider credentials, so they should be run only in an environment that has those dependencies.

## Implementation Decisions

1. Store stage metadata in PostgreSQL from the first implementation. Redis hashes are not durable or queryable enough for the main design.
2. Let `vector.coarse.asr` re-extract audio from the deterministic coarse clip or raw video. Do not add persisted audio artifacts in the first implementation.
3. Keep task-level vectorization status internal at first. Do not change public APIs until there is a separate product requirement for exposing vector progress.

## Recommendation

Use the staged migration path.

The first patch should not attempt to split all processing nodes. It should fix the unsafe ACK behavior, add the reusable queue abstraction, and introduce durable stage state. After that, split the coarse pipeline first because it is naturally fan-out/fan-in and gives the clearest scaling benefit with the least change to segmentation semantics.
