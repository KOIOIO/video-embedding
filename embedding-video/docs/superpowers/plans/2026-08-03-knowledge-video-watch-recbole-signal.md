# Knowledge Video Watch RecBole Signal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an idempotent knowledge-video watch API and use 60%-effective watches as RecBole training signals without exposing virtual items online.

**Architecture:** Vue reports absolute wall-clock seconds per UUID session; Go persists monotonic session maxima and returns a capped cross-session aggregate. The hourly exporter writes namespaced knowledge-video items to train only; RecBole masks them in full-sort evaluation and exports strictly numeric segment embeddings.

**Tech Stack:** Go, Gin, GORM/PostgreSQL, Vue 3/Vitest, Python/unittest, RecBole BPR, Bash.

---

## Change Map

- Go model/migration/repository: persisted sessions, partial unique index, atomic upsert and aggregate.
- Go application/HTTP: validation, 60% calculation, PUT DTO/handler/route.
- Vue: API transport, UUID session state, 15-second actual-play heartbeat.
- Go exporter: effective-watch aggregation, string item tokens, benchmark train/valid/test files.
- Python pipeline: benchmark config, evaluation mask, embedding filter, statistics and leak gates.

Several exporter and RecBole files already contain uncommitted company work. Before every commit run `git status --short` and `git diff --cached`; for overlapping files use `git add -p` and stage only this plan's hunks. Never reset or restore those files.

### Task 1: Persist Monotonic Watch Sessions

**Files:**
- Modify: `video-service/internal/model/knowledge_video.go`
- Modify: `video-service/internal/infrastructure/persistence/migration.go`
- Modify: `video-service/internal/infrastructure/persistence/gorm_knowledge_video_repository.go`
- Test: `video-service/internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go`

- [ ] **Step 1: Write the failing repository tests**

Add a ready 100-second video and real `sys_user`. Upsert session A with 37 then 20 seconds, session B with 90 seconds, and assert A remains 37 and the total caps at 100. Add a migration test asserting `uk_knowledge_video_play_session` exists and two legacy empty-session rows can coexist.

```go
reports := []knowledgevideo.WatchSessionReport{
 {UserID: 7, KnowledgeVideoID: 88, SessionID: "session-00000001", WatchedSeconds: 37, UpdatedAt: now},
 {UserID: 7, KnowledgeVideoID: 88, SessionID: "session-00000001", WatchedSeconds: 20, UpdatedAt: now.Add(time.Minute)},
 {UserID: 7, KnowledgeVideoID: 88, SessionID: "session-00000002", WatchedSeconds: 90, UpdatedAt: now.Add(2*time.Minute)},
}
if got.SessionWatchedSeconds != 90 || got.TotalWatchedSeconds != 100 { t.Fatalf("got %+v", got) }
```

- [ ] **Step 2: Verify RED**

Run from `video-service/`:

```bash
go test ./internal/infrastructure/persistence -run 'WatchSession|PartialSessionIndex' -count=1
```

Expected: compile failure because `WatchSessionReport` and `UpsertWatchSession` do not exist.

- [ ] **Step 3: Implement the persistence contract**

Add to `EduKnowledgeVideoPlayRecord`:

```go
SessionID string `gorm:"column:session_id;size:64"`
WatchDuration int `gorm:"column:watch_duration;not null;default:0"`
UpdateTime *time.Time `gorm:"column:update_time"`
```

Add after AutoMigrate:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS uk_knowledge_video_play_session
ON edu_knowledge_video_play_record(user_id, knowledge_video_id, session_id)
WHERE session_id IS NOT NULL AND session_id <> '';
```

Define `WatchSessionReport` and `WatchSessionAggregate`. In one transaction validate `sys_user.id`, then use:

```sql
ON CONFLICT (user_id, knowledge_video_id, session_id)
WHERE session_id IS NOT NULL AND session_id <> ''
DO UPDATE SET watch_duration=GREATEST(edu_knowledge_video_play_record.watch_duration, EXCLUDED.watch_duration),
update_time=GREATEST(edu_knowledge_video_play_record.update_time, EXCLUDED.update_time)
```

Read back the session maximum and sum only nonempty sessions; cap the sum at persisted video duration. Leave legacy `RecordPlayback` unchanged.

- [ ] **Step 4: Verify GREEN and commit**

```bash
go test ./internal/infrastructure/persistence -run 'KnowledgeVideo|WatchSession' -count=1
git add video-service/internal/model/knowledge_video.go video-service/internal/infrastructure/persistence/migration.go video-service/internal/infrastructure/persistence/gorm_knowledge_video_repository.go video-service/internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go
git diff --cached --check
git commit -m "feat: persist knowledge video watch sessions"
```

Expected: PASS and a commit containing only persistence changes.

### Task 2: Apply Validation and the 60% Rule

**Files:**
- Modify: `video-service/internal/application/knowledgevideo/contracts.go`
- Modify: `video-service/internal/application/knowledgevideo/types.go`
- Modify: `video-service/internal/application/knowledgevideo/playback.go`
- Test: `video-service/internal/application/knowledgevideo/playback_test.go`

- [ ] **Step 1: Write failing service tests**

Test zero IDs, session IDs outside `^[A-Za-z0-9_-]{16,64}$`, negative seconds, missing user/video, non-ready and zero-duration video. Test 59/100 is false, 60/100 is true, and 600/1000 is true.

```go
got, err := service.ReportWatchSession(ctx, WatchSessionInput{
 UserID: 7, KnowledgeVideoID: 88, SessionID: "session-00000001", WatchedSeconds: 40,
})
if err != nil || got.ProgressRatio != 0.6 || !got.EffectiveWatch { t.Fatalf("got=%+v err=%v", got, err) }
```

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/application/knowledgevideo -run ReportWatchSession -count=1
```

