# Knowledge Point Video Playback Design

## Summary

Add an isolated video import and playback chain that maps one existing `dict_knowledge_point` row to exactly one video. The chain accepts a ZIP archive plus an XLSX mapping file, transcodes each video to HLS, stores all source and HLS objects in a dedicated private bucket, and resolves playback by `user_id` plus `knowledge_point_id`.

This feature must not write `edu_video_resource`, enqueue vectorization, or participate in personalized recommendation.

## Confirmed Decisions

1. A knowledge point has exactly one video, and a video belongs to exactly one knowledge point.
2. `dict_knowledge_point` is authoritative. XLSX IDs must exist and names must match exactly after trimming surrounding whitespace.
3. Import validation is atomic: any mapping or archive validation error rejects the complete batch before database, object-storage, or Redis side effects.
4. Transcoding is asynchronous per video. A failure after an accepted import affects only that video and does not roll back successful siblings.
5. The feature uses a dedicated object-storage bucket. Endpoint and credentials may be shared with the existing storage service.
6. The bucket remains private and media is served through a constrained backend proxy.
7. A successful playback lookup persists a user playback access record. `user_id` does not influence selection because the mapping is one-to-one.

## Goals

1. Import a ZIP of videos and an XLSX mapping in one request.
2. Strictly validate every mapping against `dict_knowledge_point` and the ZIP contents.
3. Persist an isolated knowledge-point-to-video mapping and import status.
4. Transcode through an independent Redis Stream without vectorization.
5. Return the unique ready video for a required `user_id` and `knowledge_point_id`.
6. Record every successful playback lookup.
7. Provide batch-level processing progress and per-video errors.

## Non-Goals

1. Do not create or update `dict_knowledge_point`.
2. Do not add personalized selection, ranking, randomization, or fallback videos.
3. Do not add vector extraction, embeddings, segmentation, or recommendation data.
4. Do not add watch progress, completion reporting, or playback authorization in this phase.
5. Do not retain the uploaded ZIP after its validated video entries have been stored separately.
6. Do not modify the legacy Go backend or `hls-web`.

## Architecture

The implementation belongs in `video-service` and introduces an isolated `knowledgevideo` application boundary:

1. `ImportService` streams the multipart files to a restricted temporary directory, validates the complete batch, uploads accepted objects, creates records, and enqueues work.
2. `Repository` owns the three new tables and performs read-only validation queries against `dict_knowledge_point`.
3. `TranscodeWorker` consumes a dedicated Redis Stream, reuses the existing FFmpeg and directory-upload capabilities, and updates only knowledge-video state.
4. `PlaybackService` resolves the unique ready mapping and writes a playback record.
5. `MediaProxy` reads only the configured knowledge-video bucket and derives object keys from persisted video metadata.

The worker may run inside the existing `cmd/worker` process, but its queue, task type, repository methods, and state transitions remain independent from the existing video and vector workers.

## Data Model

### `edu_knowledge_video_batch`

One row represents one accepted ZIP/XLSX import.

| Column | Type | Rules |
| --- | --- | --- |
| `id` | bigint PK | Sequence-generated batch ID |
| `upload_user_id` | bigint | Required positive uploader ID |
| `zip_file_name` | varchar(255) | Original ZIP name |
| `xlsx_file_name` | varchar(255) | Original XLSX name |
| `xlsx_object_key` | varchar(500) | Manifest key in the dedicated bucket |
| `total_count` | integer | Number of accepted mapping rows |
| `ready_count` | integer | Successfully transcoded videos |
| `failed_count` | integer | Permanently failed videos |
| `status` | smallint | `1=processing`, `2=completed`, `3=partial_failed`, `4=failed` |
| `error_message` | text | Batch-level operational error |
| `create_time` | timestamp | Creation time |
| `update_time` | timestamp | Last update time |

Validation failures do not create a batch row. A terminal batch is `completed` when all videos are ready, `partial_failed` when ready and failed videos both exist, and `failed` when every video has permanently failed or a batch-level operational failure occurs after its row is committed.

### `edu_knowledge_video`

This table is the one-to-one mapping and the source of truth for transcode state.

| Column | Type | Rules |
| --- | --- | --- |
| `id` | bigint PK | Sequence-generated video ID |
| `batch_id` | bigint | Accepted import batch |
| `knowledge_point_id` | bigint | Logical reference to `dict_knowledge_point.id` |
| `knowledge_point_name` | varchar(255) | Validated name snapshot |
| `source_file_name` | varchar(255) | Exact ZIP filename including extension |
| `source_object_key` | varchar(500) | Original video object key |
| `hls_object_prefix` | varchar(500) | HLS object directory |
| `hls_master_object_key` | varchar(500) | Master playlist key |
| `duration` | integer | Duration in seconds after transcode |
| `status` | smallint | `0=pending`, `1=transcoding`, `2=ready`, `3=failed` |
| `error_message` | text | Last or terminal transcode error |
| `create_time` | timestamp | Creation time |
| `update_time` | timestamp | Last update time |
| `deleted` | smallint | Existing project soft-delete convention |

