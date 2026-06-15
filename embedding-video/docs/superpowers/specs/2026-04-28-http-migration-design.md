# HTTP Migration Design for video-service

## Context

The existing backend project under `nlp-video-project/` exposes its main business capabilities through a gRPC service started by `cmd/rpc`. There is also an HTTP process started by `cmd/api`, but that process is only an HTTP-to-gRPC gateway. Its handlers accept HTTP requests and forward them to the gRPC service through `internal/api/client`.

The integration path has changed. Instead of calling the gRPC service, the Java API team now needs to communicate with the backend over HTTP. The original project under `nlp-video-project/` should not be modified in place for this migration. A new sibling project named `video-service/` should be created and used as the new delivery target.

## Goals

1. Create a new project under `video-service/` instead of editing the original `nlp-video-project/` in place.
2. Replace the old external gRPC integration path with direct HTTP communication.
3. Reuse the original business capabilities as much as possible instead of rewriting logic.
4. Provide a cleaner, more stable REST-style HTTP contract for Java callers.
5. Ship built-in OpenAPI/Swagger documentation for the new HTTP service.
6. Keep authentication out of scope for now.

## Non-Goals

1. Do not redesign or rewrite the core video business logic unless a protocol-layer dependency forces a targeted refactor.
2. Do not remove or break the old gRPC project under `nlp-video-project/`.
3. Do not introduce a new auth system in this migration.
4. Do not perform a full clean-architecture rewrite with broad domain or application refactors.

## Recommended Approach

Use a new project directory `video-service/` that reuses as much of the existing logic and behavior as possible, while replacing the current `HTTP -> gRPC client -> gRPC server -> application` call chain with a direct `HTTP -> application` call chain.

This is the intentionally moderate path:

1. It is more isolated than editing the existing project in place.
2. It preserves the original business capabilities instead of rebuilding them.
3. It still creates a cleaner HTTP boundary for Java consumers.
4. It avoids the cost and risk of a full service rewrite.

## Architecture

The new project should be structured so that protocol concerns are clearly separated from reusable business logic.

### New HTTP Entry

Add a new entry point in `video-service/cmd/httpapi`.

Responsibilities:

1. Load config.
2. Initialize logging.
3. Initialize database, Redis, object storage, and any other infrastructure dependencies needed by the reused application layer.
4. Build the application service graph.
5. Register HTTP routes and Swagger routes.
6. Start and gracefully stop the HTTP server.

This entry point replaces the combined role previously split between `cmd/api` and `cmd/rpc` for external API delivery.

### HTTP Layer

Add a dedicated HTTP transport layer in the new project.

Suggested packages:

1. `internal/http/router`
2. `internal/http/handler`
3. `internal/http/dto`
4. `internal/http/errors`
5. `internal/http/mapper` if response mapping grows beyond handler-sized logic

Responsibilities:

1. Bind path, query, JSON, and multipart request data.
2. Validate HTTP-facing inputs.
3. Call application services directly.
4. Convert application results into stable HTTP DTOs.
5. Map business and infrastructure failures to HTTP status codes and stable API error codes.
6. Expose Swagger/OpenAPI documentation.

### Reused Layers

The following existing capabilities should be reused as much as possible:

1. `internal/application/videoapp`
2. `internal/infrastructure/*`
3. `internal/config`
4. `internal/lifecycle`
5. reusable logging and middleware pieces where they make sense for HTTP

The gRPC-specific transport layer should not remain part of the request path in the new project.

### Old gRPC-Specific Packages

The following packages are not part of the new HTTP request path:

1. `internal/api/client`
2. `internal/rpc/service`
3. `cmd/rpc`

They may still be copied as reference material during migration, but they should not remain runtime dependencies of the new HTTP service.

## API Contract Strategy

The new service should keep the original business capabilities, but present them through a cleaner REST-oriented HTTP contract.

Compatibility strategy:

1. Primary contract: new REST-style routes documented in Swagger.
2. Optional transition support: keep selected legacy route aliases only when they materially reduce migration pain.
3. Business field semantics should remain familiar where possible, even if path naming becomes more HTTP-native.

The new external contract should not directly expose protobuf message types as the public HTTP schema.

## Proposed HTTP Endpoints

### Videos

1. `POST /api/videos`
   - Upload a video through `multipart/form-data`
   - Form fields: `file`, `title`, `description`

2. `GET /api/videos`
   - List videos
   - Query: `type=ALL|RAW|HLS`

3. `PATCH /api/videos/{id}`
   - Update title and description

4. `DELETE /api/videos/{id}`
   - Delete a video

5. `GET /api/videos/{id}/play`
   - Get play information for a video

6. `GET /api/videos/{id}/similar`
   - Get similar videos
   - Query: `limit`

7. `GET /api/videos/{id}/view-count`
   - Get view count

8. `POST /api/videos/{id}/publish`
   - Set publish state
   - JSON body: `{ "is_published": true }`

9. `POST /api/videos/{id}/recommend`
   - Set recommend state
   - JSON body: `{ "is_recommend": true, "user_id": 1, "recommend_level": 1, "recommend_score": 0.95 }`

10. `POST /api/videos/{id}/cover`
   - Upload video cover through `multipart/form-data`

### Transcode Tasks

1. `GET /api/transcode-tasks/{taskId}`
   - Query transcode status

