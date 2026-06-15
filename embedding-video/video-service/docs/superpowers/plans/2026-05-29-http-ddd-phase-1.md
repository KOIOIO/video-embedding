# HTTP DDD Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganize the HTTP and application layers of `video-service` into clearer DDD-style package boundaries without changing runtime behavior or public APIs.

**Architecture:** Keep all business logic intact and move code in narrow phases. First extract shared contracts from `internal/application/videoapp/types.go`, then split use-case code into focused subpackages behind compatibility wrappers, and only after that split HTTP handlers by resource while preserving router paths and DTOs.

**Tech Stack:** Go, Gin, GORM, Redis, existing unit/integration tests

---

## File Structure

Application layer target:

- Modify: `video-service/internal/application/videoapp/service.go`
- Modify: `video-service/internal/application/videoapp/types.go`
- Create: `video-service/internal/application/videoapp/contracts.go`
- Create: `video-service/internal/application/videoapp/tasks.go`
- Create: `video-service/internal/application/videoapp/paths.go`
- Create: `video-service/internal/application/videoapp/upload/types.go`
- Create: `video-service/internal/application/videoapp/playback/service.go`
- Create: `video-service/internal/application/videoapp/recommendation/service.go`
- Create: `video-service/internal/application/videoapp/question/service.go`
- Create: `video-service/internal/application/videoapp/runtime/service.go`
- Create: `video-service/internal/application/videoapp/worker/service.go`
- Modify: existing `upload.go`, `upload_http.go`, `play.go`, `recommend.go`, `question.go`, `system_metrics.go`, `runtime_counters.go`, `transcode_runtime.go`, `worker.go`

HTTP layer target:

- Create: `video-service/internal/http/handler/uploads/handler.go`
- Create: `video-service/internal/http/handler/videos/handler.go`
- Create: `video-service/internal/http/handler/recommendations/handler.go`
- Create: `video-service/internal/http/handler/questions/handler.go`
- Create: `video-service/internal/http/handler/system/handler.go`
- Create: `video-service/internal/http/handler/objects/handler.go`
- Modify: `video-service/internal/http/router/router.go`
- Keep DTOs under `video-service/internal/http/dto`

Verification targets:

- Test: `go test ./internal/application/videoapp`
- Test: `go test ./internal/http/handler ./internal/http/router`
- Test: `go test ./cmd/httpapi ./cmd/worker`

### Task 1: Extract shared application contracts

**Files:**
- Modify: `video-service/internal/application/videoapp/types.go`
- Create: `video-service/internal/application/videoapp/contracts.go`
- Create: `video-service/internal/application/videoapp/tasks.go`
- Create: `video-service/internal/application/videoapp/paths.go`
- Test: `video-service/internal/application/videoapp/*_test.go`

- [ ] **Step 1: Write the failing test**

Add a narrow compile-safety regression by moving one existing test reference to the new files without changing assertions. Use one of:

```go
func TestBuildUploadPlanBuildsStablePaths(t *testing.T)
func TestFinalizeUploadEnqueuesTranscodeAndIgnoresVectorFailure(t *testing.T)
```

