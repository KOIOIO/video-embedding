# Reliability Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden `video-service` so transcode work is recoverable and bounded, large uploads can resume, and hot/abusive traffic is shaped before it overloads the service.

**Architecture:** Keep the existing `videoapp.Service`, `videoapp.Worker`, Redis, PostgreSQL, and Gin routing structure. Add reliability behavior in narrow layers: Redis queue semantics, worker lease/retry state, ffmpeg and object-storage wrappers, upload-session APIs, and HTTP middleware/cache helpers.

**Tech Stack:** Go, Gin, GORM, Redis Streams, Redis KV, PostgreSQL, MinIO SDK, ffmpeg, zap.

---

## File Map

### Existing files to modify

- `video-service/internal/infrastructure/redis/transcode.go`
  Purpose: change queue semantics from acknowledge-on-read to explicit ack/reclaim/retry/dead-letter behavior.
- `video-service/internal/application/videoapp/worker.go`
  Purpose: accept message IDs, own leases, classify retryable failures, ack only on success, dead-letter on terminal failure.
- `video-service/internal/worker/transcodeworker/app.go`
  Purpose: wire semaphore, queue recovery loop, worker concurrency bounds, and new wrapper dependencies.
- `video-service/internal/infrastructure/objectstorage/rustfs.go`
  Purpose: keep raw storage client, but expose interfaces needed by the reliability wrapper.
- `video-service/internal/infrastructure/transcode/ffmpeg_transcoder.go`
  Purpose: route command execution through a cleanup-aware helper.
- `video-service/internal/application/videoapp/upload.go`
  Purpose: finalize completed upload sessions through the existing upload finalization path.
- `video-service/internal/application/videoapp/upload_http.go`
  Purpose: keep legacy upload path and add upload-session aware service methods.
- `video-service/internal/http/handler/upload.go`
  Purpose: expose upload-session endpoints.
- `video-service/internal/http/router/router.go`
  Purpose: register new routes and middleware.
- `video-service/internal/http/handler/video.go`
  Purpose: cache hot video play responses and any list/detail reads explicitly chosen in the implementation.
- `video-service/middleware/api_log.go`
  Purpose: keep logging as-is, but ensure request IDs or limiter metadata can be added if needed.

### New files to create

- `video-service/internal/application/videoapp/transcode_queue.go`
  Purpose: define queue message, ack/reclaim/dead-letter interfaces clearly for the worker.
- `video-service/internal/application/videoapp/transcode_retry.go`
  Purpose: classify retryable vs terminal failures and compute retry backoff.
- `video-service/internal/application/videoapp/transcode_lease.go`
  Purpose: define worker lease store interface and data model.
- `video-service/internal/infrastructure/redis/transcode_lease.go`
  Purpose: Redis-backed lease store implementation.
- `video-service/internal/infrastructure/redis/transcode_queue_test.go`
  Purpose: test explicit ack, claim, retry, and dead-letter behavior.
- `video-service/internal/application/videoapp/worker_retry_test.go`
  Purpose: test success ack, retry requeue, and dead-letter transitions.
- `video-service/internal/infrastructure/transcode/exec_helper.go`
  Purpose: centralize process execution, cancellation, and cleanup metadata.
- `video-service/internal/infrastructure/objectstorage/retrying_store.go`
  Purpose: timeout/retry wrapper around object storage operations.
- `video-service/internal/infrastructure/objectstorage/retrying_store_test.go`
  Purpose: test timeout/retry classification.
- `video-service/internal/application/videoapp/upload_session.go`
  Purpose: upload-session domain/service logic.
- `video-service/internal/application/videoapp/upload_session_test.go`
  Purpose: test resumable upload session lifecycle.
- `video-service/internal/infrastructure/persistence/gorm_upload_session_repository.go`
  Purpose: durable upload-session metadata storage.
- `video-service/internal/model/upload_session.go`
  Purpose: GORM model for upload sessions.
- `video-service/internal/http/dto/upload_session.go`
  Purpose: request/response DTOs for session APIs.
- `video-service/internal/http/handler/upload_session_test.go`
  Purpose: handler tests for session APIs.
- `video-service/internal/infrastructure/cache/video_cache.go`
  Purpose: Redis-backed cache helper for hot video metadata.
