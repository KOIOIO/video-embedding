# Knowledge Video Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a knowledge-video tree API and a dedicated Vue workspace for import progress, knowledge-point browsing, and playback, then deliver a valid two-video ZIP/XLSX import sample.

**Architecture:** Extend the isolated `knowledgevideo` repository with one batched dictionary-plus-video tree query that tolerates optional dictionary columns. Expose it through the existing knowledge-video service and handler. Add a focused frontend API module and workspace rather than enlarging `VideoWorkspace.vue`; generate the spreadsheet with the bundled artifact runtime and copy the supplied videos into a ZIP without transcoding.

**Tech Stack:** Go 1.26, GORM, Gin, Vue 3, Vite, Vitest, HLS.js, `@oai/artifact-tool`, ZIP, Go and JavaScript unit tests.

---

## File Map

Backend:

- Modify `internal/application/knowledgevideo/types.go`: tree node and mapped-video summaries.
- Modify `internal/application/knowledgevideo/contracts.go`: tree reader port.
- Modify `internal/application/knowledgevideo/service.go`: tree facade method.
- Modify `internal/infrastructure/persistence/gorm_knowledge_video_repository.go`: schema-compatible batched tree rows.
- Modify `internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go`: optional `parent_id`, deleted rows, mappings, ordering.
- Modify `internal/http/dto/knowledge_video.go`: tree response DTO.
- Modify `internal/http/handler/knowledgevideos/handler.go`: tree endpoint and Swagger annotation.
- Modify `internal/http/handler/knowledgevideos/handler_test.go`: handler contract.
- Modify `internal/http/router/router.go`: register tree route.
- Modify `internal/http/router/swagger_test.go`: assert generated tree path.
- Regenerate `docs/swagger/docs.go`, `swagger.json`, and `swagger.yaml`.

Frontend:

- Modify `hls-web/src/config/consoleSession.js`: register `knowledge-video` workspace.
- Modify `hls-web/src/config/consoleSession.test.js`: workspace persistence tests.
- Modify `hls-web/src/App.vue`: third workspace switch and mount.
- Create `hls-web/src/knowledgeVideo/api.js`: tree, upload, progress, playback, normalization, and polling.
- Create `hls-web/src/knowledgeVideo/api.test.js`: API and terminal polling tests.
- Create `hls-web/src/workspaces/KnowledgeVideoWorkspace.vue`: upload-first workspace.
- Create `hls-web/src/knowledgeVideo/knowledgeVideo.css`: responsive operational layout.
- Create `hls-web/src/knowledgeVideo/workspace.test.js`: structural and responsive assertions.

Artifacts:

- Create `outputs/knowledge-video-sample/knowledge-video-mapping.xlsx`.
- Create `outputs/knowledge-video-sample/knowledge-video-sample.zip`.

## Task 1: Batched Knowledge-Point Tree Query

- [ ] **Step 1: Write failing repository tests**

Add tests that create `dict_knowledge_point` with `id`, `parent_id`, `name`, and optional `deleted`, seed parent/child/orphan rows plus ready/pending mappings, and assert stable roots, children, video summaries, deleted filtering, and fallback to flat roots when `parent_id` is absent.

```go
func TestKnowledgeVideoRepositoryListTreeUsesOptionalParentID(t *testing.T) {
	repo, db := newKnowledgeVideoTestRepository(t)
	// Seed parent 1, children 9/10, and ready video for 9.
	got, err := repo.ListKnowledgeTree(context.Background())
	if err != nil || len(got) != 1 || len(got[0].Children) != 2 {
		t.Fatalf("tree=%+v err=%v", got, err)
	}
}
```

- [ ] **Step 2: Verify red**

Run: `go test ./internal/infrastructure/persistence -run KnowledgeVideoRepositoryListTree`

Expected: FAIL because tree types and method are absent.

- [ ] **Step 3: Implement types, port, and repository query**

Define `KnowledgeTreeNode` and `KnowledgeTreeVideo`. Query all active dictionary rows once, selecting `0 AS parent_id` when the column is absent; query all active knowledge-video rows once; join in memory. Sort siblings by ID. Promote missing-parent rows to roots. With no `parent_id`, wrap all rows under a synthetic node `{ID: 0, Name: "全部知识点"}`.

- [ ] **Step 4: Verify and commit**

Run: `go test ./internal/infrastructure/persistence -run 'KnowledgeVideoRepository(ListTree|Resolve|Lookup)'`

```bash
git add internal/application/knowledgevideo internal/infrastructure/persistence/gorm_knowledge_video_repository.go internal/infrastructure/persistence/gorm_knowledge_video_repository_test.go
git commit -m "feat: list knowledge video tree"
```

## Task 2: Tree HTTP Endpoint

- [ ] **Step 1: Write failing handler and Swagger tests**

Extend the handler stub with `ListTree`. Assert `GET /api/knowledge-videos/tree` returns nested JSON and mapped video status. Add the path to Swagger assertions.

```go
func TestKnowledgeVideoTreeReturnsNestedNodes(t *testing.T) {
	r := testRouter(&handlerServiceStub{tree: []knowledgevideo.KnowledgeTreeNode{{ID: 9, Name: "一次函数"}}})
	// Expect 200 and knowledge point 9.
}
```

- [ ] **Step 2: Verify red**

Run: `go test ./internal/http/handler/knowledgevideos ./internal/http/router -run 'KnowledgeVideoTree|Swagger'`

