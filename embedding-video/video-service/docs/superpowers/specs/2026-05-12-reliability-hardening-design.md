# Reliability Hardening Design For video-service

## Summary

This design hardens `video-service` against eight operational failure scenarios without rebuilding the platform architecture:

1. transcode storm
2. Redis backlog
3. worker crash recovery
4. ffmpeg zombie processes
5. large upload interruption
6. S3 timeout
7. hot video traffic
8. API rate limiting

The recommended approach is a medium-scope reliability layer upgrade around the existing HTTP upload, Redis Stream queue, worker execution, object storage, and playback/read APIs. The current domain service structure remains in place. We do not split the system into separate microservices or replace the queueing/storage model.

## Current State

The current implementation has several behavior constraints that directly shape the design:

1. `internal/infrastructure/redis/transcode.go` uses Redis Streams, but `Dequeue()` immediately calls `XACK` and `XDEL` before the task is processed. This means worker crashes can lose tasks instead of allowing recovery from pending entries.
2. `internal/application/videoapp/upload_http.go` performs a single multipart upload request, writes the full file locally, then finalizes by uploading the raw file to object storage and enqueueing transcode work. There is no resumable upload session.
3. `internal/application/videoapp/worker.go` runs the entire task in one pass and marks failures directly, but there is no lease/heartbeat model for in-flight ownership and no restart reclaim flow.
4. `internal/infrastructure/objectstorage/rustfs.go` delegates directly to the MinIO client with no shared timeout/retry policy per operation type.
5. `internal/infrastructure/transcode/ffmpeg_transcoder.go` uses `exec.CommandContext`, which gives basic timeout cancellation, but does not add explicit process-group cleanup, orphan auditing, or a janitor for temp artifacts.
6. `internal/http/router/router.go` does not currently expose rate-limiting middleware, and the read path does not show an explicit hot-content cache or request coalescing strategy.

## Goals

1. Prevent task loss during worker crashes or temporary Redis/S3 instability.
2. Bound system degradation during burst uploads and transcode storms.
3. Add a resumable path for large uploads without replacing the existing upload API.
4. Ensure ffmpeg work is cancellable and temp resources are eventually reclaimed.
5. Reduce load amplification from hot videos and repeated metadata requests.
6. Add shared, Redis-backed rate limiting suitable for multi-instance deployment.

## Non-Goals

1. Replacing Redis Streams with Kafka, RabbitMQ, or another queue system.
2. Splitting HTTP, upload, transcode, and playback into separate deployable services.
3. Rebuilding the video domain model from scratch.
4. Implementing a full workflow/orchestration engine.
5. Replacing the current small-file multipart upload path for all clients immediately.

## Design Overview

The system will be hardened through five focused changes:

1. make Redis Stream consumption at-least-once instead of acknowledge-on-read
2. add worker lease, heartbeat, reclaim, retry, and dead-letter handling
3. wrap ffmpeg and object storage operations with bounded execution policies
4. add resumable upload sessions for large files and unstable networks
5. add hot-video caching and Redis-backed API rate limiting

These changes preserve the current service boundaries:

1. HTTP handlers remain responsible for request parsing and response shape.
2. `videoapp.Service` remains the main upload/business entry point.
3. `videoapp.Worker` remains the task executor, but with richer queue/task lifecycle integration.
4. Redis continues to hold queue, short-lived state, leases, counters, cache, and rate-limit data.

## Detailed Design

### 1. Transcode Storm Control

#### Problem

When many uploads arrive together, every completed upload is immediately enqueued for transcode. Fixed worker goroutines alone do not protect CPU, memory, local disk, Redis lag, or object storage from overload.

#### Changes

1. Introduce queue admission checks before immediately moving a new upload into active transcode readiness.
2. Add a bounded execution semaphore inside the transcode worker so only a safe number of ffmpeg tasks can execute concurrently, even if more consumer goroutines are polling.
3. Split transcode work into at least two classes:
   - normal
   - large
