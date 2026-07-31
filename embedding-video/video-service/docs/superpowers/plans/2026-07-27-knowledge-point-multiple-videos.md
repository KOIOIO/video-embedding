# Knowledge Point Multiple Videos Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Support multiple uploaded and independently playable ready videos for one knowledge point while recording playback only after a specific video starts.

**Architecture:** Keep one `edu_knowledge_video` row per video, replace the active knowledge-point uniqueness constraint with a query index, and change playback lookup to return an ordered ready-only slice. Separate playback listing from playback recording, expose plural and compatibility HTTP routes, and render each returned video in its own Vue player.

**Tech Stack:** Go 1.x, GORM, Gin, SQLite/PostgreSQL migration tests, `excelize`, Swag, Vue 3, Vite, Vitest, HLS.js.

---

### Task 1: Allow repeated knowledge point mappings and additive imports

**Files:**
- Modify: `internal/application/knowledgevideo/validator_test.go`
- Modify: `internal/application/knowledgevideo/validator.go`
- Modify: `internal/application/knowledgevideo/import_test.go`
- Modify: `internal/application/knowledgevideo/import.go`
- Modify: `internal/application/knowledgevideo/contracts.go`

- [ ] **Step 1: Replace the duplicate-ID validator failure case with an acceptance test**

Add a workbook containing two rows with ID `9`, dictionary name `一次函数`, and distinct names `first.mp4` and `second.mp4`; assert validation succeeds and returns two manifest rows. Keep the existing duplicate `video_name` failure test.

```go
func TestValidatorAllowsMultipleVideosForOneKnowledgePoint(t *testing.T) {
	manifest, err := validator.Validate(ctx, zipArchive(t,
		zipEntry{name: "first.mp4", body: "first"},
		zipEntry{name: "second.mp4", body: "second"},
	), mappingXLSX(t, [][]string{
		{"id", "name", "video_name"},
		{"9", "一次函数", "first.mp4"},
		{"9", "一次函数", "second.mp4"},
	}))
	if err != nil || len(manifest.Rows) != 2 { t.Fatalf("Validate() = %#v, %v", manifest, err) }
}
```

- [ ] **Step 2: Run the validator test and verify it fails**

Run: `go test ./internal/application/knowledgevideo -run 'TestValidatorAllowsMultipleVideosForOneKnowledgePoint|TestParseMapping'`

Expected: FAIL with `duplicate knowledge point id` before implementation.

- [ ] **Step 3: Remove only the repeated-ID rejection**

Delete `seenIDs` and its duplicate branch from `parseMapping`. Preserve ID parsing, dictionary-name equality, workbook `video_name` uniqueness, and exact ZIP matching.

- [ ] **Step 4: Add an import test proving active videos are not queried or rejected**

Remove `FindActiveDuplicateKnowledgePointIDs` from the import stub and contract, import two rows with ID `9`, and assert two videos and two queue tasks are created.

```go
if len(repo.createdVideos) != 2 || repo.createdVideos[0].KnowledgePointID != 9 || repo.createdVideos[1].KnowledgePointID != 9 {
	t.Fatalf("created videos = %#v", repo.createdVideos)
}
```

- [ ] **Step 5: Run the import test and verify it fails**

Run: `go test ./internal/application/knowledgevideo -run 'TestImport.*Multiple|TestImport'`

Expected: FAIL because `ImportService` still calls the active-duplicate detector or the stub no longer implements the interface.

- [ ] **Step 6: Remove the active-duplicate import dependency and check**

Remove `ActiveDuplicateDetector` from `ImportRepository`, delete the lookup/rejection block in `Import`, and remove the unused repository-wide contract only if no remaining caller exists.

Run: `go test ./internal/application/knowledgevideo`

Expected: PASS.

### Task 2: Migrate the index and support multi-video persistence

**Files:**
- Modify: `internal/infrastructure/persistence/migration.go`
- Modify: `internal/infrastructure/persistence/migration_test.go`
- Modify: `internal/infrastructure/persistence/gorm_knowledge_video_repository.go`
- Modify: `internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go`
- Modify: `internal/application/knowledgevideo/contracts.go`
- Modify: `internal/application/knowledgevideo/types.go`

- [ ] **Step 1: Change migration tests to require a non-unique knowledge-point index**

After `ensureKnowledgeVideoSchema`, insert two active videos with the same `knowledge_point_id`; assert both inserts succeed. Query SQLite index metadata to assert `uk_knowledge_video_knowledge_point_active` is absent and `idx_knowledge_video_knowledge_point_active` exists.

- [ ] **Step 2: Run migration tests and verify duplicate insertion fails**

Run: `go test ./internal/infrastructure/persistence -run 'Test.*KnowledgeVideo.*Index|TestEnsureKnowledgeVideoSchema'`

Expected: FAIL on the second insert or obsolete unique-index assertion.

- [ ] **Step 3: Implement explicit index replacement**

Execute the drop before the create statements:

```go
if err := db.Exec(`DROP INDEX IF EXISTS uk_knowledge_video_knowledge_point_active;`).Error; err != nil { return err }
```

Then create:

