# Gorse Full-Chain Recommendation Engine Migration Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the custom Python two-tower training and PostgreSQL embedding recall path with Gorse-managed recommendation training and serving for personalized video segment recommendation.

**Architecture:** Add Gorse as a first-class backend service. Sync local users, video segments, and behavior events into Gorse; let Gorse train and serve recommendations; keep the Go service as the frontend API, hydrator, business filter, exposure recorder, and fallback controller.

**Tech Stack:** Go, Gorse REST API, docker-compose, PostgreSQL, Redis/cache/blob stores used by Gorse, existing `video-service` recommendation service, existing unit tests.

---

## Scope Check

This plan replaces the personalized recommendation model-training-to-serving chain. It does not replace video upload, playback, vector worker content embeddings, HLS, object storage, or the public HTTP response format.

`POST /api/recommendations/by-question` remains semantic question matching unless a later product decision changes it into personalized recommendation. Gorse can rerank or supplement it later, but that is not required for retiring Python two-tower.

## File Structure

Create:

- `video-service/internal/application/videoapp/recommendation/gorse_client.go`
  - Gorse REST client and interface.
- `video-service/internal/application/videoapp/recommendation/gorse_client_test.go`
  - Client tests with `httptest`.
- `video-service/internal/application/videoapp/recommendation/gorse_mapping.go`
  - Pure mapping helpers for Gorse users/items/feedback.
- `video-service/internal/application/videoapp/recommendation/gorse_mapping_test.go`
  - Mapping tests.
- `video-service/tools/sync_gorse_recommendation_data/main.go`
  - Batch sync CLI.
- `video-service/tools/sync_gorse_recommendation_data/main_test.go`
  - CLI and dry-run tests.
- `video-service/internal/worker/gorsesync/scheduler.go`
  - Optional periodic sync worker after CLI validation.
- `video-service/internal/worker/gorsesync/scheduler_test.go`
  - Worker tests.
- `video-service/docs/gorse-recommendation-runbook.md`
  - Operations, validation, rollback.

Modify:

- `docker-compose.yml`
  - Add Gorse service and volumes.
- `video-service/internal/config/types.go`
  - Add Gorse config.
- `video-service/internal/config/defaults.go`
  - Add disabled defaults.
- `video-service/configs/video.yml`
  - Add local Gorse block.
- `video-service/configs/video_prod.yml`
  - Add prod Gorse block disabled by default.
- `video-service/internal/application/videoapp/recommendation/service.go`
  - Add Gorse-primary path behind config.
- `video-service/internal/application/videoapp/recommendation/service_test.go`
  - Add Gorse recommendation, fallback, and exposure tests.
- `video-service/internal/worker/combined/app.go`
  - Register optional Gorse sync worker.
- `two-tower-training/scripts/run_two_tower_pipeline.sh`
  - Disable from production deployment only after Gorse primary is validated.

## Task 1: Add Gorse Config

**Files:**

- Modify: `video-service/internal/config/types.go`
- Modify: `video-service/internal/config/defaults.go`
- Modify: `video-service/configs/video.yml`
- Modify: `video-service/configs/video_prod.yml`
- Test: `video-service/internal/config/loader_test.go`

- [ ] Write failing test that loads:
  - `gorse.enabled`
  - `gorse.endpoint`
  - `gorse.api_key`
  - `gorse.timeout_seconds`
  - `gorse.shadow_mode`
  - `gorse.sync_enabled`
  - `gorse.write_back_enabled`
- [ ] Add `GorseConfig` to config types.
- [ ] Set defaults:
  - `enabled=false`
  - `endpoint=http://localhost:8087`
  - `timeout_seconds=2`
  - `shadow_mode=true`
  - `sync_enabled=false`
  - `write_back_enabled=false`
- [ ] Add config blocks to local and production YAML.
- [ ] Run `cd video-service && go test ./internal/config`.

## Task 2: Add Gorse To Docker Compose

**Files:**

- Modify: `docker-compose.yml`
- Create: `video-service/docs/gorse-recommendation-runbook.md`

- [ ] Add Gorse service with explicit image tag.
- [ ] Add persistent volumes for Gorse data/cache/model state.
- [ ] Expose Gorse server only to backend network by default; expose local port only for development.
- [ ] Set an API key through environment/config.
- [ ] Add health check for the REST API.
- [ ] Document local startup, data reset, dashboard/API access, and rollback.
- [ ] Run `docker compose config` if Docker is available.

## Task 3: Build Gorse REST Client

**Files:**

- Create: `video-service/internal/application/videoapp/recommendation/gorse_client.go`
- Create: `video-service/internal/application/videoapp/recommendation/gorse_client_test.go`

- [ ] Write failing `httptest` test for `Recommend(ctx, userID, n)`.
- [ ] Verify it calls `GET /api/recommend/{user-id}?n={n}`.
- [ ] Verify it sends `X-API-Key`.
- [ ] Verify returned string IDs parse into `video_segment_id`.
- [ ] Verify invalid IDs are ignored, not fatal.
- [ ] Verify non-2xx and timeout return errors.
- [ ] Add feedback write methods for `PUT /api/feedback` and item/user batch methods for sync.
- [ ] Run `cd video-service && go test ./internal/application/videoapp/recommendation`.

## Task 4: Map Domain Data To Gorse Data

**Files:**

- Create: `video-service/internal/application/videoapp/recommendation/gorse_mapping.go`
- Create: `video-service/internal/application/videoapp/recommendation/gorse_mapping_test.go`

