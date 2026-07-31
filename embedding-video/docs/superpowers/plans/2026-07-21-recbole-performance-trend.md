# RecBole Performance Trend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Gorse performance chart with a PostgreSQL-backed RecBole offline evaluation trend.

**Architecture:** Persistence reads versioned metrics from `recsys.recommend_model_version`; the application service validates a fixed metric set and exposes a stable trend contract. A protected RecBole endpoint feeds a renamed Vue chart, and code/configuration used only by the old Gorse Dashboard trend is removed.

**Tech Stack:** Go 1.24, Gin, GORM/PostgreSQL JSONB, Vue 3, Vite, Vitest.

---

## File Map

- `internal/application/videoapp/recommendation_admin.go`: RecBole trend contract and service orchestration.
- `internal/infrastructure/persistence/gorm_video_repository.go`: model metric history query.
- `internal/http/handler/recommendationadmin/handler.go`: query validation and response mapping.
- `internal/http/dto/recommendation_admin.go`: RecBole response DTOs.
- `hls-web/src/recommendation/components/RecBolePerformanceChart.vue`: migrated chart.
- `hls-web/src/recommendation/recbolePerformance.js`: chart data and geometry helpers.
- Existing Gorse Dashboard client/config and Gorse chart files: delete after replacements pass.

### Task 1: Repository and application contract

**Files:**
- Modify: `video-service/internal/application/videoapp/recommendation_admin.go`
- Create: `video-service/internal/application/videoapp/recommendation_admin_recbole_test.go`
- Modify: `video-service/internal/infrastructure/persistence/gorm_video_repository.go`
- Test: `video-service/internal/infrastructure/persistence/gorm_video_repository_test.go`

- [ ] **Step 1: Write failing service tests**

Add a fake repository and prove that an empty metric defaults to `NDCG@20`, points retain `ModelVersion`, and `AUC` returns `ErrInvalidRecBolePerformanceMetric` before repository access.

```go
result, err := svc.RecommendationRecBolePerformance(ctx, RecommendationRecBolePerformanceInput{Begin: begin, End: end})
if err != nil || result.Metric != "NDCG@20" || result.Points[0].ModelVersion != "recbole_v2" {
    t.Fatalf("result/error = %+v/%v", result, err)
}
```

- [ ] **Step 2: Verify RED**

Run from `video-service/`: `go test ./internal/application/videoapp -run RecBolePerformance`

Expected: FAIL because the RecBole types and method do not exist.

- [ ] **Step 3: Implement the minimal service contract**

```go
var ErrInvalidRecBolePerformanceMetric = errors.New("invalid recbole performance metric")

var recommendationRecBoleMetrics = []RecommendationRecBoleMetric{
    {Value: "Recall@20", Label: "Recall@20"},
    {Value: "NDCG@20", Label: "NDCG@20"},
    {Value: "Hit@20", Label: "Hit@20"},
    {Value: "Precision@20", Label: "Precision@20"},
}

type RecommendationRecBolePerformanceRepository interface {
    ListRecommendationRecBolePerformance(context.Context, string, time.Time, time.Time) ([]RecommendationRecBolePerformancePoint, error)
}
```

Default to `NDCG@20`, validate against the list, call the repository once, and return the fixed available metrics.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./internal/application/videoapp -run RecBolePerformance`

Expected: PASS.

- [ ] **Step 5: Write a failing repository test**

Use existing sqlmock/GORM setup to return `model_version`, `metric_time`, and `value`. Assert a row maps to timestamp, `0.35`, and `recbole_v2`, and assert all query arguments.

```go
points, err := repo.ListRecommendationRecBolePerformance(ctx, "NDCG@20", begin, end)
if err != nil || len(points) != 1 || points[0].ModelVersion != "recbole_v2" || points[0].Value != 0.35 {
    t.Fatalf("points/error = %+v/%v", points, err)
}
```

- [ ] **Step 6: Verify repository RED**

Run: `go test ./internal/infrastructure/persistence -run RecBolePerformance`

Expected: FAIL because the repository method does not exist.

- [ ] **Step 7: Implement the parameterized JSONB query**

```sql
SELECT model_version,
       COALESCE(published_at, create_time) AS metric_time,
       (metrics_json ->> ?)::double precision AS value