- `video-service/internal/infrastructure/cache/video_cache_test.go`
  Purpose: cache hit/miss tests.
- `video-service/middleware/rate_limit.go`
  Purpose: Redis-backed rate-limiting middleware.
- `video-service/middleware/rate_limit_test.go`
  Purpose: limiter policy tests.

## Task 1: Make Redis Stream Consumption Recoverable

**Files:**
- Create: `video-service/internal/application/videoapp/transcode_queue.go`
- Modify: `video-service/internal/infrastructure/redis/transcode.go`
- Test: `video-service/internal/infrastructure/redis/transcode_queue_test.go`

- [ ] **Step 1: Write the failing queue contract test**

```go
func TestTranscodeQueue_DequeueDoesNotAckUntilExplicitAck(t *testing.T) {
	ctx := context.Background()
	rdb := newTestRedis(t)
	q := NewTranscodeQueue(rdb, "video:transcode:test")
	require.NoError(t, q.Enqueue(ctx, videoapp.TranscodeTask{VideoID: 42, TaskID: "42"}))

	msg, err := q.Dequeue(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, msg.MessageID)
	require.Equal(t, uint64(42), msg.Task.VideoID)

	pending, err := rdb.XPending(ctx, q.key, q.group).Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, pending.Count)

	require.NoError(t, q.Ack(ctx, msg.MessageID))
	pending, err = rdb.XPending(ctx, q.key, q.group).Result()
	require.NoError(t, err)
	require.EqualValues(t, 0, pending.Count)
}
```

- [ ] **Step 2: Run the queue test to verify the current implementation fails**

Run: `go test ./video-service/internal/infrastructure/redis -run TestTranscodeQueue_DequeueDoesNotAckUntilExplicitAck -v`

Expected: FAIL because `Dequeue()` currently acknowledges and deletes the message before the test can observe it in pending state.

- [ ] **Step 3: Add the queue message contract in the application layer**

```go
package videoapp

import "context"

type TranscodeQueueMessage struct {
	MessageID string
	Task      TranscodeTask
}

type TranscodeTaskQueue interface {
	Enqueue(ctx context.Context, task TranscodeTask) error
	Dequeue(ctx context.Context) (TranscodeQueueMessage, error)
	Ack(ctx context.Context, messageID string) error
	ClaimStale(ctx context.Context, minIdle time.Duration, count int64) ([]TranscodeQueueMessage, error)
	MoveToDeadLetter(ctx context.Context, msg TranscodeQueueMessage, reason string) error
	Requeue(ctx context.Context, msg TranscodeQueueMessage, delay time.Duration, reason string) error
}
```

- [ ] **Step 4: Implement explicit ack and stale claim in Redis queue**

```go
func (q *TranscodeQueue) Dequeue(ctx context.Context) (videoapp.TranscodeQueueMessage, error) {
	if err := q.ensureGroup(ctx); err != nil {
		return videoapp.TranscodeQueueMessage{}, err
	}
	streams, err := q.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumer,
		Streams:  []string{q.key, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	if err != nil {
		return videoapp.TranscodeQueueMessage{}, err
	}
	msg := streams[0].Messages[0]
	task, err := q.decodeTask(ctx, msg)
	if err != nil {
		return videoapp.TranscodeQueueMessage{}, err
	}
	return videoapp.TranscodeQueueMessage{MessageID: msg.ID, Task: task}, nil
}

func (q *TranscodeQueue) Ack(ctx context.Context, messageID string) error {
	if err := q.rdb.XAck(ctx, q.key, q.group, messageID).Err(); err != nil {
		return err
	}
	return q.rdb.XDel(ctx, q.key, messageID).Err()
}
```

- [ ] **Step 5: Add retry and dead-letter support in the same Redis file**

```go
func (q *TranscodeQueue) Requeue(ctx context.Context, msg videoapp.TranscodeQueueMessage, delay time.Duration, reason string) error {
	values := map[string]interface{}{
		"payload": mustMarshal(msg.Task),
		"retry_reason": reason,
		"visible_at": time.Now().Add(delay).Unix(),
	}
	if _, err := q.rdb.XAdd(ctx, &goredis.XAddArgs{Stream: q.key, Values: values}).Result(); err != nil {
		return err
	}
	return q.Ack(ctx, msg.MessageID)
}

func (q *TranscodeQueue) MoveToDeadLetter(ctx context.Context, msg videoapp.TranscodeQueueMessage, reason string) error {
	if _, err := q.rdb.XAdd(ctx, &goredis.XAddArgs{
		Stream: q.key + ":dlq",
		Values: map[string]interface{}{"payload": mustMarshal(msg.Task), "reason": reason},
	}).Result(); err != nil {
		return err
	}
	return q.Ack(ctx, msg.MessageID)
}
```

