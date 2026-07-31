# Knowledge Point Video Playback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an isolated ZIP/XLSX import, HLS transcode, and `user_id + knowledge_point_id` playback chain backed by dedicated tables, Redis Stream, and private object-storage bucket.

**Architecture:** Add a focused `internal/application/knowledgevideo` module with import, worker, and playback services. Give it a dedicated GORM repository, Redis queue, object-store instance, HTTP handlers, and media proxy; reuse only the existing FFmpeg and S3-compatible infrastructure and never call the ordinary video upload/vector APIs.

**Tech Stack:** Go 1.26, Gin, GORM/PostgreSQL, Redis Streams, S3-compatible RustFS/COS, FFmpeg HLS, `github.com/xuri/excelize/v2`, Go unit tests, SQLite repository tests, miniredis queue tests.

---

## File Map

New application files:

- `internal/application/knowledgevideo/types.go`: statuses, import/playback DTOs, queue task types, validation errors.
- `internal/application/knowledgevideo/contracts.go`: repository, queue, object store, transcoder, and filesystem ports.
- `internal/application/knowledgevideo/validator.go`: XLSX/ZIP parsing and complete side-effect-free validation.
- `internal/application/knowledgevideo/import.go`: ID reservation, object upload, transactional persistence, compensation, and enqueue.
- `internal/application/knowledgevideo/playback.go`: ready-video resolution and playback-record append.
- `internal/application/knowledgevideo/worker.go`: idempotent transcode lifecycle and batch-state refresh.
- `internal/application/knowledgevideo/reconcile.go`: stale pending-row enqueue reconciliation.

New infrastructure and protocol files:

- `internal/model/knowledge_video.go`: three GORM models.
- `internal/infrastructure/persistence/gorm_knowledge_video_repository.go`: dictionary reads and knowledge-video writes.
- `internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go`: persistence and uniqueness tests.
- `internal/infrastructure/redis/knowledge_video_transcode.go`: typed Redis Stream adapter.
- `internal/infrastructure/redis/knowledge_video_transcode_test.go`: queue serialization/reclaim tests.
- `internal/http/dto/knowledge_video.go`: public request/response structures.
- `internal/http/handler/knowledgevideos/handler.go`: import, progress, and playback handlers.
- `internal/http/handler/knowledgevideos/handler_test.go`: HTTP contract tests.
- `internal/http/handler/knowledge_video_media.go`: constrained dedicated-bucket media proxy.
- `internal/http/handler/knowledge_video_media_test.go`: media path/range/security tests.
- `internal/worker/knowledgevideoworker/app.go`: worker composition and lifecycle loops.
- `internal/worker/knowledgevideoworker/app_test.go`: worker config/registration seams.

Existing integration files:

- `go.mod`, `go.sum`: add Excel parser dependency.
- `internal/config/types.go`, `internal/config/objectstorage.go`, `internal/config/defaults.go`, `internal/config/loader_test.go`: dedicated storage, queue, limits, and worker configuration.
- `configs/video.yml`, `configs/video_prod.yml`: explicit local and production defaults.
- `internal/infrastructure/persistence/migration.go`: migrate models and partial indexes.
- `internal/infrastructure/objectstorage/rustfs.go`, `rustfs_test.go`: safe object-prefix cleanup needed by compensation.
- `internal/http/app/app.go`, `app_test.go`: construct dedicated store, repository, queue, and application service.
- `internal/http/router/router.go`, `media_route_test.go`, `swagger_test.go`: register REST and proxy routes.
- `internal/worker/combined/app.go`, `app_test.go`: start knowledge-video worker.
- `docs/swagger/generate.go`: include handler annotations in generated Swagger input.

## Task 1: Configuration and Data Model

**Files:**
- Modify: `internal/config/types.go`
- Modify: `internal/config/objectstorage.go`
- Modify: `internal/config/defaults.go`
- Modify: `internal/config/loader_test.go`
- Modify: `configs/video.yml`
- Modify: `configs/video_prod.yml`
- Create: `internal/model/knowledge_video.go`
- Modify: `internal/infrastructure/persistence/migration.go`
- Test: `internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go`

- [ ] **Step 1: Write failing config and migration tests**

Add config assertions for a dedicated bucket, route prefix, temp path, archive limits, queue key, worker count, and timeout. Add a SQLite migration test that calls `EnsureSchema`, checks all three tables, and checks the active knowledge-point unique index.