4. Route large files to a separate stream or separate execution lane so large tasks do not starve normal uploads.
5. Add system overload thresholds based on:
   - queue backlog length
   - pending message count
   - active lease count
   - recent S3 timeout rate

#### Behavior

1. Under moderate load, upload completion still enqueues normally.
2. Under heavy load, uploads are accepted but task status becomes `queued` rather than directly entering active processing.
3. Under severe load, the system may reject new transcode admission with a retryable overload response while preserving already accepted tasks.

#### Why this design

This keeps the external behavior close to the current API while preventing the worker tier from becoming an unbounded CPU and IO amplifier.

### 2. Redis Backlog And Reliable Consumption

#### Problem

Current stream consumption acknowledges tasks before processing. This defeats Redis Stream pending recovery and makes crash recovery impossible for transcode tasks.

#### Changes

1. Change `TranscodeQueue.Dequeue()` to return both the task payload and the stream message ID.
2. Remove the immediate `ackAndDelete()` call from dequeue success.
3. Add explicit queue methods:
   - `Ack(ctx, messageID)`
   - `Requeue(ctx, messageID, retryMeta)` or an equivalent retry publish path
   - `ClaimStale(ctx, minIdle)` for reclaiming orphaned pending tasks
   - `MoveToDeadLetter(ctx, messageID, reason)`
4. Keep malformed payloads acked and dropped immediately because they are unrecoverable poison messages.
5. Track retry metadata in message fields or a side-channel Redis key:
   - retry count
   - first enqueue time
   - last error stage
   - last error summary

#### Retry Policy

1. Retry transient failures such as timeout, temporary network failure, object storage 5xx, and Redis transport issues.
2. Do not retry clearly permanent failures such as missing source object, invalid media, or repeated ffmpeg decode errors beyond threshold.
3. Send tasks that exceed retry count to a dead-letter stream.

#### Why this design

This is the smallest change that turns the current stream usage into a real recoverable queue while keeping Redis as the only queue dependency.

### 3. Worker Crash Recovery

#### Problem

Even after changing queue acknowledgment timing, the worker still needs a safe ownership model for in-flight tasks and a way to recover tasks abandoned during crashes or process restarts.

#### Changes

1. Add a lease key per active task, for example `video:transcode:lease:<taskID>`.
2. Lease value contains:
   - worker ID
   - stream message ID
   - video ID
   - current stage
   - started at
3. Lease keys use TTL and are renewed periodically while the task is active.
4. Worker startup runs a recovery pass:
   - reclaim stale pending stream messages using idle time
   - scan DB or Redis task states for `processing` entries with no valid lease
5. Add a structured task state machine:
   - `uploaded`
   - `queued`
   - `processing`
   - `retry_wait`
   - `done`
   - `failed`
   - `dead`
6. Store stage-specific failure information so recovery can tell whether a full rerun is required.

#### Recovery Rules

1. If a task is pending in Redis and the lease is stale or absent, reclaim it.
2. If a task is marked `processing` but there is no valid lease and no pending message, create a compensating retry task if retries remain.
3. If a task exceeded retry threshold, move it to `dead` and publish to the dead-letter stream.

#### Why this design

Lease plus pending reclaim is enough to recover from worker crashes without introducing a separate orchestration system.

### 4. ffmpeg Zombie And Temp Resource Control

#### Problem

`exec.CommandContext` provides baseline cancellation but does not explicitly guarantee cleanup of the full process tree, docker-mode children, or stale temp directories.

#### Changes

1. Wrap ffmpeg invocation in an execution helper that:
   - starts the command in its own process group when supported
   - captures start time and execution metadata
   - kills the full process group on timeout or cancellation
2. For docker execution mode, record container/process identity and ensure forced cleanup on cancellation path.
3. Add a worker-local janitor for temp resources:
   - remove old raw temp files
   - remove old HLS output directories
   - clean abandoned task workdirs older than a configured threshold
4. Add task execution metadata logging:
   - task ID
   - video ID
   - ffmpeg mode
   - pid or container marker
   - stage
   - duration