```sql
CREATE INDEX IF NOT EXISTS idx_knowledge_video_knowledge_point_active
ON edu_knowledge_video(knowledge_point_id) WHERE deleted = 0;
```

- [ ] **Step 4: Add repository tests for ordered ready-only slices and multi-video trees**

Seed pending, failed, deleted, and two ready rows for one knowledge point in non-ID insertion order. Assert `ListReadyByKnowledgePoint` returns only ready rows ordered by `id ASC`, and `ListKnowledgeTree` retains all active rows in `Videos` with the first row mirrored in `Video`.

- [ ] **Step 5: Run repository tests and verify contract failures**

Run: `go test ./internal/infrastructure/persistence -run 'TestKnowledgeVideoRepository.*Ready|TestKnowledgeVideoRepository.*Tree'`

Expected: FAIL because the slice resolver and `Videos` field do not exist.

- [ ] **Step 6: Implement repository contracts**

Define:

```go
type ReadyVideosResolver interface {
	ListReadyByKnowledgePoint(ctx context.Context, knowledgePointID uint64) ([]Video, error)
}
```

Add `Videos []KnowledgeTreeVideo` to `KnowledgeTreeNode`, retain `Video *KnowledgeTreeVideo`, group active rows by knowledge point in ID order, and replace single-row ready resolution with an ordered `Find` query.

- [ ] **Step 7: Run persistence tests**

Run: `go test ./internal/infrastructure/persistence`

Expected: PASS.

### Task 3: Separate playback listing from playback recording

**Files:**
- Modify: `internal/application/knowledgevideo/playback_test.go`
- Modify: `internal/application/knowledgevideo/playback.go`
- Modify: `internal/application/knowledgevideo/types.go`
- Modify: `internal/application/knowledgevideo/contracts.go`

- [ ] **Step 1: Write failing listing and recording tests**

Cover: two ready results preserve order; no `RecordPlayback` call during listing; existing knowledge point with no ready rows returns an empty slice; `displayName("lesson.mp4.mp4") == "lesson.mp4"`; recording loads ID `88`, rejects non-ready state, and records its actual knowledge point ID.

```go
resolution, err := service.List(ctx, 9)
if err != nil || len(resolution.Videos) != 2 || repo.recordCalls != 0 { t.Fatalf(...) }

err = service.Record(ctx, 7, 88)
if err != nil || repo.record.KnowledgeVideoID != 88 || repo.record.KnowledgePointID != 9 { t.Fatalf(...) }
```

- [ ] **Step 2: Run playback tests and verify missing methods fail**

Run: `go test ./internal/application/knowledgevideo -run TestPlayback`

Expected: build failure for `List`, `Record`, or multi-video result types.

- [ ] **Step 3: Implement listing and recording operations**

Use these result shapes:

```go
type PlaybackVideo struct {
	KnowledgeVideoID uint64
	SourceFileName string
	DisplayName string
	Duration int
	PlaybackURL string
}
type PlaybackResolution struct {
	KnowledgePointID uint64
	KnowledgePointName string
	Videos []PlaybackVideo
}
```

`List` validates the knowledge point and maps the ready slice without writes. `Record` validates IDs, uses `GetVideo`, requires `VideoReady`, and then calls `RecordPlayback` with `Now`.

- [ ] **Step 4: Run application tests**

Run: `go test ./internal/application/knowledgevideo`

Expected: PASS.

### Task 4: Expose multi-video HTTP APIs and regenerate Swagger

**Files:**
- Modify: `internal/http/dto/knowledge_video.go`
- Modify: `internal/http/handler/knowledgevideos/handler.go`
- Modify: `internal/http/handler/knowledgevideos/handler_test.go`
- Modify: `internal/http/router/router.go`
- Modify: `internal/http/router/swagger_test.go`
- Generate: `docs/swagger/docs.go`
- Generate: `docs/swagger/swagger.json`
- Generate: `docs/swagger/swagger.yaml`

- [ ] **Step 1: Add failing handler tests**

Test both GET paths without `user_id`, `videos: []` for an existing point with no ready video, two video items plus first-item legacy fields, ignored legacy query `user_id`, POST success, invalid body 400, missing video 404, and non-ready 409.

- [ ] **Step 2: Run handler tests and verify route/contract failures**

Run: `go test ./internal/http/handler/knowledgevideos`

Expected: FAIL because plural and recording routes do not exist and GET still requires `user_id`.

- [ ] **Step 3: Implement DTOs, service interface, handlers, routes, and compatibility fields**

Add a `KnowledgeVideoPlaybackItem` with `knowledge_video_id`, `source_file_name`, `display_name`, `duration`, and `playback_url`. The response always emits `videos`; pointer legacy fields use `omitempty` and mirror the first item. Bind `user_id` JSON for POST and map `NotReady` to HTTP 409.

Register:

```go
r.GET("/api/knowledge-points/:knowledgePointId/video", knowledgeHandler.Playback)
r.GET("/api/knowledge-points/:knowledgePointId/videos", knowledgeHandler.Playback)
r.POST("/api/knowledge-videos/:knowledgeVideoId/playbacks", knowledgeHandler.RecordPlayback)
```