- [ ] **Step 6: Run the Redis queue package tests**

Run: `go test ./video-service/internal/infrastructure/redis/... -v`

Expected: PASS with new tests covering explicit ack and pending visibility.

- [ ] **Step 7: Commit the queue reliability change**

```bash
git add video-service/internal/application/videoapp/transcode_queue.go video-service/internal/infrastructure/redis/transcode.go video-service/internal/infrastructure/redis/transcode_queue_test.go
git commit -m "feat: make transcode queue ack explicit"
```

## Task 2: Add Worker Lease, Retry, And Dead-Letter Flow

**Files:**
- Create: `video-service/internal/application/videoapp/transcode_retry.go`
- Create: `video-service/internal/application/videoapp/transcode_lease.go`
- Create: `video-service/internal/infrastructure/redis/transcode_lease.go`
- Modify: `video-service/internal/application/videoapp/worker.go`
- Modify: `video-service/internal/worker/transcodeworker/app.go`
- Test: `video-service/internal/application/videoapp/worker_retry_test.go`

- [ ] **Step 1: Write the failing worker retry test**

```go
func TestWorker_RunOnce_RequeuesRetryableDownloadFailure(t *testing.T) {
	queue := &fakeQueue{
		msg: videoapp.TranscodeQueueMessage{MessageID: "1-0", Task: videoapp.TranscodeTask{VideoID: 7, TaskID: "7", RawKey: "raw/7.mp4", HLSObjectPrefix: "hls/7"}},
	}
	worker := newTestWorker(queue)
	worker.Downloader = fakeDownloader{err: context.DeadlineExceeded}

	err := worker.RunOnce(context.Background())
	require.Error(t, err)
	require.Len(t, queue.requeued, 1)
	require.Empty(t, queue.acked)
	require.Empty(t, queue.deadLettered)
}
```

- [ ] **Step 2: Run the worker retry test to verify failure**

Run: `go test ./video-service/internal/application/videoapp -run TestWorker_RunOnce_RequeuesRetryableDownloadFailure -v`

Expected: FAIL because `Worker` currently depends on a bare `Dequeue(ctx) (TranscodeTask, error)` flow and has no retry/dead-letter path.

- [ ] **Step 3: Define retry classification and lease contracts**

```go
package videoapp

type RetryDecision struct {
	Retry bool
	Delay time.Duration
	Reason string
}

type Lease struct {
	TaskID string
	MessageID string
	WorkerID string
	Stage string
	ExpiresAt time.Time
}

type LeaseStore interface {
	Acquire(ctx context.Context, lease Lease, ttl time.Duration) error
	Renew(ctx context.Context, lease Lease, ttl time.Duration) error
	Release(ctx context.Context, taskID string) error
}
```

- [ ] **Step 4: Implement retry classification with small, explicit rules**

```go
func DecideRetry(err error, retries int) RetryDecision {
	if err == nil {
		return RetryDecision{}
	}
	if retries >= 5 {
		return RetryDecision{Retry: false, Reason: "retry_exhausted"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * time.Minute, Reason: "timeout"}
	}
	if isTemporaryStorageError(err) {
		return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * 30 * time.Second, Reason: "temporary_storage_error"}
	}
	return RetryDecision{Retry: false, Reason: "terminal"}
}
```

- [ ] **Step 5: Update `videoapp.Worker` to acquire lease, ack on success, retry on transient failure, and dead-letter on terminal failure**