5. Continue to use per-task timeout, but separate overall task timeout from per-stage limits where useful.

#### Why this design

This is sufficient to address zombie/orphan process risk and disk residue without replacing the transcoder implementation.

### 5. Large Upload Interruption And Resumable Uploads

#### Problem

The current multipart upload API is all-or-nothing. Any network interruption forces the client to restart from byte zero.

#### Changes

1. Add upload session APIs alongside the existing upload endpoint:
   - `POST /api/upload-sessions`
   - `PUT /api/upload-sessions/:id/parts/:partNo`
   - `POST /api/upload-sessions/:id/complete`
   - `GET /api/upload-sessions/:id`
   - `DELETE /api/upload-sessions/:id`
2. Persist upload-session metadata in PostgreSQL, including:
   - session ID
   - file name
   - declared total size
   - part size
   - completed parts
   - storage upload ID if object-storage multipart is used
   - status
   - expires at
3. Prefer object-storage multipart upload coordination rather than indefinitely buffering large files on local disk.
4. Only after `complete` succeeds should the system invoke the existing finalize flow that creates the video record and enqueues transcode.
5. Keep the current direct multipart upload endpoint for smaller files or legacy clients.

#### Behavior

1. Weak or mobile networks can retry missing parts only.
2. Completed uploads produce the same business result as the current upload flow.
3. Incomplete sessions expire and are garbage-collected.

#### Why this design

This adds resumability where it matters most, without forcing all clients to migrate at once.

### 6. S3 Timeout Hardening

#### Problem

Object storage operations currently rely on caller context but do not define operation-specific timeout and retry semantics.

#### Changes

1. Add an object-storage wrapper layer around `RustFS` operations with explicit per-operation timeouts.
2. Define baseline timeouts by operation type, for example:
   - `Stat`: short timeout
   - `Put small object`: medium timeout
   - `Download source video`: long timeout
   - `Upload HLS directory objects`: per-file bounded timeout
3. Add exponential backoff with jitter for transient object-storage errors.
4. Retry only retry-safe failures:
   - timeout
   - connection reset
   - temporary DNS/network issue
   - 429
   - 5xx
5. Track stage-specific object-storage errors so the worker can decide between retry and permanent failure.
6. Feed timeout/error-rate metrics into admission control for storm handling.

#### Why this design

It contains storage instability without rewriting the object-storage integration.

### 7. Hot Video Protection

#### Problem

Hot videos can amplify load on metadata lookup, play endpoints, and object proxy traffic.

#### Changes

1. Add Redis caching for video play metadata and frequently requested listing/detail data.
2. Add request coalescing for cache-miss rebuilds so only one request fetches and repopulates a hot key at a time.
3. Add cache headers for cover assets, HLS manifests where safe, and segment/object proxy responses according to content type.
4. Prewarm a small hot-set cache for top played videos.
5. Consider moving high-frequency view count writes to buffered or asynchronous aggregation to reduce direct database write pressure.

#### Why this design

This directly reduces application and database load under concentrated hot-video traffic without changing the playback model.

### 8. API Rate Limiting

#### Problem

The router currently has no obvious shared rate-limiting layer. Burst traffic or abuse can compete with legitimate uploads and read traffic.

#### Changes

1. Add Redis-backed rate-limiting middleware so limits are shared across instances.
2. Apply route-class-specific policies:
   - upload/session create and part upload: strict
   - recommendation/question APIs: moderate
   - read/play metadata APIs: softer but still bounded
3. Return `429 Too Many Requests` with `Retry-After` for clear client behavior.
4. Add dynamic tightening under system stress, driven by:
   - queue backlog
   - active worker saturation
   - object-storage timeout rate

#### Why this design

Rate limiting becomes both abuse control and an automatic protection mechanism during degradation.

## Data Model And State Additions

The design assumes a small amount of additional persistent or semi-persistent state.

### Redis

1. transcode stream pending entries remain until success ack
2. dead-letter stream for failed terminal tasks
3. lease keys for active transcode ownership
4. retry metadata keys if not stored in stream payload
5. hot video cache keys
6. rate-limit counters/buckets