Expected: compile failure for the missing method and types.

- [ ] **Step 3: Implement minimal business logic**

Add `WatchSessionInput` and result fields `SessionID`, `SessionWatchedSeconds`, `TotalWatchedSeconds`, `DurationSeconds`, `ProgressRatio`, `EffectiveWatch`. Extend `PlaybackRepository` with `UserExists` and `UpsertWatchSession`. Validate before persistence, clamp the session report to duration, and calculate:

```go
effective := aggregate.TotalWatchedSeconds*100 >= video.Duration*60
ratio := math.Min(1, float64(aggregate.TotalWatchedSeconds)/float64(video.Duration))
```

Return `NotFound` for missing user/video and `NotReady` for non-ready or invalid-duration video.

- [ ] **Step 4: Verify GREEN and commit**

```bash
go test ./internal/application/knowledgevideo -count=1
git add video-service/internal/application/knowledgevideo/contracts.go video-service/internal/application/knowledgevideo/types.go video-service/internal/application/knowledgevideo/playback.go video-service/internal/application/knowledgevideo/playback_test.go
git diff --cached --check
git commit -m "feat: calculate effective knowledge video watches"
```

Expected: all application tests PASS.

### Task 3: Expose the PUT Endpoint

**Files:**
- Modify: `video-service/internal/http/dto/knowledge_video.go`
- Modify: `video-service/internal/http/handler/knowledgevideos/handler.go`
- Modify: `video-service/internal/http/handler/knowledgevideos/handler_test.go`
- Modify: `video-service/internal/http/router/router.go`
- Generated: `video-service/docs/{docs.go,swagger.json,swagger.yaml}`

- [ ] **Step 1: Write failing handler tests**

```go
req := httptest.NewRequest(http.MethodPut,
 "/api/knowledge-videos/88/watch-sessions/session-00000001",
 strings.NewReader(`{"user_id":7,"watched_seconds":40}`))
// Assert 200 and session_watched_seconds=40, total_watched_seconds=60,
// duration_seconds=100, progress_ratio=0.6, effective_watch=true.
```