Required indexes:

```sql
CREATE UNIQUE INDEX ... ON edu_knowledge_video (knowledge_point_id) WHERE deleted = 0;
CREATE UNIQUE INDEX ... ON edu_knowledge_video (batch_id, source_file_name) WHERE deleted = 0;
CREATE INDEX ... ON edu_knowledge_video (batch_id, status);
```

Do not add a foreign key to `dict_knowledge_point`. That table is externally owned and already has schema compatibility variations in the service. The application performs authoritative validation, while the partial unique index enforces the local one-to-one invariant.

### `edu_knowledge_video_play_record`

| Column | Type | Rules |
| --- | --- | --- |
| `id` | bigint PK | Playback access ID |
| `user_id` | bigint | Required request user |
| `knowledge_point_id` | bigint | Requested knowledge point |
| `knowledge_video_id` | bigint | Returned local video ID |
| `create_time` | timestamp | Access time |

Add indexes on `(user_id, create_time)`, `(knowledge_point_id, create_time)`, and `(knowledge_video_id, create_time)`. Every successful lookup appends one row; there is no deduplication in this phase.

## Dedicated Object Storage

Add a separate configuration section, using a bucket such as `knowledge-point-videos`:

```yaml
KnowledgeVideoStorage:
  Endpoint: ""
  AccessKey: ""
  SecretKey: ""
  Bucket: knowledge-point-videos
  UseSSL: true
  Region: ""
  BucketLookup: auto
  MediaRoutePrefix: /knowledge-video-media
```

The implementation may reuse the existing S3-compatible `RustFS` client implementation, but it must construct a distinct client/configuration instance for this bucket.

Object layout:

```text
manifests/{batch_id}/mapping.xlsx
raw/{knowledge_video_id}/source.{ext}
hls/{knowledge_video_id}/master.m3u8
hls/{knowledge_video_id}/segment_000001.ts
hls/{knowledge_video_id}/segment_000002.ts
```

The ZIP is a transport container and is removed from temporary storage after processing. The XLSX is retained for audit. Lifecycle rules for raw sources and HLS outputs can be managed independently because the bucket is isolated, but automated expiration is outside this phase.

## Import API

### Create Batch

```http
POST /api/admin/knowledge-videos/batches
Content-Type: multipart/form-data
```

Fields:

1. `archive`: required ZIP containing videos
2. `mapping`: required XLSX mapping file
3. `upload_user_id`: required positive integer

The XLSX first worksheet must use the exact headers:

```text
id | name | video_name
```

`video_name` includes the extension and must exactly match the basename of one ZIP entry.

An accepted request returns `202 Accepted`:

```json
{
  "batch_id": 10001,
  "total_count": 30,
  "status": "processing",
  "progress_url": "/api/admin/knowledge-videos/batches/10001"
}
```

### Get Batch Progress

```http
GET /api/admin/knowledge-videos/batches/{batch_id}
```

The response includes batch totals and each video's knowledge point, filename, status, and error message. Terminal status follows the all-success, mixed-result, and all-failed rules defined by the batch table.

## Validation

Multipart inputs must be streamed to a restricted temporary directory. Do not load an unbounded archive into memory.

Before any persistent side effect, validate all of the following:

1. Both files are present and use allowed extensions.
2. Configured compressed size, uncompressed size, file-count, and per-entry limits are respected.
3. ZIP entries contain no absolute paths, traversal, unsafe links, metadata entries, or duplicate basenames.
4. Only supported video formats are accepted.
5. The first XLSX worksheet exists and has the exact three headers.
6. Every data row has a positive integer `id` and non-empty `name` and `video_name`.
7. IDs and video names are unique within the XLSX.
8. Every ID exists in `dict_knowledge_point`.
9. Each trimmed XLSX name exactly equals the authoritative dictionary name.
10. Every row matches exactly one ZIP video by basename.
11. Every accepted ZIP video is referenced by exactly one XLSX row; unreferenced videos are rejected.
12. No active `edu_knowledge_video` mapping already exists for any imported ID.

Return `400 Bad Request` with structured row, field, and reason details for validation errors. The response may contain all safely discoverable validation errors so the uploader can correct the batch once.

## Persistence and Compensation

PostgreSQL, object storage, and Redis cannot participate in one atomic transaction. Use validation plus explicit compensation:

1. Complete all validation with no persistent side effects.
2. Reserve batch and video IDs from PostgreSQL sequences. Sequence gaps after failure are acceptable.
3. Upload the XLSX and source videos using object keys derived from the reserved IDs.
4. In one database transaction, insert the batch and all video rows. The unique knowledge-point index resolves races with concurrent imports.
5. If object upload or database insertion fails, delete every object uploaded for this batch. Do not leave active business rows.
6. After commit, enqueue one independent transcode task per video.
7. If enqueue fails, leave that video `pending`; a reconciliation loop scans stale pending rows and retries enqueueing.
8. Remove local temporary files after success or failure.

If a concurrent import wins the unique constraint, the losing import returns `409 Conflict` and compensates its objects.

## Transcoding

Use an independent Redis Stream and consumer group:

```text
knowledge_video:transcode:stream
knowledge_video:transcode:group
knowledge_video:transcode:status:{task_id}
```

Each task contains only `knowledge_video_id`, the source object key, HLS output prefix, and stable task ID. It never enqueues vector work.

Processing rules:

1. A `ready` video is an idempotent no-op on redelivery.
2. A worker atomically claims `pending` or retryable work and sets `transcoding`.
3. FFmpeg writes HLS locally, then the directory uploader writes the fixed HLS prefix.
4. Success stores duration and master key, clears errors, and sets `ready`.
5. Retryable failures return to pending processing through Redis pending reclaim.
6. Exhausted retries set `failed` and preserve the terminal error.
7. Batch counters and terminal status are recalculated transactionally after each terminal video transition.

Writing retries to the same fixed prefix makes object output idempotent. A failed partial upload may be overwritten by the next attempt.

## Playback and Media APIs

### Resolve Knowledge Point Video

```http
GET /api/knowledge-points/{knowledge_point_id}/video?user_id={user_id}
```

Both IDs must be positive integers. Selection does not use user state because the mapping is one-to-one.

For a ready video, insert a playback record and return:

```json
{
  "knowledge_point_id": 1001,
  "knowledge_point_name": "一次函数",
  "video_id": 8801,
  "video_name": "一次函数.mp4",
  "duration": 320,
  "playback_url": "/knowledge-video-media/hls/8801/master.m3u8"
}
```

Response rules:

1. `400` for invalid parameters.
2. `404` when the dictionary ID does not exist or no active mapping exists.
3. `409 VIDEO_NOT_READY` for `pending` or `transcoding`.
4. `409 VIDEO_TRANSCODE_FAILED` for `failed`.
5. `200` only after the playback record insert succeeds.

### Media Proxy

The public route shape is:

```text
/knowledge-video-media/hls/{video_id}/{file_name}
```

The bucket remains private. The proxy looks up `video_id`, requires `ready` and `deleted=0`, and joins the requested safe relative filename to the persisted HLS prefix. It must not accept a bucket name or arbitrary object key from the caller.

The proxy supports HLS content types, byte ranges, ETag/cache headers, and relative playlist segment requests. It rejects traversal and cross-video object access.

## Error Handling and Observability

1. Validation errors are client-visible and identify XLSX row, field, and reason without exposing local paths.
2. Operational errors are logged with `batch_id`, `knowledge_video_id`, task ID, and stage.
3. Batch progress exposes terminal per-video errors but not internal credentials or local paths.
4. Metrics should cover accepted/rejected batches, videos pending/ready/failed, transcode duration, enqueue reconciliation, playback lookup outcomes, and proxy errors.
5. Temporary-file cleanup and object compensation failures are logged separately for operations follow-up.

## Testing

Required focused coverage:

1. XLSX parsing: valid rows, wrong headers, empty cells, invalid IDs, duplicate IDs, and duplicate filenames.
2. Dictionary validation: missing ID and exact-name mismatch.
3. ZIP validation: missing, extra, duplicate, unsafe, oversized, and unsupported entries.
4. Atomic validation rejection with no database, object, or Redis side effects.
5. Object upload and database failure compensation.
6. Concurrent imports for the same knowledge point.
7. Enqueue failure followed by pending-row reconciliation.
8. Worker success, retry, exhausted retry, redelivery, and ready-state idempotency.
9. Proof that no vector task and no `edu_video_resource` row is created.
10. Playback success and access-record persistence.
11. Missing mapping, pending, transcoding, and failed playback responses.
12. Media proxy range behavior, content types, traversal rejection, and cross-video rejection.
13. Batch counter and terminal-status transitions for all-success, mixed-result, and all-failed cases.

## Acceptance Criteria

1. A valid ZIP/XLSX import creates one unique local mapping per authoritative knowledge point.
2. Every accepted video reaches either `ready` with a playable HLS URL or `failed` with a visible reason.
3. The old vectorization and personalized recommendation chains receive no tasks or records from this feature.
4. `user_id` plus `knowledge_point_id` returns exactly one ready video and appends one playback record.
5. All source, manifest, and HLS objects live only in the dedicated private bucket.
6. Invalid batches cause no persistent side effects.