```go
func TestKnowledgeVideoConfigDefaults(t *testing.T) {
	cfg := Config{}
	applyDefaults(&cfg)
	if cfg.KnowledgeVideoStorage.Bucket != "knowledge-point-videos" {
		t.Fatalf("bucket = %q", cfg.KnowledgeVideoStorage.Bucket)
	}
	if cfg.KnowledgeVideoStorage.MediaRoutePrefix != "/knowledge-video-media" {
		t.Fatalf("route = %q", cfg.KnowledgeVideoStorage.MediaRoutePrefix)
	}
	if cfg.RedisKeys.KnowledgeVideoTranscodeQueue != "knowledge_video:transcode:stream" {
		t.Fatalf("queue = %q", cfg.RedisKeys.KnowledgeVideoTranscodeQueue)
	}
}
```

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./internal/config ./internal/infrastructure/persistence -run 'TestKnowledgeVideo'`

Expected: FAIL because the new config fields, models, and migration are absent.

- [ ] **Step 3: Add concrete config and models**

Define configuration without overloading the existing `RustFS` bucket:

```go
type KnowledgeVideoStorageConfig struct {
	Endpoint          string `yaml:"Endpoint"`
	AccessKey         string `yaml:"AccessKey"`
	SecretKey         string `yaml:"SecretKey"`
	Bucket            string `yaml:"Bucket"`
	UseSSL            bool   `yaml:"UseSSL"`
	Region            string `yaml:"Region"`
	BucketLookup      string `yaml:"BucketLookup"`
	MediaRoutePrefix  string `yaml:"MediaRoutePrefix"`
	TempPath          string `yaml:"TempPath"`
	MaxArchiveBytes   int64  `yaml:"MaxArchiveBytes"`
	MaxExpandedBytes  int64  `yaml:"MaxExpandedBytes"`
	MaxEntryBytes     int64  `yaml:"MaxEntryBytes"`
	MaxEntries        int    `yaml:"MaxEntries"`
}

type KnowledgeVideoWorkerConfig struct {
	WorkerCount        int `yaml:"WorkerCount"`
	TaskTimeoutMinutes int `yaml:"TaskTimeoutMinutes"`
	ShutdownTimeoutSec int `yaml:"ShutdownTimeoutSec"`
}
```

Add `KnowledgeVideoStorage` and `KnowledgeVideoWorker` to `Config`, plus `KnowledgeVideoTranscodeQueue` to `RedisKeysConfig`. Add a `KnowledgeVideoObjectStorageConfig` helper that falls back to the existing endpoint/credentials only when dedicated values are empty, while never falling back to the existing bucket.

Create three GORM models with exact table names and status fields from the design. Use `int16` for status/deleted, `uint64` for IDs, and `time.Time` for timestamps. Add nullable `enqueue_time` to `edu_knowledge_video`; it records the most recent enqueue attempt and lets reconciliation select only stale pending rows. Register the models in `EnsureSchema`, then create PostgreSQL-compatible partial indexes with `CREATE UNIQUE INDEX IF NOT EXISTS`.

- [ ] **Step 4: Run tests and format**

Run: `gofmt -w internal/config/types.go internal/config/objectstorage.go internal/config/defaults.go internal/config/loader_test.go internal/model/knowledge_video.go internal/infrastructure/persistence/migration.go internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go`

Run: `go test ./internal/config ./internal/infrastructure/persistence -run 'TestKnowledgeVideo'`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add configs/video.yml configs/video_prod.yml internal/config internal/model/knowledge_video.go internal/infrastructure/persistence/migration.go internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go
git commit -m "feat: add knowledge video schema and config"
```

## Task 2: Repository and Application Contracts

**Files:**
- Create: `internal/application/knowledgevideo/types.go`
- Create: `internal/application/knowledgevideo/contracts.go`
- Create: `internal/infrastructure/persistence/gorm_knowledge_video_repository.go`
- Modify: `internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go`

- [ ] **Step 1: Write repository tests first**

Cover batched dictionary lookup, sequence reservation, transactional batch creation, active duplicate detection, ready lookup, playback append, state transition, batch refresh, and stale-pending listing.

