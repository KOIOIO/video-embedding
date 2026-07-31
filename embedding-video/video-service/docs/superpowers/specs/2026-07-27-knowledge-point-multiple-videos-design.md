# Knowledge Point Multiple Videos Design

## Goal

Allow one knowledge point to own multiple independently uploaded videos. An upload may add more videos to a knowledge point that already has active videos, and a playback lookup returns every ready video for that knowledge point without recording playback until the user actually starts a specific video.

## Scope

This change covers the active Go service in `video-service/` and the Vue debug console in `hls-web/`. It changes database indexes, XLSX/ZIP validation, importing, repository contracts, playback and tree APIs, Swagger output, and the knowledge-video frontend.

The XLSX format remains exactly three columns: `id`, `name`, and `video_name`.

## Data Model And Migration

`edu_knowledge_video` remains the source of truth, with one row per video. Multiple active rows may have the same `knowledge_point_id`.

The migration must explicitly drop `uk_knowledge_video_knowledge_point_active`, because creating another index does not remove the existing uniqueness constraint. It then creates a non-unique partial query index on `knowledge_point_id` for active rows. The existing partial unique index on `(batch_id, source_file_name)` remains and continues to enforce uniqueness within one upload batch.

The migration is forward-only and preserves all existing video rows. It neither merges nor deletes data.

## Upload And Validation

Each non-header XLSX row represents one knowledge-point-to-video relationship.

- `id` must be a positive knowledge point ID, but may repeat within the workbook.
- `name` must exactly match the local `dict_knowledge_point` name for that row's `id`.
- `video_name` must be non-empty and unique within the workbook.
- `video_name` must exactly identify one video entry in the ZIP archive.
- Every accepted ZIP video must have exactly one XLSX row; missing, extra, ambiguous, or duplicate mappings remain validation errors.
- `video_name` is independent of the knowledge point name. No equality or derived-name check is allowed.
- macOS metadata filtering and the existing supported-video checks remain unchanged.

Importing no longer checks whether a knowledge point already has an active video. Repeated rows for one knowledge point and later batches for that knowledge point each create a new `edu_knowledge_video` row and a separate transcode task. Existing rows are not overwritten, soft-deleted, or otherwise treated as a reupload target. The same source filename may appear in different batches because uniqueness remains batch-scoped.

## Repository And Application Contracts

The knowledge-point resolver changes from returning one arbitrary video to returning all ready videos for a knowledge point, ordered by video ID ascending. Pending, transcoding, failed, and deleted videos are excluded from playback results.

Playback listing and playback recording are separate application operations:

1. Listing validates that the knowledge point exists, loads all ready videos, and constructs playback URLs. It performs no write.
2. Recording validates `user_id`, loads the requested `knowledge_video_id`, verifies that the video is active and ready, and writes a play record using the video's own `knowledge_point_id`.

The display name is derived by removing only the final filename extension. For example, `lesson.mp4` becomes `lesson`, while `lesson.mp4.mp4` becomes `lesson.mp4`.

## HTTP API

### Playback Listing

The preferred endpoint is:

`GET /api/knowledge-points/{knowledgePointId}/videos`

The existing compatibility endpoint remains:

`GET /api/knowledge-points/{knowledgePointId}/video`

Both return the same multi-video data shape and do not require `user_id`. A legacy `user_id` query parameter is accepted and ignored, so old URLs keep working without causing a play record.

The response data contains:

```json
{
  "knowledge_point_id": 9,
  "knowledge_point_name": "一次函数",
  "videos": [
    {
      "knowledge_video_id": 88,
      "source_file_name": "一次函数解析式.mp4.mp4",
      "display_name": "一次函数解析式.mp4",
      "duration": 123,
      "playback_url": "/knowledge-video-media/hls/88/master.m3u8"
    }
  ],
  "video_id": 88,
  "video_name": "一次函数解析式.mp4.mp4",
  "duration": 123,
  "playback_url": "/knowledge-video-media/hls/88/master.m3u8"
}
```

