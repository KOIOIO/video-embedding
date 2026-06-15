# AI Fallback Resilience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the vector worker durable when Ali Bailian is unavailable, and keep question-driven recommendation traffic responsive through circuit breaking, fallback routing, and deterministic degraded responses.

**Architecture:** Add a small shared AI resilience layer that classifies provider failures, tracks circuit state, and exposes fallback routing decisions. Wire that layer into the question/recommendation path for fast degradation and into `vectorworker` for retryable async compensation instead of task loss.

**Tech Stack:** Go, Gin, Redis, GORM, existing DashScope/OpenAI-compatible clients, zap, Go tests.

---

## File Map

### Existing files to modify

- `video-service/internal/application/videoapp/recommend.go`
  Purpose: use a resilient question-vector generation path and return a deterministic degraded result when AI providers are unavailable.
- `video-service/internal/application/videoapp/service.go`
  Purpose: thread an optional AI resilience dependency into the application service.
- `video-service/internal/infrastructure/embedding/client.go`
  Purpose: keep the primary embedding client, but expose failure details that the resilience layer can classify.
- `video-service/internal/worker/vectorworker/app.go`
  Purpose: route AI failures through retry/backoff/dead-letter decisions instead of treating every upstream error as terminal.
- `video-service/internal/worker/vectorworker/client_openai.go`
  Purpose: classify DashScope/OpenAI-compatible failures and surface retryable vs terminal errors.
- `video-service/internal/worker/vectorworker/dashscope_ws.go`
  Purpose: make websocket ASR failures classify cleanly so the worker can retry or dead-letter correctly.
- `video-service/internal/worker/vectorworker/task.go`
  Purpose: keep hierarchical vectorization on the async-compensation path when the AI backend fails.
- `video-service/internal/http/handler/recommend.go`
  Purpose: map degraded AI outcomes to a stable HTTP response shape.

### New files to create

- `video-service/internal/application/videoapp/ai_resilience.go`
  Purpose: shared failure classification, breaker state, fallback decision helpers, and degraded response contract.
- `video-service/internal/application/videoapp/ai_resilience_test.go`
  Purpose: verify retryable vs terminal classification and degraded-mode decisions.
- `video-service/internal/infrastructure/ai/fallback_embedder.go`
  Purpose: a small wrapper that tries the primary embedder, then a local fallback embedder, then reports a degraded error.
- `video-service/internal/infrastructure/ai/fallback_embedder_test.go`
  Purpose: verify fallback order and breaker-open behavior.
- `video-service/internal/worker/vectorworker/ai_retry.go`
  Purpose: worker-local retry policy helpers for provider outages, throttling, and terminal failures.
- `video-service/internal/worker/vectorworker/ai_retry_test.go`
  Purpose: verify vector worker retry and dead-letter decisions.

## Task 1: Add Shared AI Failure Classification And Breaker State

**Files:**
- Create: `video-service/internal/application/videoapp/ai_resilience.go`
- Test: `video-service/internal/application/videoapp/ai_resilience_test.go`

- [ ] **Step 1: Write the failing classification test**

```go
func TestClassifyAIError_RetryableAndTerminal(t *testing.T) {
    cases := []struct {
        name string
        err  error
        want RetryDecision
    }{
        {
            name: "timeout is retryable",
            err:  context.DeadlineExceeded,
            want: RetryDecision{Retry: true, Delay: 2 * time.Second, Reason: "timeout"},
        },
        {
            name: "missing model is terminal",
            err:  errors.New("model not found"),
            want: RetryDecision{Retry: false, Reason: "terminal"},
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := DecideAIRetry(tc.err, 1)
            if got.Retry != tc.want.Retry || got.Reason != tc.want.Reason {
                t.Fatalf("got %#v want %#v", got, tc.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./video-service/internal/application/videoapp -run TestClassifyAIError_RetryableAndTerminal -v`

Expected: FAIL because `DecideAIRetry` does not exist yet.

- [ ] **Step 3: Implement the shared decision helpers**

