# Logging Reduction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce `httpapi` and `worker` log noise while preserving startup summaries, task lifecycle summaries, and failure/retry visibility.

**Architecture:** Keep the existing zap/file logging setup, but reduce output by making HTTP access logging selective and by downgrading or removing high-frequency success-path worker logs. Preserve `Warn`/`Error` paths and top-level `start`/`done` summaries so operators still see actionable events.

**Tech Stack:** Go 1.26, Gin, zap

---

### Task 1: Make HTTP Access Logging Selective

**Files:**
- Modify: `video-service/middleware/api_log.go`
- Test: `video-service/middleware/api_log_test.go`

- [ ] **Step 1: Write the failing tests**

Create `video-service/middleware/api_log_test.go` with tests that cover the new policy:

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestShouldLogAccessSuppressesRoutineHealthyEndpoints(t *testing.T) {
	if shouldLogAccess("/healthz", http.StatusOK, 10*time.Millisecond, "") {
		t.Fatal("expected /healthz 200 to be suppressed")
	}
	if shouldLogAccess("/api/healthz", http.StatusOK, 10*time.Millisecond, "") {
		t.Fatal("expected /api/healthz 200 to be suppressed")
	}
	if shouldLogAccess("/swagger/index.html", http.StatusOK, 10*time.Millisecond, "") {
		t.Fatal("expected swagger route to be suppressed")
	}
}

func TestShouldLogAccessKeepsFailuresAndSlowRequests(t *testing.T) {
	if !shouldLogAccess("/api/videos", http.StatusInternalServerError, 10*time.Millisecond, "") {
		t.Fatal("expected 500 response to be logged")
	}
	if !shouldLogAccess("/api/videos", http.StatusOK, 1500*time.Millisecond, "") {
		t.Fatal("expected slow request to be logged")
	}
	if !shouldLogAccess("/api/videos", http.StatusOK, 10*time.Millisecond, "handler failed") {
		t.Fatal("expected request with private error to be logged")
	}
}

func TestAccessLogMiddlewareStillServesRequestWhenSuppressed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLogMiddleware())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./middleware -run 'TestShouldLogAccess|TestAccessLogMiddlewareStillServesRequestWhenSuppressed'`

Expected: FAIL because `shouldLogAccess` does not exist yet.

- [ ] **Step 3: Write minimal implementation**

Update `video-service/middleware/api_log.go` to add a small helper and use it before writing the access log:

```go
const slowRequestThreshold = time.Second