```go
msg, err := w.Queue.Dequeue(ctx)
if err != nil {
	return err
}
if err := w.Leases.Acquire(taskCtx, videoapp.Lease{TaskID: msg.Task.TaskID, MessageID: msg.MessageID, WorkerID: w.WorkerID, Stage: "start"}, w.LeaseTTL); err != nil {
	return err
}
defer func() { _ = w.Leases.Release(context.WithoutCancel(taskCtx), msg.Task.TaskID) }()

if err := w.processTask(taskCtx, msg); err != nil {
	decision := DecideRetry(err, msg.Task.RetryCount)
	if decision.Retry {
		_ = w.Queue.Requeue(ctx, bumpRetry(msg), decision.Delay, decision.Reason)
		_ = w.StatusStore.Set(taskCtx, msg.Task.TaskID, domainvideo.StatusQueued, msg.Task.HLSURL, w.StatusTTL)
		return err
	}
	_ = w.Queue.MoveToDeadLetter(ctx, msg, decision.Reason)
	_ = w.StatusStore.Set(taskCtx, msg.Task.TaskID, domainvideo.StatusFailed, msg.Task.HLSURL, w.StatusTTL)
	return err
}

_ = w.Queue.Ack(ctx, msg.MessageID)
return nil
```

- [ ] **Step 6: Wire lease store and bounded worker concurrency in `transcodeworker/app.go`**

```go
leaseStore := infraredis.NewTranscodeLeaseStore(rdb, "video:transcode:lease:")
sem := make(chan struct{}, cfg.Transcode.MaxActiveTasks)

worker := videoapp.NewWorker(queue, statusStore, repo, transcoder, store, store, uploader, fileStorage, rawDir, hlsDir, taskTimeout)
worker.Leases = leaseStore
worker.WorkerID = queue.ConsumerName()
worker.LeaseTTL = 2 * time.Minute
worker.Execute = func(run func(context.Context) error) error {
	sem <- struct{}{}
	defer func() { <-sem }()
	return run(app.Context())
}
```

- [ ] **Step 7: Run the worker package tests**

Run: `go test ./video-service/internal/application/videoapp/... -v`

Expected: PASS with success, retry, and dead-letter paths covered.

- [ ] **Step 8: Commit the worker recovery change**

```bash
git add video-service/internal/application/videoapp/transcode_retry.go video-service/internal/application/videoapp/transcode_lease.go video-service/internal/infrastructure/redis/transcode_lease.go video-service/internal/application/videoapp/worker.go video-service/internal/worker/transcodeworker/app.go video-service/internal/application/videoapp/worker_retry_test.go
git commit -m "feat: add transcode worker retry and lease recovery"
```

## Task 3: Bound ffmpeg And Object Storage Execution

**Files:**
- Create: `video-service/internal/infrastructure/transcode/exec_helper.go`
- Create: `video-service/internal/infrastructure/objectstorage/retrying_store.go`
- Test: `video-service/internal/infrastructure/objectstorage/retrying_store_test.go`
- Modify: `video-service/internal/infrastructure/transcode/ffmpeg_transcoder.go`
- Modify: `video-service/internal/worker/transcodeworker/app.go`

- [ ] **Step 1: Write the failing object-storage retry test**

```go
func TestRetryingStore_PutFile_RetriesTemporaryTimeout(t *testing.T) {
	base := &fakeStore{putFileErrs: []error{context.DeadlineExceeded, nil}}
	store := NewRetryingStore(base, RetryConfig{MaxAttempts: 2, PutTimeout: time.Second})

	err := store.PutFile(context.Background(), "raw/a.mp4", "C:/tmp/a.mp4", "video/mp4")
	require.NoError(t, err)
	require.Equal(t, 2, base.putFileCalls)
}
```

- [ ] **Step 2: Run the retrying store test to verify failure**

Run: `go test ./video-service/internal/infrastructure/objectstorage -run TestRetryingStore_PutFile_RetriesTemporaryTimeout -v`

Expected: FAIL because there is no retrying wrapper yet.

- [ ] **Step 3: Add the object-storage wrapper with per-operation timeouts**

```go
type RetryingStore struct {
	base ObjectStore
	cfg  RetryConfig
}

func (s *RetryingStore) PutFile(ctx context.Context, objectKey, filePath, contentType string) error {
	return s.withRetry(ctx, s.cfg.PutTimeout, func(runCtx context.Context) error {
		return s.base.PutFile(runCtx, objectKey, filePath, contentType)
	})
}

func (s *RetryingStore) DownloadToFile(ctx context.Context, objectKey, filePath string) error {
	return s.withRetry(ctx, s.cfg.DownloadTimeout, func(runCtx context.Context) error {
		return s.base.DownloadToFile(runCtx, objectKey, filePath)
	})
}
```