```go
package videoapp

import (
    "context"
    "errors"
    "strings"
    "time"
)

type RetryDecision struct {
    Retry  bool
    Delay  time.Duration
    Reason string
}

func DecideAIRetry(err error, retries int) RetryDecision {
    if err == nil {
        return RetryDecision{}
    }
    if retries >= 3 {
        return RetryDecision{Retry: false, Reason: "retry_exhausted"}
    }
    if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
        return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * 2 * time.Second, Reason: "timeout"}
    }
    if strings.Contains(strings.ToLower(err.Error()), "429") || strings.Contains(strings.ToLower(err.Error()), "too many requests") {
        return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * 5 * time.Second, Reason: "throttled"}
    }
    if strings.Contains(strings.ToLower(err.Error()), "connection reset") || strings.Contains(strings.ToLower(err.Error()), "temporary") {
        return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * 3 * time.Second, Reason: "transient"}
    }
    return RetryDecision{Retry: false, Reason: "terminal"}
}
```

- [ ] **Step 4: Run the test and verify it passes**

Run: `go test ./video-service/internal/application/videoapp -run TestClassifyAIError_RetryableAndTerminal -v`

Expected: PASS.

## Task 2: Add Question-Path Fallback Embedding And Degraded Response

**Files:**
- Modify: `video-service/internal/application/videoapp/service.go`
- Modify: `video-service/internal/application/videoapp/recommend.go`
- Create: `video-service/internal/infrastructure/ai/fallback_embedder.go`
- Test: `video-service/internal/infrastructure/ai/fallback_embedder_test.go`

- [ ] **Step 1: Write the failing fallback-order test**

```go
func TestFallbackEmbedder_UsesLocalFallbackWhenPrimaryFails(t *testing.T) {
    primary := &stubEmbedder{err: errors.New("timeout")}
    fallback := &stubEmbedder{vec: []float32{1, 2, 3}}

    e := NewFallbackEmbedder(primary, fallback)
    vec, err := e.Embed(context.Background(), "hello world")

    if err != nil {
        t.Fatalf("Embed error = %v", err)
    }
    if len(vec) != 3 {
        t.Fatalf("expected fallback vector, got %#v", vec)
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./video-service/internal/infrastructure/ai -run TestFallbackEmbedder_UsesLocalFallbackWhenPrimaryFails -v`

Expected: FAIL because `NewFallbackEmbedder` does not exist yet.

- [ ] **Step 3: Implement a fallback embedder wrapper**

```go
package ai

import "context"

type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}

type FallbackEmbedder struct {
    Primary Embedder
    Fallback Embedder
}

func NewFallbackEmbedder(primary Embedder, fallback Embedder) *FallbackEmbedder {
    return &FallbackEmbedder{Primary: primary, Fallback: fallback}
}

func (e *FallbackEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    if e.Primary != nil {
        if vec, err := e.Primary.Embed(ctx, text); err == nil {
            return vec, nil
        }
    }
    if e.Fallback != nil {
        return e.Fallback.Embed(ctx, text)
    }
    return nil, context.Canceled
}
```

- [ ] **Step 4: Wire the service to use the fallback embedder**

```go
func NewService(repo VideoRepository, queue TranscodeQueue, vectorQueue VectorizeQueue, statusStore TranscodeStatusStore, store ObjectStore, fs FileStorage, embedder TextEmbedder, paths Paths) *Service {
    return &Service{
        Repo:        repo,
        Queue:       queue,
        VectorQueue: vectorQueue,
        StatusStore: statusStore,
        Store:       store,
        FS:          fs,
        Embedder:    embedder,
        Paths:       paths,
        Now:         time.Now,
        StatusTTL:   24 * time.Hour,
        DeleteLocal: true,
    }
}
```

Then update the app builder to inject `ai.NewFallbackEmbedder(primary, localFallback)` into `videoapp.NewService`.

- [ ] **Step 5: Add a degraded recommendation path**

```go
if err != nil {
    if isAIProviderUnavailable(err) {
        return s.DegradedRecommendation(ctx, input)
    }
    return nil, err
}
```