func shouldLogAccess(path string, status int, latency time.Duration, errMsg string) bool {
	if status >= http.StatusBadRequest {
		return true
	}
	if latency >= slowRequestThreshold {
		return true
	}
	if strings.TrimSpace(errMsg) != "" {
		return true
	}
	if path == "/healthz" || path == "/api/healthz" {
		return false
	}
	if strings.HasPrefix(path, "/swagger/") {
		return false
	}
	return false
}
```

Then gate the existing `zap.L().Info("api_access", ...)` call:

```go
if !shouldLogAccess(path, status, latency, errMsg) {
	return
}
```

Add the required imports used by the helper:

```go
import (
	"net/http"
	"strings"
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./middleware -run 'TestShouldLogAccess|TestAccessLogMiddlewareStillServesRequestWhenSuppressed'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add video-service/middleware/api_log.go video-service/middleware/api_log_test.go
git commit -m "chore: reduce access log noise"
```

### Task 2: Trim Transcode Worker Success-Path Noise

**Files:**
- Modify: `video-service/internal/application/videoapp/worker.go`
- Test: `video-service/internal/application/videoapp/worker_retry_test.go`

- [ ] **Step 1: Write the failing test**

Add a focused test to `video-service/internal/application/videoapp/worker_retry_test.go` that exercises a successful task and verifies the functional flow still succeeds after log-only edits:

```go
func TestWorkerHandleSuccessStillCompletesTask(t *testing.T) {
	// Reuse the existing worker test doubles in this file.
	// Configure downloader, transcoder, uploader, store, repo, queue, and status store to succeed.
	// Call the worker task handler and assert:
	// - queue ack happened
	// - status moved to done
	// - repo status updated to done
}
```

Use the same fake types and assertions style already present in the file instead of creating new helpers.

- [ ] **Step 2: Run test to verify it fails if the scenario is missing**

Run: `go test ./internal/application/videoapp -run TestWorkerHandleSuccessStillCompletesTask`

Expected: FAIL until the new scenario is added.

- [ ] **Step 3: Write minimal implementation**

In `video-service/internal/application/videoapp/worker.go`:

- Keep these logs at current severity:
  - task `start`
  - task `done`
  - task `failed`
- Remove or downgrade non-essential cover success-path logs:
  - `cover status=skip` -> `Debug`
  - `cover status=db_not_found` -> `Debug`
  - `cover status=ok` -> `Debug`
- Keep cover upload/database failures at `Error`

The intended shape is:

```go
zap.L().Debug("cover",
	zap.String("task_id", task.TaskID),
	zap.Uint64("video_id", task.VideoID),
	zap.String("status", "ok"),
	zap.String("url", coverURL),
)
```

Do not change task status, retry, cleanup, or queue behavior.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/application/videoapp -run TestWorkerHandleSuccessStillCompletesTask`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add video-service/internal/application/videoapp/worker.go video-service/internal/application/videoapp/worker_retry_test.go
git commit -m "chore: trim transcode worker logs"
```

### Task 3: Downgrade Noisy Vector Worker Progress Logs

**Files:**
- Modify: `video-service/internal/worker/vectorworker/app.go`
- Modify: `video-service/internal/worker/vectorworker/task.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/hierarchical.go`
- Test: `video-service/internal/worker/vectorworker/app_test.go`
- Test: `video-service/internal/worker/vectorworker/task_test.go`

- [ ] **Step 1: Write the failing tests**

Add focused tests that preserve behavior while allowing log-level changes:

```go
func TestVectorWorkerRunLoopHandlesDequeuedTask(t *testing.T) {
	// Use existing vector worker test doubles/setup in app_test.go.
	// Assert the dequeued task still reaches the handler path.
}

func TestHandleVectorizeTaskInvalidTaskStillFails(t *testing.T) {
	err := handleVectorizeTask(context.Background(), nil, nil, nil, nil, "", "", 0, 0, 0, 0, 0, 0, 0, 0, 0, "", 0, tasks.TailAlignmentConfig{}, 0, "", "")
	if err == nil {
		t.Fatal("expected invalid task error")
	}
}
```

Keep tests focused on behavior, not on capturing logs.

- [ ] **Step 2: Run tests to verify they fail or are absent**

Run: `go test ./internal/worker/vectorworker -run 'TestVectorWorkerRunLoopHandlesDequeuedTask|TestHandleVectorizeTaskInvalidTaskStillFails'`

Expected: FAIL until the scenarios are present.

- [ ] **Step 3: Write minimal implementation**

Apply these rules:

- Keep as `Info`:
  - `vector_worker_start`
  - top-level dequeued task `start`
  - top-level task `done`
- Keep as `Warn`/`Error`:
  - `retry`
  - `failed`
  - `dequeue_failed`
  - `queue_len_failed`
  - probe/extract/asr/llm failures
- Downgrade noisy success-path detail logs from `Info` to `Debug`, including examples such as:
  - `dequeue`
  - `vectorize_start`
  - `vectorize_downloaded`
  - `vectorize_probe_ok`
  - `vectorize_hierarchical_resume`
  - `vectorize_hierarchical_resume_done`
  - `vectorize_hierarchical_coarse_start`
  - `vectorize_progress`
  - `vectorize_extract_ok`
  - `vectorize_asr_ok`
  - `vectorize_embedding_start`
  - `ants_pool_metrics`
  - detailed LLM/raw segment logs in `tasks/hierarchical.go`
  - detailed refine/tail-alignment success logs in `tasks/asr.go`

Representative edit pattern:

```go
zap.L().Debug("vectorize_probe_ok",
	zap.Uint64("video_id", videoID),
	zap.String("task_id", taskID),
	zap.Int("duration_sec", durationSec),
)
```

Do not change retry timing, queue polling, task processing, database writes, or counters.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/worker/vectorworker -run 'TestVectorWorkerRunLoopHandlesDequeuedTask|TestHandleVectorizeTaskInvalidTaskStillFails'`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add video-service/internal/worker/vectorworker/app.go video-service/internal/worker/vectorworker/task.go video-service/internal/worker/vectorworker/tasks/asr.go video-service/internal/worker/vectorworker/tasks/hierarchical.go video-service/internal/worker/vectorworker/app_test.go video-service/internal/worker/vectorworker/task_test.go
git commit -m "chore: reduce vector worker log volume"
```

### Task 4: Final Verification

**Files:**
- Review: `video-service/middleware/api_log.go`
- Review: `video-service/internal/application/videoapp/worker.go`
- Review: `video-service/internal/worker/vectorworker/app.go`
- Review: `video-service/internal/worker/vectorworker/task.go`

- [ ] **Step 1: Run focused package tests**

Run:

```bash
go test ./middleware ./internal/application/videoapp ./internal/worker/vectorworker
```

Expected: PASS

- [ ] **Step 2: Run one HTTP entrypoint test for regression coverage**

Run:

```bash
go test ./cmd/httpapi -run TestPrepareServerPrefersHTTPAddr
```

Expected: PASS

- [ ] **Step 3: Inspect diff for scope control**

Run:

```bash
git diff -- video-service/middleware/api_log.go video-service/middleware/api_log_test.go video-service/internal/application/videoapp/worker.go video-service/internal/application/videoapp/worker_retry_test.go video-service/internal/worker/vectorworker/app.go video-service/internal/worker/vectorworker/task.go video-service/internal/worker/vectorworker/tasks/asr.go video-service/internal/worker/vectorworker/tasks/hierarchical.go video-service/internal/worker/vectorworker/app_test.go video-service/internal/worker/vectorworker/task_test.go
```

Expected: Only logging policy/level changes and supporting tests.

- [ ] **Step 4: Commit**

```bash
git add video-service/middleware/api_log.go video-service/middleware/api_log_test.go video-service/internal/application/videoapp/worker.go video-service/internal/application/videoapp/worker_retry_test.go video-service/internal/worker/vectorworker/app.go video-service/internal/worker/vectorworker/task.go video-service/internal/worker/vectorworker/tasks/asr.go video-service/internal/worker/vectorworker/tasks/hierarchical.go video-service/internal/worker/vectorworker/app_test.go video-service/internal/worker/vectorworker/task_test.go
git commit -m "chore: make service logs easier to read"
```