Add exact 400/404/409 cases from Task 2.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/http/handler/knowledgevideos ./internal/http/router -run WatchSession -count=1
```

Expected: missing handler/service method failure.

- [ ] **Step 3: Add DTO, handler, route and Swagger annotations**

```go
type KnowledgeVideoWatchSessionRequest struct {
 UserID uint64 `json:"user_id" binding:"required"`
 WatchedSeconds int `json:"watched_seconds" binding:"required,min=0"`
}
```

Map all six response fields from the service and register:

```go
public.PUT("/api/knowledge-videos/:knowledgeVideoId/watch-sessions/:sessionId", knowledgeHandler.ReportWatchSession)
```

Annotate the PUT route with 200/400/404/409/500. Preserve the existing POST route.

- [ ] **Step 4: Verify, generate docs, and commit**

```bash
go test ./internal/http/handler/knowledgevideos ./internal/http/router -count=1
go generate ./...
go test ./internal/http/... -count=1
git add video-service/internal/http/dto/knowledge_video.go video-service/internal/http/handler/knowledgevideos/handler.go video-service/internal/http/handler/knowledgevideos/handler_test.go video-service/internal/http/router/router.go video-service/docs
git diff --cached --check
git commit -m "feat: add knowledge video watch session api"
```

Expected: PASS and Swagger contains the new PUT path.

### Task 4: Add Frontend Session Transport

**Files:**
- Modify/Test: `hls-web/src/knowledgeVideo/api.js`, `api.test.js`
- Create/Test: `hls-web/src/knowledgeVideo/watchSession.js`, `watchSession.test.js`

- [ ] **Step 1: Write failing tests**

```js
await reportKnowledgeWatchSession(88, 'session-00000001', 7, 40, { fetchImpl, keepalive: true })
expect(fetchImpl).toHaveBeenCalledWith('/api/knowledge-videos/88/watch-sessions/session-00000001', {
 method: 'PUT', headers: { 'Content-Type': 'application/json' },
 body: JSON.stringify({ user_id: 7, watched_seconds: 40 }), keepalive: true,
})
```

Also reject once then succeed, proving acknowledged seconds advance only after success and retry sends the larger absolute value.

- [ ] **Step 2: Verify RED**

```bash
npm test -- src/knowledgeVideo/api.test.js src/knowledgeVideo/watchSession.test.js
```

Expected: missing exports/modules.

- [ ] **Step 3: Implement transport and session state**

Use `crypto.randomUUID()` by default. `flush(seconds)` floors and clamps to zero, skips values no greater than the acknowledged value, sends the absolute value, and updates acknowledgement only after a successful request. Serialize concurrent flushes so an older completion cannot overwrite newer state.

- [ ] **Step 4: Verify GREEN and commit**

```bash
npm test -- src/knowledgeVideo/api.test.js src/knowledgeVideo/watchSession.test.js
git add hls-web/src/knowledgeVideo/api.js hls-web/src/knowledgeVideo/api.test.js hls-web/src/knowledgeVideo/watchSession.js hls-web/src/knowledgeVideo/watchSession.test.js
git diff --cached --check
git commit -m "feat: add knowledge watch session client"
```

### Task 5: Report Actual Playing Time

**Files:**
- Modify/Test: `hls-web/src/components/HlsPlayer.vue`, `HlsPlayer.test.js`
- Modify/Test: `hls-web/src/workspaces/KnowledgeVideoWorkspace.vue`, `knowledgeVideo/workspace.test.js`

- [ ] **Step 1: Write failing fake-timer tests**

Assert 15 active seconds emits 15, seeking while paused adds zero, pause/end includes the currently running interval, and unmount emits the latest snapshot with `keepalive: true`. Require workspace wiring:

```js
expect(source).toContain('@watch-progress="reportWatchProgress(video.knowledge_video_id, $event)"')
expect(source).toContain('createWatchSession')
```

- [ ] **Step 2: Verify RED**

```bash
npm test -- src/components/HlsPlayer.test.js src/knowledgeVideo/workspace.test.js
```

Expected: heartbeat/unload assertions fail.

- [ ] **Step 3: Implement snapshot accounting**

```js
function watchedSnapshotMs(now = performance.now()) {
 return watchedMs + (playingSince == null ? 0 : Math.max(0, now - playingSince))
}
```

Start a 15-second interval only while playing; clear it on pause, end, source change and unmount. Seeking must not alter accumulated time. Keep one session per rendered knowledge-video player, catch report failures without blocking playback, and retain the session for retry. Keep the legacy first-play POST.

- [ ] **Step 4: Verify and commit**

```bash
npm test
npm run build
git add hls-web/src/components/HlsPlayer.vue hls-web/src/components/HlsPlayer.test.js hls-web/src/workspaces/KnowledgeVideoWorkspace.vue hls-web/src/knowledgeVideo/workspace.test.js
git diff --cached --check
git commit -m "feat: report actual knowledge video watch time"
```

Expected: all Vitest tests and Vite build PASS.

### Task 6: Export Effective Virtual Interactions

**Files:**
- Modify carefully/Test: `video-service/tools/export_recbole_dataset/main.go`, `main_test.go`

- [ ] **Step 1: Write failing aggregation tests**

```go
sessions := []knowledgeWatchSession{
 {UserID: 7, KnowledgeVideoID: 88, Duration: 100, WatchedSeconds: 20},
 {UserID: 7, KnowledgeVideoID: 88, Duration: 100, WatchedSeconds: 40},
 {UserID: 8, KnowledgeVideoID: 99, Duration: 1000, WatchedSeconds: 599},
}
// Assert one row: ItemID "knowledge_video:88", Source "knowledge_video_watch".
```

Assert session/effective/short/effective-user/effective-video/training-interaction counters and latest-update cutoff behavior.

- [ ] **Step 2: Verify RED**

```bash
go test ./tools/export_recbole_dataset -run KnowledgeWatch -count=1
```

Expected: missing types/builder.

- [ ] **Step 3: Implement aggregation**

Give `interactionRow` a string `ItemID`; normal rows use decimal segment IDs. Query only nonempty sessions joined to ready, undeleted, positive-duration videos with latest update in `DAYS_BACK`. Aggregate once per user/video, cap at duration, apply `total*100 >= duration*60`, and never emit short watches as BPR rows. Add the six approved JSON statistics fields.

- [ ] **Step 4: Verify and guarded commit**

```bash
go test ./tools/export_recbole_dataset -count=1
git add -p video-service/tools/export_recbole_dataset/main.go
git add -p video-service/tools/export_recbole_dataset/main_test.go
git diff --cached --check
git commit -m "feat: export effective knowledge video interactions"
```

### Task 7: Produce Benchmark Train/Valid/Test Files

**Files:**
- Modify carefully/Test: exporter files from Task 6
- Modify carefully/Test: `recbole-training/src/recbole_recommendation/config.py`, `tests/test_recbole_config.py`

- [ ] **Step 1: Write failing split/config tests**

```go
train, valid, test := benchmarkSplit(rows)
assertContainsItem(t, train, "knowledge_video:88")
assertNotContainsItem(t, valid, "knowledge_video:88")
assertNotContainsItem(t, test, "knowledge_video:88")
```

```python
self.assertEqual(cfg["benchmark_filename"], ["train", "valid", "test"])
```

- [ ] **Step 2: Verify RED**

```bash
go test ./tools/export_recbole_dataset -run BenchmarkSplit -count=1
cd ../recbole-training && PYTHONPATH=src python3 -m unittest tests.test_recbole_config -v
```

Expected: missing splitter/config failure.

- [ ] **Step 3: Implement deterministic pre-splits**

Sort normal interactions by user, timestamp and item ID; allocate each eligible user's last normal row to test, penultimate to valid, and earlier rows to train while preserving the existing cold-user policy. Append all virtual rows to train. Write `<dataset>.train.inter`, `.valid.inter`, and `.test.inter` with identical headers; fail if virtual tokens appear in valid/test. Include virtual items in `.item` with `video_id=0`. Configure `benchmark_filename` and remove the conflicting ratio split.

- [ ] **Step 4: Verify and guarded commit**

```bash
go test ./tools/export_recbole_dataset -count=1
cd ../recbole-training && PYTHONPATH=src python3 -m unittest tests.test_recbole_config -v
git add -p ../video-service/tools/export_recbole_dataset/main.go
git add -p ../video-service/tools/export_recbole_dataset/main_test.go
git add -p src/recbole_recommendation/config.py tests/test_recbole_config.py
git diff --cached --check
git commit -m "feat: isolate virtual items in recbole training split"
```

Expected: PASS and byte-identical split output for identical input.

### Task 8: Mask Evaluation and Filter Embeddings

**Files:**
- Create/Test: `recbole-training/src/recbole_recommendation/virtual_items.py`, `tests/test_virtual_items.py`
- Modify/Test: `src/recbole_recommendation/train.py`, `tests/test_recbole_train.py`
- Modify/Test: `src/recbole_recommendation/export_embeddings.py`, `tests/test_recbole_export_embeddings.py`

- [ ] **Step 1: Write failing mask/export tests**

```python
scores = torch.tensor([[0.1, 0.9, 0.8]])
masked = mask_virtual_item_scores(scores, [1])
self.assertTrue(torch.isneginf(masked[0, 1]))
```

Create an atomic item fixture containing `101` and `knowledge_video:88`; assert both checkpoint and deterministic fallback exports contain only `101`.

- [ ] **Step 2: Verify RED**

```bash
PYTHONPATH=src python3 -m unittest tests.test_virtual_items tests.test_recbole_export_embeddings -v
```

Expected: missing mask and fallback-export leakage.

- [ ] **Step 3: Implement the mask and strict filter**

```python
def is_virtual_token(token): return token.startswith("knowledge_video:")
def mask_virtual_item_scores(scores, ids):
    result = scores.clone()
    if ids: result[..., ids] = float("-inf")
    return result