Expected: FAIL because handler/service route is absent and generated Swagger lacks the path.

- [ ] **Step 3: Implement facade, DTO mapping, handler, and route**

Add `ListTree(ctx)` to `knowledgevideo.Service` and handler interface. Return `dto.SuccessResponse[dto.KnowledgeVideoTreeData]`. Register `GET /api/knowledge-videos/tree`. Add Swagger annotation using an explicit response wrapper DTO.

- [ ] **Step 4: Generate Swagger, verify, and commit**

Run: `go generate ./docs/swagger`

Run: `go test ./internal/http/handler/knowledgevideos ./internal/http/router -run 'KnowledgeVideoTree|Swagger'`

```bash
git add internal/application/knowledgevideo/service.go internal/http/dto/knowledge_video.go internal/http/handler/knowledgevideos/handler.go internal/http/handler/knowledgevideos/handler_test.go internal/http/router/router.go internal/http/router/swagger_test.go docs/swagger
git commit -m "feat: expose knowledge video tree"
```

## Task 3: Frontend API And Polling

- [ ] **Step 1: Write failing Vitest cases**

Create tests for nested response normalization, XHR multipart upload progress, progress fetch, terminal states, polling continuation, polling stop, and playback URL requests.

```js
it('stops polling on partial_failed', async () => {
  const request = vi.fn().mockResolvedValue({ status: 'partial_failed' })
  const result = await pollKnowledgeVideoBatch(101, { request, delay: async () => {} })
  expect(result.status).toBe('partial_failed')
  expect(request).toHaveBeenCalledTimes(1)
})
```

- [ ] **Step 2: Verify red**

Run from `hls-web`: `npm test -- src/knowledgeVideo/api.test.js`

Expected: FAIL because `api.js` is absent.

- [ ] **Step 3: Implement API helpers**

Use `fetch` for tree/progress/playback and `XMLHttpRequest` for multipart upload. Treat `completed`, `partial_failed`, and `failed` as terminal. Accept an `AbortSignal` for tree/playback and a cancellation predicate for polling. Preserve structured validation issues on thrown errors.

- [ ] **Step 4: Verify and commit**

Run: `npm test -- src/knowledgeVideo/api.test.js`

```bash
git add hls-web/src/knowledgeVideo/api.js hls-web/src/knowledgeVideo/api.test.js
git commit -m "feat: add knowledge video frontend API"
```

## Task 4: Dedicated Upload-First Workspace

- [ ] **Step 1: Write failing navigation and structure tests**

Assert `knowledge-video` is a known persisted workspace, `App.vue` exposes the third switch, the workspace contains two file inputs, upload progress, batch rows, search/filter controls, tree nodes, and `HlsPlayer`, and CSS switches to one column on narrow screens.

- [ ] **Step 2: Verify red**

Run from `hls-web`: `npm test -- src/config/consoleSession.test.js src/knowledgeVideo/workspace.test.js`

Expected: FAIL because the workspace is not registered or implemented.

- [ ] **Step 3: Implement the workspace**

Create the selected B layout. Left column keeps upload and current batch progress visible. Right column provides tree search/status filter and playback. Poll every two seconds after `202`, stop on terminal/unmount, refresh tree on terminal, and only resolve playback for ready nodes. Use the existing `HlsPlayer` and current console variables; do not add a UI framework.

- [ ] **Step 4: Run frontend verification and commit**

Run: `npm test`

Run: `npm run build`

Use Playwright screenshots at desktop and mobile widths to verify no overlaps, clipped labels, or blank player/tree regions.

```bash
git add hls-web/src/App.vue hls-web/src/config/consoleSession.js hls-web/src/config/consoleSession.test.js hls-web/src/workspaces/KnowledgeVideoWorkspace.vue hls-web/src/knowledgeVideo/knowledgeVideo.css hls-web/src/knowledgeVideo/workspace.test.js
git commit -m "feat: add knowledge video workspace"
```

## Task 5: Sample ZIP/XLSX And End-To-End Verification

- [ ] **Step 1: Load spreadsheet runtime and build XLSX**

Use `codex_app__load_workspace_dependencies`, read the required artifact-tool quick start and style guide, and create one formatted worksheet with exact headers `id`, `name`, `video_name` and rows `9 / 一次函数 / 一次函数.mp4`, `10 / 二次函数 / 二次函数.mp4`.

- [ ] **Step 2: Verify spreadsheet**

Inspect `A1:C3`, scan formula errors, render the used range, visually inspect it, and export to `outputs/knowledge-video-sample/knowledge-video-mapping.xlsx`.

- [ ] **Step 3: Build and inspect ZIP**

Copy the two supplied MP4 files to a temporary directory using the target names, create `knowledge-video-sample.zip`, then list entries and verify exactly two files with nonzero sizes. Do not modify the source files.

- [ ] **Step 4: Validate the generated pair**

Add or run a focused Go validation harness using dictionary values `{9: "一次函数", 10: "二次函数"}` and the generated files. Expected: a two-row manifest with matching entry names.

- [ ] **Step 5: Final verification**

Run backend knowledge-video tests, full frontend tests/build, `git diff --check`, and inspect `git status --short`. The known unrelated fixed-date recommendation metric test remains outside this feature scope.

The final response links both artifacts and reports the local frontend URL used for visual verification.