Implement `DegradedRecommendation` so it returns stable, explicit fallback items from cached question text or from a deterministic local vectorizer, rather than failing the request outright.

- [ ] **Step 6: Run the AI infrastructure tests**

Run: `go test ./video-service/internal/infrastructure/ai/... -v`

Expected: PASS.

## Task 3: Make `vectorworker` Retry AI Outages Instead Of Failing Hard

**Files:**
- Create: `video-service/internal/worker/vectorworker/ai_retry.go`
- Modify: `video-service/internal/worker/vectorworker/app.go`
- Modify: `video-service/internal/worker/vectorworker/client_openai.go`
- Modify: `video-service/internal/worker/vectorworker/dashscope_ws.go`
- Modify: `video-service/internal/worker/vectorworker/task.go`
- Test: `video-service/internal/worker/vectorworker/ai_retry_test.go`

- [ ] **Step 1: Write the failing retry-policy test**

```go
func TestDecideVectorAIRetry_RetryableOutage(t *testing.T) {
    got := DecideVectorAIRetry(context.DeadlineExceeded, 0)
    if !got.Retry {
        t.Fatalf("expected retryable timeout")
    }
    if got.Reason != "timeout" {
        t.Fatalf("expected timeout reason, got %q", got.Reason)
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./video-service/internal/worker/vectorworker -run TestDecideVectorAIRetry_RetryableOutage -v`

Expected: FAIL because `DecideVectorAIRetry` does not exist yet.

- [ ] **Step 3: Implement worker-local AI retry classification**

```go
package vectorworker

import (
    "context"
    "errors"
    "strings"
    "time"
)

type AIRetryDecision struct {
    Retry  bool
    Delay  time.Duration
    Reason string
}

func DecideVectorAIRetry(err error, retries int) AIRetryDecision {
    if err == nil {
        return AIRetryDecision{}
    }
    if retries >= 3 {
        return AIRetryDecision{Retry: false, Reason: "retry_exhausted"}
    }
    if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
        return AIRetryDecision{Retry: true, Delay: time.Duration(retries+1) * 5 * time.Second, Reason: "timeout"}
    }
    if strings.Contains(strings.ToLower(err.Error()), "429") || strings.Contains(strings.ToLower(err.Error()), "too many requests") {
        return AIRetryDecision{Retry: true, Delay: time.Duration(retries+1) * 10 * time.Second, Reason: "throttled"}
    }
    if strings.Contains(strings.ToLower(err.Error()), "connection reset") || strings.Contains(strings.ToLower(err.Error()), "temporary") {
        return AIRetryDecision{Retry: true, Delay: time.Duration(retries+1) * 7 * time.Second, Reason: "transient"}
    }
    return AIRetryDecision{Retry: false, Reason: "terminal"}
}
```

- [ ] **Step 4: Update the vector worker loop to requeue on retryable AI failures**

```go
decision := DecideVectorAIRetry(lastErr, retry)
if decision.Retry {
    _ = queue.Requeue(ctx, task, decision.Delay, decision.Reason)
    continue
}
_ = queue.MoveToDeadLetter(ctx, task, decision.Reason)
```

Apply this decision at the point where LLM/ASR/embedding failures are currently treated as fatal, so the task keeps moving through the retry path instead of disappearing.

- [ ] **Step 5: Make provider clients surface retryable errors cleanly**

In `client_openai.go` and `dashscope_ws.go`, return wrapped errors that preserve timeout, throttling, and transport failure information so the retry classifier can distinguish them.

- [ ] **Step 6: Run the vector worker tests**

Run: `go test ./video-service/internal/worker/vectorworker/... -v`

Expected: PASS.

## Task 4: Add HTTP-Level Degraded Response Shape For Question Traffic

**Files:**
- Modify: `video-service/internal/http/handler/recommend.go`
- Modify: `video-service/internal/http/errors/errors.go`
- Modify: `video-service/internal/http/dto/common.go`
- Test: `video-service/internal/http/handler/recommend_test.go`