FROM recsys.recommend_model_version
WHERE model_name = ? AND framework = ? AND status = 1 AND deleted = 0
  AND COALESCE(published_at, create_time) BETWEEN ? AND ?
  AND jsonb_typeof(metrics_json -> ?) = 'number'
ORDER BY metric_time ASC, id ASC
```

Pass `metric`, `recbole`, `recbole`, `begin`, `end`, `metric`; scan and map to application points.

- [ ] **Step 8: Verify and commit Task 1**

Run both focused tests above, then commit the four Task 1 files with `feat(recommendation): query RecBole performance trends`.

### Task 2: HTTP endpoint

**Files:**
- Modify: `video-service/internal/http/dto/recommendation_admin.go`
- Modify: `video-service/internal/http/handler/recommendationadmin/handler.go`
- Modify: `video-service/internal/http/handler/recommendation_admin.go`
- Test: `video-service/internal/http/handler/recommendation_admin_test.go`
- Modify: `video-service/internal/http/router/router.go`
- Test: `video-service/internal/http/router/swagger_test.go`

- [ ] **Step 1: Write failing handler and route tests**

Replace the stub method with `RecommendationRecBolePerformance`. Test the new path with `NDCG@20`, valid RFC3339 dates, and a response point containing `model_version`. Keep bad begin/end, reversed range, invalid metric, and service error cases. Assert the old path is no longer registered.

```json
{"metric":"NDCG@20","points":[{"timestamp":"2026-07-14T00:00:00Z","value":0.35,"model_version":"recbole_v2"}]}
```

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/http/handler -run RecBolePerformance`

Expected: FAIL because the handler does not exist.

- [ ] **Step 3: Implement DTO, handler, mapper, and route**

```go
type RecommendationRecBolePerformancePointData struct {
    Timestamp    time.Time `json:"timestamp"`
    Value        float64   `json:"value"`
    ModelVersion string    `json:"model_version"`
}
```

Parse required RFC3339 dates, map the application validation error to 400 and persistence errors to 500, forward `RecBolePerformance` through the public handler, and register only:

```go
r.GET("/api/admin/recommendation/recbole/performance", recommendationAdminHandler.RecBolePerformance)
```

- [ ] **Step 4: Verify GREEN and commit Task 2**

Run: `go test ./internal/http/handler -run RecBolePerformance` and `go test ./internal/http/router`.

Expected: PASS. Commit Task 2 files with `feat(recommendation): expose RecBole performance API`.

### Task 3: Frontend migration

**Files:**
- Create: `hls-web/src/recommendation/components/RecBolePerformanceChart.vue`
- Create: `hls-web/src/recommendation/components/RecBolePerformanceChart.test.js`
- Create: `hls-web/src/recommendation/recbolePerformance.js`
- Create: `hls-web/src/recommendation/recbolePerformance.test.js`
- Modify: `hls-web/src/recommendation/api/recommendationConsole.js`
- Test: `hls-web/src/recommendation/api/recommendationConsole.test.js`
- Modify: `hls-web/src/workspaces/RecommendationWorkspace.vue`
- Modify: `hls-web/src/recommendation/recommendation.css`

- [ ] **Step 1: Write failing API/helper tests**

Expect `fetchRecBolePerformance` to request `/api/admin/recommendation/recbole/performance?metric=NDCG%4020&...`. Copy the geometry tests under the RecBole helper and prove normalization retains the model version.

```js
expect(normalizePerformancePoints([
  { timestamp: '2026-07-14T00:00:00Z', value: 0.35, model_version: ' recbole_v2 ' },
])[0].modelVersion).toBe('recbole_v2')
```

- [ ] **Step 2: Verify RED**

Run from `hls-web/`: `npm test -- src/recommendation/api/recommendationConsole.test.js src/recommendation/recbolePerformance.test.js`

Expected: FAIL because the API/helper do not exist.

- [ ] **Step 3: Implement API and helper**

Add the RecBole endpoint and fetch function. Preserve existing chart geometry; normalize `model_version` to `modelVersion`.

- [ ] **Step 4: Verify helper GREEN**

Repeat the Step 2 test command. Expected: PASS.

- [ ] **Step 5: Write a failing component contract test**

Assert the component exists, uses `aria-label="RecBole 推荐性能趋势"`, defaults to `NDCG@20`, has stable loading/error/empty states, references `tooltip.point.modelVersion`, and mounts before business effects.

