# HTTP DDD Phase 1 Design

## Summary

This phase reorganizes `video-service` toward a clearer DDD-style structure without changing any business logic, runtime behavior, API contract, queue semantics, persistence behavior, or worker flow.

The scope is intentionally narrow:

1. Reshape the HTTP layer under `internal/http`.
2. Reshape the application layer under `internal/application/videoapp`.
3. Leave domain rules, infrastructure adapters, worker logic, and database models behaviorally unchanged.

The purpose is to make the codebase easier to navigate and safer to extend. The output of this phase should be structural clarity, not new features.

## Problem

The repository already has partial layering:

- `internal/domain/video` contains domain rules.
- `internal/application/videoapp` contains use-case logic.
- `internal/http` contains transport adapters.
- `internal/infrastructure` contains external integrations.

However, the current application and HTTP packages are still too coarse:

1. `internal/application/videoapp` mixes multiple use cases in one package namespace.
2. `types.go` acts as a large shared bag of interfaces and DTO-like structs.
3. HTTP handlers are grouped in one directory but are still fairly broad, especially `video.go` and `upload.go`.
4. The current structure forces readers to jump between unrelated concerns such as upload, playback, recommendation, transcode runtime, and question querying.
5. The code communicates a technical layering, but not clear bounded responsibilities inside the application layer.

This makes change isolation harder than it needs to be even when the business logic itself is acceptable.

## Goals

1. Reorganize the application layer by use case.
2. Reorganize the HTTP layer by resource boundary.
3. Make dependency direction easier to see from directory structure alone.
4. Keep all existing behavior exactly the same.
5. Preserve all public API paths, payloads, and compatibility aliases.
6. Preserve all existing tests or migrate them without weakening coverage.

## Non-Goals

1. Do not redesign domain models.
2. Do not change repository behavior or database queries.
3. Do not change worker behavior, vectorization flow, or transcode flow.
4. Do not change Redis, object storage, embedding, or transcode infrastructure structure in this phase.
5. Do not rename public APIs.
6. Do not introduce new abstractions unless they directly support the structural split.

## Existing Context

The current code already separates coarse layers:

- `internal/domain/video`
- `internal/application/videoapp`
- `internal/http`
- `internal/infrastructure`

The main structural pressure points are:

- `internal/application/videoapp/types.go`
- `internal/application/videoapp/recommend.go`
- `internal/application/videoapp/upload_http.go`
- `internal/application/videoapp/worker.go`
- `internal/http/handler/video.go`
- `internal/http/handler/upload.go`

These files are not necessarily wrong in logic. They are simply carrying too many responsibilities per package boundary.

## Proposed Application Structure

Keep `internal/application/videoapp` as the application root, but split the implementation into subpackages by use case:

```text
internal/application/videoapp/
  service/
  contracts/
  upload/
  playback/
  recommendation/
  question/
  runtime/
  worker/
```

### `service/`

Purpose:

- Keep the composition root for the application service.
- Hold dependency wiring that is shared across use cases.

Expected contents:

- the current `Service` constructor logic
- shared clock / path / config fields
- minimal forwarding methods only where compatibility requires them

### `contracts/`

Purpose:

- Hold ports and cross-use-case data contracts now scattered in `types.go`.

Expected contents:

- repository interfaces
- queue interfaces
- object store interfaces
- status store interfaces
- file storage interfaces
- shared task payload structs
- shared path/config structs

Constraint:

- only keep types that are genuinely cross-use-case
- do not move use-case-local request/response types here unless needed by multiple packages

### `upload/`

Purpose:

- Own direct upload use cases and upload orchestration.

Expected contents:

- upload planning
- file write orchestration
- archive upload orchestration
- cover upload orchestration
- finalize upload flow

### `playback/`

Purpose:

- Own playback URL resolution and transcode status reads.

Expected contents:

- current `play.go`
- playback-specific helpers

### `recommendation/`

Purpose:

- Own recommendation lookup, watch reporting, and related AI-resilience glue that belongs to recommendation use cases.

Expected contents:

- current `recommend.go`
- current `ai_resilience.go`

### `question/`

Purpose:

- Own question list/detail application logic.

Expected contents:

- current `question.go`

### `runtime/`

Purpose:

- Hold runtime counters, transcode runtime state helpers, and system metrics helpers that are application-facing but not part of a single end-user use case.

Expected contents:

- current `runtime_counters.go`
- current `transcode_runtime.go`
- current `system_metrics.go`

### `worker/`

Purpose:

- Hold application-layer transcode worker orchestration that belongs to the video application, but is not part of HTTP request handling.

Expected contents:

- current `worker.go`
- current retry-specific helpers if still application-level

## Proposed HTTP Structure

Keep `internal/http` as the transport root, but split handlers by resource:

```text
internal/http/handler/
  uploads/
  videos/
  recommendations/
  questions/
  system/
  objects/
```

Each handler package should contain only:

1. request parsing
2. protocol validation
3. app service invocation
4. response mapping

Router registration can remain centralized in `internal/http/router/router.go`. This keeps route discovery simple while still allowing each handler package to stay focused.

## Compatibility Strategy

Behavior preservation is mandatory. The following must remain unchanged:

1. `Service` constructor signature unless a compatibility shim makes migration transparent.
2. HTTP endpoint paths under both REST and legacy aliases.
3. Existing DTO JSON shape.
4. Existing queue payload shape.
5. Existing repository interfaces from the perspective of infrastructure packages.
6. Existing command entrypoints.

If package moves would otherwise force large import churn, compatibility wrappers are allowed temporarily:

- forwarding methods
- type aliases
- thin adapter files

These wrappers are acceptable in this phase if they reduce risk and keep behavior stable.

## Implementation Approach

This refactor should be done incrementally:

1. Extract shared contracts from `types.go` into smaller focused files.
2. Move one use-case slice at a time inside `internal/application/videoapp`.
3. Keep tests moving with their corresponding use-case package.
4. Split HTTP handlers after the application split is stable.
5. Update router imports last.

This ordering minimizes the chance of broad breakage because the application layer is the more important boundary.

## Testing Strategy

This phase does not need new behavioral coverage for feature logic. It needs regression protection during movement.

Required verification:

1. Existing `internal/application/videoapp` tests still pass after package moves.
2. Existing `internal/http/handler` tests still pass after handler moves.
3. Existing `internal/http/router` tests still pass.
4. `cmd/httpapi` and `cmd/worker` build- and startup-related tests still pass where already covered.

If a package move forces test rewrites, the rewritten tests must assert the same behavior as before.

## Risks

1. Over-splitting can create ceremony without clarity.
2. Moving interfaces too aggressively can create import cycles.
3. Compatibility wrappers can linger if not kept thin and intentional.
4. Refactoring `videoapp` package names may cascade into tests and HTTP app wiring.

The way to control these risks is to prefer shallow extraction over theoretical purity.

## Success Criteria

This phase is successful when:

1. The code remains behaviorally identical.
2. Upload, playback, recommendation, question, runtime, and worker concerns are structurally separated in the application layer.
3. Upload/video/recommend/question/system/object concerns are structurally separated in the HTTP layer.
4. Shared contracts are smaller and easier to locate than the current monolithic `types.go`.
5. Existing tests pass after the refactor.

## Recommended Scope Boundary

This phase should stop before touching:

- `internal/infrastructure/*`
- `internal/worker/vectorworker/*`
- `internal/worker/transcodeworker/*`
- `internal/domain/video/*`
- `internal/model/*`

Those areas can be a later DDD Phase 2 once the HTTP and application layers are stable.