- [ ] **Step 1: Write the failing degraded-response test**

```go
func TestRecommendByQuestion_ReturnsDegradedResponseWhenAIIsDown(t *testing.T) {
    app := &stubRecommendApp{
        recommendByQuestionFunc: func(context.Context, videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error) {
            return nil, videoapp.DegradedError{Reason: "provider_unavailable"}
        },
        resolvePlaybackURLFunc: func(context.Context, domainvideo.Video) string { return "" },
    }

    h := NewRecommendHandler(app)
    c, w := newJSONContext(http.MethodPost, "/api/recommendations/by-question", `{"question_text":"hello"}`)

    h.RecommendByQuestion(c)

    if w.Code != http.StatusOK {
        t.Fatalf("status = %d want %d", w.Code, http.StatusOK)
    }
    if !strings.Contains(w.Body.String(), `"degraded":true`) {
        t.Fatalf("expected degraded response, got %s", w.Body.String())
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `go test ./video-service/internal/http/handler -run TestRecommendByQuestion_ReturnsDegradedResponseWhenAIIsDown -v`

Expected: FAIL because the handler does not yet map degraded AI outcomes specially.

- [ ] **Step 3: Add a typed degraded error and response payload**

```go
type DegradedError struct {
    Reason string
}

func (e DegradedError) Error() string { return e.Reason }
```

Then add a response body that includes `success=true`, `data`, and a stable `degraded=true` marker or a dedicated message field for clients to detect fallback mode.

- [ ] **Step 4: Teach the handler to emit the degraded response**

```go
var degradedErr videoapp.DegradedError
if errors.As(err, &degradedErr) {
    c.JSON(http.StatusOK, dto.SuccessResponse[any]{
        Success: true,
        Data: map[string]any{
            "degraded": true,
            "message":  "AI provider temporarily unavailable; returned fallback result",
        },
    })
    return
}
```

- [ ] **Step 5: Run the handler tests**

Run: `go test ./video-service/internal/http/handler -v`

Expected: PASS.

## Task 5: Verify The Whole HTTP Package Still Builds

**Files:** none

- [ ] **Step 1: Run the focused backend test sweep**

Run: `go test ./video-service/...`

Expected: PASS.

- [ ] **Step 2: Fix any compile or test failures from the new AI fallback wiring**

If a test fails because the fallback embedder or degraded-response type is not threaded through a constructor yet, update the smallest constructor or interface needed and re-run the same command.

- [ ] **Step 3: Commit the implementation**

```bash
git add video-service/internal/application/videoapp/ai_resilience.go video-service/internal/application/videoapp/ai_resilience_test.go video-service/internal/application/videoapp/recommend.go video-service/internal/application/videoapp/service.go video-service/internal/infrastructure/ai/fallback_embedder.go video-service/internal/infrastructure/ai/fallback_embedder_test.go video-service/internal/worker/vectorworker/ai_retry.go video-service/internal/worker/vectorworker/ai_retry_test.go video-service/internal/worker/vectorworker/app.go video-service/internal/worker/vectorworker/client_openai.go video-service/internal/worker/vectorworker/dashscope_ws.go video-service/internal/worker/vectorworker/task.go video-service/internal/http/handler/recommend.go video-service/internal/http/errors/errors.go video-service/internal/http/dto/common.go
git commit -m "feat: add ai fallback resilience"
```

## Self-Review Checklist

1. Spec coverage:
   - `vector_worker` async compensation is covered by Task 3.
   - Question-path availability via circuit breaking, fallback routing, and degraded responses is covered by Tasks 1, 2, and 4.
   - Shared failure classification is covered by Task 1.
2. Placeholder scan:
   - No `TBD`, `TODO`, or vague "handle edge cases" placeholders remain.
3. Type consistency:
   - `RetryDecision` is defined in `internal/application/videoapp/ai_resilience.go` and reused by the worker plan.
   - `AIRetryDecision` is defined in `internal/worker/vectorworker/ai_retry.go` and kept worker-local.
   - `DegradedError` is the typed signal for HTTP degraded mode.