### Recommendations

1. `POST /api/recommendations/by-question`
   - Query recommendations by question content

2. `GET /api/recommendations`
   - List historical recommendations
   - Query: `question_id`, `user_id`, `limit`

### Watch Records

1. `POST /api/watch-records`
   - Report watch behavior

### Questions

1. `GET /api/questions`
   - List questions

2. `GET /api/questions/{id}`
   - Get question detail

## Response Model

The new service should standardize the JSON envelope.

### Success Response

```json
{
  "success": true,
  "data": {}
}
```

### List Response

```json
{
  "success": true,
  "data": {
    "items": [],
    "total": 0
  }
}
```

### Error Response

```json
{
  "success": false,
  "error": {
    "code": "video_not_found",
    "message": "视频不存在"
  }
}
```

The envelope should be consistent across all endpoints, even when the inner data shapes differ.

Business field naming should remain familiar where practical, for example `video_id`, `task_id`, `start_time_sec`, and `end_time_sec`, to reduce downstream migration churn.

## Error Mapping

The new HTTP layer should expose HTTP-native status codes and stable application error codes instead of leaking gRPC status codes to callers.

### HTTP Status Mapping

1. `400 Bad Request`
   - invalid path/query/body/form input
   - missing required fields
   - semantic validation failures such as empty `question_text`

2. `404 Not Found`
   - missing video
   - missing question
   - any other absent resource with a clear identity

3. `409 Conflict`
   - explicit business conflicts if such cases exist during migration or emerge later

4. `500 Internal Server Error`
   - database errors
   - Redis errors
   - object storage errors
   - other unexpected internal failures

### Stable Error Codes

Suggested initial codes:

1. `invalid_argument`
2. `video_not_found`
3. `question_not_found`
4. `upload_failed`
5. `transcode_status_unavailable`
6. `internal_error`

These codes should be independent of old gRPC `codes.*` values.

## OpenAPI and Swagger

The new service should include built-in API documentation suitable for Java team integration.

### Requirements

1. Serve Swagger UI from the HTTP service.
2. Expose the main REST contract, not legacy transition aliases.
3. Provide typed schemas for requests and responses.
4. Include sample payloads where useful for multipart and recommendation-related endpoints.

### Suggested Routes

1. `GET /swagger/index.html`
2. `GET /swagger/*any`
3. optional `GET /openapi.json`

### Documentation Scope

Each documented operation should include:

1. method and path
2. path parameters
3. query parameters
4. JSON or multipart body schema
5. success response schema
6. error response schema

A common Go Swagger/OpenAPI generation approach is sufficient. The priority is usability and speed of delivery, not a perfect tooling stack.

## Migration Strategy

Use an incremental migration to keep the new project runnable as capabilities are moved over.

### Phase 1: Project Bootstrap

1. Create `video-service/`.
2. Add a new HTTP entry point.
3. Add router, handler, DTO, error mapping, and Swagger scaffolding.
4. Reuse configuration and infrastructure initialization patterns from the original project.

### Phase 2: Business Endpoint Migration

Migrate the main HTTP-exposed capabilities one by one so handlers call application logic directly instead of using the gRPC client.

Priority order:

1. video CRUD and play-related endpoints
2. upload and cover upload
3. transcode status
4. recommendation and watch reporting
5. question endpoints
6. static or proxied video/object access support as needed

### Phase 3: gRPC Dependency Removal From New Project

1. Remove reliance on `internal/api/client`.
2. Stop using protobuf messages as public HTTP DTOs.
3. Keep only the reused business logic and infrastructure pieces that are still necessary.

## Testing Strategy

Testing should focus on preserving behavior while changing the transport boundary.

### Unit-Level Focus

1. handler request binding and validation
2. error mapping to HTTP status and error code
3. DTO mapping from application outputs

### Integration-Level Focus

1. upload path through multipart handling
2. video list and metadata update
3. recommendation query responses
4. watch reporting
5. transcode status lookup

### Verification Command Targets

At minimum, the new project should support a backend compile and test sweep equivalent in spirit to the current project checks. The exact commands may differ after the new project layout is created, but the migration should preserve a single-command way to verify the new backend compiles and its tests pass.

## Risks and Mitigations

### Risk: Hidden gRPC Coupling

Some existing handler behavior may depend on gRPC-specific request, response, metadata, or status handling.

Mitigation:

1. move behavior into HTTP-native handlers and DTOs
2. only extract shared logic when truly reusable
3. keep migration incremental to catch coupling early

### Risk: Public Contract Drift During Migration

If path names or response shapes change too aggressively, downstream Java integration cost increases.

Mitigation:

1. preserve business semantics and familiar field names
2. use a single standardized envelope
3. document the new contract clearly in Swagger
4. keep selected aliases only if they materially reduce migration pain

### Risk: Accidental Logic Rewrite

Transport migration can expand into business rewrite if boundaries are not respected.

Mitigation:

1. reuse application and infrastructure code first
2. limit refactors to protocol decoupling needs
3. treat behavior parity as the default unless a contract improvement is explicitly required

## Final Design Summary

Build `video-service/` as a separate HTTP-focused project that reuses the original logic and business capabilities as much as possible, replaces the old gRPC integration path with direct application-layer invocation, exposes a cleaner REST-style contract, and includes Swagger/OpenAPI documentation for Java consumers.
