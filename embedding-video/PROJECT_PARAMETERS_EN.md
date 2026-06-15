# Project Parameter Reference

## Table of Contents

- [1. Document Scope](#1-document-scope)
- [2. Parameter Summary Table](#2-parameter-summary-table)
- [3. Runtime Configuration Parameters](#3-runtime-configuration-parameters)
  - [3.1 Top-Level Config](#31-top-level-config)
  - [3.2 HTTPConfig](#32-httpconfig)
  - [3.3 CORSConfig](#33-corsconfig)
  - [3.4 GRPCConfig](#34-grpcconfig)
  - [3.5 VideoConfig](#35-videoconfig)
  - [3.6 StorageConfig](#36-storageconfig)
  - [3.7 FFmpegConfig](#37-ffmpegconfig)
  - [3.8 RedisConfig](#38-redisconfig)
  - [3.9 RedisKeysConfig](#39-rediskeysconfig)
  - [3.10 PostgresConfig](#310-postgresconfig)
  - [3.11 RustFSConfig](#311-rustfsconfig)
  - [3.12 TransConfig](#312-transconfig)
  - [3.13 VectorWorkerConfig](#313-vectorworkerconfig)
  - [3.14 VectorStageWorkersConfig](#314-vectorstageworkersconfig)
  - [3.15 WorkerPoolsConfig](#315-workerpoolsconfig)
  - [3.16 EmbeddingConfig](#316-embeddingconfig)
  - [3.17 ASRConfig](#317-asrconfig)
  - [3.18 AIConfig](#318-aiconfig)
- [4. Environment Variables](#4-environment-variables)
- [5. HTTP API Parameters](#5-http-api-parameters)
  - [5.1 Common Response Shape](#51-common-response-shape)
  - [5.2 Health Check and Static Entry Points](#52-health-check-and-static-entry-points)
  - [5.3 Upload API Parameters](#53-upload-api-parameters)
  - [5.4 Video Management API Parameters](#54-video-management-api-parameters)
  - [5.5 Playback and Task Status API Parameters](#55-playback-and-task-status-api-parameters)
  - [5.6 Recommendation API Parameters](#56-recommendation-api-parameters)
  - [5.7 Question Bank API Parameters](#57-question-bank-api-parameters)
- [6. Worker / Internal Default Parameters](#6-worker--internal-default-parameters)
  - [6.1 HTTP Runtime Defaults](#61-http-runtime-defaults)
  - [6.2 Transcode Worker Defaults](#62-transcode-worker-defaults)
  - [6.3 Vector Worker Defaults](#63-vector-worker-defaults)
  - [6.4 Hierarchical Segmentation Internal Parameters](#64-hierarchical-segmentation-internal-parameters)
  - [6.5 Tail Alignment Internal Parameters](#65-tail-alignment-internal-parameters)
- [7. Usage Notes](#7-usage-notes)

## 1. Document Scope

This document summarizes the major parameters visible in the repository, with a primary focus on the `video-service/` project. It covers four categories:

1. Runtime configuration parameters
2. Environment variables
3. HTTP API parameters
4. Worker and internal default parameters

Notes:

1. This document focuses on `video-service/` because it is the currently recommended deployment target and the recommended downstream video service.
2. It does not attempt to enumerate every database column or every local variable in the codebase.
3. Here, a “parameter” means a field that is externally configurable, accepted by an API, has a runtime default, or otherwise influences system behavior.

## 2. Parameter Summary Table

| Parameter Category | Source Location | Typical Examples | Purpose |
|---|---|---|---|
| Runtime configuration | `configs/video.yml`, `configs/video_prod.yml`, `internal/config/types.go` | `HTTP.Addr`, `Storage.MediaRoutePrefix`, `RedisKeys.TranscodeQueue`, `VectorWorker.Mode` | Controls listening, CORS, paths, queues, transcoding, vectorization, and external dependencies |
| Environment variables | `internal/config/loader.go`, `internal/http/app/app.go`, worker initialization logic | `HTTP_ADDR`, `CONFIG_FILE`, `DASHSCOPE_API_KEY`, `RUSTFS_ACCESS_KEY` | Overrides configuration, selects the config file, or injects sensitive values |
| HTTP API parameters | `internal/http/router/router.go`, `handler/*.go`, `dto/*.go` | `question_id`, `limit`, `file`, `is_published` | Defines API request behavior |
| Worker internal defaults | `internal/worker/*`, `tasks/*` | `taskTimeout`, `maxRetryTimes`, `boundaryStartLookBackSec` | Controls background execution rhythm, retries, segmentation, and boundary refinement |

## 3. Runtime Configuration Parameters

The runtime configuration structures are defined in:

- `video-service/internal/config/types.go`

The main configuration files are:

- `video-service/configs/video.yml`
- `video-service/configs/video_prod.yml`

Default loading rules:

1. macOS and Windows load `configs/video.yml` by default.
2. Other environments load `configs/video_prod.yml` by default.
3. `CONFIG_FILE` and `VIDEO_CONFIG_FILE` can both override the default config path.
4. When both are set, `CONFIG_FILE` wins.
5. `cmd/httpapi` and `cmd/worker` call `config.EnsureProjectRoot()` first, so relative config paths are anchored to `video-service/`.

### 3.1 Top-Level Config

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `Name` | `string` | `video-rpc` | Service name, mainly for identification and logging |
| `Host` | `string` | `localhost` | Legacy host field; the current HTTP service does not listen directly based on this value |
| `Port` | `int` | `9090` | Legacy port field; the current HTTP service does not listen directly based on this value |
| `HTTP` | `HTTPConfig` | see below | HTTP API listening, logging, slow-request, and CORS configuration |
| `GRPC` | `GRPCConfig` | see below | gRPC-related configuration, mainly kept for compatibility with the historical project |
| `Video` | `VideoConfig` | see below | Local video path configuration |
| `FFmpeg` | `FFmpegConfig` | see below | Transcoding, snapshot, and audio extraction configuration |
| `Storage` | `StorageConfig` | see below | Object key prefixes, public URL prefixes, media proxy route, and vector temp path configuration |
| `Redis` | `RedisConfig` | see below | Redis connection configuration |
| `RedisKeys` | `RedisKeysConfig` | see below | Redis Stream queue, status, and runtime counter key configuration |
| `Postgres` | `PostgresConfig` | see below | PostgreSQL connection and pool configuration |
| `RustFS` | `RustFSConfig` | see below | Object storage connection configuration |
| `Transcode` | `TransConfig` | see below | Transcode worker configuration |
| `VectorWorker` | `VectorWorkerConfig` | see below | Vector worker configuration |
| `VectorStageWorkers` | `VectorStageWorkersConfig` | see below | Consumer-count configuration for the four hierarchical vector Redis stages |
| `WorkerPools` | `WorkerPoolsConfig` | see below | ants pool concurrency configuration used inside the vector worker |
| `embedding` | `EmbeddingConfig` | see below | Embedding service configuration |
| `asr` | `ASRConfig` | see below | Speech recognition service configuration |
| `AI` | `AIConfig` | see below | Base AI parameters, such as embedding dimension |

### 3.2 HTTPConfig

| Parameter | Type | Example | Default | Purpose |
|---|---|---|---|---|
| `Addr` | `string` | `:8081` | `:8081` | HTTP API listening address; can be overridden by `HTTP_ADDR` |
| `ShutdownTimeoutSec` | `int` | `30` | `30` | Graceful HTTP shutdown timeout in seconds |
| `LogDir` | `string` | `logs` | `logs` | Log directory for the HTTP API and worker |
| `SlowRequestMs` | `int` | `1000` | `1000` | Slow request logging threshold in milliseconds |
| `CORS` | `CORSConfig` | see below | see below | Browser CORS response header configuration |

### 3.3 CORSConfig

| Parameter | Type | Example | Default | Purpose |
|---|---|---|---|---|
| `AllowOrigin` | `string` | `*` | `*` | `Access-Control-Allow-Origin` |
| `AllowMethods` | `string` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` | Same as example | `Access-Control-Allow-Methods` |
| `AllowHeaders` | `string` | `Origin, Content-Type, Accept, Authorization, X-Requested-With` | Same as example | `Access-Control-Allow-Headers` |
| `ExposeHeaders` | `string` | `Content-Length, Content-Type` | Same as example | `Access-Control-Expose-Headers` |
| `MaxAge` | `string` | `86400` | `86400` | `Access-Control-Max-Age` |

### 3.4 GRPCConfig

| Parameter | Type | Purpose |
|---|---|---|
| `MaxMsgSize` | `int` | Maximum gRPC message size |
| `KeepaliveTime` | `int` | gRPC keepalive interval |
| `KeepaliveTimeout` | `int` | gRPC keepalive timeout |
| `MaxConnectionAge` | `int` | Maximum lifetime of a single connection |
| `MaxConnectionAgeGrace` | `int` | Grace period after a connection reaches max age |

Note: in `video-service/`, the main service path is HTTP. These fields are retained mainly for structural compatibility.

### 3.5 VideoConfig

| Parameter | Type | Example | Default | Purpose |
|---|---|---|---|---|
| `RawPath` | `string` | `./storage/videos/raw` | `os.TempDir()/nlp-video-project/tmp/raw` | Local raw video path configuration |
| `HlsPath` | `string` | `./storage/videos/hls` | `os.TempDir()/nlp-video-project/tmp/hls` | Local HLS output path configuration |

### 3.6 StorageConfig

| Parameter | Type | Example | Default | Purpose |
|---|---|---|---|---|
| `RawObjectPrefix` | `string` | `raw` | `raw` | Object storage key prefix for raw videos |
| `HLSObjectPrefix` | `string` | `hls` | `hls` | Object storage key prefix for HLS outputs |
| `MediaRoutePrefix` | `string` | `/videos` | `/videos` | Object proxy route prefix; if changed, `/videos` remains registered as a compatibility route |
| `RawURLPrefix` | `string` | `/videos/raw` | `/videos/raw` | Raw video URL prefix returned to callers |
| `HLSURLPrefix` | `string` | `/videos/hls` | `/videos/hls` | HLS URL prefix returned to callers |
| `CoverURLPrefix` | `string` | `/videos` | `/videos` | Cover URL prefix returned to callers |
| `VectorTempPath` | `string` | `./storage/tmp/video_vectorize` | `os.TempDir()/nlp-video-project/tmp/video_vectorize` | Temporary file path for the vector worker |

### 3.7 FFmpegConfig

#### 3.7.1 Top-Level Fields

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `UseDocker` | `bool` | `true` | Whether Docker-based ffmpeg execution is allowed |
| `DockerImage` | `string` | `jrottenberg/ffmpeg` | Docker image used in ffmpeg Docker mode |
| `HLS` | `FFmpegHLSConfig` | see below | HLS output configuration |
| `Fast` | `FFmpegFastConfig` | see below | Fast transcoding configuration |
| `Cover` | `FFmpegCoverConfig` | see below | Cover image snapshot configuration |
| `Audio` | `FFmpegAudioConfig` | see below | Audio extraction configuration |

#### 3.7.2 FFmpegHLSConfig

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `Time` | `int` | `6` | Duration of each HLS ts segment |
| `ListSize` | `int` | `0` | m3u8 list size; `0` usually means unlimited/full playlist |
| `MasterName` | `string` | `master.m3u8` | Master playlist filename; defaults to `master.m3u8` when empty |
| `SegmentPattern` | `string` | `v0_%03d.ts` | ts segment naming pattern |

#### 3.7.3 FFmpegFastConfig

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `ScaleW` | `int` | `1280` | Output width |
| `ScaleH` | `int` | `720` | Output height |
| `Preset` | `string` | `ultrafast` | Encoding speed/compression preset |
| `Crf` | `int` | `28` | Compression quality parameter; lower means higher quality |
| `PixFmt` | `string` | `yuv420p` | Output pixel format |
| `AudioBitrate` | `string` | `96k` | Audio bitrate |
| `AudioChannels` | `int` | `2` | Audio channel count |
| `PadToFit` | `bool` | `true` | Whether to pad video to fit target dimensions |

#### 3.7.4 FFmpegCoverConfig

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `SeekSec` | `int` | `2` | Preferred timestamp for cover extraction |
| `FallbackSeekSec` | `int` | `0` | Fallback timestamp if the preferred frame fails |
| `Quality` | `int` | `2` | Output image quality |

#### 3.7.5 FFmpegAudioConfig

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `SampleRate` | `int` | `16000` | Audio extraction sample rate |
| `Channels` | `int` | `1` | Audio extraction channel count |

### 3.8 RedisConfig

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `Addr` | `string` | `localhost:6379` | Redis address |
| `Password` | `string` | `""` | Redis password |
| `DB` | `int` | `0` | Redis DB index |

### 3.9 RedisKeysConfig

| Parameter | Type | Example | Default | Purpose |
|---|---|---|---|---|
| `TranscodeQueue` | `string` | `video:transcode:queue` | `video:transcode:queue` | Redis Stream key for transcode tasks |
| `VectorizeQueue` | `string` | `video:vectorize:queue` | `video:vectorize:queue` | Redis Stream key for vectorization tasks |
| `VectorPrepareQueue` | `string` | `video:vector:prepare` | `video:vector:prepare` | Redis Stream key for the hierarchical vector prepare stage |
| `VectorCoarseQueue` | `string` | `video:vector:coarse` | `video:vector:coarse` | Redis Stream key for the hierarchical vector coarse stage |
| `VectorRefineQueue` | `string` | `video:vector:refine` | `video:vector:refine` | Redis Stream key for the hierarchical vector refine stage |
| `VectorFinalizeQueue` | `string` | `video:vector:finalize` | `video:vector:finalize` | Redis Stream key for the hierarchical vector finalize stage |
| `VideoReactionQueue` | `string` | `video:reaction:queue` | `video:reaction:queue` | Redis key for the video reaction async queue |
| `VideoReactionCounts` | `string` | `video:reaction:counts:` | `video:reaction:counts:` | Prefix for video reaction count keys |
| `VideoReactionUser` | `string` | `video:reaction:user:` | `video:reaction:user:` | Prefix for per-user video reaction state keys |
| `TranscodeStatus` | `string` | `video:transcode:status:` | `video:transcode:status:` | Prefix for transcode task status keys |
| `RuntimeActiveCounter` | `string` | `video:runtime:active:` | `video:runtime:active:` | Prefix for runtime active counter keys |

### 3.10 PostgresConfig

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `DSN` | `string` | `host=... user=...` | PostgreSQL DSN |
| `MaxOpenConns` | `int` | `20` | Maximum number of open connections |
| `MaxIdleConns` | `int` | `10` | Maximum number of idle connections |
| `ConnMaxLifetime` | `int` | `300` | Maximum connection lifetime in seconds |
| `ConnMaxIdleTime` | `int` | `60` | Maximum idle connection retention time in seconds |

### 3.11 RustFSConfig

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `Endpoint` | `string` | `localhost:9000` | Object storage endpoint |
| `AccessKey` | `string` | `minioadmin` | Access key |
| `SecretKey` | `string` | `minioadmin` | Secret key |
| `Bucket` | `string` | `hengshui-tablet-cloud-drive` | Bucket name |
| `UseSSL` | `bool` | `false` | Whether HTTPS is used |

### 3.12 TransConfig

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `WorkerCount` | `int` | `2` | Transcode worker concurrency |
| `QueueSize` | `int` | `50` | Reserved configuration slot; not the main queue controller in current logic |
| `Mode` | `string` | `fast` | Transcoding mode |
| `TaskTimeoutMinutes` | `int` | `30` | Timeout per transcode task |
| `ShutdownTimeoutSec` | `int` | `120` | Allowed wait time when shutting down workers |

### 3.13 VectorWorkerConfig

#### 3.13.1 Basic Mode Parameters

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `Mode` | `string` | `hierarchical` | Vectorization mode |
| `CoarseSegmentSec` | `int` | `30` | Coarse segmentation length |
| `RefineMinSegmentSec` | `int` | `10` | Minimum refined segment length |
| `RefineMaxSegmentSec` | `int` | `60` | Maximum refined segment length |

#### 3.13.2 LLM Parameters

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `LLMModel` | `string` | `qwen-plus` | Model used for hierarchical content segmentation |
| `LLMTimeoutMinutes` | `int` | `2` | Timeout for LLM calls |

#### 3.13.3 Tail Alignment Parameters

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `TailAlignmentEnabled` | `bool` | `true` | Whether tail alignment is enabled |
| `TailAlignmentConfigured` | `bool` | `true` | Whether tail alignment was explicitly configured |
| `TailAlignmentMaxExtendSec` | `int` | `3` | Maximum number of seconds to extend the tail |
| `TailAlignmentProbeStepSec` | `int` | `1` | Probe step size |
| `TailAlignmentMaxOverlapSec` | `int` | `6` | Maximum overlap allowed with the next segment |

#### 3.13.4 ASR / Concurrency Parameters

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `SegmentWindowSec` | `int` | `30` | Fixed window length for non-hierarchical mode |
| `SegmentStepSec` | `int` | `30` | Fixed window step size for non-hierarchical mode |
| `ASRWorkers` | `int` | `30` | ASR worker count |
| `CoarseWorkers` | `int` | `60` | Coarse-stage related worker parameter; currently also affects video-level worker count |
| `EmbedBatch` | `int` | `10` | Embedding batch size |
| `SampleCount` | `int` | `6` | Number of sampled windows in sample mode |
| `SampleDurSec` | `int` | `30` | Duration of each sampled window |
| `TaskTimeoutMinutes` | `int` | `30` | Timeout per vectorization task |
| `ShutdownTimeoutSec` | `int` | `120` | Allowed wait time when shutting down vector workers |

### 3.14 VectorStageWorkersConfig

`VectorStageWorkers` controls how many consumers are started for each Redis-backed hierarchical vector stage. `full` and `sample` modes do not use these stage queues.

| Parameter | Type | Example | Default | Purpose |
|---|---|---|---|---|
| `Prepare` | `int` | `1` | `1` | Consumer count for the `video:vector:prepare` stage |
| `Coarse` | `int` | `2` | `2` | Consumer count for the `video:vector:coarse` stage |
| `Refine` | `int` | `2` | `2` | Consumer count for the `video:vector:refine` stage |
| `Finalize` | `int` | `1` | `1` | Consumer count for the `video:vector:finalize` stage |

### 3.15 WorkerPoolsConfig

`WorkerPools` is a name-keyed concurrency pool map used inside a single stage by ants pools; it does not create additional Redis queues. The currently used keys are:

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `vector.coarse.Size` | `int` | `12` / `60` | Pool size for the vector coarse stage |
| `vector.sample_asr.Size` | `int` | `8` / `30` | Pool size for the vector sample ASR stage |
| `vector.refine_asr.Size` | `int` | `8` / `30` | Pool size for the vector refine ASR stage |

If a pool is missing or `Size <= 0`, the code falls back to the related concurrency setting in `VectorWorker`.

### 3.16 EmbeddingConfig

| Parameter | Type | Purpose |
|---|---|---|
| `Options.Model` | `string` | Embedding model name |
| `BaseURL` | `string` | Embedding service endpoint |
| `APIKey` | `string` | Embedding service API key; production deployments should inject it through environment variables |

The recommendation embedding client resolves the API key in this order: `DASHSCOPE_API_KEY`, `OPENAI_API_KEY`, `EMBEDDING_API_KEY`, then `embedding.api-key`.

### 3.17 ASRConfig

| Parameter | Type | Purpose |
|---|---|---|
| `Options.Model` | `string` | Standard ASR model name |
| `Options.WSModel` | `string` | WebSocket real-time ASR model name |
| `Options.WSFallbacks` | `[]string` | WebSocket ASR fallback model list, tried in order when the preferred model quota is unavailable |
| `BaseURL` | `string` | ASR service endpoint |
| `WSURL` | `string` | ASR WebSocket endpoint; can also be overridden by `ASR_WS_URL` |
| `APIKey` | `string` | ASR service API key; production deployments should inject it through environment variables |

The vector worker AI client resolves the API key in this order: `DASHSCOPE_API_KEY`, `OPENAI_API_KEY`, `ASR_API_KEY`, `asr.api-key`, then `embedding.api-key`.

### 3.18 AIConfig

| Parameter | Type | Example | Default | Purpose |
|---|---|---|---|---|
| `EmbeddingDim` | `int` | `1536` | `1536` | Embedding vector dimension; used by local fallback embeddings and by the vector worker before persisted embeddings are written |

## 4. Environment Variables

The project currently uses the following environment variables explicitly:

| Environment Variable | Source Location | Purpose |
|---|---|---|
| `HTTP_ADDR` | `internal/http/app/app.go` | Overrides the HTTP listening address; default is `:8081` |
| `CONFIG_FILE` | `internal/config/loader.go` | Overrides the default config file path; has higher priority than `VIDEO_CONFIG_FILE` |
| `VIDEO_CONFIG_FILE` | `internal/config/loader.go` | Overrides the default config file path |
| `RUSTFS_ACCESS_KEY` | `internal/http/app/app.go`, worker initialization | Fallback access key when config file does not provide one |
| `RUSTFS_SECRET_KEY` | `internal/http/app/app.go`, worker initialization | Fallback secret key when config file does not provide one |
| `DASHSCOPE_API_KEY` | embedding client, vector worker AI client | DashScope / Bailian-compatible API key |
| `OPENAI_API_KEY` | embedding client, vector worker AI client | Fallback OpenAI-compatible API key |
| `EMBEDDING_API_KEY` | embedding client | Fallback API key for recommendation embedding |
| `DASHSCOPE_BASE_URL` | vector worker AI client | OpenAI-compatible base URL used by the vector worker |
| `OPENAI_BASE_URL` | vector worker AI client | Fallback OpenAI-compatible base URL |
| `ASR_API_KEY` | vector worker AI client | Fallback ASR API key |
| `ASR_BASE_URL` | vector worker AI client | ASR HTTP base URL |
| `ASR_WS_URL` | `internal/config/defaults.go`, vector worker AI client | ASR WebSocket URL |
| `ASR_WS_MODEL` | vector worker AI client | Preferred ASR WebSocket model |
| `EMBED_MODEL` | vector worker AI client | Embedding model name |
| `UPLOAD_BENCH_BASE_URL` | `tools/upload_bench` | Target service URL for the upload benchmark tool |
| `SOURCE_DSN` | `tools/db_migrate_except_video_tables` | Source database DSN for the migration tool |
| `TARGET_DSN` | `tools/db_migrate_except_video_tables` | Target database DSN for the migration tool |

### 4.1 `HTTP_ADDR`

- Type: `string`
- Default: `:8081`
- Example: `HTTP_ADDR=:8081`
- Purpose: Overrides the HTTP API listening address

### 4.2 `CONFIG_FILE` / `VIDEO_CONFIG_FILE`

- Type: `string`
- Purpose: Overrides the default configuration file path
- Priority: `CONFIG_FILE` is higher than `VIDEO_CONFIG_FILE`; both are higher than the built-in OS-based config file selection logic

### 4.3 `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`

- Type: `string`
- Purpose: Fallback source of object storage credentials
- Use case: Inject sensitive values from environment variables instead of config files

### 4.4 AI Service Environment Variables

- `DASHSCOPE_API_KEY`, `OPENAI_API_KEY`, and `EMBEDDING_API_KEY` are used by the recommendation embedding client.
- `DASHSCOPE_API_KEY`, `OPENAI_API_KEY`, and `ASR_API_KEY` are used by the vector worker ASR, LLM, and embedding client.
- `DASHSCOPE_BASE_URL` and `OPENAI_BASE_URL` override the vector worker OpenAI-compatible base URL.
- `ASR_BASE_URL`, `ASR_WS_URL`, and `ASR_WS_MODEL` control the ASR service endpoint and WebSocket model.
- `EMBED_MODEL` controls the embedding model used by the vector worker.

Production deployments should inject API keys through environment variables or a secret manager, not commit real secrets to config files.

### 4.5 Tool Script Environment Variables

- `UPLOAD_BENCH_BASE_URL`: target service URL for the upload benchmark tool; if missing, the tool derives a default from `HTTP_ADDR`.
- `SOURCE_DSN`, `TARGET_DSN`: source and target PostgreSQL DSNs for the database migration tool. They can also be provided explicitly with `-source-dsn` and `-target-dsn`.

## 5. HTTP API Parameters

The main routes are registered in:

- `video-service/internal/http/router/router.go`

The main DTOs are defined in:

- `internal/http/dto/common.go`
- `internal/http/dto/upload.go`
- `internal/http/dto/video.go`
- `internal/http/dto/recommend.go`

### 5.1 Common Response Shape

#### Success Response

```json
{
  "success": true,
  "data": { ... }
}
```

Fields:

| Field | Type | Purpose |
|---|---|---|
| `success` | `bool` | Whether the request succeeded |
| `data` | `any` | Business payload |

#### Error Response

```json
{
  "success": false,
  "error": {
    "code": "...",
    "message": "..."
  }
}
```

Fields:

| Field | Type | Purpose |
|---|---|---|
| `success` | `bool` | Whether the request succeeded |
| `error.code` | `string` | Error code |
| `error.message` | `string` | Error message |

### 5.2 Health Check and Static Entry Points

#### `GET /healthz`
#### `GET /api/healthz`

- No request parameters
- Returns `{ "status": "ok" }`

#### `GET /swagger/*any`

- Path parameter: `*any`
- Purpose: Access Swagger page resources

#### `GET /api/system/metrics`

- No request parameters
- Purpose: Query system runtime metrics

#### `GET /videos/*filepath`

- Path parameter: `*filepath`
- Purpose: Proxy access to video resources stored in object storage
- Note: the actual media proxy route prefix is controlled by `Storage.MediaRoutePrefix` and defaults to `/videos`; if another prefix is configured, `/videos/*filepath` remains registered as a compatibility route

### 5.3 Upload API Parameters

#### `POST /api/videos`

Request type: `multipart/form-data`

Note: this endpoint is for regular multipart uploads. For large files or resumable uploads, use the chunked upload endpoints below.

Form fields:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `file` | `file` | Yes | Uploaded video file |
| `title` | `string` | No | Video title |
| `description` | `string` | No | Video description |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `video_id` | `uint64` | Video ID |
| `task_id` | `string` | Transcode task ID |
| `raw_url` | `string` | Raw video access URL |
| `hls_url` | `string` | HLS playback URL |
| `file_name` | `string` | Stored file name |

#### `POST /api/videos/archive`

Request type: `multipart/form-data`

Note: this compatibility endpoint accepts a ZIP archive directly. The backend first writes the ZIP to disk and then streams entries from the local ZIP file, so it no longer reads the whole archive into memory. For resumable uploads, use the ZIP chunked upload endpoints.

Form fields:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `file` | `file` | Yes | Zip archive containing video files |
| `description` | `string` | No | Description written to each uploaded video |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `total` | `int` | Total candidate files detected in the archive |
| `uploaded` | `int` | Number of successfully uploaded videos |
| `failed` | `int` | Number of failed uploads |
| `skipped` | `int` | Number of skipped files |
| `videos` | `[]UploadVideoData` | Successfully uploaded videos |
| `errors` | `[]UploadArchiveError` | Failed files and their errors |
| `skipped_files` | `[]string` | Skipped file names |

#### `POST /api/videos/uploads`

Request type: `application/json`

Purpose: create a chunked upload session for a single video.

Request fields:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `file_name` | `string` | Yes | Original video file name |
| `content_type` | `string` | No | File MIME type |
| `title` | `string` | No | Video title |
| `description` | `string` | No | Video description |
| `file_size` | `int64` | Yes | Total file size in bytes; must be greater than 0 |
| `chunk_size` | `int64` | Yes | Chunk size in bytes; must be greater than 0 |
| `total_chunks` | `int` | Yes | Total chunk count; must equal `ceil(file_size / chunk_size)` |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `upload_id` | `string` | Chunked upload session ID |
| `file_name` | `string` | Original file name |
| `file_size` | `int64` | Total file size in bytes |
| `chunk_size` | `int64` | Chunk size in bytes |
| `total_chunks` | `int` | Total chunk count |
| `uploaded_chunks` | `[]int` | Uploaded chunk indexes that passed size validation |
| `completed` | `bool` | Whether all chunks have been uploaded |

#### `POST /api/videos/archive/uploads`

Request type: `application/json`

Purpose: create a chunked upload session for ZIP batch import. The request and response fields are the same as `POST /api/videos/uploads`, but `file_name` must be a `.zip` file. `description` is written to every successfully imported video in the archive.

#### `PUT /api/videos/uploads/:uploadId/chunks/:chunkIndex`

Request type: raw binary body

Purpose: upload one chunk. Single-video uploads and ZIP batch uploads share this endpoint.

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `uploadId` | `string` | Yes | Chunked upload session ID |
| `chunkIndex` | `int` | Yes | Chunk index, starting from `0` |

Request body:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| request body | `binary` | Yes | Current chunk content |

Note: every chunk except the last one must have exactly the configured `chunk_size`. The last chunk must match the remaining file size. Chunks with invalid sizes are not counted as uploaded.

Response fields: same as `ChunkedUploadData`, including `upload_id`, `file_name`, `file_size`, `chunk_size`, `total_chunks`, `uploaded_chunks`, and `completed`.

#### `GET /api/videos/uploads/:uploadId`

Purpose: query chunked upload status. Single-video uploads and ZIP batch uploads share this endpoint.

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `uploadId` | `string` | Yes | Chunked upload session ID |

Response fields: same as `ChunkedUploadData`.

#### `POST /api/videos/uploads/:uploadId/complete`

Purpose: complete a single-video chunked upload. The backend validates all chunks, merges them into a local file, uploads the raw object, creates the video record, and enqueues transcoding.

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `uploadId` | `string` | Yes | Chunked upload session ID |

Response fields: same as `POST /api/videos`, including `video_id`, `task_id`, `raw_url`, `hls_url`, and `file_name`.

#### `POST /api/videos/archive/uploads/:uploadId/complete`

Purpose: complete a ZIP batch chunked upload. The backend validates and merges the ZIP file, then streams entries from the local ZIP file and imports supported videos.

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `uploadId` | `string` | Yes | Chunked upload session ID |

Response fields: same as `POST /api/videos/archive`, including `total`, `uploaded`, `failed`, `skipped`, `videos`, `errors`, and `skipped_files`.

#### `POST /api/videos/:id/cover`

Request type: `multipart/form-data`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

Form fields:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `file` | `file` | Yes | Uploaded cover file |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `video_id` | `uint64` | Video ID |
| `cover_url` | `string` | Cover access URL |

### 5.4 Video Management API Parameters

#### `GET /api/videos`

Query parameters:

| Parameter | Type | Required | Default | Purpose |
|---|---|---|---|---|
| `type` | `string` | No | `ALL` | List filter type: `ALL`, `RAW`, or `HLS` |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `videos` | `[]VideoItem` | Video list |
| `total` | `int` | Total count |
| `type` | `string` | Current filter type |

#### `PATCH /api/videos/:id`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

JSON body:

| Parameter | Type | Required | Constraint | Purpose |
|---|---|---|---|---|
| `title` | `string` | Yes | `required,max=200` | Title |
| `description` | `string` | No | `max=5000` | Description |

#### `DELETE /api/videos/:id`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

#### `POST /api/videos/:id/publish`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

JSON body:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `is_published` | `bool` | Yes | Whether the video should be published |

#### `POST /api/videos/:id/recommend`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

JSON body:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `is_recommend` | `bool` | Yes | Whether the video should be marked as recommended |
| `user_id` | `uint64` | No | Operator user ID |
| `recommend_level` | `int16` | No | Recommendation level |
| `recommend_score` | `float64` | No | Recommendation score |

#### `POST /api/videos/:id/reactions`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

JSON body:

| Parameter | Type | Required | Constraint | Purpose |
|---|---|---|---|---|
| `user_id` | `uint64` | Yes | `> 0` | User ID |
| `reaction_type` | `string` | Yes | `like`, `double_like`, `dislike` | Video reaction type; submitting the same reaction again cancels it |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `video_id` | `uint64` | Video ID |
| `user_id` | `uint64` | User ID |
| `reaction_type` | `string` | Submitted reaction type |
| `active` | `bool` | Whether the reaction is currently active |
| `like_count` | `int64` | Like count |
| `double_like_count` | `int64` | Double-like count |
| `updated` | `bool` | Whether the update was applied |

#### `GET /api/videos/:id/reaction-counts`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `video_id` | `uint64` | Video ID |
| `like_count` | `int64` | Like count |
| `double_like_count` | `int64` | Double-like count |

### 5.5 Playback and Task Status API Parameters

#### `GET /api/videos/:id/play`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `play_url` | `string` | Actual playback URL |
| `video` | `VideoItem` | Video information |

#### `GET /api/videos/:id/similar`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

Query parameters:

| Parameter | Type | Required | Default | Purpose |
|---|---|---|---|---|
| `limit` | `int` | No | `6` | Number of similar videos to return |

#### `GET /api/videos/:id/view-count`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video ID |

#### `GET /api/transcode-tasks/:taskId`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `taskId` | `string` | Yes | Transcode task ID |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `task_id` | `string` | Task ID |
| `status` | `string` | Current task status |
| `hls_url` | `string` | HLS URL |

### 5.6 Recommendation API Parameters

#### `POST /api/recommendations/by-question`

JSON body:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `question_id` | `uint64` | No | Question ID in the question bank |
| `question_text` | `string` | No, but required when `question_id` is absent | Question text |
| `user_id` | `uint64` | No | User ID |
| `limit` | `int` | No | Recommendation limit; handler default is `3`, service-level upper cap is `50` |

#### `GET /api/recommendations`

Query parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `question_id` | `uint64` | Yes | Question ID |
| `user_id` | `uint64` | No | User ID |
| `limit` | `int` | No | Number of results to return |

#### `POST /api/watch-records`

JSON body:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `question_id` | `uint64` | No | Related question ID |
| `user_id` | `uint64` | No | User ID |
| `video_segment_id` | `uint64` | Yes | Video segment ID |
| `is_watched` | `bool` | No | Whether the segment has been watched |
| `watch_duration` | `int` | No | Watch duration; must be `>= 0` |

### 5.7 Question Bank API Parameters

#### `GET /api/questions`

Query parameters:

| Parameter | Type | Required | Default | Purpose |
|---|---|---|---|---|
| `page` | `int` | No | `1` | Page number |
| `page_size` | `int` | No | `20` | Page size |

#### `GET /api/questions/:id`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Question ID |

## 6. Worker / Internal Default Parameters

These are not typically supplied by external callers, but they strongly affect runtime behavior.

### 6.1 HTTP Runtime Defaults

#### HTTP / CORS / Path Defaults

File: `internal/config/defaults.go`

| Parameter | Default | Purpose |
|---|---|---|
| `HTTPAddr()` | `:8081` | Default HTTP listening address |
| `HTTPShutdownTimeout()` | `30s` | Default graceful HTTP shutdown timeout |
| `HTTPLogDir()` | `logs` | Default log directory |
| `HTTPSlowRequestThreshold()` | `1000ms` | Default slow request threshold |
| `CORSAllowOrigin()` | `*` | Default allowed origin |
| `CORSAllowMethods()` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` | Default allowed methods |
| `CORSAllowHeaders()` | `Origin, Content-Type, Accept, Authorization, X-Requested-With` | Default allowed request headers |
| `CORSExposeHeaders()` | `Content-Length, Content-Type` | Default exposed response headers |
| `CORSMaxAge()` | `86400` | Default preflight cache duration |
| `RawPath()` | `os.TempDir()/nlp-video-project/tmp/raw` | Fallback local raw video path |
| `HLSPath()` | `os.TempDir()/nlp-video-project/tmp/hls` | Fallback local HLS path |
| `VectorTempPath()` | `os.TempDir()/nlp-video-project/tmp/video_vectorize` | Fallback vector worker temp path |
| `MediaRoutePrefix()` | `/videos` | Fallback media proxy route |
| `RawURLPrefix()` | `/videos/raw` | Fallback raw video URL prefix |
| `HLSURLPrefix()` | `/videos/hls` | Fallback HLS URL prefix |
| `CoverURLPrefix()` | `/videos` | Fallback cover URL prefix |
| `EmbeddingDim()` | `1536` | Fallback embedding dimension |
| `ASRWSURL()` | `wss://dashscope.aliyuncs.com/api-ws/v1/inference/` | Fallback ASR WebSocket URL |

#### Redis Key Defaults

File: `internal/config/defaults.go`

| Parameter | Default | Purpose |
|---|---|---|
| `TranscodeQueueKey()` | `video:transcode:queue` | Transcode queue |
| `VectorizeQueueKey()` | `video:vectorize:queue` | Vectorization queue |
| `VideoReactionQueueKey()` | `video:reaction:queue` | Video reaction queue |
| `VideoReactionCountsPrefix()` | `video:reaction:counts:` | Video reaction count prefix |
| `VideoReactionUserPrefix()` | `video:reaction:user:` | User reaction state prefix |
| `TranscodeStatusPrefix()` | `video:transcode:status:` | Transcode status prefix |
| `RuntimeActiveCounterPrefix()` | `video:runtime:active:` | Runtime active counter prefix |

### 6.2 Transcode Worker Defaults

Files:

- `internal/worker/transcodeworker/app.go`
- `internal/application/videoapp/worker.go`

| Parameter | Default | Purpose |
|---|---|---|
| `taskTimeout` | `6h` | Fallback task timeout when `Transcode.TaskTimeoutMinutes <= 0` |
| `StatusTTL` | `24h` | Transcode status cache TTL |
| `LeaseTTL` | `1m` | Default lease TTL for worker runtime state |
| `maxRetryAttempts` | `5` | Maximum retry budget for transcode tasks |
| `retryDelayBase` | `500ms` | Base delay for Redis transient retry attempts |

### 6.3 Vector Worker Defaults

File:

- `internal/worker/vectorworker/app.go`

| Parameter | Default | Purpose |
|---|---|---|
| `maxASRWorkers` | `20` | Upper bound for vector ASR worker count |
| `normalizeASRWorkers()` fallback | `4` | Default ASR worker count when `ASRWorkers <= 0` |
| `windowSec` | `60` | Default fixed window size when `SegmentWindowSec <= 0` |
| `stepSec` | `windowSec` | Default fixed window step when `SegmentStepSec <= 0` |
| `coarseWorkers` fallback | `asrWorkers` | Fallback coarse worker count when `CoarseWorkers <= 0` |
| `embedBatch` | `64` | Default embedding batch size |
| `sampleCount` | `3` | Default sample window count |
| `sampleDurSec` | `10` | Default sample window duration |
| `coarseSegmentSec` | `15` | Default coarse segment length |
| `refineMinSegmentSec` | `20` | Default minimum refined segment length |
| `refineMaxSegmentSec` | `180` | Default maximum refined segment length |
| `llmModel` | `qwen-plus` | Default LLM model |
| `llmTimeoutMinutes` | `3` | Default LLM timeout |
| `taskTimeout` | `3h` | Default timeout per vectorization task |
| `workerCount` | `1` | Fallback video-level vector worker count |
| `maxRetryTimes` | `3` | Maximum retry count per vectorization task |
| `retryDelay` initial value | `5s` | Initial retry delay |

### 6.4 Hierarchical Segmentation Internal Parameters

File:

- `internal/worker/vectorworker/tasks/hierarchical.go`

| Parameter | Default | Purpose |
|---|---|---|
| `defaultSegmentOverlapSec` | `3` | Default overlap allowed between adjacent segments |
| `maxSegmentOverlapSec` | `8` | Maximum allowed overlap between adjacent segments |
| `minValidLen` | `5` | Minimum valid segment length counted in `CalcUniformStats` |
| `binWidth` | `10` | Histogram bin width for uniformity stats |
| `modeRatio` threshold | `0.6` | Threshold for identifying overly uniform segmentation |
| `st.MaxLen-st.MinLen` threshold | `15` | Secondary threshold for identifying uniform segmentation |
| `continuationPrefixes` | `然后/所以/因为/接下来/也就是说/我们继续/继续` | Prefix list used for low-confidence continuation merging |

### 6.5 Tail Alignment Internal Parameters

Files:

- `internal/worker/vectorworker/tasks/tail_alignment.go`
- `internal/worker/vectorworker/tasks/boundary_alignment.go`

#### Boundary Alignment Window Parameters

| Parameter | Default | Purpose |
|---|---|---|
| `boundaryStartLookBackSec` | `3` | Start boundary backward search window |
| `boundaryStartLookAheadSec` | `2` | Start boundary forward search window |
| `boundaryEndLookBackSec` | `2` | End boundary backward search window |
| `boundaryEndLookAheadSec` | `4` | End boundary forward search window |
| `maxRecommendedOverlapSec` | `3` | Recommended maximum overlap after boundary correction |

#### Tail Alignment Defaults

| Parameter | Default | Purpose |
|---|---|---|
| `MaxExtendSec` | `3` | Maximum number of seconds the tail may be extended |
| `ProbeStepSec` | `1` | Probe step size |
| `MaxOverlapSec` | `6` | Maximum allowed overlap with the next segment |

#### Sentence Heuristic Dictionaries

These are not external configuration fields, but they influence content boundary detection:

1. `sentenceEndTokens`
2. `sentenceEndPhrases`
3. `trailingConnectors`
4. `sentenceStartPhrases`

Together they affect:

1. `LooksLikeSentenceEnd(text)`
2. `LooksLikeSentenceStart(text)`
3. `NormalizeBoundaryConfidence(s)`
4. `NeedsTailExtension(text)`

## 7. Usage Notes

### 7.1 If You Are a Developer

Focus first on:

1. `configs/video.yml`
2. `configs/video_prod.yml`
3. `internal/config/types.go`
4. `internal/http/router/router.go`
5. `internal/worker/vectorworker/app.go`

### 7.2 If You Are an Upstream Caller

Focus first on:

1. Upload API parameters
2. Recommendation API parameters
3. Transcode status query parameters
4. The unified response shape

### 7.3 If You Are Debugging Worker Issues

Focus first on:

1. `Transcode.WorkerCount`
2. `VectorWorker.ASRWorkers`
3. `VectorWorker.CoarseWorkers`
4. `VectorWorker.Mode`
5. `VectorWorker.LLMTimeoutMinutes`
6. `TailAlignment*` related parameters

### 7.4 If You Are Productizing the Service as a Downstream Capability

You should additionally pay attention to:

1. Authentication-related parameters
2. Idempotency-related parameters
3. Callback-related parameters
4. Trace / source system parameters

These are not yet a complete contract in the current project, but they should eventually become part of the formal downstream service interface.