When at least one ready video exists, the legacy `video_id`, `video_name`, `duration`, and `playback_url` fields mirror the first video in ID order. When `videos` is empty, those legacy fields are omitted.

If the knowledge point exists but has no videos or has only non-ready videos, the endpoint returns HTTP 200 with `videos: []`. If the knowledge point does not exist, it returns HTTP 404.

### Playback Recording

The new endpoint is:

`POST /api/knowledge-videos/{knowledgeVideoId}/playbacks`

Request body:

```json
{
  "user_id": 7
}
```

It returns success after writing one play record. A missing or invalid `user_id` returns HTTP 400, a missing video returns HTTP 404, and an active video that is not ready returns HTTP 409. Repository failures return HTTP 500.

### Knowledge Tree

Each knowledge tree node returns `videos`, containing all active videos and their current statuses so the management UI can show the complete state. For compatibility, `video` remains and mirrors the first item in ID order when at least one video exists. It is omitted when the array is empty.

Swagger annotations are updated and generated output under `docs/swagger/` is regenerated rather than edited manually.

## Frontend Behavior

Selecting a knowledge point requests the preferred plural playback endpoint. Every returned ready video is rendered in its own independent `HlsPlayer` window. Each window title uses `display_name`; the frontend may derive the same value from `source_file_name` only as a defensive fallback.

The UI does not render pending, transcoding, or failed videos as playback windows. An empty `videos` array shows a clear empty state rather than a playback error.

Each player listens for its first actual `play` event. On that event, the frontend posts `user_id` and that player's `knowledge_video_id` to the playback-record endpoint. A player records at most once during its component lifecycle, so pause/resume does not create duplicate records. Recording failure does not stop media that has already started; it is reported as a non-blocking client error.

## Error Handling

XLSX/ZIP structural and mapping failures continue to return row-oriented validation issues. Allowing duplicate knowledge point IDs must not weaken unique `video_name` or exact ZIP matching checks.

Playback listing treats readiness as filtering, not as an error. Playback recording treats non-ready state as a conflict because the client attempted an action that is invalid for the video's current state. Failed videos follow the same HTTP 409 contract and do not expose failed items as playable data.

## Test Strategy

Development follows TDD: add or update the relevant test, confirm that it fails for the intended reason, then make the smallest implementation change to pass it.

Backend coverage includes:

- Validator acceptance of repeated knowledge point IDs and video filenames unrelated to knowledge point names.
- Continued rejection of duplicate `video_name`, missing ZIP entries, extra ZIP videos, and non-exact names.
- Importing multiple rows for one knowledge point and adding videos when active rows already exist.
- Migration removal of the unique knowledge-point index and creation of the ordinary active-row index.
- Repository support for multiple rows and ready-only, ID-ordered playback lookup.
- Read-only playback listing, empty-list success, display-name derivation, and explicit playback recording by video ID.
- HTTP coverage for singular and plural listing paths, compatibility fields, empty arrays, recording, and error mappings.
- Tree responses containing `videos` plus the compatibility `video` field.

Frontend coverage includes:

- API requests for plural playback lookup and playback recording.
- One independent player per ready video with the expected display title.
- One record request on the first `play` event per player and no duplicate on resume.
- Empty playback state when no ready videos are returned.

Verification runs focused Go tests first, then `go test ./...` from `video-service/`. It also runs the relevant frontend tests and the production frontend build from `hls-web/`, followed by browser verification of upload progress, multi-player rendering, responsive layout, and actual play-event recording against the local applications when required infrastructure is available.

## Acceptance Criteria

- A workbook may contain several rows with the same valid knowledge point ID and distinct ZIP video names.
- Uploading another batch for a knowledge point with existing videos succeeds without replacing those videos.
- A playback lookup returns all and only ready videos in stable order and writes no play records.
- Starting one specific player records that specific `knowledge_video_id` once for that player lifecycle.
- The frontend exposes every ready video as a separately selectable player and displays its filename without the final extension.
- Existing singular playback and tree consumers retain their compatibility fields.