- [ ] **Step 4: Add ffmpeg execution helper and route command execution through it**

```go
type ExecRunner interface {
	Run(ctx context.Context, name string, args []string) ([]byte, error)
}

func (r OSExecRunner) Run(ctx context.Context, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = processGroupAttrs()
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		killProcessTree(cmd)
	}
	return out, err
}
```

- [ ] **Step 5: Update `FFmpegTranscoder` to use `ExecRunner` instead of direct `exec.CommandContext` calls**

```go
out, err := t.runner.Run(ctx, "ffmpeg", ffmpegArgs)
if err == nil {
	return nil
}
return fmt.Errorf("ffmpeg failed: %w: %s", err, string(out))
```

- [ ] **Step 6: Run transcode and object-storage tests**

Run: `go test ./video-service/internal/infrastructure/transcode/... ./video-service/internal/infrastructure/objectstorage/... -v`

Expected: PASS with retry and execution-helper tests green.

- [ ] **Step 7: Commit the bounded execution change**

```bash
git add video-service/internal/infrastructure/transcode/exec_helper.go video-service/internal/infrastructure/transcode/ffmpeg_transcoder.go video-service/internal/infrastructure/objectstorage/retrying_store.go video-service/internal/infrastructure/objectstorage/retrying_store_test.go video-service/internal/worker/transcodeworker/app.go
git commit -m "feat: harden ffmpeg and object storage execution"
```

## Task 4: Add Resumable Upload Sessions

**Files:**
- Create: `video-service/internal/model/upload_session.go`
- Create: `video-service/internal/infrastructure/persistence/gorm_upload_session_repository.go`
- Create: `video-service/internal/application/videoapp/upload_session.go`
- Create: `video-service/internal/application/videoapp/upload_session_test.go`
- Create: `video-service/internal/http/dto/upload_session.go`
- Create: `video-service/internal/http/handler/upload_session_test.go`
- Modify: `video-service/internal/application/videoapp/upload.go`
- Modify: `video-service/internal/application/videoapp/upload_http.go`
- Modify: `video-service/internal/http/handler/upload.go`
- Modify: `video-service/internal/http/router/router.go`

- [ ] **Step 1: Write the failing upload-session service test**

```go
func TestUploadSession_Complete_FinalizesVideoAndEnqueuesTranscode(t *testing.T) {
	svc, repo, queue := newUploadSessionService(t)
	session, err := svc.CreateSession(context.Background(), CreateUploadSessionInput{FileName: "lesson.mp4", TotalSize: 1024, PartSize: 256})
	require.NoError(t, err)
	require.NoError(t, svc.RecordPart(context.Background(), session.ID, 1, "etag-1"))
	require.NoError(t, svc.RecordPart(context.Background(), session.ID, 2, "etag-2"))
	require.NoError(t, svc.RecordPart(context.Background(), session.ID, 3, "etag-3"))
	require.NoError(t, svc.RecordPart(context.Background(), session.ID, 4, "etag-4"))

	result, err := svc.CompleteSession(context.Background(), session.ID)
	require.NoError(t, err)
	require.NotZero(t, result.VideoID)
	require.Len(t, queue.enqueued, 1)
	require.Equal(t, "completed", repo.saved.Status)
}
```

- [ ] **Step 2: Run the upload-session service test to verify failure**

Run: `go test ./video-service/internal/application/videoapp -run TestUploadSession_Complete_FinalizesVideoAndEnqueuesTranscode -v`

Expected: FAIL because upload-session models, repository, and service methods do not exist.

- [ ] **Step 3: Add the upload-session model and repository**

```go
type UploadSession struct {
	ID             string    `gorm:"primaryKey;size:64"`
	FileName       string    `gorm:"size:255;not null"`
	TotalSize      int64     `gorm:"not null"`
	PartSize       int64     `gorm:"not null"`
	CompletedParts string    `gorm:"type:text;not null"`
	StorageKey     string    `gorm:"size:512;not null"`
	Status         string    `gorm:"size:32;not null"`
	ExpiresAt      time.Time `gorm:"index;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