- [ ] **Step 4: Run HTTP tests**

Run: `go test ./internal/http/handler/knowledgevideos ./internal/http/router`

Expected: PASS.

- [ ] **Step 5: Regenerate and verify Swagger**

Extend `internal/http/router/swagger_test.go` to require `/api/knowledge-points/{knowledgePointId}/videos` and `/api/knowledge-videos/{knowledgeVideoId}/playbacks`. Run `go generate ./docs/swagger` from `video-service/`, then run `go test ./internal/http/router -run Swagger` and verify generated output contains both paths.

### Task 5: Return multi-video knowledge tree data

**Files:**
- Modify: `internal/http/dto/knowledge_video.go`
- Modify: `internal/http/handler/knowledgevideos/handler.go`
- Modify: `internal/http/handler/knowledgevideos/handler_test.go`

- [ ] **Step 1: Add a failing tree response test**

Create a node with two `KnowledgeTreeVideo` entries and assert JSON contains `"videos":[...]` with both IDs and `"video"` with the first ID.

- [ ] **Step 2: Run the focused tree test and verify failure**

Run: `go test ./internal/http/handler/knowledgevideos -run TestKnowledgeVideoTree`

Expected: FAIL because DTO mapping only serializes one video.

- [ ] **Step 3: Map plural and compatibility tree fields**

Add `Videos []KnowledgeVideoTreeVideo` with a non-nil empty slice. Derive `Video` from the first mapped video, rather than from an independently selected repository value, so compatibility ordering cannot diverge.

- [ ] **Step 4: Run handler and repository tree tests**

Run: `go test ./internal/http/handler/knowledgevideos ./internal/infrastructure/persistence -run 'Tree'`

Expected: PASS.

### Task 6: Render and record independent frontend players

**Files:**
- Modify: `hls-web/src/knowledgeVideo/api.js`
- Modify: `hls-web/src/knowledgeVideo/api.test.js`
- Modify: `hls-web/src/workspaces/KnowledgeVideoWorkspace.vue`
- Modify: `hls-web/src/components/HlsPlayer.vue` if it does not currently emit a `play` event
- Modify: `hls-web/src/knowledgeVideo/workspace.test.js`
- Modify: `hls-web/src/knowledgeVideo/knowledgeVideo.css`

- [ ] **Step 1: Add failing API tests**

Assert lookup calls `/api/knowledge-points/9/videos` without a user query, and recording sends JSON `{"user_id":7}` to `/api/knowledge-videos/88/playbacks` with POST and content type headers.

- [ ] **Step 2: Run API tests and verify failure**

Run: `npm test -- --run src/knowledgeVideo/api.test.js`

Expected: FAIL because the old API uses singular GET with `user_id` and no record helper exists.

- [ ] **Step 3: Implement frontend API functions**

Change `resolveKnowledgePlayback(knowledgePointId, options)` to use the plural route and add `recordKnowledgePlayback(knowledgeVideoId, userId, options)` using the existing `requestJson` helper.

- [ ] **Step 4: Add failing workspace tests for multiple players and one-time recording**

Mount or inspect the workspace using the repository's established Vue test pattern. Assert two response entries render two `HlsPlayer` instances with `display_name` titles, and two `play` events from one player produce one record call.

- [ ] **Step 5: Run workspace tests and verify failure**

Run: `npm test -- --run src/knowledgeVideo/workspace.test.js`

Expected: FAIL because the workspace stores one playback object and renders one player.

- [ ] **Step 6: Implement stable multi-player UI**

Store `playback.videos` as a list, render each entry keyed by `knowledge_video_id`, and keep a `Set` of recorded video IDs for the current selection. Reset that set when selecting another knowledge point. On first emitted `play`, call the recording API and do not block the player on failure. Use a responsive grid with stable player dimensions and no nested cards.

- [ ] **Step 7: Run frontend tests and production build**

Run: `npm test -- --run`

Run: `npm run build`

Expected: all tests PASS and Vite build exits 0.

### Task 7: Full verification and final review

**Files:**
- Verify all changed files from Tasks 1-6.

- [ ] **Step 1: Format and inspect the diff**

Run `gofmt` on changed Go files, then run `git diff --check` and inspect `git diff --stat` plus the complete diff for unrelated changes.

- [ ] **Step 2: Run the full backend suite**

Run: `go test ./...`

Expected: PASS from `video-service/`.

- [ ] **Step 3: Run final frontend verification**

Run: `npm test -- --run && npm run build`

Expected: PASS from `hls-web/`.

- [ ] **Step 4: Start local services and verify in browser when infrastructure is available**

Start the API with `go run ./cmd/httpapi`, worker with `go run ./cmd/worker`, and frontend with `npm run dev` on an available port. Verify desktop and mobile layouts, all ready players render with correct titles, media controls do not overlap, and starting a player creates a record for its `knowledge_video_id`. If PostgreSQL, Redis, or object storage is unavailable, report that environmental limitation explicitly while retaining automated build and test evidence.

- [ ] **Step 5: Commit the implementation**

Stage only files belonging to this feature and commit with a message such as `feat: support multiple videos per knowledge point` after all available verification passes.