### Database Or Durable Metadata

1. upload session records
2. a transcode task execution history or error summary table if the existing video status/error fields cannot hold retry-stage diagnostics cleanly
3. video status/state expansion if current status enum cannot express `queued`, `retry_wait`, and `dead`

## Error Handling Strategy

The system should distinguish four classes of failure:

1. permanent client/input failure
   - invalid upload
   - invalid media
   - missing record
2. transient infrastructure failure
   - Redis timeout
   - object-storage timeout
   - temporary network error
3. bounded worker execution failure
   - ffmpeg timeout
   - temp disk pressure
   - process cleanup failure
4. terminal task failure after retries exhausted
   - move to dead-letter
   - persist reason

This classification drives whether to fail immediately, retry, reclaim, or dead-letter.

## Observability

Add metrics and logs that make the new lifecycle operationally visible.

### Metrics

1. queue length
2. queue pending count
3. stale pending count
4. active lease count
5. retry count by stage
6. dead-letter count
7. upload-session incomplete count
8. object-storage timeout/error rate
9. ffmpeg timeout count
10. rate-limit rejection count
11. cache hit ratio for hot video metadata

### Logs

All task lifecycle logs should include at least:

1. task ID
2. video ID
3. stream message ID when applicable
4. worker ID
5. stage
6. retry count
7. elapsed time

## Testing Strategy

### Unit Tests

1. queue ack/reclaim/dead-letter behavior
2. retry policy classification
3. lease renewal and stale-lease detection
4. upload session completion/expiry logic
5. rate-limit middleware policy behavior
6. cache hit/miss and singleflight behavior

### Integration Tests

1. worker crash before ack leads to pending reclaim and retry
2. S3 timeout during raw download retries and either succeeds or dead-letters after threshold
3. ffmpeg timeout kills execution and cleans temp resources
4. upload session interruption resumes from remaining parts only
5. backlog overload triggers admission control and 429/system-busy behavior as designed

### Manual Or Environment Verification

1. large-file upload from unstable network
2. simulated Redis restart or latency injection
3. simulated object-storage 5xx/timeout
4. burst upload and hot-video replay traffic

## Rollout Order

To minimize production risk, implement in this order:

1. reliable Redis Stream ack and reclaim semantics
2. worker lease, retry, and dead-letter support
3. object-storage timeout/retry wrapper and ffmpeg cleanup helper
4. transcode storm admission control and concurrency bounds
5. hot-video cache and rate-limiting middleware
6. resumable upload sessions

This order addresses task-loss and worker-stability risks first, then traffic-shaping and upload resilience.

## Trade-Offs

1. At-least-once queue semantics mean duplicate execution becomes possible. Handlers and worker stages must therefore remain idempotent where feasible.
2. Upload sessions increase metadata complexity, but this is the most practical way to address interrupted large uploads.
3. Redis becomes more operationally central because it now holds queue, leases, cache, and rate-limit state. This is acceptable because Redis is already a required dependency in the current system.
4. Dynamic admission control can delay transcode start time during load, but this is preferable to unbounded collapse.

## Implementation Notes

1. Prefer keeping new reliability components under `internal/infrastructure/redis`, `internal/application/videoapp`, and middleware layers rather than introducing a parallel architecture.
2. Preserve the existing upload API and add upload-session APIs as a second path.
3. Do not hand-edit generated Swagger files; update source annotations and regenerate if API shape changes.
4. The first implementation will keep vectorization behavior separate from transcode hardening, but the queue/reclaim patterns should be designed for later reuse.

## Recommended Outcome

The final system behavior should shift from best-effort asynchronous processing to bounded, recoverable, and observable asynchronous processing. The biggest functional improvements are:

1. worker crashes no longer silently lose tasks
2. Redis backlog becomes reclaimable instead of opaque
3. large uploads can resume after interruption
4. ffmpeg timeouts and temp residue become operationally manageable
5. hot traffic and abusive traffic are shaped before they overload core processing
