# MinIO Knowledge Video Import Design

## Goal

Add an operator-only CLI that imports existing MinIO video objects into the
knowledge-video pipeline without HTTP authentication, ZIP creation, or archive
size limits. This exception is isolated to the CLI. Existing HTTP validation is
unchanged.

## Command

The executable is `cmd/knowledgevideo-import` and supports:

- `--mapping <path>`: required XLSX mapping file.
- `--source-prefix <prefix>`: required allowed MinIO object prefix.
- `--upload-user-id <id>`: required batch owner.
- `--batch-key <key>`: required idempotency key.
- `--dry-run`: validate and print the proposed import without writes.

Runtime database, Redis, and object-storage settings come from the existing
service configuration and environment override mechanism. Credentials are not
accepted as command-line flags and are never logged.

## Mapping Compatibility

The CLI reads the first worksheet. Its first row must contain exactly `id`,
`name`, and `video_name`. Each row must contain a positive knowledge-point ID,
a non-empty name, and a non-empty object reference in `video_name`.

For compatibility with existing delivery workbooks, `video_name` may be either:

- an object key relative to `--source-prefix`; or
- a basename that resolves to exactly one object below the prefix.

The CLI looks up every ID in `dict_knowledge_point`. Database names are the
source of truth. A mapping name that includes a hierarchy path may be normalized
to its final path component when that component exactly equals the dictionary
name. Other mismatches are rejected and reported with worksheet row numbers.

Supported media extensions remain `.mp4`, `.mov`, `.mkv`, `.avi`, `.webm`, and
`.m4v`. Objects below `__MACOSX`, `.DS_Store`, and `._*` paths are ignored.

## Intentional CLI Exception

The same resolved MinIO object key may appear in multiple rows and may be linked
to multiple knowledge points. The CLI does not duplicate the object. Each row
creates a distinct `edu_knowledge_video` record with its own video ID, batch ID,
HLS prefix, status, and transcode task. All such records may share the same
source object key.

The HTTP ZIP/XLSX endpoint continues to require unique `video_name` values and
one-to-one archive matching. The CLI does not weaken or reuse that validator.

## Validation

Before any write, the CLI reports all issues in one pass:

- invalid or extra XLSX headers;
- invalid IDs, empty names, or empty object references;
- missing/deleted knowledge points;
- dictionary-name mismatches after safe leaf-name normalization;
- missing, ambiguous, unsupported, or out-of-prefix MinIO objects;
- metadata-only objects;
- knowledge points that occur in the mapping without a valid video row.

ZIP size, ZIP CRC, archive basename uniqueness, and unreferenced archive entries
do not apply because the CLI reads objects directly from MinIO. It still reports
the total referenced bytes and the number of unique versus reused source
objects.

## Write Flow

After validation succeeds, the CLI:

1. Rejects an already completed `--batch-key` to make retries idempotent.
2. Reserves one batch ID and one video ID per mapping row.
3. Creates the batch and video records in one database transaction.
4. Stores the original mapping under the batch manifest prefix.
5. Enqueues one existing knowledge-video transcode task per video record.
6. Records enqueue time for successful queue writes.
7. Prints the batch ID, row count, unique source count, reused source count,
   enqueued count, and failures.

No raw video is copied. A queue failure is reported and left recoverable through
the existing stale-pending recovery process.

## Idempotency And Audit

The idempotency key is recorded with the batch. A repeated formal import with the
same key returns the existing batch rather than creating duplicate rows. Dry-run
never reserves IDs or writes state. Logs include object keys and IDs but never
credentials.

## Testing

Unit tests cover header validation, leaf-name normalization, missing and
ambiguous objects, prefix enforcement, supported extensions, metadata filtering,
same-object multi-knowledge-point mappings, dry-run no-write behavior,
idempotency, batch creation, and one task per mapping row. Focused package tests
and `go test ./...` provide final verification.