```go
func TestKnowledgeVideoRepositoryResolveReadyAndRecordPlayback(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	seedKnowledgeVideo(t, db, 41, 9, 2)
	got, ok, err := repo.ResolveReady(context.Background(), 9)
	if err != nil || !ok || got.ID != 41 {
		t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
	}
	if err := repo.RecordPlayback(context.Background(), knowledgevideo.PlayRecord{UserID: 7, KnowledgePointID: 9, KnowledgeVideoID: 41}); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run repository tests and verify failure**

Run: `go test ./internal/infrastructure/persistence -run KnowledgeVideoRepository`

Expected: FAIL because the application contracts and repository do not exist.

- [ ] **Step 3: Define stable application types and ports**

Use explicit status constants and typed errors:

```go
type VideoStatus int16
const (
	VideoPending VideoStatus = iota
	VideoTranscoding
	VideoReady
	VideoFailed
)

type BatchStatus int16
const (
	BatchProcessing BatchStatus = iota + 1
	BatchCompleted
	BatchPartialFailed
	BatchFailed
)

type TranscodeTask struct {
	KnowledgeVideoID uint64 `json:"knowledge_video_id"`
	SourceObjectKey  string `json:"source_object_key"`
	HLSObjectPrefix  string `json:"hls_object_prefix"`
	TaskID           string `json:"task_id"`
	RetryCount       int    `json:"retry_count"`
}
```

Define small interfaces for dictionary lookup, ID reservation, batch creation, state updates, playback, queue operations, object put/delete/download, directory upload, FFmpeg conversion, and filesystem cleanup. Keep HTTP/GORM/Redis types out of this package.

- [ ] **Step 4: Implement the GORM repository**

Use a single batched query against `dict_knowledge_point`, respecting an optional `deleted` column as the existing repository does. Reserve IDs with PostgreSQL sequences in production and make the test constructor injectable for SQLite. Create the batch plus all video rows in one transaction. Refresh counters with conditional aggregate counts and derive `completed`, `partial_failed`, or `failed` in that same transaction.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/application/knowledgevideo internal/infrastructure/persistence/gorm_knowledge_video_repository.go internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go`

Run: `go test ./internal/application/knowledgevideo ./internal/infrastructure/persistence -run KnowledgeVideo`

Expected: PASS.

```bash
git add internal/application/knowledgevideo internal/infrastructure/persistence/gorm_knowledge_video_repository.go internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go
git commit -m "feat: add knowledge video repository"
```

## Task 3: Side-Effect-Free XLSX and ZIP Validation

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Create: `internal/application/knowledgevideo/validator.go`
- Create: `internal/application/knowledgevideo/validator_test.go`

- [ ] **Step 1: Add the parser dependency**

Run: `go get github.com/xuri/excelize/v2@v2.10.0`

Expected: `go.mod` and `go.sum` add a Go-1.26-compatible Excel parser version.

- [ ] **Step 2: Write table-driven validation tests**

Build ZIP and XLSX fixtures in memory. Cover exact headers, empty cells, invalid/duplicate IDs, duplicate filenames, missing dictionary IDs, mismatched names, traversal, symlinks, duplicate basenames, unsupported extensions, missing videos, extra videos, entry count, entry size, and expanded-size limits.

```go
func TestValidatorRejectsKnowledgePointNameMismatch(t *testing.T) {
	v := Validator{Dictionary: fakeDictionary{9: "一次函数"}, Limits: testLimits()}
	_, err := v.Validate(context.Background(), validArchive(t, "lesson.mp4"), mappingXLSX(t, [][]string{
		{"id", "name", "video_name"},
		{"9", "二次函数", "lesson.mp4"},
	}))
	assertValidationIssue(t, err, 2, "name", "knowledge point name does not match dictionary")
}
```

- [ ] **Step 3: Verify tests fail**

Run: `go test ./internal/application/knowledgevideo -run Validator`

Expected: FAIL because `Validator` is not implemented.

- [ ] **Step 4: Implement the validator**

Use `excelize.OpenReader` and read only the first worksheet. Normalize only surrounding whitespace; do not case-fold names or filenames. Open ZIP from a seekable temporary file, inspect `zip.FileHeader.Mode()` to reject symlinks, validate clean relative paths, index by basename, and accumulate expanded sizes before extracting. Return a validated manifest containing rows and exact ZIP entries; extraction happens only after every check passes.

Expose structured issues:

```go
type ValidationIssue struct {
	Row     int    `json:"row,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type ValidationError struct {
	Issues []ValidationIssue
}
```

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/application/knowledgevideo/validator.go internal/application/knowledgevideo/validator_test.go`

Run: `go test ./internal/application/knowledgevideo -run Validator`

