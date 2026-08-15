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
  - [3.19 RecommendationConfig](#319-recommendationconfig)
  - [3.20 GorseConfig](#320-gorseconfig)
  - [3.21 AuthConfig](#321-authconfig)
  - [3.22 KnowledgeVideoStorageConfig](#322-knowledgevideostorageconfig)
  - [3.23 KnowledgeVideoWorkerConfig](#323-knowledgevideoworkerconfig)
- [4. Environment Variables](#4-environment-variables)
- [5. HTTP API Parameters](#5-http-api-parameters)
  - [5.1 Common Response Shape](#51-common-response-shape)
  - [5.2 Health Check and Static Entry Points](#52-health-check-and-static-entry-points)
  - [5.3 Upload API Parameters](#53-upload-api-parameters)
  - [5.4 Video Management API Parameters](#54-video-management-api-parameters)
  - [5.5 Playback and Task Status API Parameters](#55-playback-and-task-status-api-parameters)
  - [5.6 Recommendation API Parameters](#56-recommendation-api-parameters)
  - [5.7 Question Bank API Parameters](#57-question-bank-api-parameters)
  - [5.8 Authentication and Administrator Sessions](#58-authentication-and-administrator-sessions)
  - [5.9 Knowledge-video APIs](#59-knowledge-video-apis)
  - [5.10 Recommendation Administration and Internal Candidates](#510-recommendation-administration-and-internal-candidates)
- [6. Worker / Internal Default Parameters](#6-worker--internal-default-parameters)
  - [6.1 HTTP Runtime Defaults](#61-http-runtime-defaults)
  - [6.2 Transcode Worker Defaults](#62-transcode-worker-defaults)
  - [6.3 Vector Worker Defaults](#63-vector-worker-defaults)
  - [6.4 Hierarchical Segmentation Internal Parameters](#64-hierarchical-segmentation-internal-parameters)
  - [6.5 Tail Alignment Internal Parameters](#65-tail-alignment-internal-parameters)
- [7. Usage Notes](#7-usage-notes)
  - [7.1 If You Are a Developer](#71-if-you-are-a-developer)
  - [7.2 If You Are an Upstream Caller](#72-if-you-are-an-upstream-caller)
  - [7.3 If You Are Debugging Worker Issues](#73-if-you-are-debugging-worker-issues)
  - [7.4 DLQ Inspection and Replay](#74-dlq-inspection-and-replay)
  - [7.5 MinIO Knowledge-video Initialization Import Tool](#75-minio-knowledge-video-initialization-import-tool)
  - [7.6 Downstream Service Governance](#76-if-you-are-productizing-the-service-as-a-downstream-capability)

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

Current object storage environment convention:

1. `configs/video.yml` is for local testing. `RustFS.Endpoint = localhost:9000`.
2. `configs/video_prod.yml` is for server/production deployment and currently uses the COS endpoint.
3. Object storage credentials are not stored in YAML. Inject them from private `.env.local` / `.env.deploy` files through `COS_SECRET_ID` / `COS_SECRET_KEY` or `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`.
4. The general video-storage bucket is `video-object-storage` in the local example and `video-object-storage` in the HTTP-service production/delivery example; knowledge videos use an isolated bucket described in 3.22.

### 3.1 Top-Level Config

| Parameter | Type | Example | Purpose |
|---|---|---|---|
| `Name` | `string` | `video-rpc` | Service name, mainly for identification and logging |
| `Host` | `string` | `localhost` | Legacy host field; the current HTTP service does not listen directly based on this value |
| `Port` | `int` | `9090` | Legacy port field; the current HTTP service does not listen directly based on this value |
| `HTTP` | `HTTPConfig` | see below | HTTP API listening, logging, slow-request, and CORS configuration |
| `Auth` | `AuthConfig` | see 3.21 | Administrator JWT signing secret and lifetime |
| `GRPC` | `GRPCConfig` | see below | gRPC-related configuration, mainly kept for compatibility with the historical project |
| `Video` | `VideoConfig` | see below | Local video path configuration |
| `FFmpeg` | `FFmpegConfig` | see below | Transcoding, snapshot, and audio extraction configuration |
| `Storage` | `StorageConfig` | see below | Object key prefixes, public URL prefixes, media proxy route, and vector temp path configuration |
| `Redis` | `RedisConfig` | see below | Redis connection configuration |
| `RedisKeys` | `RedisKeysConfig` | see below | Redis Stream queue, status, and runtime counter key configuration |
| `Postgres` | `PostgresConfig` | see below | PostgreSQL connection and pool configuration |
| `RustFS` | `RustFSConfig` | see below | Object storage connection configuration |
| `KnowledgeVideoStorage` | `KnowledgeVideoStorageConfig` | see 3.22 | Isolated knowledge-video storage, media proxy, and archive limits |
| `KnowledgeVideoWorker` | `KnowledgeVideoWorkerConfig` | see 3.23 | Knowledge-video transcode worker count and timeouts |
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
| `RawPath` | `string` | `./storage/videos/raw` | `os.TempDir()/video-embedding/tmp/raw` | Local raw video path configuration |
| `HlsPath` | `string` | `./storage/videos/hls` | `os.TempDir()/video-embedding/tmp/hls` | Local HLS output path configuration |

### 3.6 StorageConfig

| Parameter | Type | Example | Default | Purpose |
|---|---|---|---|---|
| `RawObjectPrefix` | `string` | `raw` | `raw` | Object storage key prefix for raw videos |
| `HLSObjectPrefix` | `string` | `hls` | `hls` | Object storage key prefix for HLS outputs |
| `MediaRoutePrefix` | `string` | `/videos` | `/videos` | Object proxy route prefix; if changed, `/videos` remains registered as a compatibility route |
| `RawURLPrefix` | `string` | `/videos/raw` | `/videos/raw` | Raw video URL prefix returned to callers |
| `HLSURLPrefix` | `string` | `/videos/hls` | `/videos/hls` | HLS URL prefix returned to callers |
| `CoverURLPrefix` | `string` | `/videos` | `/videos` | Cover URL prefix returned to callers |
| `VectorTempPath` | `string` | `./storage/tmp/video_vectorize` | `os.TempDir()/video-embedding/tmp/video_vectorize` | Temporary file path for the vector worker |

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
| `KnowledgeVideoTranscodeQueue` | `string` | `knowledge_video:transcode:stream` | `knowledge_video:transcode:stream` | Isolated knowledge-video transcode queue; `cmd/dlqctl` does not support it |
| `VectorizeQueue` | `string` | `video:vectorize:queue` | `video:vectorize:queue` | Redis Stream key for vectorization tasks |
| `VectorPrepareQueue` | `string` | `video:vector:prepare` | `video:vector:prepare` | Redis Stream key for the hierarchical vector prepare stage |
| `VectorCoarseQueue` | `string` | `video:vector:coarse` | `video:vector:coarse` | Redis Stream key for the hierarchical vector coarse stage |
| `VectorRefineQueue` | `string` | `video:vector:refine` | `video:vector:refine` | Redis Stream key for the hierarchical vector refine stage |
| `VectorFinalizeQueue` | `string` | `video:vector:finalize` | `video:vector:finalize` | Redis Stream key for the hierarchical vector finalize stage |
| `VideoReactionQueue` | `string` | `video:reaction:queue` | `video:reaction:queue` | Redis key for the video reaction async queue |
| `VideoReactionCounts` | `string` | `video:reaction:counts:` | `video:reaction:counts:` | Prefix for video reaction count keys |
| `VideoReactionUser` | `string` | `video:reaction:user:` | `video:reaction:user:` | Prefix for per-user video reaction state keys |
| `SegmentReactionQueue` | `string` | `segment:reaction:queue` | `segment:reaction:queue` | Redis key for the segment reaction async queue |
| `SegmentReactionCounts` | `string` | `segment:reaction:counts:` | `segment:reaction:counts:` | Prefix for segment reaction count keys |
| `SegmentReactionUser` | `string` | `segment:reaction:user:` | `segment:reaction:user:` | Prefix for per-user segment reaction state keys |
| `TranscodeStatus` | `string` | `video:transcode:status:` | `video:transcode:status:` | Prefix for transcode task status keys |
| `RuntimeActiveCounter` | `string` | `video:runtime:active:` | `video:runtime:active:` | Prefix for runtime active counter keys |
| `RandomPlayRecent` | `string` | `video:random_play:recent:` | `video:random_play:recent:` | Per-user recent-play de-duplication records |
| `RandomPlayBucket` | `string` | `video:random_play:bucket:` | `video:random_play:bucket:` | Per-user prefetched recommendation bucket |

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
| `Endpoint` | `string` | local `localhost:9000`; HTTP production COS URL | Object storage endpoint; delivery uses COS rather than the old RustFS address |
| `AccessKey` | `string` | `""` | Access key, injected through environment variables |
| `SecretKey` | `string` | `""` | Secret key, injected through environment variables |
| `Bucket` | `string` | `video-object-storage` | Bucket name |
| `UseSSL` | `bool` | `false` | Whether HTTPS is used |
| `Region` | `string` | empty locally; `ap-beijing` in production | S3/COS region |
| `BucketLookup` | `string` | `auto` / `dns` | Bucket resolution mode |

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
| `LLMModel` | `string` | local/production `qwen3.7-plus` | Model used for hierarchical content segmentation |
| `LLMTimeoutMinutes` | `int` | `5` | Timeout for LLM calls; do not use the old two-minute example |

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
| `ASRWorkers` | `int` | `8` | ASR worker count; the runtime also enforces a code-level upper bound |
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

### 3.19 RecommendationConfig

| Parameter | Type | Default | Purpose |
|---|---|---|---|
| `Engine` | `string` | `knowledge_match` | Primary random-play chain: `knowledge_match`, `gorse`, or `recbole`; both checked-in example configs use `recbole` |
| `RandomPlayDedupeWindowSec` | `int` | `1800` | Recent-play de-duplication window per user, in seconds |
| `RandomPlayRecentMaxSize` | `int` | `200` | Maximum recent-play records kept per user |

### 3.20 GorseConfig

Gorse is an optional external candidate service. It is used as the primary recommendation chain only when `Recommendation.Engine=gorse`; the HTTP-service example config enables `SyncEnabled`, while the delivery config `deployment/config/video_prod.yml` disables synchronization and write-back and uses RecBole as the online recommendation engine. The root Compose file runs one Gorse service, not a cluster. The local service endpoint defaults to `localhost:8087`; the diagnostic override exposes the master HTTP endpoint to the host at `localhost:8088`.

| Parameter | Type | Current example / delivery value | Purpose |
|---|---|---|---|
| `Endpoint` | `string` | local `http://localhost:8087`; Compose/delivery `http://gorse:8088` | Gorse server endpoint; the host diagnostic override is `http://localhost:8088` |
| `APIKey` | `string` | empty | Gorse API key; `GORSE_API_KEY` overrides it |
| `TimeoutSeconds` | `int` | `2` | Gorse request timeout in seconds |
| `ShadowMode` | `bool` | `false` | Observe Gorse candidates without switching the primary chain |
| `SyncEnabled` / `SyncIntervalMins` | `bool` / `int` | HTTP-service example: `true` / `60`; delivery config: `false` / `60` | Enable periodic data synchronization and set its interval |
| `WriteBackEnabled` | `bool` | HTTP-service example: `true`; delivery config: `false` | Write online feedback back to Gorse |
| `CandidateLimit` | `int` | `100` | Candidate count requested from Gorse |
| `EnableGate` / `MinFeedbackCount` / `MinRecommendItems` | `bool` / `int` / `int` | HTTP/delivery examples: `true` / `20` / `1` | Data-volume and candidate guards before switching |
| `CleanupEnabled` / `DataRetentionDays` | `bool` / `int` | HTTP/delivery examples: `true` / `30` | Reserved for synchronized-data cleanup; current production code does not consume these fields and does not delete data automatically |

### 3.21 AuthConfig

| Parameter | Type | Local example | HTTP-service production example | Delivery fallback / notes |
|---|---|---|---|---|
| `JWTSecret` | `string` | Config file contains a local-development fallback only | Empty in YAML; must be injected through `JWT_SECRET` | `JWT_SECRET` takes precedence over YAML; startup requires at least 32 characters |
| `JWTExpireHour` | `int` | `8` | `24` (`configs/video_prod.yml`) | When omitted by `deployment/config/video_prod.yml`, code falls back to `8` hours |

Administrator login uses `POST /api/auth/login` and accepts only enabled administrator rows in `sys_user`. The service has no administrator-account creation or password-reset API; provision a qualifying `sys_user` row and password hash through the existing identity/database operations process before deployment. Protected requests send `Authorization: Bearer <access_token>`. Never commit a real signing secret or a default password to the repository.

### 3.22 KnowledgeVideoStorageConfig

| Parameter | Type | Local example | Production / delivery example | Purpose |
|---|---|---|---|---|
| `Endpoint` | `string` | `localhost:9000` | COS endpoint (`https://cos.ap-beijing.myqcloud.com` for the HTTP service) | Knowledge-video object storage endpoint |
| `AccessKey` / `SecretKey` | `string` | Injected from the environment | Injected from the environment | Isolated object-storage credentials |
| `Bucket` | `string` | `knowledge-point-videos` | `knowledge-point-videos` | Dedicated knowledge-video bucket |
| `UseSSL` | `bool` | `false` | `true` | Whether HTTPS is used |
| `Region` | `string` | empty | `ap-beijing` | S3/COS region |
| `BucketLookup` | `string` | `auto` | `dns` | Bucket resolution mode |
| `MediaRoutePrefix` | `string` | `/knowledge-video-media` | same | HLS media proxy prefix |
| `TempPath` | `string` | `./storage/tmp/knowledge_video` | same structure; falls back to the system temp directory when missing | Import/transcode temporary directory |
| `MaxArchiveBytes` | `int64` | `1073741824` | `1073741824` | Maximum ZIP size (1 GiB) |
| `MaxExpandedBytes` | `int64` | `4294967296` | `4294967296` | Maximum total expanded size (4 GiB) |
| `MaxEntryBytes` | `int64` | `536870912` | `536870912` | Maximum size of one archive entry (512 MiB) |
| `MaxEntries` | `int` | `1000` | `1000` | Maximum number of archive entries |

### 3.23 KnowledgeVideoWorkerConfig

| Parameter | Type | Local / production default | Purpose |
|---|---|---:|---|
| `WorkerCount` | `int` | `2` | Knowledge-video transcode consumer count; missing or less than `1` falls back to `2` |
| `TaskTimeoutMinutes` | `int` | `30` | Timeout in minutes for one knowledge-video transcode task |
| `ShutdownTimeoutSec` | `int` | `120` | Seconds to wait for tasks during worker shutdown |

This block has no separate blocking, temporary-file retention, or DLQ-management fields. The queue key comes from `RedisKeys.KnowledgeVideoTranscodeQueue`.

## 4. Environment Variables

The project currently uses the following environment variables explicitly:

| Environment Variable | Source Location | Purpose |
|---|---|---|
| `HTTP_ADDR` | `internal/http/app/app.go` | Overrides the HTTP listening address; default is `:8081` |
| `JWT_SECRET` | `internal/config/defaults.go`, `internal/http/app/app.go` | Administrator JWT signing secret; takes precedence over `Auth.JWTSecret` and must be at least 32 characters |
| `VIDEO_APP_ENV_FILE` | `internal/config/loader.go` | Selects an additional dotenv file; `.env` is loaded first and existing shell variables are not overwritten by the file |
| `REDIS_ADDR` | `internal/config/loader.go` | Overrides `Redis.Addr` |
| `CONFIG_FILE` | `internal/config/loader.go` | Overrides the default config file path; has higher priority than `VIDEO_CONFIG_FILE` |
| `VIDEO_CONFIG_FILE` | `internal/config/loader.go` | Overrides the default config file path |
| `POSTGRES_DSN` | `internal/config/loader.go` | Overrides `Postgres.DSN` |
| `REDIS_PASSWORD` | `internal/config/loader.go` | Overrides `Redis.Password` |
| `COS_SECRET_ID` | `internal/config/loader.go` | Overrides `RustFS.AccessKey` |
| `COS_SECRET_KEY` | `internal/config/loader.go` | Overrides `RustFS.SecretKey` |
| `RUSTFS_ACCESS_KEY` | `internal/config/loader.go` | Overrides object storage access key when COS variables are not set |
| `RUSTFS_SECRET_KEY` | `internal/config/loader.go` | Overrides object storage secret key when COS variables are not set |
| `GORSE_API_KEY` | `internal/config/loader.go` | Overrides `Gorse.APIKey` |
| `GORSE_ENDPOINT` | `internal/config/loader.go` | Overrides `Gorse.Endpoint`; local diagnostics usually use `http://localhost:8088` |
| `GORSE_SERVER_API_KEY` | `gorse/entrypoint.sh` | API key checked by the Gorse server in root Compose; must exactly match `GORSE_API_KEY` |
| `GORSE_VERSION` | `docker-compose.yml` | Gorse image interpolation version, default `0.5.11`; supplied only by the shell or Compose root `.env`, not by a service `env_file` |
| `DASHSCOPE_API_KEY` | `internal/config/loader.go`, embedding client, vector worker AI client | DashScope / Bailian-compatible API key |
| `OPENAI_API_KEY` | `internal/config/loader.go`, embedding client, vector worker AI client | Fallback OpenAI-compatible API key |
| `EMBEDDING_API_KEY` | `internal/config/loader.go`, embedding client | Fallback API key for recommendation embedding |
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
| `RECBOLE_TRAINER_ENABLED` | `internal/worker/recboletrainer` | Registers the training scheduler; disabled by default and must be `true` for a standalone trainer |
| `SOURCE_MINIO_ENDPOINT` | `cmd/knowledgevideo-import/main.go` | Source MinIO S3 API endpoint for the knowledge-video initialization import tool; required |
| `SOURCE_MINIO_BUCKET` | `cmd/knowledgevideo-import/main.go` | Source MinIO bucket; required |
| `SOURCE_MINIO_ACCESS_KEY` | `cmd/knowledgevideo-import/main.go` | Source MinIO access key; required |
| `SOURCE_MINIO_SECRET_KEY` | `cmd/knowledgevideo-import/main.go` | Source MinIO secret key; required |
| `SOURCE_MINIO_USE_SSL` | `cmd/knowledgevideo-import/main.go` | Whether the source MinIO uses HTTPS; default `false` |
| `SERVICE_DIR` | `recbole-training/scripts/run_recbole_pipeline.sh` | HTTP-service tools and config directory |
| `CONFIG_FILE` (RecBole script) | RecBole shell pipeline | Config path passed to export/import tools; it has the same name as the service selector but is passed explicitly by the script |
| `MODEL_VERSION`, `MODEL_NAME`, `RECBOLE_MODEL` | RecBole shell pipeline | Candidate version, online model name, and model class |
| `DATASET`, `DIM`, `EPOCHS`, `SAMPLE_LIMIT`, `DAYS_BACK` | RecBole shell pipeline | Dataset prefix, embedding dimension, training epochs, sample limit, and lookback days |
| `PYTHON_BIN`, `DATA_ROOT`, `DATA_DIR`, `ARTIFACT_DIR`, `BASELINE_METRICS` | RecBole shell pipeline | Python executable, data/artifact directories, and baseline metrics file |
| `PUBLISH_GATE_ENABLED` | RecBole shell pipeline | Runs the publish gate; default is `true` |

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
- Purpose: Object storage credential source when COS variables are not set
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
- `MODEL_VERSION`, `MODEL_NAME`, `RECBOLE_MODEL`: model version, online model name, and RecBole algorithm; defaults are a timestamped version, `recbole`, and `BPR`.
- `DATASET`, `SAMPLE_LIMIT`, `DAYS_BACK`: exported dataset name, interaction limit, and lookback days; defaults are `video_app`, `10000`, and `30`.
- `DIM`, `EPOCHS`, `PYTHON_BIN`: embedding dimension, epochs, and Python executable; defaults are `64`, `20`, and `.venv/bin/python` when available.
- `DATA_ROOT`, `DATA_DIR`, `ARTIFACT_DIR`, `BASELINE_METRICS`: locations for atomic files, artifacts, and active-model metrics.
- `PUBLISH_GATE_ENABLED`: runs the RecBole publish gate, default `true`. Its defaults are `Recall@20 >= 0.01`, `NDCG@20 >= 0.005`, and no more than `20%` relative `NDCG@20` decline from the active model; adjust them with the `recbole_recommendation.publish_gate` command-line flags.

The RecBole shell script does not consume `MIN_RECALL_AT_20`, `MIN_NDCG_AT_20`, `MAX_RELATIVE_NDCG_DROP`, or `ARTIFACT_RETENTION_DAYS`. Gate thresholds are CLI flags: `--min-recall-at-20`, `--min-ndcg-at-20`, `--max-relative-ndcg-drop`, `--min-positive-rows`, and `--min-positive-users`; the last two default to `1`. The current exporter does not yet write `positive_rows` / `positive_users`; `publish_gate` checks them only when those fields exist in the metrics file. The current pipeline is therefore fail-open for these two data-volume conditions; do not treat the default `1` as an already-enforced requirement.

### 4.6 Object Storage Migration Tool Parameters

`tools/migrate_rustfs_bucket` migrates objects from the old MinIO bucket to RustFS. Defaults:

| Parameter | Default | Purpose |
|---|---|---|
| `--source-endpoint` | `10.200.10.12:9000` | Old MinIO S3 API endpoint |
| `--source-access-key` | From `MIGRATE_SOURCE_ACCESS_KEY` | Source access key |
| `--source-secret-key` | From `MIGRATE_SOURCE_SECRET_KEY` | Source secret key |
| `--source-bucket` | `video-object-storage` | Source bucket |
| `--target-endpoint` | `10.200.10.201:9001` | RustFS S3 API endpoint |
| `--target-access-key` | From `MIGRATE_TARGET_ACCESS_KEY` | Target access key |
| `--target-secret-key` | From `MIGRATE_TARGET_SECRET_KEY` | Target secret key |
| `--target-bucket` | `video-object-storage` | Target bucket |
| `--prefix` | empty | Migrate only objects under this key prefix |
| `--workers` | `4` | Parallel copy workers |
| `--overwrite` | `false` | Overwrite existing target objects |
| `--dry-run` | `true` | Preview actions without copying |

Common commands:

```bash
cd video-service
go run ./tools/migrate_rustfs_bucket
go run ./tools/migrate_rustfs_bucket --dry-run=false
```

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

Business APIs use the envelope above. `/healthz` and `/api/healthz` return health-status JSON, `/swagger/*any` returns Swagger resources, and `/videos/*filepath` plus `/knowledge-video-media/*` may return raw media or Range/206 responses; these entry points do not use the business envelope.

### 5.2 Health Check and Static Entry Points

#### `GET /healthz`
#### `GET /api/healthz`

- No request parameters
- Returns `{ "status": "ok" }`

#### `GET /swagger/*any`

- Path parameter: `*any`
- Purpose: Access Swagger page resources

#### `GET /api/system/metrics`

- Authentication: administrator JWT
- No request parameters
- Purpose: Query system runtime metrics

#### `GET /videos/*filepath`

- Path parameter: `*filepath`
- Purpose: Proxy access to video resources stored in object storage
- Note: the actual media proxy route prefix is controlled by `Storage.MediaRoutePrefix` and defaults to `/videos`; if another prefix is configured, `/videos/*filepath` remains registered as a compatibility route

### 5.3 Upload API Parameters

Every endpoint in this section requires an administrator JWT and derives the uploader ID from the authenticated administrator. Requests must send `Authorization: Bearer <access_token>`.

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

#### `GET /api/videos/archive/batches/:batchId/progress`

- Authentication: administrator JWT.
- Path parameter: `batchId`, a positive integer archive-processing batch ID.
- Returns progress fields such as `batch_id`, `status`, `total`, `processed`, `succeeded`, `failed`, `skipped`, and `errors`; use the current Swagger schema as the authoritative field reference.

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

Authentication boundary: `PATCH /api/videos/:id`, `DELETE /api/videos/:id`, `POST /api/videos/:id/publish`, and `POST /api/videos/:id/recommend` require an administrator JWT. The remaining list, playback, and reaction endpoints in this section are public.

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

#### `GET /api/video-segments/random-play`

Query parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `user_id` | `uint64` | No | When provided, the API uses RecBole, Gorse, or knowledge-match recall according to `Recommendation.Engine`; when omitted, the handler uses default user `6` and follows the same chain |

Purpose: returns one playable segment for refresh/play scenarios. The checked-in configs use `recbole`; an omitted `user_id` first attempts personalized recall for default user `6`, then falls back to random playback when active-model data, user embeddings, or candidates are unavailable. Non-positive or invalid `user_id` values return an argument error.

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `video_id` | `uint64` | Parent video ID |
| `segment_id` | `uint64` | Segment ID |
| `start_time` | `float64` | Segment start time (seconds) |
| `end_time` | `float64` | Segment end time (seconds) |
| `text` | `string` | Segment text content |
| `play_url` | `string` | Playback URL |
| `video` | `VideoItem` | Parent video information |

#### `POST /api/video-segments/:id/reactions`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video segment ID |

JSON body:

| Parameter | Type | Required | Constraint | Purpose |
|---|---|---|---|---|
| `user_id` | `uint64` | Yes | `> 0` | User ID |
| `reaction_type` | `string` | Yes | `like`, `double_like`, `dislike` | Reaction type; submitting the same reaction again cancels it |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `segment_id` | `uint64` | Segment ID |
| `user_id` | `uint64` | User ID |
| `reaction_type` | `string` | Submitted reaction type |
| `active` | `bool` | Whether the reaction is currently active |
| `like_count` | `int64` | Like count |
| `double_like_count` | `int64` | Double-like count |
| `updated` | `bool` | Whether the update was applied |

#### `GET /api/video-segments/:id/reaction-counts`

Path parameters:

| Parameter | Type | Required | Purpose |
|---|---|---|---|
| `id` | `uint64` | Yes | Video segment ID |

Response fields:

| Field | Type | Purpose |
|---|---|---|
| `segment_id` | `uint64` | Segment ID |
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

- Authentication: administrator JWT

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

### 5.8 Authentication and Administrator Sessions

#### `POST /api/auth/login`

- Authentication: public login entry point.
- Request type: `application/json`.

| Field | Type | Required | Purpose |
|---|---|---|---|
| `username` | `string` | Yes | `sys_user.username` |
| `password` | `string` | Yes | Administrator password; the repository does not provide a default password |

Only administrator accounts with `user_type=3`, `status=1`, and `deleted=0` are accepted. On success, the response includes `data.access_token`, `data.expires_at`, and a redacted `data.admin`; subsequent administrator requests use `Authorization: Bearer <access_token>`. The service has no administrator-account creation or password-reset endpoint; provision the account through the existing identity/database operations process before deployment.

#### `GET /api/auth/me`

- Authentication: administrator JWT.
- No request parameters.
- Returns the current administrator's `id`, `username`, and `real_name`; an invalid account or expired token returns `401`.

### 5.9 Knowledge-video APIs

#### `POST /api/admin/knowledge-videos/batches`

- Authentication: administrator JWT; the uploader ID is taken from the token.
- Request type: `multipart/form-data`.

| Field | Type | Required | Purpose |
|---|---|---|---|
| `archive` | `file` | Yes | ZIP video archive |
| `mapping` | `file` | Yes | XLSX knowledge-point mapping |

XLSX/ZIP contract:

1. The first worksheet is read. Row 1 must contain exactly three columns: `id`, `name`, and `video_name`; a fourth column is rejected, and at least one data row is required.
2. `id` must be an existing positive knowledge-point ID, and `name` must exactly match the dictionary name. Multiple rows may refer to the same knowledge point.
3. `video_name` must be unique in the workbook and match exactly one video basename in the ZIP. ZIP entries may be nested in directories, but duplicate basenames, missing mapped videos, or extra unreferenced videos reject the whole batch.
4. Supported extensions are `.mp4`, `.mov`, `.mkv`, `.avi`, `.webm`, and `.m4v`. Traversal paths, absolute paths, symbolic links, and other extensions are rejected. `__MACOSX`, `.DS_Store`, and `._*` metadata entries are ignored.

Success returns `202` with `batch_id`, `total_count`, `status`, and `progress_url`. Archive size, total expanded size, single-entry size, and entry count are limited by `KnowledgeVideoStorage`.

#### `GET /api/admin/knowledge-videos/batches/:batchId`

- Authentication: administrator JWT.
- Path parameter: positive integer `batchId`.
- Returns `batch_id`, `total_count`, `ready_count`, `failed_count`, `status`, and `videos[]`; each video includes its knowledge point, source filename, duration, status, and an optional error.

#### `GET /api/knowledge-videos/tree`

- Authentication: public.
- No request parameters.
- Returns a `nodes[]` tree; nodes contain `id`, `parent_id`, `name`, `children`, and associated `videos`.

#### `GET /api/knowledge-points/:knowledgePointId/video`
#### `GET /api/knowledge-points/:knowledgePointId/videos`

- Authentication: public; the singular path is a compatibility route and the plural path is the standard route.
- Path parameter: positive integer `knowledgePointId`.
- Returns `knowledge_point_id`, `knowledge_point_name`, and `videos[]`; each video contains `knowledge_video_id`, source filename, display name, duration, and `playback_url`. Compatibility responses may also expose `video_id`, `video_name`, `duration`, and `playback_url` for the first video.

#### `POST /api/knowledge-videos/:knowledgeVideoId/playbacks`

- Authentication: public; records a playback start / playback record.
- Path parameter: positive integer `knowledgeVideoId`.
- JSON body: `user_id` (`uint64`, required and greater than `0`).
- Success returns an empty `data` envelope; repeated calls follow the business recording semantics.

#### `PUT /api/knowledge-videos/:knowledgeVideoId/watch-sessions/:sessionId`

- Authentication: public; the player reports progress with session-level retries.
- Path parameters: positive integer `knowledgeVideoId`; the client generates `sessionId` for one playback session and it must match `^[A-Za-z0-9_-]{16,64}$`. Its idempotency scope is `(user_id, knowledge_video_id, session_id)`.
- JSON body: `user_id` (required and greater than `0`) and `watched_seconds` (the current session's required absolute watched seconds, non-negative). Retries for the same session keep the larger value; different sessions are summed per user and video, and the service caps both each report and the aggregate at the video duration. Clients must not send an increment and should generate a new `sessionId` for a genuinely new playback session.
- Returns `session_id`, `session_watched_seconds`, `total_watched_seconds`, `duration_seconds`, `progress_ratio`, and `effective_watch`. `effective_watch=true` when accumulated watching reaches `60%` of the video duration. This signal enters only the RecBole training-time virtual item and is never an online candidate.

Security boundary: these playback-record and watch-session endpoints currently do not use JWT; `user_id` is supplied by the caller, and the service only verifies that the user exists for watch-session reports. It does not verify that the caller owns that identity. For untrusted clients, add identity binding, rate limiting, and anti-abuse controls in an upstream gateway/service; otherwise any valid user ID can be impersonated and RecBole training signals can be polluted.

#### `GET /knowledge-video-media/hls/:videoId/*filepath`

- Authentication: public media proxy.
- Path parameters: `videoId` and `filepath`.
- Returns a raw HLS/media response and may support Range/206; it does not use the business JSON envelope.

### 5.10 Recommendation Administration and Internal Candidates

All `/api/admin/recommendation/*` routes require an administrator JWT. Successful responses use the success envelope and errors use the common error shape.

#### `GET /api/admin/recommendation/overview`

- No parameters; returns the current engine, Gorse/RecBole status, and a Redis random-play state summary.

#### `GET /api/admin/recommendation/diagnostics`

- Query: optional `days` (default `14`) and optional `limit` (recent request count). Returns health checks, data freshness, recent requests, strategy effects, and task status.

#### `GET /api/admin/recommendation/datasources`

- No parameters; returns counts and ratios for video, segment, exposure, watch, reaction, and RecBole data sources.

#### `GET /api/admin/recommendation/effects`

- Query: optional `days` (default `14`); returns daily and per-strategy exposure, watch, watch rate, average rank, and average score.

#### `GET /api/admin/recommendation/recbole/performance`

- Query: `metric`, `begin`, and `end` are all required. `begin` and `end` use RFC3339; `metric` is a performance metric name such as `Recall@20` or `NDCG@20`.
- Returns available metrics and point series by model version and time.

#### `GET /api/admin/recommendation/trace/random-play`

- Query: required positive integer `user_id`; optional `limit`.
- Returns random-play pipeline stages, candidate items, strategy, model version, and filter reasons.

#### `POST /api/admin/recommendation/trace/by-question`

- JSON body is the same `RecommendByQuestionRequest` used by `POST /api/recommendations/by-question`; returns a question-recommendation pipeline trace.

#### `GET /api/admin/recommendation/redis-state`

- Query: required positive integer `user_id`; returns that user's random-play bucket/recent TTLs, counts, limits, and segment IDs.

#### `GET /api/admin/recommendation/preview/random-play`

- Query: required positive integer `user_id`; optional `limit`; returns preview candidates without writing exposure or recommendation records.

#### `POST /api/admin/recommendation/preview/by-question`

- JSON body is the same `RecommendByQuestionRequest`; returns candidates with `preview_only=true` and does not count them as normal business exposure.

#### `GET /api/internal/recommendations/external/recbole`

- Authentication: none at the application layer (no JWT or API key). This route is intended only for the Gorse external script or trusted internal callers, so deployment must restrict it with a reverse-proxy ACL, network policy, or firewall. It is not a public Java/browser contract, and the `internal` path segment is not a security boundary.
- Query: required positive integer `user_id`; optional `n`, default `100`, maximum `500`.
- Success returns a plain numeric `video_segment_id` array (not the business envelope) for the Gorse external script. Knowledge-video virtual items never appear in this list.

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
| `RawPath()` | `os.TempDir()/video-embedding/tmp/raw` | Fallback local raw video path |
| `HLSPath()` | `os.TempDir()/video-embedding/tmp/hls` | Fallback local HLS path |
| `VectorTempPath()` | `os.TempDir()/video-embedding/tmp/video_vectorize` | Fallback vector worker temp path |
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
| `SegmentReactionQueueKey()` | `segment:reaction:queue` | Segment reaction async queue |
| `SegmentReactionCountsPrefix()` | `segment:reaction:counts:` | Segment reaction count prefix |
| `SegmentReactionUserPrefix()` | `segment:reaction:user:` | User segment reaction state prefix |

All Redis Stream queue dead-letter streams use `<queue-key>:dlq`. For example, `VectorCoarseQueueKey()` maps to `video:vector:coarse:dlq`.

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

### 7.4 DLQ Inspection and Replay

`cmd/dlqctl` inspects and explicitly replays Redis Stream dead-letter messages. It follows the same `CONFIG_FILE` / `VIDEO_CONFIG_FILE` loading rules as the service and derives Redis connection and primary queue keys from `Redis` and `RedisKeys`.

Supported queue names are:

| Queue name | Configuration |
|---|---|
| `transcode` | `RedisKeys.TranscodeQueue` |
| `vectorize` | `RedisKeys.VectorizeQueue` |
| `vector-prepare` | `RedisKeys.VectorPrepareQueue` |
| `vector-coarse` | `RedisKeys.VectorCoarseQueue` |
| `vector-refine` | `RedisKeys.VectorRefineQueue` |
| `vector-finalize` | `RedisKeys.VectorFinalizeQueue` |
| `video-reaction` | `RedisKeys.VideoReactionQueue` |
| `segment-reaction` | `RedisKeys.SegmentReactionQueue` |

Terminal knowledge-video transcode failures are written to `RedisKeys.KnowledgeVideoTranscodeQueue + ":dlq"` (default `knowledge_video:transcode:stream:dlq`), but the current `cmd/dlqctl` does not include this queue and `--queue all` does not inspect or replay it. The batch API exposes the failed status and error. The repository currently ships no supported knowledge-video DLQ recovery tool or runbook. Production recovery must address the persisted failed state, retry count, and Redis payload together; simply reinserting the original payload will immediately be treated as a terminal failure again. Do not apply the generic commands below.

```bash
cd video-service

go run ./cmd/dlqctl list --queue all --limit 20
go run ./cmd/dlqctl list --queue transcode --limit 20
go run ./cmd/dlqctl replay --queue vector-coarse --limit 10 --dry-run
go run ./cmd/dlqctl replay --queue transcode --id <dlq-message-id>
go run ./cmd/dlqctl replay --queue vector-coarse --limit 10 --keep-dlq
```

By default, `replay` writes the payload from the DLQ message back to the primary queue and removes the original DLQ message. `--keep-dlq` retains the original message; `--dry-run` only shows what would be replayed. Inspect the failure and recover its dependency before replaying. Corrupted media, missing object keys, and invalid payloads are permanent failures and should not be replayed directly.

Recommended workflow:

1. Run `list` or `replay --dry-run` to inspect the failure reason and payload summary.
2. Confirm that the node, object storage, AI service, or database dependency has recovered.
3. Replay clearly recoverable tasks by ID; use `--queue all --limit` cautiously.
4. Do not replay permanently failed tasks with corrupted videos, missing object keys, or invalid parameters.

### 7.5 MinIO Knowledge-video Initialization Import Tool

Files:

- `cmd/knowledgevideo-import/main.go`
- `cmd/knowledgevideo-import/bulk.go`

`cmd/knowledgevideo-import` is an ops initialization tool: it scans a source MinIO prefix directly, bypassing HTTP, ZIP, and browser authentication, copies source objects to the knowledge-video object store, creates batch/video records, and enqueues transcode tasks. The target PostgreSQL, Redis, and knowledge-video object store keep using the project config; the source MinIO is configured only through the `SOURCE_MINIO_*` environment variables.

The XLSX mapping contract matches the HTTP batch API: the first row must be exactly the three headers `id`, `name`, `video_name`; `video_name` can be a basename unique under the source prefix or a relative object path. The database knowledge-point name is authoritative; `parent / leaf` style inputs are normalized to the leaf name on an exact match. The same source object may appear in multiple rows attached to multiple knowledge points; each unique object is stream-copied only once.

#### Single-batch mode CLI flags

| Flag | Required | Purpose |
|---|---|---|
| `--mapping <path>` | Yes | Path to the XLSX mapping file |
| `--source-prefix <prefix>` | Yes | Allowed source MinIO object prefix |
| `--batch-key <key>` | Yes | Idempotency key; skipped when a batch with the same key already exists, stored as `minio:<key>` in `zip_file_name` |
| `--upload-user-id <id>` | Yes | Owner user ID of the batch; positive integer |
| `--dry-run` | No | Validate and count only (`validated rows / knowledge_points / unique_objects / bytes`) without writing |
| `--list-only` | No | List objects and total bytes under the source prefix without database access |
| `--inspect-object <key>` | No | Download and print the raw header, parsed rows, and validation issues of one XLSX |

After validation the tool prints `validated rows=N knowledge_points=N unique_objects=N ...`; a real run creates `direct-import/<batch_id>/<hash>-<basename>` objects, a `manifests/<batch_id>/mapping.xlsx` manifest, and enqueues tasks to `RedisKeys.KnowledgeVideoTranscodeQueue`.

#### Bulk mode CLI flags

| Flag | Required | Purpose |
|---|---|---|
| `--all-prefix <prefix>` | Yes (in this mode) | Process every `batch-*.xlsx` + `batch-*.zip` batch below the prefix |
| `--batch-key <key>` | Yes | Idempotency key; batches are deduplicated per `minio:<key>:<batch-name>` |
| `--upload-user-id <id>` | Yes | Owner user ID of the batches |
| `--report-dir <dir>` | No | Report directory; defaults to `knowledge-video-import-<batch-key>` under the system temp dir |
| `--continue-on-error` | No | Continue after a batch failure; default `true` |

Bulk report files:

| File | Content |
|---|---|
| `summary.json` | Result for every batch (`batch`, `status`, `rows`, `unique_objects`, `skipped_empty_rows`, `error`) |
| `valid-batches.csv` | `imported` / `skipped` / `dry-run` / `skipped-empty` batches |
| `invalid-batches.csv` | `failed` / `invalid` batches with error details |

Common failure modes: an imprecise header (the three `row=1 field=...: header must be exactly ...` issues) fails the whole batch; data rows with an empty `video_name` (`row=N field=video_name: video name is required`) also fail the whole batch. The reports and batch records are not auto-recovered; fix the mappings and re-run (the idempotency key prevents already-created batches from being inserted again).

Common commands:

```bash
cd video-service
SOURCE_MINIO_ENDPOINT=http://minio.example:9000 \
SOURCE_MINIO_BUCKET=source-bucket \
SOURCE_MINIO_ACCESS_KEY=... \
SOURCE_MINIO_SECRET_KEY=... \
go run ./cmd/knowledgevideo-import \
  --all-prefix 'batch_imports/2026-08' \
  --upload-user-id 1 \
  --batch-key bulk-import-202608 \
  --report-dir /private/tmp/knowledge-video-import-report
```

### 7.6 If You Are Productizing the Service as a Downstream Capability

You should additionally pay attention to:

1. Authentication-related parameters
2. Idempotency-related parameters
3. Callback-related parameters
4. Trace / source system parameters

These are not yet a complete contract in the current project, but they should eventually become part of the formal downstream service interface.