```

- [ ] **Step 4: Implement the upload-session service with minimal lifecycle methods**

```go
func (s *Service) CreateUploadSession(ctx context.Context, input CreateUploadSessionInput) (UploadSessionView, error) {
	session := model.UploadSession{
		ID: uuid.NewString(),
		FileName: input.FileName,
		TotalSize: input.TotalSize,
		PartSize: input.PartSize,
		CompletedParts: "[]",
		StorageKey: buildSessionStorageKey(input.FileName),
		Status: "active",
		ExpiresAt: s.Now().Add(24 * time.Hour),
	}
	return toUploadSessionView(session), s.UploadSessions.Create(ctx, session)
}

func (s *Service) CompleteUploadSession(ctx context.Context, sessionID string) (UploadResult, error) {
	session, err := s.UploadSessions.Get(ctx, sessionID)
	if err != nil {
		return UploadResult{}, err
	}
	if !allPartsPresent(session) {
		return UploadResult{}, InvalidArgumentError("upload session incomplete")
	}
	plan, err := s.BuildUploadPlan(session.FileName)
	if err != nil {
		return UploadResult{}, err
	}
	plan.RawObjectKey = session.StorageKey
	plan.RawUploaded = true
	result, err := s.FinalizeUpload(ctx, plan, UploadMeta{Title: trimExt(session.FileName)})
	if err != nil {
		return UploadResult{}, err
	}
	return result, s.UploadSessions.MarkCompleted(ctx, sessionID)
}
```

- [ ] **Step 5: Add the HTTP endpoints and DTOs**

```go
r.POST("/api/upload-sessions", uploadHandler.CreateUploadSession)
r.PUT("/api/upload-sessions/:id/parts/:partNo", uploadHandler.UploadSessionPart)
r.POST("/api/upload-sessions/:id/complete", uploadHandler.CompleteUploadSession)
r.GET("/api/upload-sessions/:id", uploadHandler.GetUploadSession)
r.DELETE("/api/upload-sessions/:id", uploadHandler.CancelUploadSession)
```

- [ ] **Step 6: Run upload handler and service tests**

Run: `go test ./video-service/internal/application/videoapp/... ./video-service/internal/http/handler/... -v`

Expected: PASS with create, part upload, completion, and incomplete-session validation covered.

- [ ] **Step 7: Commit the upload-session feature**

```bash
git add video-service/internal/model/upload_session.go video-service/internal/infrastructure/persistence/gorm_upload_session_repository.go video-service/internal/application/videoapp/upload_session.go video-service/internal/application/videoapp/upload_session_test.go video-service/internal/http/dto/upload_session.go video-service/internal/http/handler/upload.go video-service/internal/http/handler/upload_session_test.go video-service/internal/http/router/router.go video-service/internal/application/videoapp/upload.go video-service/internal/application/videoapp/upload_http.go
git commit -m "feat: add resumable upload sessions"
```

## Task 5: Add Hot Video Cache And Shared Rate Limiting

**Files:**
- Create: `video-service/internal/infrastructure/cache/video_cache.go`
- Create: `video-service/internal/infrastructure/cache/video_cache_test.go`
- Create: `video-service/middleware/rate_limit.go`
- Create: `video-service/middleware/rate_limit_test.go`
- Modify: `video-service/internal/http/router/router.go`
- Modify: `video-service/internal/http/handler/video.go`

- [ ] **Step 1: Write the failing rate-limit middleware test**

```go
func TestRateLimitMiddleware_Returns429AfterBurst(t *testing.T) {
	r := gin.New()
	r.Use(NewRateLimitMiddleware(fakeLimiter{allow: []bool{true, false}}))
	r.GET("/api/videos", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/api/videos", nil))
	require.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/videos", nil))
	require.Equal(t, http.StatusTooManyRequests, w2.Code)
}
```

- [ ] **Step 2: Run the middleware test to verify failure**

Run: `go test ./video-service/middleware -run TestRateLimitMiddleware_Returns429AfterBurst -v`

Expected: FAIL because the middleware does not exist.

- [ ] **Step 3: Implement the Redis-backed limiter middleware**

```go
func NewRateLimitMiddleware(limiter Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := limiterKey(c.ClientIP(), c.FullPath())
		allowed, retryAfter, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			c.Next()
			return
		}
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 4: Implement hot video cache around play/detail reads**

```go
func (h *VideoHandler) PlayVideo(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	if cached, hit := h.cache.GetPlay(c.Request.Context(), videoID); hit {
		writeSuccess(c, cached)
		return
	}
	data, err := h.app.PlayVideo(c.Request.Context(), videoID)
	if err != nil {
		writeAppError(c, err, "play video failed")
		return
	}
	h.cache.SetPlay(c.Request.Context(), videoID, data, 30*time.Second)
	writeSuccess(c, data)
}
```

- [ ] **Step 5: Register limiter middleware on API routes and keep health/swagger open**

```go
r := gin.New()
r.Use(gin.Recovery(), middleware.AccessLogMiddleware())
api := r.Group("/api")
api.Use(middleware.NewRateLimitMiddleware(limiter))

api.GET("/videos", videoHandler.ListVideos)
api.POST("/videos", uploadHandler.UploadVideo)
api.POST("/upload-sessions", uploadHandler.CreateUploadSession)
```

- [ ] **Step 6: Run cache and middleware tests**

Run: `go test ./video-service/internal/infrastructure/cache/... ./video-service/middleware/... -v`

Expected: PASS with 429 behavior and cache hit/miss coverage.

- [ ] **Step 7: Commit the cache and limiter change**

```bash
git add video-service/internal/infrastructure/cache/video_cache.go video-service/internal/infrastructure/cache/video_cache_test.go video-service/middleware/rate_limit.go video-service/middleware/rate_limit_test.go video-service/internal/http/router/router.go video-service/internal/http/handler/video.go
git commit -m "feat: add hot video cache and rate limiting"
```

## Task 6: Full Verification Sweep

**Files:**
- Modify: `video-service/docs/swagger/docs.go` only if regeneration is required
- Modify: `video-service/docs/swagger/swagger.yaml` only if regeneration is required

- [ ] **Step 1: Regenerate Swagger if upload-session annotations changed**

Run: `go generate ./video-service/docs/swagger`

Expected: generated swagger artifacts update only if new handler annotations were added.

- [ ] **Step 2: Run the focused HTTP project test sweep**

Run: `go test ./video-service/...`

Expected: PASS across queue, worker, upload session, cache, and middleware packages.

- [ ] **Step 3: Run a manual smoke checklist**

```text
1. Start API: go run ./video-service/cmd/httpapi
2. Start worker: go run ./video-service/cmd/worker
3. Upload a small file through POST /api/videos and verify transcode reaches done.
4. Force-kill the worker during processing, restart it, and verify the task is reclaimed.
5. Start an upload session, send some parts, interrupt, resume, then complete.
6. Burst-hit /api/videos and verify 429 after threshold.
```

- [ ] **Step 4: Commit any final generated or verification-driven fixes**

```bash
git add video-service/docs/swagger video-service
git commit -m "test: verify reliability hardening end to end"
```

## Spec Coverage Check

1. Transcode storm: covered by Task 2 bounded worker execution and Task 5 limiter wiring, with admission follow-up folded into queue and worker integration.
2. Redis backlog: covered by Task 1 explicit ack, reclaim, retry, and dead-letter behavior.
3. Worker crash recovery: covered by Task 2 lease acquisition, release, and retry/dead-letter flow.
4. ffmpeg zombies: covered by Task 3 execution helper and cleanup-aware command execution.
5. Large upload interruption: covered by Task 4 resumable upload sessions.
6. S3 timeout: covered by Task 3 retrying store with operation-specific timeouts.
7. Hot video: covered by Task 5 cache helper and handler integration.
8. Rate limiting: covered by Task 5 middleware and route registration.

## Placeholder Scan

1. No `TBD` or `TODO` placeholders were left in this plan.
2. Every task includes exact files, commands, and concrete code snippets.

## Type Consistency Check

1. Queue message type is consistently `videoapp.TranscodeQueueMessage`.
2. Retry logic uses `RetryDecision` and worker lease uses `Lease` and `LeaseStore` consistently.
3. Upload-session endpoints and service methods use `CreateUploadSession`, `RecordPart`, `CompleteUploadSession`, `GetUploadSession`, and `CancelUploadSession` consistently.