- [ ] **Step 6: Verify component RED**

Run: `npm test -- src/recommendation/components/RecBolePerformanceChart.test.js`

Expected: FAIL because the new component is absent.

- [ ] **Step 7: Implement component and style migration**

```js
const defaultMetrics = [
  { value: 'Recall@20', label: 'Recall@20' },
  { value: 'NDCG@20', label: 'NDCG@20' },
  { value: 'Hit@20', label: 'Hit@20' },
  { value: 'Precision@20', label: 'Precision@20' },
]
const metric = ref('NDCG@20')
```

Rename imports, selectors, ARIA, eyebrow, loading/error text and summary from Gorse to RecBole. Add model version to the tooltip without changing `viewBox="0 0 720 280"`.

- [ ] **Step 8: Verify and commit Task 3**

Run the three focused frontend test files. Expected: PASS. Commit frontend files with `feat(recommendation): migrate trend chart to RecBole`.

### Task 4: Remove old Gorse trend infrastructure

**Files:**
- Delete: `video-service/internal/application/videoapp/recommendation/gorse_dashboard_client.go`
- Delete: `video-service/internal/application/videoapp/recommendation/gorse_dashboard_client_test.go`
- Delete: `video-service/internal/application/videoapp/recommendation_admin_gorse_test.go`
- Delete: `hls-web/src/recommendation/components/GorsePerformanceChart.vue`
- Delete: `hls-web/src/recommendation/components/GorsePerformanceChart.test.js`
- Delete: `hls-web/src/recommendation/gorsePerformance.js`
- Delete: `hls-web/src/recommendation/gorsePerformance.test.js`
- Modify: `.env.deploy.example`, `.env.local.example`, `docker-compose.yml`, `gorse/config.toml`, `gorse/entrypoint.sh`
- Modify: backend config/service/app wiring and tests containing `GorseDashboardClient` or dashboard credentials.
- Modify: `README.md`, `hls-web/README.md`, `video-service/README.md`, `video-service/docs/gorse-recommendation-runbook.md`

- [ ] **Step 1: Add failing absence assertions**

Update source-contract tests to reject `GORSE_DASHBOARD_USERNAME`, `GORSE_DASHBOARD_PASSWORD`, `GorseDashboardClient`, `/gorse/performance`, `fetchGorsePerformance`, and `GorsePerformanceChart`, while keeping Gorse recall/sync configuration assertions.

- [ ] **Step 2: Verify RED**

Run: `go test ./internal/config ./internal/http/app` and `npm test -- src/recommendation`.

Expected: FAIL while old trend symbols remain.

- [ ] **Step 3: Delete trend-only code and update docs**

Remove Dashboard client wiring/config and old frontend files. Preserve Gorse endpoint/API key configuration used by recall and sync. Replace API tables and console descriptions with the RecBole endpoint and four offline metrics.

- [ ] **Step 4: Prove old references are gone**

```bash
rg -n "gorse/performance|GorsePerformance|fetchGorsePerformance|gorsePerformance|Gorse 推荐性能趋势|GORSE_DASHBOARD_(USERNAME|PASSWORD)|GorseDashboardClient" --glob '!docs/superpowers/**' --glob '!**/*.log' .
```

Expected: no matches.

- [ ] **Step 5: Format, verify, and commit Task 4**

Run `gofmt` on changed Go files, `go test ./internal/config ./internal/http/app ./internal/http/handler ./internal/http/router`, and `npm test -- src/recommendation`.

Expected: PASS. Commit only scoped cleanup/docs with `refactor(recommendation): remove Gorse performance trend`; exclude the pre-existing untracked `deploy/`.

### Task 5: Final verification

**Files:** No production changes expected.

- [ ] **Step 1: Run full backend tests**

From `video-service/`: `go test ./...`

Expected: zero failing packages.

- [ ] **Step 2: Run full frontend tests and build**

From `hls-web/`: run `npm test`, then `npm run build`.

Expected: all Vitest tests pass and Vite exits 0 with a populated `dist/`.

- [ ] **Step 3: Inspect final scope**

Run `git status --short`, `git diff --check`, and `git log -6 --oneline`.

Expected: only pre-existing unrelated work such as `deploy/` remains; no whitespace errors; implementation commits match the plan.
