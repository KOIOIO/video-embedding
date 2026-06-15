# Video Reactions Design

## Summary

Add a three-level video reaction feature to `video-service` with these user-visible behaviors:

1. Users can react to a video with exactly one of `like`, `double_like`, or `dislike`.
2. Repeating the same reaction cancels the existing reaction.
3. Changing to a different reaction removes the old reaction and applies the new one.
4. Aggregated counts are persisted on `edu_video_resource`.
5. The read API returns only `like_count` and `double_like_count`, not `dislike_count`.

The implementation must stay aligned with the current service structure:

- HTTP handlers under `internal/http/handler/videos`
- application logic under `internal/application/videoapp`
- persistence under `internal/infrastructure/persistence`
- GORM-managed schema via `AutoMigrate`

## Confirmed Decisions

The following product decisions were explicitly confirmed:

1. Reactions are user-specific, not pure append-only counters.
2. The client sends `user_id` in the request body.
3. A user can have at most one active reaction per video.
4. Clicking the same reaction again cancels it.
5. Aggregated counts must be stored in `edu_video_resource`.
6. The count API must not expose `dislike_count`.

These rules mean `edu_video_resource` alone is insufficient. The system must persist both:

1. aggregate counters on the video resource row
2. a per-user reaction record that identifies the currently active reaction

## Problem

The current service already supports video listing, playback, recommendation, and watch reporting, but it has no notion of per-user video reactions.

Without a per-user record, the service cannot correctly support:

1. switching from one reaction to another
2. canceling the same reaction on repeat click
3. decrementing the correct counter when a user changes their mind

Without aggregate counters on `edu_video_resource`, the read API would need to count rows on every request, which conflicts with the requirement to store the data on the resource table.

## Goals

1. Persist video reaction aggregates on `edu_video_resource`.
2. Persist one user-to-video reaction record for the active reaction state.
3. Expose one write API for submitting a reaction.
4. Expose one read API for returning `like_count` and `double_like_count`.
5. Keep compatibility with the existing REST and legacy route style.
6. Make reaction updates transactional so aggregate counters and user state cannot drift apart.

## Non-Goals

1. Do not add authentication or derive `user_id` from session state in this phase.
2. Do not redesign recommendation or watch-record tables.
3. Do not expose `dislike_count` from the count API.
4. Do not add analytics, ranking, or recommendation behavior based on reactions.
5. Do not batch or async-process reaction writes.

## Existing Context

Relevant existing patterns in the codebase:

1. `internal/model/video.go` owns `edu_video_resource` and other video-related models.
2. `internal/http/app/app.go` uses `AutoMigrate(...)` for video-related tables.
3. `internal/http/router/router.go` keeps both REST routes and legacy compatibility aliases.
4. `internal/http/handler/videos/handler.go` already hosts video-specific request parsing and response mapping.
5. `internal/application/videoapp/contracts.go` defines the `VideoRepository` port.
6. `internal/infrastructure/persistence/gorm_video_repository.go` owns GORM-backed video persistence logic.

The new feature should follow these boundaries instead of introducing a separate subsystem.

## Data Model

### `edu_video_resource`

Add three aggregate integer columns:

1. `like_count`
2. `double_like_count`
3. `dislike_count`

All three default to `0` and are non-negative by behavior.

These fields are the source for fast count reads. The count API will use `like_count` and `double_like_count` only.

### `edu_video_user_reaction`

Add one new table for the active user-to-video reaction state:

1. `id`
2. `user_id`
3. `video_id`
4. `reaction_type`
5. `create_time`
6. `update_time`
7. `deleted`

Modeling rules:

1. `reaction_type` is restricted to `like`, `double_like`, or `dislike`.
2. There is exactly one logical row per `(user_id, video_id)`.
3. The row uses the project's existing soft-delete convention via `deleted`.

### Uniqueness and Soft Delete

Use a unique constraint on `(user_id, video_id)`.

Because the row is soft-deleted on cancel, the implementation must not create multiple historical rows for the same pair. Instead:

1. first reaction inserts the row
2. cancel sets `deleted = 1`
3. a later new reaction for the same user/video pair revives the same row with `deleted = 0` and updates `reaction_type`

This keeps the unique constraint simple and avoids partial-index-only logic.

## API Design

### Write API

REST route:

- `POST /api/videos/:id/reactions`

Legacy compatibility route:

- `POST /api/video/reaction/:id`

Request body:

```json
{
  "user_id": 123,
  "reaction_type": "double_like"
}
```

Validation:

1. `id` must be a positive integer
2. `user_id` must be a positive integer
3. `reaction_type` must be one of `like`, `double_like`, `dislike`

Response body:

```json
{
  "success": true,
  "data": {
    "video_id": 10,
    "user_id": 123,
    "reaction_type": "double_like",
    "active": true,
    "updated": true
  }
}
```

Response semantics:

1. `active = true` means the requested reaction is active after the operation
2. `active = false` means the requested reaction was canceled by repeating the same click

### Count API

REST route:

- `GET /api/videos/:id/reaction-counts`

Legacy compatibility route:

- `GET /api/video/reaction_counts/:id`

Response body:

```json
{
  "success": true,
  "data": {
    "video_id": 10,
    "like_count": 5,
    "double_like_count": 2
  }
}
```

The API must not return `dislike_count`.

## Behavior Rules

For a given `(user_id, video_id)` pair:

1. No existing active reaction + new reaction request
   - store the requested reaction as active
   - increment the corresponding aggregate counter by `1`

2. Existing active reaction is different from requested reaction
   - decrement the old reaction counter by `1`
   - increment the new reaction counter by `1`
   - update the stored reaction type

3. Existing active reaction matches requested reaction
   - cancel the reaction
   - decrement the matching aggregate counter by `1`
   - mark the user reaction row as deleted

These three branches define the full business behavior. There is no separate delete API.

## Transaction Design

The write API must run inside one database transaction.

Recommended sequence:

1. Lock the target `edu_video_resource` row and ensure it exists with `deleted = 0`.
2. Load the existing user reaction row for `(user_id, video_id)`, including rows with `deleted = 1`.
3. Decide whether this request is an insert, switch, or cancel.
4. Apply the user reaction row mutation.
5. Apply the aggregate counter mutation on `edu_video_resource`.
6. Commit the transaction.

### Why lock the video row first

Locking the video row serializes reaction writes for the same video and gives one stable place to protect aggregate counter updates. This is the simplest way to keep the counters and user state aligned without introducing more complicated coordination.

### Counter Safety

Counter updates should be expressed as relative updates, not read-modify-write in memory. The persistence layer should use SQL updates that increment or decrement the affected columns directly inside the transaction.

The implementation must preserve non-negative counters by construction:

1. increment only when activating a reaction
2. decrement only when an active stored reaction is being removed or replaced

## Application Layer Changes

Add minimal application methods under `internal/application/videoapp`:

1. submit a video reaction
2. get reaction counts

The service should validate:

1. `video_id > 0`
2. `user_id > 0`
3. `reaction_type` is supported

The application layer should not contain HTTP details. It should orchestrate the repository call and return a small domain-oriented result:

1. which reaction was requested
2. whether it is active after the operation
3. whether the target video existed

## Persistence Layer Changes

Extend `VideoRepository` with narrowly scoped methods for:

1. submitting a video reaction transactionally
2. reading reaction counts

The GORM repository implementation should own:

1. row locking
2. loading or reviving the user reaction row
3. incrementing or decrementing aggregate counters
4. handling not-found video behavior

This keeps transaction details in the persistence layer, where the database semantics already live.

## HTTP Layer Changes

Add handler methods in `internal/http/handler/videos/handler.go` and expose them through the compatibility wrapper in `internal/http/handler/video.go`.

Responsibilities:

1. parse path `id`
2. bind JSON request body for the write API
3. validate missing or malformed parameters
4. map application outcomes to HTTP status codes and DTOs

Expected status behavior:

1. `200` for successful set, switch, or cancel
2. `400` for invalid `id`, `user_id`, or `reaction_type`
3. `404` when the video does not exist
4. `500` for unexpected persistence failures

## DTO Shape

Add new DTOs in `internal/http/dto/video.go` for:

1. reaction write request
2. reaction write response data
3. reaction count response data

Keep naming and envelope style consistent with existing video DTOs:

1. top-level `{ "success": true, "data": ... }`
2. field names in snake_case

## Routing

Register both route styles in `internal/http/router/router.go`:

1. `POST /api/videos/:id/reactions`
2. `POST /api/video/reaction/:id`
3. `GET /api/videos/:id/reaction-counts`
4. `GET /api/video/reaction_counts/:id`

This preserves the repository's current pattern of supporting both preferred REST endpoints and historical compatibility paths.

## Error Handling

Validation errors should be explicit and match existing handler style:

1. `id must be a positive integer`
2. `user_id is required`
3. `user_id must be a positive integer`
4. `reaction_type is required`
5. `reaction_type must be one of like, double_like, dislike`

Not-found behavior should use the existing video-not-found style.

## Testing Strategy

Follow TDD for implementation. Minimum coverage should include:

### Application / persistence behavior

1. first `like` creates an active reaction and increments `like_count`
2. first `double_like` creates an active reaction and increments `double_like_count`
3. first `dislike` creates an active reaction and increments `dislike_count`
4. switching `double_like -> like` decrements one counter and increments the other
5. repeating `like` cancels the reaction and decrements `like_count`
6. reviving a canceled reaction reuses the same logical `(user_id, video_id)` record
7. reading counts returns only `like_count` and `double_like_count`
8. missing video returns not found

### HTTP behavior

1. invalid `id` returns `400`
2. missing or zero `user_id` returns `400`
3. invalid `reaction_type` returns `400`
4. successful write returns `active = true` for set/switch
5. repeated same reaction returns `active = false` for cancel
6. count endpoint returns the expected JSON shape without `dislike_count`

## Success Criteria

The feature is complete when all of the following are true:

1. video reactions can be set, switched, and canceled correctly
2. a user has at most one active reaction per video
3. aggregate counters on `edu_video_resource` stay in sync with user reaction state
4. the count API returns only `like_count` and `double_like_count`
5. REST and legacy routes both work
6. focused automated tests cover the core branches