Expected: PASS.

```bash
git add go.mod go.sum internal/application/knowledgevideo/validator.go internal/application/knowledgevideo/validator_test.go
git commit -m "feat: validate knowledge video imports"
```

## Task 4: Import Orchestration and Compensation

**Files:**
- Create: `internal/application/knowledgevideo/import.go`
- Create: `internal/application/knowledgevideo/import_test.go`
- Modify: `internal/infrastructure/objectstorage/rustfs.go`
- Modify: `internal/infrastructure/objectstorage/rustfs_test.go`

- [ ] **Step 1: Write failing orchestration tests**

Test successful object keys and enqueue calls, zero side effects on validation failure, cleanup after the second object upload fails, cleanup after database conflict, pending preservation after enqueue failure, and temporary directory removal on every exit.

```go
func TestImportEnqueueFailureLeavesCommittedPendingVideo(t *testing.T) {
	deps := successfulImportDependencies(t)
	deps.Queue.EnqueueErr = errors.New("redis unavailable")
	result, err := deps.Service.Import(context.Background(), validImportInput(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != BatchProcessing || deps.Repository.Created[0].Status != VideoPending {
		t.Fatalf("result=%+v rows=%+v", result, deps.Repository.Created)
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/application/knowledgevideo -run Import`

Expected: FAIL because `ImportService` is absent.

- [ ] **Step 3: Implement bounded extraction and import sequencing**

Implement the exact sequence: validate, reserve IDs, extract each accepted entry with `io.LimitReader`, upload `manifests/{batch}/mapping.xlsx` and `raw/{video}/source{ext}`, transactionally create rows, enqueue each task, and remove temp files. Track every successfully uploaded key and delete it in reverse order when object upload or database creation fails.

Return an accepted result even when a post-commit enqueue fails. Log that failure and let reconciliation recover it.

- [ ] **Step 4: Add prefix cleanup to the object store**