Expected failure after the first move: missing `Paths`, `UploadPlan`, or queue task types if extraction is incomplete.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/application/videoapp -run TestBuildUploadPlanBuildsStablePaths`
Expected: FAIL only if the first extraction leaves unresolved symbols.

- [ ] **Step 3: Write minimal implementation**

Move only shared contracts:

- repository and store interfaces to `contracts.go`
- `Paths`, `UploadPlan`, `UploadResult` to `paths.go`
- `TranscodeTask`, `VectorizeTask`, `TranscodeStatus` to `tasks.go`

Leave package name as `videoapp` to avoid behavior changes.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/application/videoapp`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add video-service/internal/application/videoapp
git commit -m "refactor(videoapp): extract shared contracts"
```

### Task 2: Split application use cases behind compatibility wrappers

**Files:**
- Create: `video-service/internal/application/videoapp/upload/*.go`
- Create: `video-service/internal/application/videoapp/playback/*.go`
- Create: `video-service/internal/application/videoapp/recommendation/*.go`
- Create: `video-service/internal/application/videoapp/question/*.go`
- Create: `video-service/internal/application/videoapp/runtime/*.go`
- Create: `video-service/internal/application/videoapp/worker/*.go`
- Modify: `video-service/internal/application/videoapp/service.go`
- Modify: existing top-level application files to forward into subpackages
- Test: `video-service/internal/application/videoapp/*_test.go`

- [ ] **Step 1: Write the failing test**

Use existing high-signal tests that cover multiple use cases:

```go
func TestUploadVideoSuccessRemovesTempFileAfterFinalize(t *testing.T)
func TestPlayVideoReturnsCachedHLSAndRepairsDoneStatus(t *testing.T)
func TestRecommendByQuestion... // pick one existing recommend test
```

Expected failure after partial moves: unresolved `Service` methods or import-cycle errors.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/application/videoapp -run 'TestUploadVideoSuccessRemovesTempFileAfterFinalize|TestPlayVideoReturnsCachedHLSAndRepairsDoneStatus'`
Expected: FAIL only during partial movement.

- [ ] **Step 3: Write minimal implementation**

Move code by use case, but keep the public surface stable:

- `videoapp.Service` remains the entry point
- top-level methods forward to focused subpackage services
- no business rule changes
- no signature changes visible to HTTP or infrastructure

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/application/videoapp`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add video-service/internal/application/videoapp
git commit -m "refactor(videoapp): split application use cases"
```

### Task 3: Split HTTP handlers by resource

**Files:**
- Create: `video-service/internal/http/handler/uploads/handler.go`
- Create: `video-service/internal/http/handler/videos/handler.go`
- Create: `video-service/internal/http/handler/recommendations/handler.go`
- Create: `video-service/internal/http/handler/questions/handler.go`
- Create: `video-service/internal/http/handler/system/handler.go`
- Create: `video-service/internal/http/handler/objects/handler.go`
- Modify: `video-service/internal/http/router/router.go`
- Modify: existing handler tests to point at new package paths if needed
- Test: `video-service/internal/http/handler/*_test.go`

- [ ] **Step 1: Write the failing test**

Use existing router/handler regression tests:

```go
func TestUploadVideo_Success(t *testing.T)
func TestSwaggerDocOmitsLegacyAliasPaths(t *testing.T)
func TestLegacyAliasRoutesAreRegistered(t *testing.T)
```

Expected failure after partial moves: missing constructors or wrong router imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/http/handler ./internal/http/router -run 'TestUploadVideo_Success|TestSwaggerDocOmitsLegacyAliasPaths|TestLegacyAliasRoutesAreRegistered'`
Expected: FAIL only while movement is incomplete.

- [ ] **Step 3: Write minimal implementation**

Split handler packages by resource and keep:

- constructor names stable where practical
- request parsing unchanged
- DTO mapping unchanged
- router paths unchanged

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/http/handler ./internal/http/router`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add video-service/internal/http
git commit -m "refactor(http): split handlers by resource"
```

### Task 4: Final compatibility verification

**Files:**
- Modify: only small compatibility wrappers or imports if verification exposes drift
- Test: `video-service/cmd/httpapi/main_test.go`
- Test: `video-service/cmd/worker/main_test.go`

- [ ] **Step 1: Run full focused verification**

Run: `go test ./internal/application/videoapp ./internal/http/handler ./internal/http/router ./cmd/httpapi ./cmd/worker`
Expected: PASS

- [ ] **Step 2: Fix only compatibility regressions**

Allowed fixes:

- missing forwarding method
- wrong import path
- temporary alias file

Not allowed:

- logic change
- API shape change
- worker behavior change

- [ ] **Step 3: Re-run verification**

Run: `go test ./internal/application/videoapp ./internal/http/handler ./internal/http/router ./cmd/httpapi ./cmd/worker`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add video-service
git commit -m "refactor: complete http ddd phase 1 reorganization"
```

## Self-Review

Spec coverage:

- application layer split: covered by Tasks 1 and 2
- HTTP layer split: covered by Task 3
- behavior preservation: covered by Task 4
- worker/infrastructure exclusion: enforced by explicit scope and allowed-fix limits

Placeholder scan:

- No TODO/TBD placeholders remain.
- Verification commands are concrete.
- File targets are concrete enough to start implementation while allowing exact new filenames to match the final package split.

Type consistency:

- `videoapp.Service` remains the stable application entry point.
- Shared ports stay in `videoapp` package-level files during Phase 1 to avoid import churn and behavior changes.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-29-http-ddd-phase-1.md`.

Two execution options:

1. Subagent-Driven (recommended) - I dispatch a fresh subagent per task, review between tasks, fast iteration
2. Inline Execution - Execute tasks in this session using executing-plans, batch execution with checkpoints