- [ ] Write tests mapping `sys_user` data to Gorse user payloads.
- [ ] Write tests mapping `edu_video_segment` plus `edu_video_resource` to Gorse item payloads.
- [ ] Verify deleted/disabled/unpublished/unplayable segments map to `IsHidden=true`.
- [ ] Verify knowledge tags and subjects map to categories.
- [ ] Verify existing segment embedding can be included in item labels.
- [ ] Write tests mapping feedback:
  - `double_like -> double_like value 3`
  - `like -> like value 2`
  - long watch -> `watch` value ratio
  - exposure -> `exposure` value 1
  - `dislike -> dislike value 1`
- [ ] Ensure exposure-no-click is read feedback or omitted, not negative.
- [ ] Run `cd video-service && go test ./internal/application/videoapp/recommendation`.

## Task 5: Build Batch Sync CLI

**Files:**

- Create: `video-service/tools/sync_gorse_recommendation_data/main.go`
- Create: `video-service/tools/sync_gorse_recommendation_data/main_test.go`

- [ ] Write option parsing tests for:
  - `--config`
  - `--endpoint`
  - `--api-key`
  - `--batch-size`
  - `--dry-run`
  - `--users`
  - `--items`
  - `--feedback`
- [ ] Load users from `sys_user`.
- [ ] Load items from valid video segments/resources.
- [ ] Load feedback from reactions, watch records, and exposures.
- [ ] Use mapping helpers from Task 4.
- [ ] In dry-run, print counts and representative payloads.
- [ ] In live mode, upsert users/items and put feedback into Gorse.
- [ ] Run `cd video-service && go test ./tools/sync_gorse_recommendation_data`.

## Task 6: Add Periodic Sync Worker

**Files:**

- Create: `video-service/internal/worker/gorsesync/scheduler.go`
- Create: `video-service/internal/worker/gorsesync/scheduler_test.go`
- Modify: `video-service/internal/worker/combined/app.go`

- [ ] Write tests that worker is disabled when `gorse.sync_enabled=false`.
- [ ] Write tests that worker does not overlap runs.
- [ ] Start with full periodic sync every configured interval.
- [ ] Add incremental sync later only if full sync is too slow.
- [ ] Register worker in combined worker when enabled.
- [ ] Run `cd video-service && go test ./internal/worker/gorsesync ./internal/worker/combined`.

## Task 7: Make Gorse Primary For Personalized Video Recommendation

**Files:**

- Modify: `video-service/internal/application/videoapp/recommendation/service.go`
- Modify: `video-service/internal/application/videoapp/recommendation/service_test.go`
- Modify: repository interface and persistence implementation only if a new hydration method is needed.

- [ ] Add `GorseClient` optional dependency to `Service`.
- [ ] Add a repository method to hydrate a list of `video_segment_id` values while preserving Gorse order.
- [ ] Write test: when enabled and `shadow_mode=false`, `random-play` returns Gorse candidates.
- [ ] Write test: Gorse IDs are hydrated and unavailable segments are dropped.
- [ ] Write test: exposures are saved with strategy `gorse`.
- [ ] Write test: if Gorse returns too few valid candidates, fallback path fills remaining results.
- [ ] Write test: Gorse error falls back without failing the public API.
- [ ] Implement the Gorse-primary path for personalized random-play/list recommendation.
- [ ] Run `cd video-service && go test ./internal/application/videoapp/recommendation`.

## Task 8: Feedback Writeback

**Files:**

- Modify: `video-service/internal/application/videoapp/recommendation/service.go`
- Modify: reaction/watch workers if needed.
- Modify tests in affected packages.

- [ ] On recommendation exposure, write `exposure` feedback to Gorse when `write_back_enabled=true`.
- [ ] On watch event, write `watch` feedback with duration ratio.
- [ ] On like/double_like/dislike reaction, write matching feedback type.
- [ ] Use short timeouts; do not fail user-facing actions when Gorse writeback fails.
- [ ] Ensure local PostgreSQL remains source of truth and periodic sync can repair missed writebacks.
- [ ] Run affected package tests.

## Task 9: Shadow Metrics Then Promotion

**Files:**

- Modify service tests and add comparison storage only if existing exposure tables cannot hold the data.

- [ ] In `shadow_mode=true`, call Gorse but return existing path.
- [ ] Record Gorse candidate IDs, returned IDs, valid candidate count, overlap, explicit dislike hits, and fallback reason.
- [ ] Add a small export/query tool for shadow metrics if needed.
- [ ] Promote only when:
  - Gorse valid candidate rate is high enough.
  - Explicit dislike hit rate is not worse.
  - Watch/click conversion is not worse.
  - Coverage does not collapse.
  - Gorse fallback rate is acceptable.

## Task 10: Retire Python Two-Tower Production Path

**Files:**

- Modify: `docker-compose.yml`
- Modify: two-tower trainer scheduler config or deployment environment.
- Modify docs.

- [ ] Disable `two_tower_trainer` in production once Gorse primary is stable.
- [ ] Stop publishing new `two_tower` model versions.
- [ ] Keep existing embedding tables and artifacts for one rollback window.
- [ ] Remove `two-tower-training` container from production compose after rollback window.
- [ ] Update docs to mark Python two-tower as deprecated.

## Verification Commands

Focused:

```bash
cd video-service
go test ./internal/config
go test ./internal/application/videoapp/recommendation
go test ./tools/sync_gorse_recommendation_data
go test ./internal/worker/gorsesync ./internal/worker/combined
```

Full:

```bash
cd video-service
go test ./...
```

Operational:

```bash
docker compose config
docker compose up gorse
go run ./tools/sync_gorse_recommendation_data --config configs/video.yml --dry-run
```

## Rollback

Set:

```text
gorse.enabled=false
gorse.write_back_enabled=false
```

The service should immediately fall back to existing two-tower/random behavior until the Python path is intentionally retired. During the rollback window, keep old embedding tables and the Python trainer available.