```

Replace the opaque quick-start call with RecBole's explicit `Config/create_dataset/data_preparation/model/trainer` sequence. Use a local trainer subclass that masks the resolved virtual internal IDs in every full-sort evaluation batch before Top-K collection. Fail startup if valid/test targets contain a virtual ID or virtual IDs exist without the masking trainer. In both embedding export paths require `usable_token(item_id)` (`str.isdigit()`); retain all valid user embeddings and count filtered virtual items.

- [ ] **Step 4: Verify and commit**

```bash
PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py' -v
git add recbole-training/src/recbole_recommendation/virtual_items.py recbole-training/tests/test_virtual_items.py recbole-training/src/recbole_recommendation/train.py recbole-training/tests/test_recbole_train.py recbole-training/src/recbole_recommendation/export_embeddings.py recbole-training/tests/test_recbole_export_embeddings.py
git diff --cached --check
git commit -m "feat: mask recbole virtual items from evaluation"
```

Expected: a highest-scoring virtual item cannot enter evaluated Top-K.

### Task 9: Add Fail-Closed Pipeline Checks and Verify

**Files:**
- Modify/Test: `recbole-training/scripts/run_recbole_pipeline.sh`, `tests/test_recbole_pipeline_script.py`
- Modify/Test: `src/recbole_recommendation/metrics.py`, `tests/test_recbole_metrics.py`
- Modify: repository RecBole operations documentation

- [ ] **Step 1: Write failing gate/statistics tests**

Require all three split files, reject virtual tokens in valid/test, and reject nonnumeric first-column values in `item_embeddings.csv` before importer execution. Test that normalized metrics retain exporter statistics:

```python
self.assertEqual(result["knowledge_watch_training_interactions"], 3)
self.assertEqual(result["filtered_virtual_item_embeddings"], 2)
```

- [ ] **Step 2: Verify RED**

```bash
PYTHONPATH=src python3 -m unittest tests.test_recbole_pipeline_script tests.test_recbole_metrics -v
```

Expected: missing gates/statistics failures.

- [ ] **Step 3: Implement gates and documentation**

Before training, assert benchmark files exist and use `awk` to reject `knowledge_video:` in valid/test. Before import/publish, require every non-header `video_segment_id` to match `^[1-9][0-9]*$`. Merge exporter and filtered-item counters into `metrics.json`. Document the PUT request, 60% accumulation, hourly `:15` cadence, virtual isolation, statistics, and failure behavior; do not describe knowledge videos as online candidates.

- [ ] **Step 4: Run the complete verification matrix**

```bash
cd video-service
gofmt -w internal/model/knowledge_video.go internal/application/knowledgevideo/*.go internal/infrastructure/persistence/migration.go internal/infrastructure/persistence/gorm_knowledge_video_repository*.go internal/http/dto/knowledge_video.go internal/http/handler/knowledgevideos/*.go tools/export_recbole_dataset/*.go
go test ./internal/application/knowledgevideo ./internal/infrastructure/persistence ./internal/http/handler/knowledgevideos ./internal/http/router ./tools/export_recbole_dataset -count=1
go test ./... -count=1
cd ../hls-web && npm test && npm run build
cd ../recbole-training
PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py' -v
bash -n scripts/run_recbole_pipeline.sh
```

Expected: all commands PASS. Record exact unavailable integration dependencies instead of claiming success.

- [ ] **Step 5: Run acceptance leak checks**

With fixture sessions of 20 and 40 seconds for a 100-second video:

```bash
rg -n 'knowledge_video:' data/test_fixture/_video/_video.train.inter
! rg -n 'knowledge_video:' data/test_fixture/_video/_video.valid.inter data/test_fixture/_video/_video.test.inter
! rg -n 'knowledge_video:' artifacts/test_fixture/item_embeddings.csv
```

Expected: exactly one train row and no valid/test/artifact matches. Run the existing random-play regressions and confirm responses remain numeric `video_segment_id` items.

- [ ] **Step 6: Commit final safeguards**

```bash
git add recbole-training/scripts/run_recbole_pipeline.sh recbole-training/tests/test_recbole_pipeline_script.py recbole-training/src/recbole_recommendation/metrics.py recbole-training/tests/test_recbole_metrics.py
git add -p README.md
git diff --cached --check
git commit -m "chore: guard recbole artifacts from virtual item leaks"
```

## Final Review

- [ ] Same-session retry/ordering is monotonic; cross-session total caps at duration.
- [ ] Server validates user/video/session and computes the integer 60% boundary.
- [ ] Legacy playback rows remain compatible and never become training rows.
- [ ] Heartbeat uses active wall-clock time; seek adds no time; failures retry absolute totals.
- [ ] Virtual items exist only in train and are absent from valid/test targets, evaluation Top-K, and item embeddings.
- [ ] User embeddings still benefit; online SQL/API still use ordinary numeric segment IDs.
- [ ] Zero effective watches preserve prior behavior and existing quality/publish gates remain intact.
- [ ] No immediate training trigger was added; the existing hourly `:15` schedule consumes the next export.
- [ ] No unrelated dirty work was reverted or committed.