Add `DeletePrefix(ctx, prefix)` using MinIO `ListObjects` plus `RemoveObjects`, collect per-object deletion errors, and never accept an empty prefix. Test empty-prefix rejection and exact-prefix deletion with a fake/list seam rather than requiring live storage.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/application/knowledgevideo/import.go internal/application/knowledgevideo/import_test.go internal/infrastructure/objectstorage/rustfs.go internal/infrastructure/objectstorage/rustfs_test.go`

Run: `go test ./internal/application/knowledgevideo ./internal/infrastructure/objectstorage`

Expected: PASS.

```bash
git add internal/application/knowledgevideo/import.go internal/application/knowledgevideo/import_test.go internal/infrastructure/objectstorage/rustfs.go internal/infrastructure/objectstorage/rustfs_test.go
git commit -m "feat: import knowledge video batches"
```

## Task 5: Dedicated Redis Stream Queue

**Files:**
- Create: `internal/infrastructure/redis/knowledge_video_transcode.go`
- Create: `internal/infrastructure/redis/knowledge_video_transcode_test.go`

- [ ] **Step 1: Write failing miniredis tests**

Test enqueue/dequeue JSON round trip, ACK deletion, delayed requeue, pending reclaim, malformed payload removal, and DLQ transfer after terminal failure.

```go
func TestKnowledgeVideoQueueRoundTrip(t *testing.T) {
	q := newKnowledgeVideoTestQueue(t)
	want := knowledgevideo.TranscodeTask{KnowledgeVideoID: 8, SourceObjectKey: "raw/8/source.mp4", HLSObjectPrefix: "hls/8", TaskID: "knowledge-video-8"}
	if err := q.Enqueue(context.Background(), want); err != nil { t.Fatal(err) }
	msg, err := q.Dequeue(context.Background())
	if err != nil || msg.Task != want { t.Fatalf("msg=%+v err=%v", msg, err) }
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/infrastructure/redis -run KnowledgeVideo`

Expected: FAIL because the adapter is absent.

- [ ] **Step 3: Implement the queue using existing stream primitives**

Create a separate typed adapter over `enqueueStreamPayload`, `claimPendingStreamMessage`, `promoteDueDelayed`, `streamGroupName`, and `streamConsumerName`. Do not import or convert ordinary `videoapp.TranscodeTask`.

- [ ] **Step 4: Verify and commit**

Run: `gofmt -w internal/infrastructure/redis/knowledge_video_transcode.go internal/infrastructure/redis/knowledge_video_transcode_test.go`

Run: `go test ./internal/infrastructure/redis -run KnowledgeVideo`

Expected: PASS.

```bash
git add internal/infrastructure/redis/knowledge_video_transcode.go internal/infrastructure/redis/knowledge_video_transcode_test.go
git commit -m "feat: add knowledge video transcode queue"
```

## Task 6: Idempotent Transcode Worker and Reconciliation

**Files:**
- Create: `internal/application/knowledgevideo/worker.go`
- Create: `internal/application/knowledgevideo/worker_test.go`
- Create: `internal/application/knowledgevideo/reconcile.go`
- Create: `internal/application/knowledgevideo/reconcile_test.go`

- [ ] **Step 1: Write worker lifecycle tests**

Cover missing/deleted rows, ready redelivery, pending-to-transcoding-to-ready success, download/transcode/upload retries, exhausted retry to failed/DLQ, fixed output prefix, local cleanup, batch refresh, and stale-pending reconciliation.

```go
func TestWorkerReadyRedeliveryOnlyAcks(t *testing.T) {
	deps := readyWorkerDependencies()
	if err := deps.Worker.RunOnce(context.Background()); err != nil { t.Fatal(err) }
	if deps.Queue.AckCount != 1 || deps.Transcoder.CallCount != 0 {
		t.Fatalf("acks=%d transcodes=%d", deps.Queue.AckCount, deps.Transcoder.CallCount)
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/application/knowledgevideo -run 'Worker|Reconcile'`

Expected: FAIL because worker and reconciliation services are absent.

- [ ] **Step 3: Implement worker state transitions**

Download to `{temp}/raw/{task_id}{ext}`, convert into `{temp}/hls/{task_id}`, upload to the task's fixed prefix, verify the configured master playlist exists locally, then update duration/master key/status and ACK. Use five attempts with increasing bounded delays, Redis pending reclaim, and DLQ on exhaustion. Update video terminal state and batch counters in repository transactions.

Reuse `transcode.FFmpegTranscoder.ConvertToHLS`; do not generate covers, audio, segments, vectors, or ordinary video status cache entries.

- [ ] **Step 4: Implement pending reconciliation**

Every minute, query rows with `status=pending` and `enqueue_time` null or older than five minutes in bounded pages of 100. Reconstruct the stable task (`knowledge-video-{id}`), enqueue it, and update `enqueue_time` after a successful enqueue so each scan does not flood Redis. Cover both the initial enqueue and reconciliation update paths in repository tests.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/application/knowledgevideo/worker.go internal/application/knowledgevideo/worker_test.go internal/application/knowledgevideo/reconcile.go internal/application/knowledgevideo/reconcile_test.go internal/model/knowledge_video.go internal/infrastructure/persistence/gorm_knowledge_video_repository.go`

Run: `go test ./internal/application/knowledgevideo ./internal/infrastructure/persistence`

Expected: PASS.

```bash
git add internal/application/knowledgevideo internal/model/knowledge_video.go internal/infrastructure/persistence/gorm_knowledge_video_repository.go internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go
git commit -m "feat: transcode knowledge videos"
```

## Task 7: Playback Service

**Files:**
- Create: `internal/application/knowledgevideo/playback.go`
- Create: `internal/application/knowledgevideo/playback_test.go`

- [ ] **Step 1: Write playback behavior tests**

Test invalid IDs, missing dictionary ID, missing mapping, pending/transcoding/failed mapping, ready mapping, playback URL construction, and record-insert failure. Assert the service returns success only after `RecordPlayback` succeeds.

```go
func TestPlaybackReadyRecordsAccessBeforeReturning(t *testing.T) {
	repo := &playbackRepoStub{Video: readyVideo(88, 1001)}
	svc := PlaybackService{Repo: repo, MediaRoutePrefix: "/knowledge-video-media", Now: fixedTime}
	got, err := svc.Resolve(context.Background(), 7, 1001)
	if err != nil || got.PlaybackURL != "/knowledge-video-media/hls/88/master.m3u8" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if len(repo.Records) != 1 || repo.Records[0].UserID != 7 { t.Fatalf("records=%+v", repo.Records) }
}
```

- [ ] **Step 2: Verify failure, implement, and rerun**

Run: `go test ./internal/application/knowledgevideo -run Playback`

Expected before implementation: FAIL because `PlaybackService` is absent.

Implement typed `InvalidArgument`, `NotFound`, `NotReady`, and `TranscodeFailed` errors, then rerun the same command.

Expected after implementation: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/application/knowledgevideo/playback.go internal/application/knowledgevideo/playback_test.go internal/application/knowledgevideo/types.go
git commit -m "feat: resolve knowledge point video playback"
```

## Task 8: HTTP DTOs, Handlers, Routes, and Private Media Proxy

**Files:**
- Create: `internal/http/dto/knowledge_video.go`
- Create: `internal/http/handler/knowledgevideos/handler.go`
- Create: `internal/http/handler/knowledgevideos/handler_test.go`
- Create: `internal/http/handler/knowledge_video_media.go`
- Create: `internal/http/handler/knowledge_video_media_test.go`
- Modify: `internal/http/router/router.go`
- Modify: `internal/http/router/media_route_test.go`

- [ ] **Step 1: Write failing HTTP tests**

Cover multipart fields `archive`, `mapping`, `upload_user_id`; `202` accepted response; structured `400` validation issues; progress response; required playback query; `404` and both `409` codes; and successful JSON mapping. Add proxy tests for ready checks, relative segments, traversal, cross-video access, content type, ETag, and byte ranges.

```go
func TestPlaybackHandlerRequiresUserID(t *testing.T) {
	r := testKnowledgeVideoRouter(t, &handlerStub{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/knowledge-points/9/video", nil))
	if w.Code != http.StatusBadRequest { t.Fatalf("status=%d body=%s", w.Code, w.Body.String()) }
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/http/handler/... ./internal/http/router -run KnowledgeVideo`

Expected: FAIL because DTOs, handlers, and routes are absent.

- [ ] **Step 3: Implement handlers and error mapping**

Add:

```text
POST /api/admin/knowledge-videos/batches
GET  /api/admin/knowledge-videos/batches/:batchId
GET  /api/knowledge-points/:knowledgePointId/video?user_id=:userId
GET  /knowledge-video-media/hls/:videoId/*filepath
```

Set `MaxBytesReader` before multipart parsing using the configured archive limit plus bounded XLSX/form overhead. Stream both form files into `ImportService`; never call `io.ReadAll`. Map application errors to exact response statuses/codes from the design.

- [ ] **Step 4: Implement the constrained media proxy**

Parse a positive video ID, resolve its persisted ready HLS prefix, clean only the relative wildcard path, reject absolute/traversal paths, and then use the dedicated store's `Stat`/`Open`. Preserve existing proxy range semantics and MIME mapping without accepting raw object keys or bucket names.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/http/dto/knowledge_video.go internal/http/handler/knowledgevideos internal/http/handler/knowledge_video_media.go internal/http/handler/knowledge_video_media_test.go internal/http/router/router.go internal/http/router/media_route_test.go`

Run: `go test ./internal/http/handler/... ./internal/http/router -run KnowledgeVideo`

Expected: PASS.

```bash
git add internal/http/dto/knowledge_video.go internal/http/handler/knowledgevideos internal/http/handler/knowledge_video_media.go internal/http/handler/knowledge_video_media_test.go internal/http/router/router.go internal/http/router/media_route_test.go
git commit -m "feat: expose knowledge video APIs"
```

## Task 9: HTTP and Worker Composition

**Files:**
- Modify: `internal/http/app/app.go`
- Modify: `internal/http/app/app_test.go`
- Create: `internal/worker/knowledgevideoworker/app.go`
- Create: `internal/worker/knowledgevideoworker/app_test.go`
- Modify: `internal/worker/combined/app.go`
- Modify: `internal/worker/combined/app_test.go`

- [ ] **Step 1: Write failing composition tests**

Add seams that assert the HTTP app creates and exposes a distinct knowledge store/service and closes resources correctly. Assert combined worker registration includes the new worker and that worker count normalizes to at least one.

```go
func TestKnowledgeVideoWorkerCountFromConfigDefaultsToOne(t *testing.T) {
	if got := WorkerCountFromConfig(config.Config{}); got != 1 {
		t.Fatalf("got %d", got)
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/http/app ./internal/worker/knowledgevideoworker ./internal/worker/combined -run KnowledgeVideo`

Expected: FAIL because runtime composition is absent.

- [ ] **Step 3: Wire the HTTP runtime**

Extend `app.App` with `KnowledgeVideoService`, `KnowledgeVideoStore`, and `KnowledgeVideoMediaRoutePrefix`. Create/ensure the dedicated bucket at startup, construct the GORM repository and Redis queue, and inject configured limits/temp path. Update router construction to consume these explicit fields.

- [ ] **Step 4: Wire the worker runtime**

Register a dedicated store, repository, queue, FFmpeg transcoder, directory uploader, worker loops, and one reconciliation loop in `knowledgevideoworker.Register`. Call it from `combined.Run`, include its shutdown timeout in `maxShutdownTimeout`, and log the normalized worker count.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w internal/http/app/app.go internal/http/app/app_test.go internal/worker/knowledgevideoworker internal/worker/combined/app.go internal/worker/combined/app_test.go`

Run: `go test ./internal/http/app ./internal/worker/knowledgevideoworker ./internal/worker/combined -run KnowledgeVideo`

Expected: PASS.

```bash
git add internal/http/app internal/worker/knowledgevideoworker internal/worker/combined
git commit -m "feat: wire knowledge video runtime"
```

## Task 10: Swagger, Regression Proof, and Full Verification

**Files:**
- Modify: `internal/http/handler/knowledgevideos/handler.go`
- Modify: `internal/http/router/swagger_test.go`
- Modify: `docs/swagger/generate.go`
- Generated: `docs/swagger/docs.go`
- Generated: `docs/swagger/swagger.json`
- Generated: `docs/swagger/swagger.yaml`
- Create: `internal/application/knowledgevideo/isolation_test.go`

- [ ] **Step 1: Add Swagger and isolation tests**

Add annotations for all three JSON endpoints and assert generated paths exist. Add an import test with a panic-on-call ordinary video repository and vector queue; successful knowledge import must never invoke either dependency.

```go
func TestImportDoesNotDependOnOrdinaryVideoOrVectorChain(t *testing.T) {
	svc := newIsolatedImportService(t)
	if _, err := svc.Import(context.Background(), validImportInput(t)); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run isolation and router tests before generation**

Run: `go test ./internal/application/knowledgevideo ./internal/http/router -run 'Isolation|Swagger'`

Expected before Swagger regeneration: isolation PASS and Swagger path assertions FAIL.

- [ ] **Step 3: Regenerate Swagger**

Run: `go generate ./docs/swagger`

Expected: generated documentation contains `/api/admin/knowledge-videos/batches`, `/api/admin/knowledge-videos/batches/{batchId}`, and `/api/knowledge-points/{knowledgePointId}/video`.

- [ ] **Step 4: Run focused verification**

Run: `go test ./internal/application/knowledgevideo ./internal/infrastructure/persistence ./internal/infrastructure/redis ./internal/infrastructure/objectstorage ./internal/http/handler/... ./internal/http/router ./internal/http/app ./internal/worker/knowledgevideoworker ./internal/worker/combined`

Expected: PASS.

- [ ] **Step 5: Run full service verification**

Run: `go test ./...`

Expected: PASS with no PostgreSQL, Redis, object-storage, or FFmpeg service required by unit tests.

Run: `git diff --check`

Expected: no output.

- [ ] **Step 6: Review the final diff for isolation**

Run: `git diff --stat HEAD~10..HEAD`

Expected: changes are limited to knowledge-video files plus the named config, migration, app, router, worker, object-storage, dependency, and Swagger integration files. Confirm no changes to recommendation/vector logic.

- [ ] **Step 7: Commit**

```bash
git add internal/application/knowledgevideo/isolation_test.go internal/http/handler/knowledgevideos/handler.go internal/http/router/swagger_test.go docs/swagger
git commit -m "docs: publish knowledge video API"
```

## Operational Smoke Test

This is optional in local development because it requires PostgreSQL, Redis, the dedicated bucket, and FFmpeg. Run it in an integration environment after unit tests pass:

- [ ] Configure `KnowledgeVideoStorage`, start `go run ./cmd/worker`, and start `go run ./cmd/httpapi`.
- [ ] Upload a two-row XLSX and matching two-video ZIP; expect HTTP `202` and a batch ID.
- [ ] Poll the batch endpoint until both rows are `ready`; expect batch `completed`.
- [ ] Call the playback endpoint with a positive user ID; expect one HLS proxy URL.
- [ ] Fetch the master playlist and one segment through `/knowledge-video-media`; expect `200` and correct content types.
- [ ] Query the three new tables; expect one batch, two mappings, and one playback record.
- [ ] Inspect existing video/vector Redis Streams and `edu_video_resource`; expect no knowledge-video task or row.
