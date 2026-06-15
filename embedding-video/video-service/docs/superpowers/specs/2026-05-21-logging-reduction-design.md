# Logging Reduction Design

## Goal

Reduce log volume from `cmd/httpapi` and `cmd/worker` so log files are easier to read and store, without losing the summaries and failure signals needed for routine operations and debugging.

## Scope

- `video-service/middleware/api_log.go`
- `video-service/internal/http/router/router.go`
- High-volume success-path logs in transcode and vector worker code

## Non-Goals

- No new logging backend
- No repo-wide logging framework rewrite
- No change to business logic, retries, or task flow
- No new operator-facing config unless current code structure makes it clearly cheaper than repeated callsite edits

## Current Problems

### HTTP API

- Every request is logged at `Info`, including routine health checks and Swagger traffic.
- Successful high-frequency requests dominate the file and make failures harder to spot.

### Worker

- Vector worker emits many `Info` logs for per-step and per-segment progress.
- Transcode/vector task internals log many success events that are useful during development but too noisy in persistent service logs.
- Startup summaries are useful, but detailed success-path progress logs produce poor signal-to-noise in production.

## Chosen Approach

Use a conservative reduction strategy:

1. Keep startup summaries.
2. Keep task-level `start` and `done` summaries.
3. Keep `Warn` and `Error` logs for retries, failures, and degraded paths.
4. Suppress or downgrade high-frequency success-path detail logs.
5. Make HTTP access logs selective so routine endpoints and normal fast successes do not flood the output.

## Design

### HTTP Logging

- Keep the access-log middleware in place.
- Do not emit `Info` for every request.
- Suppress routine noise for:
  - `/healthz`
  - `/api/healthz`
  - `/swagger/*`
- Emit logs for:
  - non-2xx/3xx responses
  - slow requests
  - requests carrying Gin private errors
- For normal successful requests, either skip logging entirely or log them below the default output threshold.

Recommended implementation shape:

- Introduce a small decision helper inside `api_log.go` that classifies requests by path, status, latency, and presence of errors.
- Keep the change local to middleware instead of spreading route-specific conditionals elsewhere.

### Worker Logging

- Keep process startup logs such as worker boot summaries.
- Keep one `start` and one `done` log per top-level task.
- Keep retry, failure, dequeue failure, and queue failure logs.
- Downgrade or remove detailed success-path logs including:
  - download/probe success
  - segment resume summaries beyond the top-level task summary
  - coarse pipeline internal progress
  - per-segment upload/extract/ASR success
  - periodic pool metrics if they are only operational noise

Recommended implementation shape:

- Leave existing error and warning callsites unchanged.
- Change only noisy `Info` success callsites to `Debug` or remove them when the information is already implied by a later `done` summary.
- Preserve transcode/video/vector task identifiers on remaining summary logs.

## Tradeoffs

### Benefits

- Much smaller log files
- Failures and retries become easier to scan
- Minimal code risk because behavior changes are limited to logging level and selection

### Costs

- Less step-by-step visibility during deep debugging
- If a future issue needs internal progress tracing, developers may temporarily re-enable lower-level logs in code

## Verification

1. Run focused tests for touched packages.
2. Review diff to confirm only logging behavior changed.
3. Sanity-check that remaining `Info` logs still include startup and task lifecycle summaries.

## Files Likely To Change

- `video-service/middleware/api_log.go`
- `video-service/internal/application/videoapp/worker.go`
- `video-service/internal/worker/vectorworker/app.go`
- `video-service/internal/worker/vectorworker/task.go`
- Possibly `video-service/internal/worker/vectorworker/tasks/*.go` if the noisiest step logs live there
