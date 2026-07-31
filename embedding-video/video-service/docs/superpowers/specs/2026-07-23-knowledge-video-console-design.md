# Knowledge Video Console Design

## Goal

Add a dedicated knowledge-video workspace to `hls-web` for ZIP/XLSX import, upload and transcode progress, knowledge-point browsing, and playback. Add one backend tree endpoint that exposes dictionary knowledge points together with knowledge-video state. Produce a two-video sample ZIP and XLSX that match the existing import contract.

## Scope

### Included

- A third top-level frontend workspace named `知识点视频`.
- ZIP plus XLSX upload with HTTP upload progress.
- Batch polling every two seconds with per-video status and errors.
- A searchable, collapsible knowledge-point tree with video-state badges.
- Playback through the existing knowledge-point playback API and private HLS proxy.
- A backend tree endpoint that reads the authoritative dictionary and joins active knowledge-video mappings.
- A sample archive using knowledge points `9 / 一次函数` and `10 / 二次函数`.

### Excluded

- Editing the authoritative knowledge-point dictionary.
- Replacing an existing active knowledge-video mapping.
- Inferring knowledge-point hierarchy from names.
- Inspecting or semantically classifying the supplied video content.
- Changing the ordinary video upload, recommendation, or vector pipelines.

## Backend API

Add:

```http
GET /api/knowledge-videos/tree
```

The response contains knowledge-point nodes with:

- `id`
- `parent_id`
- `name`
- `children`
- `video`, omitted when no active mapping exists

The optional video summary contains:

- `id`
- `batch_id`
- `video_name`
- `duration`
- `status`: `pending`, `transcoding`, `ready`, or `failed`
- `error_message`

The repository inspects the dictionary schema for an optional `parent_id` column, as it already does for optional `deleted` and naming columns. When `parent_id` is unavailable, the endpoint returns all knowledge points as children of one synthetic root node. It never derives hierarchy from the knowledge-point name.

Only active dictionary rows and active knowledge-video mappings are returned. The endpoint performs batched reads rather than one mapping query per node.

## Frontend Architecture

Add `KnowledgeVideoWorkspace.vue` as a separate top-level workspace. Keep its API and polling logic in a focused `knowledgeVideo` module rather than adding more responsibilities to `VideoWorkspace.vue`.

The selected layout is upload-first:

- Left column: upload controls and current/recent batch progress.
- Right upper area: searchable knowledge-point tree and status filters.
- Right lower area: selected knowledge-point details and HLS player.

The layout stays dense and operational, matching the existing console's white surfaces, dark navigation, teal actions, amber processing state, and restrained 8px radii.

## Upload And Progress Flow

1. The operator selects one ZIP archive and one XLSX mapping and enters a positive upload user ID.
2. The frontend submits multipart form data to `POST /api/admin/knowledge-videos/batches` through `XMLHttpRequest` so upload progress is observable.
3. During transfer, the left panel shows byte-based HTTP progress.
4. After `202 Accepted`, the frontend stores the batch ID and polls `GET /api/admin/knowledge-videos/batches/{batchId}` every two seconds.
5. The batch panel shows total, ready, failed, overall percent, and each video's status/error.
6. Polling stops for `completed`, `partial_failed`, or `failed`, and on workspace unmount.
7. Terminal completion refreshes the knowledge tree. The first ready node may be selected automatically when no node is selected.

Validation issues retain backend `row`, `field`, and `message` details so the operator can correct the XLSX in one pass.

## Knowledge Tree And Playback Flow

The tree supports:

- Search by knowledge-point ID or name.
- Expand/collapse for parent nodes.
- Filters for all, unassigned, processing, ready, and failed.
- Status badges on leaf nodes.

Selecting a node updates the detail pane. A ready mapped node calls:

```http
GET /api/knowledge-points/{knowledgePointId}/video?user_id={userId}
```

The returned `playback_url` is passed to the existing `HlsPlayer`. Pending and transcoding nodes show progress state without making a playback request. Failed nodes show the persisted transcode error. Unassigned nodes show that no video has been imported.

## Sample Import Package

The delivered ZIP contains only:

- `一次函数.mp4`
- `二次函数.mp4`

The delivered XLSX first worksheet contains the exact required header and rows:

| id | name | video_name |
|---:|---|---|
| 9 | 一次函数 | 一次函数.mp4 |
| 10 | 二次函数 | 二次函数.mp4 |

The source files are copied and renamed without transcoding or content inspection.

## Error Handling

- Multipart and XLSX validation errors remain `400` and display structured issues.
- Existing active mappings display as a conflict and do not start progress polling.
- Network or upload interruption leaves the form retryable.
- A batch-level terminal failure remains visible with failed rows and messages.
- `VIDEO_NOT_READY` and `VIDEO_TRANSCODE_FAILED` are rendered as state, not generic frontend failures.
- Tree fetch failures do not erase the last successfully loaded tree.

## Testing

Backend tests cover:

- Dictionary schemas with and without `parent_id` and `deleted`.
- Tree construction, orphan handling, stable ordering, and mapped video summaries.
- HTTP response contract and error mapping.

Frontend tests cover:

- Workspace navigation registration.
- Tree response normalization and filtering.
- Multipart upload progress callbacks.
- Polling continuation and terminal stop conditions.
- Playback requests only for ready nodes.
- Required layout and responsive constraints.

Artifact verification covers:

- XLSX values and exact headers.
- A rendered XLSX visual pass.
- ZIP entry names and source file sizes.
- Validation of the generated ZIP/XLSX pair with the backend validator.
