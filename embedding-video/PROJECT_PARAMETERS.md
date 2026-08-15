# 项目参数总览文档

## 目录

- [1. 文档说明](#1-文档说明)
- [2. 参数总表](#2-参数总表)
- [3. 运行配置参数](#3-运行配置参数)
  - [3.1 顶层配置 Config](#31-顶层配置-config)
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
- [4. 环境变量参数](#4-环境变量参数)
- [5. HTTP API 参数](#5-http-api-参数)
  - [5.1 通用返回结构](#51-通用返回结构)
  - [5.2 健康检查与静态入口](#52-健康检查与静态入口)
  - [5.3 上传相关接口参数](#53-上传相关接口参数)
  - [5.4 视频管理接口参数](#54-视频管理接口参数)
  - [5.5 播放与状态接口参数](#55-播放与状态接口参数)
  - [5.6 推荐接口参数](#56-推荐接口参数)
  - [5.7 题库接口参数](#57-题库接口参数)
  - [5.8 鉴权与管理员会话](#58-鉴权与管理员会话)
  - [5.9 知识点视频接口](#59-知识点视频接口)
  - [5.10 管理推荐与内部候选接口](#510-管理推荐与内部候选接口)
- [6. Worker / 内部默认参数](#6-worker--内部默认参数)
  - [6.1 HTTP 运行时默认参数](#61-http-运行时默认参数)
  - [6.2 转码 worker 默认参数](#62-转码-worker-默认参数)
  - [6.3 向量化 worker 默认参数](#63-向量化-worker-默认参数)
  - [6.4 hierarchical 内容分段相关内部参数](#64-hierarchical-内容分段相关内部参数)
  - [6.5 tail alignment 相关内部参数](#65-tail-alignment-相关内部参数)
- [7. 使用建议](#7-使用建议)
  - [7.1 如果你是开发者](#71-如果你是开发者)
  - [7.2 如果你是上游调用方](#72-如果你是上游调用方)
  - [7.3 如果你在排查 worker 问题](#73-如果你在排查-worker-问题)
  - [7.4 DLQ 查看与重放工具](#74-dlq-查看与重放工具)
  - [7.5 知识点视频 MinIO 初始化导入工具](#75-知识点视频-minio-初始化导入工具)
  - [7.6 如果你要做下游服务化治理](#76-如果你要做下游服务化治理)

## 1. 文档说明

本文档总结当前仓库中，尤其是 `video-service/` 主项目里可见的主要参数，覆盖四类来源：

1. 运行配置参数
2. 环境变量参数
3. HTTP API 接口参数
4. Worker 和内部默认参数

说明：

1. 这里重点围绕 `video-service/` 展开，因为它是当前推荐部署和推荐作为下游服务使用的主项目。
2. 本文档不尝试枚举数据库每一列，也不列出所有函数的所有局部变量。
3. 这里的“参数”强调的是：外部可配置、接口可传入、运行时有默认值、会影响系统行为的字段。

## 2. 参数总表

| 参数类别 | 来源位置 | 典型示例 | 作用 |
|---|---|---|---|
| 运行配置参数 | `configs/video.yml`、`configs/video_prod.yml`、`internal/config/types.go` | `HTTP.Addr`、`Storage.MediaRoutePrefix`、`RedisKeys.TranscodeQueue`、`VectorWorker.Mode` | 控制服务监听、跨域、路径、队列、转码、向量化和外部依赖行为 |
| 环境变量参数 | `internal/config/loader.go`、`internal/http/app/app.go`、worker 初始化逻辑 | `HTTP_ADDR`、`CONFIG_FILE`、`DASHSCOPE_API_KEY`、`RUSTFS_ACCESS_KEY` | 覆盖配置、选择配置文件或注入敏感信息 |
| HTTP 接口参数 | `internal/http/router/router.go`、`handler/*.go`、`dto/*.go` | `question_id`、`limit`、`file`、`is_published` | 决定 API 调用行为 |
| Worker 内部默认参数 | `internal/worker/*`、`tasks/*` | `taskTimeout`、`maxRetryTimes`、`boundaryStartLookBackSec` | 控制后台任务执行节奏、重试、内容分段和边界修正 |

## 3. 运行配置参数

运行配置结构定义在：

- `video-service/internal/config/types.go`

实际配置文件主要是：

- `video-service/configs/video.yml`
- `video-service/configs/video_prod.yml`

默认加载规则：

1. macOS 和 Windows 默认加载 `configs/video.yml`。
2. 其他环境默认加载 `configs/video_prod.yml`。
3. `CONFIG_FILE` 和 `VIDEO_CONFIG_FILE` 都可覆盖默认配置路径。
4. 两者同时设置时，`CONFIG_FILE` 优先生效。
5. `cmd/httpapi` 和 `cmd/worker` 会先调用 `config.EnsureProjectRoot()`，因此相对配置路径会锚定到 `video-service/`。

对象存储当前环境约定：

1. `configs/video.yml` 用于本地测试，`RustFS.Endpoint = localhost:9000`。
2. `configs/video_prod.yml` 用于服务器/生产部署，当前使用 COS endpoint。
3. 对象存储账号不写入 YAML，通过 `.env.local` / `.env.deploy` 中的 `COS_SECRET_ID` / `COS_SECRET_KEY` 或 `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY` 注入。
4. 通用视频对象存储 bucket 在本地示例中为 `video-object-storage`，在 HTTP 服务生产/交付示例中为 `video-object-storage`；知识点视频使用独立 bucket，见 3.22。

### 3.1 顶层配置 Config

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `Name` | `string` | `video-rpc` | 服务名称标识，主要用于日志与配置描述 |
| `Host` | `string` | `localhost` | 历史主机配置字段，当前 HTTP 主服务监听不直接依赖它 |
| `Port` | `int` | `9090` | 历史端口配置字段，当前 HTTP 主服务监听不直接依赖它 |
| `HTTP` | `HTTPConfig` | 见下文 | HTTP API 监听、日志、慢请求和 CORS 配置 |
| `Auth` | `AuthConfig` | 见 3.21 | 管理员 JWT 签名密钥和有效期 |
| `GRPC` | `GRPCConfig` | 见下文 | gRPC 相关配置，主要兼容历史工程 |
| `Video` | `VideoConfig` | 见下文 | 本地视频目录配置 |
| `FFmpeg` | `FFmpegConfig` | 见下文 | 转码、截图、音频提取配置 |
| `Storage` | `StorageConfig` | 见下文 | 对象存储 key、对外 URL 前缀、媒体代理路由和向量化临时目录配置 |
| `Redis` | `RedisConfig` | 见下文 | Redis 连接配置 |
| `RedisKeys` | `RedisKeysConfig` | 见下文 | Redis Stream 队列、状态和运行计数 key 配置 |
| `Postgres` | `PostgresConfig` | 见下文 | PostgreSQL 连接与连接池配置 |
| `RustFS` | `RustFSConfig` | 见下文 | 对象存储连接配置 |
| `KnowledgeVideoStorage` | `KnowledgeVideoStorageConfig` | 见 3.22 | 知识点视频独立对象存储、媒体代理和归档限制 |
| `KnowledgeVideoWorker` | `KnowledgeVideoWorkerConfig` | 见 3.23 | 知识点视频转码 worker 数量与超时 |
| `Transcode` | `TransConfig` | 见下文 | 转码 worker 配置 |
| `VectorWorker` | `VectorWorkerConfig` | 见下文 | 向量化 worker 配置 |
| `VectorStageWorkers` | `VectorStageWorkersConfig` | 见下文 | hierarchical 向量化四阶段 Redis consumer 数配置 |
| `WorkerPools` | `WorkerPoolsConfig` | 见下文 | vector worker 内部 ants pool 并发配置 |
| `embedding` | `EmbeddingConfig` | 见下文 | Embedding 服务配置 |
| `asr` | `ASRConfig` | 见下文 | 语音识别服务配置 |
| `AI` | `AIConfig` | 见下文 | AI 基础参数，例如 embedding 维度 |

### 3.2 HTTPConfig

| 参数名 | 类型 | 示例 | 默认值 | 作用 |
|---|---|---|---|---|
| `Addr` | `string` | `:8081` | `:8081` | HTTP API 监听地址，可被 `HTTP_ADDR` 覆盖 |
| `ShutdownTimeoutSec` | `int` | `30` | `30` | HTTP 服务优雅关闭超时，单位秒 |
| `LogDir` | `string` | `logs` | `logs` | HTTP API 和 worker 日志目录 |
| `SlowRequestMs` | `int` | `1000` | `1000` | 慢请求日志阈值，单位毫秒 |
| `CORS` | `CORSConfig` | 见下文 | 见下文 | 浏览器跨域响应头配置 |

### 3.3 CORSConfig

| 参数名 | 类型 | 示例 | 默认值 | 作用 |
|---|---|---|---|---|
| `AllowOrigin` | `string` | `*` | `*` | `Access-Control-Allow-Origin` |
| `AllowMethods` | `string` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` | 同示例 | `Access-Control-Allow-Methods` |
| `AllowHeaders` | `string` | `Origin, Content-Type, Accept, Authorization, X-Requested-With` | 同示例 | `Access-Control-Allow-Headers` |
| `ExposeHeaders` | `string` | `Content-Length, Content-Type` | 同示例 | `Access-Control-Expose-Headers` |
| `MaxAge` | `string` | `86400` | `86400` | `Access-Control-Max-Age` |

### 3.4 GRPCConfig

| 参数名 | 类型 | 作用 |
|---|---|---|
| `MaxMsgSize` | `int` | gRPC 消息大小上限 |
| `KeepaliveTime` | `int` | gRPC keepalive 时间 |
| `KeepaliveTimeout` | `int` | gRPC keepalive 超时时间 |
| `MaxConnectionAge` | `int` | 单连接最大生命周期 |
| `MaxConnectionAgeGrace` | `int` | 连接到期后的宽限时间 |

说明：在 `video-service/` 里，当前主路径是 HTTP，对这组参数依赖较弱，更多是兼容历史结构。

### 3.5 VideoConfig

| 参数名 | 类型 | 示例 | 默认值 | 作用 |
|---|---|---|---|---|
| `RawPath` | `string` | `./storage/videos/raw` | 系统临时目录下的 `video-embedding/tmp/raw` | 原始视频本地目录配置 |
| `HlsPath` | `string` | `./storage/videos/hls` | 系统临时目录下的 `video-embedding/tmp/hls` | HLS 文件本地目录配置 |

### 3.6 StorageConfig

| 参数名 | 类型 | 示例 | 默认值 | 作用 |
|---|---|---|---|---|
| `RawObjectPrefix` | `string` | `raw` | `raw` | 原视频对象存储 key 前缀 |
| `HLSObjectPrefix` | `string` | `hls` | `hls` | HLS 产物对象存储 key 前缀 |
| `MediaRoutePrefix` | `string` | `/videos` | `/videos` | 对象存储代理路由前缀；如果改成其他值，仍保留 `/videos` 兼容路由 |
| `RawURLPrefix` | `string` | `/videos/raw` | `/videos/raw` | 返回给调用方的原视频 URL 前缀 |
| `HLSURLPrefix` | `string` | `/videos/hls` | `/videos/hls` | 返回给调用方的 HLS URL 前缀 |
| `CoverURLPrefix` | `string` | `/videos` | `/videos` | 返回给调用方的封面 URL 前缀 |
| `VectorTempPath` | `string` | `./storage/tmp/video_vectorize` | 系统临时目录下的 `video-embedding/tmp/video_vectorize` | 向量化 worker 临时文件目录 |

### 3.7 FFmpegConfig

#### 3.7.1 顶层字段

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `UseDocker` | `bool` | `true` | 是否允许通过 Docker 调用 ffmpeg |
| `DockerImage` | `string` | `jrottenberg/ffmpeg` | Docker 模式下使用的 ffmpeg 镜像 |
| `HLS` | `FFmpegHLSConfig` | 见下文 | HLS 切片输出配置 |
| `Fast` | `FFmpegFastConfig` | 见下文 | 快速转码参数 |
| `Cover` | `FFmpegCoverConfig` | 见下文 | 封面截帧配置 |
| `Audio` | `FFmpegAudioConfig` | 见下文 | 音频提取配置 |

#### 3.7.2 FFmpegHLSConfig

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `Time` | `int` | `6` | 每个 HLS ts 片段时长 |
| `ListSize` | `int` | `0` | m3u8 列表大小，`0` 通常表示不截断 |
| `MasterName` | `string` | `master.m3u8` | 主播放列表文件名；为空时默认 `master.m3u8` |
| `SegmentPattern` | `string` | `v0_%03d.ts` | ts 片段命名模板 |

#### 3.7.3 FFmpegFastConfig

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `ScaleW` | `int` | `1280` | 输出宽度 |
| `ScaleH` | `int` | `720` | 输出高度 |
| `Preset` | `string` | `ultrafast` | 编码速度/压缩 preset |
| `Crf` | `int` | `28` | 编码质量参数，越小质量越高 |
| `PixFmt` | `string` | `yuv420p` | 输出像素格式 |
| `AudioBitrate` | `string` | `96k` | 音频码率 |
| `AudioChannels` | `int` | `2` | 音频声道数 |
| `PadToFit` | `bool` | `true` | 是否按目标比例补边适配 |

#### 3.7.4 FFmpegCoverConfig

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `SeekSec` | `int` | `2` | 首选截封面时间点 |
| `FallbackSeekSec` | `int` | `0` | 首选失败时的回退截帧点 |
| `Quality` | `int` | `2` | 截图输出质量 |

#### 3.7.5 FFmpegAudioConfig

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `SampleRate` | `int` | `16000` | 抽音频采样率 |
| `Channels` | `int` | `1` | 抽音频声道数 |

### 3.8 RedisConfig

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `Addr` | `string` | `localhost:6379` | Redis 地址 |
| `Password` | `string` | `""` | Redis 密码 |
| `DB` | `int` | `0` | Redis DB 编号 |

### 3.9 RedisKeysConfig

| 参数名 | 类型 | 示例 | 默认值 | 作用 |
|---|---|---|---|---|
| `TranscodeQueue` | `string` | `video:transcode:queue` | `video:transcode:queue` | 转码任务 Redis Stream key |
| `KnowledgeVideoTranscodeQueue` | `string` | `knowledge_video:transcode:stream` | `knowledge_video:transcode:stream` | 知识点视频独立转码队列；当前 `cmd/dlqctl` 不支持该队列 |
| `VectorizeQueue` | `string` | `video:vectorize:queue` | `video:vectorize:queue` | 向量化任务 Redis Stream key |
| `VectorPrepareQueue` | `string` | `video:vector:prepare` | `video:vector:prepare` | hierarchical 向量化 prepare 阶段 Redis Stream key |
| `VectorCoarseQueue` | `string` | `video:vector:coarse` | `video:vector:coarse` | hierarchical 向量化 coarse 阶段 Redis Stream key |
| `VectorRefineQueue` | `string` | `video:vector:refine` | `video:vector:refine` | hierarchical 向量化 refine 阶段 Redis Stream key |
| `VectorFinalizeQueue` | `string` | `video:vector:finalize` | `video:vector:finalize` | hierarchical 向量化 finalize 阶段 Redis Stream key |
| `VideoReactionQueue` | `string` | `video:reaction:queue` | `video:reaction:queue` | 视频反馈异步队列 key |
| `VideoReactionCounts` | `string` | `video:reaction:counts:` | `video:reaction:counts:` | 视频反馈计数 key 前缀 |
| `VideoReactionUser` | `string` | `video:reaction:user:` | `video:reaction:user:` | 用户视频反馈状态 key 前缀 |
| `SegmentReactionQueue` | `string` | `segment:reaction:queue` | `segment:reaction:queue` | 视频片段反馈异步队列 key |
| `SegmentReactionCounts` | `string` | `segment:reaction:counts:` | `segment:reaction:counts:` | 视频片段反馈计数 key 前缀 |
| `SegmentReactionUser` | `string` | `segment:reaction:user:` | `segment:reaction:user:` | 用户视频片段反馈状态 key 前缀 |
| `TranscodeStatus` | `string` | `video:transcode:status:` | `video:transcode:status:` | 转码任务状态 key 前缀 |
| `RuntimeActiveCounter` | `string` | `video:runtime:active:` | `video:runtime:active:` | 运行中任务计数 key 前缀 |
| `RandomPlayRecent` | `string` | `video:random_play:recent:` | `video:random_play:recent:` | 每个用户的近期播放去重记录 |
| `RandomPlayBucket` | `string` | `video:random_play:bucket:` | `video:random_play:bucket:` | 每个用户的预取推荐 bucket |

说明：任务队列的死信流不是独立配置项，而是按 `<queue-key>:dlq` 派生。例如 `TranscodeQueue=video:transcode:queue` 时，死信流为 `video:transcode:queue:dlq`。`cmd/dlqctl` 会复用这些配置值来定位对应 DLQ。

### 3.10 PostgresConfig

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `DSN` | `string` | `host=... user=...` | PostgreSQL 连接串 |
| `MaxOpenConns` | `int` | `20` | 最大打开连接数 |
| `MaxIdleConns` | `int` | `10` | 最大空闲连接数 |
| `ConnMaxLifetime` | `int` | `300` | 连接最大生命周期，单位秒 |
| `ConnMaxIdleTime` | `int` | `60` | 空闲连接最大保留时间，单位秒 |

### 3.11 RustFSConfig

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `Endpoint` | `string` | 本地 `localhost:9000`；HTTP 服务生产 COS URL | 对象存储地址；交付配置使用 COS，不再使用旧 RustFS 地址 |
| `AccessKey` | `string` | `""` | AccessKey，生产/本地通过环境变量注入 |
| `SecretKey` | `string` | `""` | SecretKey，生产/本地通过环境变量注入 |
| `Bucket` | `string` | `video-object-storage` | 存储桶名称 |
| `UseSSL` | `bool` | `false` | 是否走 HTTPS |
| `Region` | `string` | 本地空；生产 `ap-beijing` | S3/COS region |
| `BucketLookup` | `string` | `auto` / `dns` | Bucket 解析方式 |

### 3.12 TransConfig

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `WorkerCount` | `int` | `2` | 转码 worker 并发数 |
| `QueueSize` | `int` | `50` | 预留配置位，当前逻辑不是主要控制器 |
| `Mode` | `string` | `fast` | 转码模式 |
| `TaskTimeoutMinutes` | `int` | `30` | 单个转码任务超时分钟数 |
| `ShutdownTimeoutSec` | `int` | `120` | 关闭 worker 时允许的等待时间 |

### 3.13 VectorWorkerConfig

#### 3.13.1 基础模式参数

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `Mode` | `string` | `hierarchical` | 向量化模式 |
| `CoarseSegmentSec` | `int` | `30` | coarse 粗分段长度 |
| `RefineMinSegmentSec` | `int` | `10` | 细分段最短时长 |
| `RefineMaxSegmentSec` | `int` | `60` | 细分段最长时长 |

#### 3.13.2 LLM 参数

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `LLMModel` | `string` | 本地/生产 `qwen3.7-plus` | 用于 hierarchical 内容分段的模型 |
| `LLMTimeoutMinutes` | `int` | `5` | LLM 调用超时时间；不要使用旧的 2 分钟示例 |

#### 3.13.3 Tail Alignment 参数

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `TailAlignmentEnabled` | `bool` | `true` | 是否启用尾部对齐 |
| `TailAlignmentConfigured` | `bool` | `true` | 是否显式配置过 tail alignment |
| `TailAlignmentMaxExtendSec` | `int` | `3` | 结尾最多往后补几秒 |
| `TailAlignmentProbeStepSec` | `int` | `1` | 每次探测的步长 |
| `TailAlignmentMaxOverlapSec` | `int` | `6` | 允许与下一段的最大重叠 |

#### 3.13.4 ASR / 并发参数

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `SegmentWindowSec` | `int` | `30` | 非 hierarchical 下固定窗口长度 |
| `SegmentStepSec` | `int` | `30` | 非 hierarchical 下窗口步长 |
| `ASRWorkers` | `int` | `8` | ASR worker 数；运行时还会受代码上限约束 |
| `CoarseWorkers` | `int` | `60` | coarse worker 相关参数，当前还影响视频级 worker 数 |
| `EmbedBatch` | `int` | `10` | embedding 批量大小 |
| `SampleCount` | `int` | `6` | sample 模式采样段数 |
| `SampleDurSec` | `int` | `30` | sample 模式每段时长 |
| `TaskTimeoutMinutes` | `int` | `30` | 单个 vectorize 任务超时 |
| `ShutdownTimeoutSec` | `int` | `120` | 向量化 worker 关闭等待时间 |

### 3.14 VectorStageWorkersConfig

`VectorStageWorkers` 控制 hierarchical 向量化四个 Redis 阶段各自启动多少个 consumer。`full` 和 `sample` 模式不使用这些阶段队列。

| 参数名 | 类型 | 示例 | 默认值 | 作用 |
|---|---|---|---|---|
| `Prepare` | `int` | `1` | `1` | `video:vector:prepare` 阶段 consumer 数 |
| `Coarse` | `int` | `2` | `2` | `video:vector:coarse` 阶段 consumer 数 |
| `Refine` | `int` | `2` | `2` | `video:vector:refine` 阶段 consumer 数 |
| `Finalize` | `int` | `1` | `1` | `video:vector:finalize` 阶段 consumer 数 |

### 3.15 WorkerPoolsConfig

`WorkerPools` 是按名称配置的并发池 map，用于单个阶段内部的 ants pool 并发；它不创建新的 Redis 队列。当前常用 key 如下：

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `vector.coarse.Size` | `int` | `12` / `60` | vector coarse 阶段并发池大小 |
| `vector.sample_asr.Size` | `int` | `8` / `30` | vector sample ASR 阶段并发池大小 |
| `vector.refine_asr.Size` | `int` | `8` / `30` | vector refine ASR 阶段并发池大小 |

说明：如果对应 pool 未配置或 `Size <= 0`，代码会回退到 `VectorWorker` 中的相关并发参数。

### 3.16 EmbeddingConfig

| 参数名 | 类型 | 作用 |
|---|---|---|
| `Options.Model` | `string` | Embedding 模型名 |
| `BaseURL` | `string` | Embedding 服务地址 |
| `APIKey` | `string` | Embedding 服务密钥；生产环境建议用环境变量注入 |

说明：推荐链路的 embedding 客户端读取 API Key 的优先级是 `DASHSCOPE_API_KEY`、`OPENAI_API_KEY`、`EMBEDDING_API_KEY`、配置文件 `embedding.api-key`。

### 3.17 ASRConfig

| 参数名 | 类型 | 作用 |
|---|---|---|
| `Options.Model` | `string` | 普通 ASR 模型名 |
| `Options.WSModel` | `string` | WebSocket 实时 ASR 模型名 |
| `Options.WSFallbacks` | `[]string` | WebSocket ASR 模型回退列表，首选模型额度不足时按顺序尝试 |
| `BaseURL` | `string` | ASR 服务地址 |
| `WSURL` | `string` | ASR WebSocket 地址；也可由 `ASR_WS_URL` 覆盖 |
| `APIKey` | `string` | ASR 服务密钥；生产环境建议用环境变量注入 |

说明：向量化 worker 初始化 AI client 时，API Key 的优先级是 `DASHSCOPE_API_KEY`、`OPENAI_API_KEY`、`ASR_API_KEY`、配置文件 `asr.api-key`、配置文件 `embedding.api-key`。

### 3.18 AIConfig

| 参数名 | 类型 | 示例 | 默认值 | 作用 |
|---|---|---|---|---|
| `EmbeddingDim` | `int` | `1536` | `1536` | 向量维度；用于本地 fallback embedding 和 vector worker 写入 embedding 前的维度标准化 |

### 3.19 RecommendationConfig

| 参数名 | 类型 | 默认值 | 作用 |
|---|---|---|---|
| `Engine` | `string` | `knowledge_match` | 随机播放的主推荐链路：`knowledge_match`、`gorse` 或 `recbole`；当前两份示例配置均设置为 `recbole` |
| `RandomPlayDedupeWindowSec` | `int` | `1800` | 同一用户近期播放去重窗口（秒） |
| `RandomPlayRecentMaxSize` | `int` | `200` | 每位用户保存的近期播放去重记录上限 |

### 3.20 GorseConfig

Gorse 是可选的外部候选服务。只有 `Recommendation.Engine=gorse` 时才作为主推荐链路使用；HTTP 服务示例配置会启用 `SyncEnabled`，但交付配置 `deployment/config/video_prod.yml` 关闭同步和回写，并以 RecBole 为线上推荐引擎。根 Compose 运行一个 Gorse 服务而不是集群；本地服务默认 endpoint 为 `localhost:8087`，诊断 override 从宿主机暴露 master HTTP `localhost:8088`。

| 参数名 | 类型 | 当前示例 / 交付值 | 作用 |
|---|---|---|---|
| `Endpoint` | `string` | 本地 `http://localhost:8087`；Compose/交付 `http://gorse:8088` | Gorse server 地址；宿主机诊断 override 为 `http://localhost:8088` |
| `APIKey` | `string` | 空 | Gorse API key，可由 `GORSE_API_KEY` 覆盖 |
| `TimeoutSeconds` | `int` | `2` | 调用 Gorse 的超时秒数 |
| `ShadowMode` | `bool` | `false` | 仅观测候选结果，不切换主链路 |
| `SyncEnabled` / `SyncIntervalMins` | `bool` / `int` | HTTP 服务示例 `true` / `60`；交付配置 `false` / `60` | 是否启动周期数据同步及其间隔 |
| `WriteBackEnabled` | `bool` | HTTP 服务示例 `true`；交付配置 `false` | 是否将在线反馈写回 Gorse |
| `CandidateLimit` | `int` | `100` | 一次向 Gorse 请求的候选数 |
| `EnableGate` / `MinFeedbackCount` / `MinRecommendItems` | `bool` / `int` / `int` | HTTP/交付示例均为 `true` / `20` / `1` | 切换前的数据量与候选数保护条件 |
| `CleanupEnabled` / `DataRetentionDays` | `bool` / `int` | HTTP/交付示例均为 `true` / `30` | 为未来同步数据清理预留；当前生产代码未消费这两个字段，不会自动删除数据 |

### 3.21 AuthConfig

| 参数名 | 类型 | 本地示例 | HTTP 服务生产示例 | 交付回退/说明 |
|---|---|---|---|---|
| `JWTSecret` | `string` | 配置文件仅含本地开发回退值 | 配置为空，必须由 `JWT_SECRET` 注入 | `JWT_SECRET` 优先于 YAML；服务启动要求至少 32 个字符 |
| `JWTExpireHour` | `int` | `8` | `24`（`configs/video_prod.yml`） | `deployment/config/video_prod.yml` 未设置时由代码回退到 `8` 小时 |

管理员登录使用 `POST /api/auth/login`，只接受启用的 `sys_user` 管理员；受保护请求发送 `Authorization: Bearer <access_token>`。本服务没有创建或重置管理员账号的 API，部署前必须通过现有身份/数据库运维流程预置符合条件的 `sys_user` 记录和密码哈希。不要把真实签名密钥或默认密码写入仓库。

### 3.22 KnowledgeVideoStorageConfig

| 参数名 | 类型 | 本地示例 | 生产/交付示例 | 作用 |
|---|---|---|---|---|
| `Endpoint` | `string` | `localhost:9000` | COS endpoint（HTTP 服务为 `https://cos.ap-beijing.myqcloud.com`） | 知识点视频对象存储地址 |
| `AccessKey` / `SecretKey` | `string` | 环境注入 | 环境注入 | 独立对象存储凭证 |
| `Bucket` | `string` | `knowledge-point-videos` | `knowledge-point-videos` | 知识点视频专用 bucket |
| `UseSSL` | `bool` | `false` | `true` | 是否使用 HTTPS |
| `Region` | `string` | 空 | `ap-beijing` | S3/COS region |
| `BucketLookup` | `string` | `auto` | `dns` | bucket 解析方式 |
| `MediaRoutePrefix` | `string` | `/knowledge-video-media` | 同左 | HLS 媒体代理前缀 |
| `TempPath` | `string` | `./storage/tmp/knowledge_video` | 同结构；缺失时使用系统临时目录 | 导入/转码临时目录 |
| `MaxArchiveBytes` | `int64` | `1073741824` | `1073741824` | ZIP 最大字节数（1 GiB） |
| `MaxExpandedBytes` | `int64` | `4294967296` | `4294967296` | 解压后总大小上限（4 GiB） |
| `MaxEntryBytes` | `int64` | `536870912` | `536870912` | 单个归档条目上限（512 MiB） |
| `MaxEntries` | `int` | `1000` | `1000` | 归档条目数量上限 |

### 3.23 KnowledgeVideoWorkerConfig

| 参数名 | 类型 | 本地/生产默认 | 作用 |
|---|---|---:|---|
| `WorkerCount` | `int` | `2` | 知识点视频转码 consumer 数；缺失或小于 1 时回退到 `2` |
| `TaskTimeoutMinutes` | `int` | `30` | 单个知识点视频转码任务超时分钟数 |
| `ShutdownTimeoutSec` | `int` | `120` | worker 关闭时等待任务完成的秒数 |

该配置块没有独立的消费阻塞、临时目录保留或 DLQ 管理字段；队列 key 来自 `RedisKeys.KnowledgeVideoTranscodeQueue`。

## 4. 环境变量参数

项目中当前明确使用到的环境变量包括：

| 环境变量 | 来源位置 | 作用 |
|---|---|---|
| `HTTP_ADDR` | `internal/http/app/app.go` | 覆盖 HTTP 服务监听地址，默认 `:8081` |
| `JWT_SECRET` | `internal/config/defaults.go`、`internal/http/app/app.go` | 管理员 JWT 签名密钥；优先于 `Auth.JWTSecret`，至少 32 个字符 |
| `VIDEO_APP_ENV_FILE` | `internal/config/loader.go` | 指定额外 dotenv 文件；`.env` 先加载，shell 已存在变量不会被文件覆盖 |
| `REDIS_ADDR` | `internal/config/loader.go` | 覆盖 `Redis.Addr` |
| `CONFIG_FILE` | `internal/config/loader.go` | 覆盖默认配置文件路径，优先级高于 `VIDEO_CONFIG_FILE` |
| `VIDEO_CONFIG_FILE` | `internal/config/loader.go` | 覆盖默认配置文件路径 |
| `POSTGRES_DSN` | `internal/config/loader.go` | 覆盖 `Postgres.DSN` |
| `REDIS_PASSWORD` | `internal/config/loader.go` | 覆盖 `Redis.Password` |
| `COS_SECRET_ID` | `internal/config/loader.go` | 覆盖 `RustFS.AccessKey` |
| `COS_SECRET_KEY` | `internal/config/loader.go` | 覆盖 `RustFS.SecretKey` |
| `RUSTFS_ACCESS_KEY` | `internal/config/loader.go` | 当 COS 变量未设置时覆盖 AccessKey |
| `RUSTFS_SECRET_KEY` | `internal/config/loader.go` | 当 COS 变量未设置时覆盖 SecretKey |
| `GORSE_API_KEY` | `internal/config/loader.go` | 覆盖 `Gorse.APIKey` |
| `GORSE_ENDPOINT` | `internal/config/loader.go` | 覆盖 `Gorse.Endpoint`；本地诊断通常使用 `http://localhost:8088` |
| `GORSE_SERVER_API_KEY` | `gorse/entrypoint.sh` | 根 Compose 中 Gorse 服务端校验的 API key；必须与 `GORSE_API_KEY` 完全一致 |
| `GORSE_VERSION` | `docker-compose.yml` | Gorse 镜像插值版本，默认 `0.5.11`；仅由 shell 或 Compose 根 `.env` 提供，服务级 `env_file` 不参与插值 |
| `DASHSCOPE_API_KEY` | `internal/config/loader.go`、embedding 客户端、vector worker AI client | DashScope / 百炼兼容接口 API Key |
| `OPENAI_API_KEY` | `internal/config/loader.go`、embedding 客户端、vector worker AI client | OpenAI 兼容接口 API Key 兜底 |
| `EMBEDDING_API_KEY` | `internal/config/loader.go`、embedding 客户端 | 推荐链路 embedding API Key 兜底 |
| `DASHSCOPE_BASE_URL` | vector worker AI client | 向量化 worker 的 OpenAI 兼容接口地址 |
| `OPENAI_BASE_URL` | vector worker AI client | OpenAI 兼容接口地址兜底 |
| `ASR_API_KEY` | vector worker AI client | ASR API Key 兜底 |
| `ASR_BASE_URL` | vector worker AI client | ASR HTTP 基础地址 |
| `ASR_WS_URL` | `internal/config/defaults.go`、vector worker AI client | ASR WebSocket 地址 |
| `ASR_WS_MODEL` | vector worker AI client | ASR WebSocket 首选模型 |
| `EMBED_MODEL` | vector worker AI client | Embedding 模型名 |
| `UPLOAD_BENCH_BASE_URL` | `tools/upload_bench` | 上传压测工具目标服务地址 |
| `SOURCE_DSN` | `tools/db_migrate_except_video_tables` | 数据迁移工具源库 DSN |
| `TARGET_DSN` | `tools/db_migrate_except_video_tables` | 数据迁移工具目标库 DSN |
| `RECBOLE_TRAINER_ENABLED` | `internal/worker/recboletrainer` | 是否注册训练调度器；默认关闭，独立 trainer 必须设为 `true` |
| `SOURCE_MINIO_ENDPOINT` | `cmd/knowledgevideo-import/main.go` | 知识点视频初始化导入工具的源 MinIO S3 API 地址，必填 |
| `SOURCE_MINIO_BUCKET` | `cmd/knowledgevideo-import/main.go` | 源 MinIO bucket，必填 |
| `SOURCE_MINIO_ACCESS_KEY` | `cmd/knowledgevideo-import/main.go` | 源 MinIO AccessKey，必填 |
| `SOURCE_MINIO_SECRET_KEY` | `cmd/knowledgevideo-import/main.go` | 源 MinIO SecretKey，必填 |
| `SOURCE_MINIO_USE_SSL` | `cmd/knowledgevideo-import/main.go` | 源 MinIO 是否走 HTTPS，默认 `false` |
| `SERVICE_DIR` | `recbole-training/scripts/run_recbole_pipeline.sh` | HTTP 服务工具和配置目录 |
| `CONFIG_FILE`（RecBole 脚本） | RecBole shell pipeline | 传给导出/导入工具的配置路径；与服务配置选择同名但由脚本显式传递 |
| `MODEL_VERSION`、`MODEL_NAME`、`RECBOLE_MODEL` | RecBole shell pipeline | 候选版本、线上模型名和模型类 |
| `DATASET`、`DIM`、`EPOCHS`、`SAMPLE_LIMIT`、`DAYS_BACK` | RecBole shell pipeline | 数据集前缀、embedding 维度、训练轮数、样本上限和回溯天数 |
| `PYTHON_BIN`、`DATA_ROOT`、`DATA_DIR`、`ARTIFACT_DIR`、`BASELINE_METRICS` | RecBole shell pipeline | Python 可执行文件、数据/产物目录和 baseline 指标文件 |
| `PUBLISH_GATE_ENABLED` | RecBole shell pipeline | 是否运行发布门禁，默认 `true` |

### 4.1 `HTTP_ADDR`

- 类型：`string`
- 默认值：`:8081`
- 示例：`HTTP_ADDR=:8081`
- 作用：覆盖 HTTP API 服务监听地址

### 4.2 `CONFIG_FILE` / `VIDEO_CONFIG_FILE`

- 类型：`string`
- 作用：覆盖默认配置文件加载路径
- 优先级：`CONFIG_FILE` 高于 `VIDEO_CONFIG_FILE`，两者都高于系统默认配置文件选择逻辑

### 4.3 `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`

- 类型：`string`
- 作用：作为对象存储访问凭证来源
- 使用场景：`COS_SECRET_ID` / `COS_SECRET_KEY` 未设置时，由环境变量注入敏感信息

### 4.4 AI 服务相关环境变量

- `DASHSCOPE_API_KEY`、`OPENAI_API_KEY`、`EMBEDDING_API_KEY` 用于推荐链路 embedding 客户端。
- `DASHSCOPE_API_KEY`、`OPENAI_API_KEY`、`ASR_API_KEY` 用于向量化 worker 的 ASR、LLM 和 Embedding 客户端。
- `DASHSCOPE_BASE_URL`、`OPENAI_BASE_URL` 会覆盖向量化 worker 的 OpenAI 兼容接口地址。
- `ASR_BASE_URL`、`ASR_WS_URL`、`ASR_WS_MODEL` 控制 ASR 服务地址和 WebSocket 模型。
- `EMBED_MODEL` 控制向量化 worker 调用的 embedding 模型。

生产环境建议优先通过环境变量或密钥管理系统注入 API Key，不要把真实密钥提交到配置文件。

### 4.5 工具脚本环境变量

- `UPLOAD_BENCH_BASE_URL`：上传压测工具目标服务地址；未设置时会结合 `HTTP_ADDR` 生成默认地址。
- `SOURCE_DSN`、`TARGET_DSN`：数据库迁移工具的源库和目标库 DSN，也可以通过 `-source-dsn`、`-target-dsn` 显式传入。
- `MODEL_VERSION`、`MODEL_NAME`、`RECBOLE_MODEL`：本次模型版本、线上模型名和 RecBole 算法；默认分别为时间戳版本、`recbole`、`BPR`。
- `DATASET`、`SAMPLE_LIMIT`、`DAYS_BACK`：导出数据集名称、交互样本上限和回看天数；默认 `video_app`、`10000`、`30`。
- `DIM`、`EPOCHS`、`PYTHON_BIN`：embedding 维度、训练轮数和 Python 解释器；默认 `64`、`20`，解释器优先使用 `.venv/bin/python`。
- `DATA_ROOT`、`DATA_DIR`、`ARTIFACT_DIR`、`BASELINE_METRICS`：RecBole atomic 文件、训练产物和上一版指标文件的位置。
- `PUBLISH_GATE_ENABLED`：是否执行 RecBole 发布门禁，默认 `true`。门禁的默认阈值为 `Recall@20 >= 0.01`、`NDCG@20 >= 0.005`，且相对上一版 `NDCG@20` 的降幅不超过 `20%`；如需调整，直接向 `recbole_recommendation.publish_gate` 传入对应命令行参数。

RecBole shell 脚本不消费 `MIN_RECALL_AT_20`、`MIN_NDCG_AT_20`、`MAX_RELATIVE_NDCG_DROP` 或 `ARTIFACT_RETENTION_DAYS`。门禁阈值使用 `--min-recall-at-20`、`--min-ndcg-at-20`、`--max-relative-ndcg-drop`、`--min-positive-rows`、`--min-positive-users` CLI flags，后两项默认均为 `1`。当前 exporter 尚未写出 `positive_rows` / `positive_users` 字段，`publish_gate` 只有在指标文件存在这些字段时才执行对应检查；因此当前流水线对这两个数据量条件是 fail-open，不能把默认值 `1` 理解为已经强制生效。

### 4.6 对象存储迁移工具参数

`tools/migrate_rustfs_bucket` 用于把旧 MinIO 桶数据迁移到 RustFS。默认值：

| 参数 | 默认值 | 作用 |
|---|---|---|
| `--source-endpoint` | `10.200.10.12:9000` | 旧 MinIO S3 API 地址 |
| `--source-access-key` | 从 `MIGRATE_SOURCE_ACCESS_KEY` 读取 | 源端 AccessKey |
| `--source-secret-key` | 从 `MIGRATE_SOURCE_SECRET_KEY` 读取 | 源端 SecretKey |
| `--source-bucket` | `video-object-storage` | 源端 Bucket |
| `--target-endpoint` | `10.200.10.201:9001` | RustFS S3 API 地址 |
| `--target-access-key` | 从 `MIGRATE_TARGET_ACCESS_KEY` 读取 | 目标端 AccessKey |
| `--target-secret-key` | 从 `MIGRATE_TARGET_SECRET_KEY` 读取 | 目标端 SecretKey |
| `--target-bucket` | `video-object-storage` | 目标端 Bucket |
| `--prefix` | 空 | 只迁移指定对象 key 前缀 |
| `--workers` | `4` | 并发复制 worker 数 |
| `--overwrite` | `false` | 目标对象已存在时是否覆盖 |
| `--dry-run` | `true` | 是否只预演不复制 |

常用命令：

```bash
cd video-service
go run ./tools/migrate_rustfs_bucket
go run ./tools/migrate_rustfs_bucket --dry-run=false
```

## 5. HTTP API 参数

主要路由注册在：

- `video-service/internal/http/router/router.go`

主要 DTO 定义在：

- `internal/http/dto/common.go`
- `internal/http/dto/upload.go`
- `internal/http/dto/video.go`
- `internal/http/dto/recommend.go`

### 5.1 通用返回结构

#### 成功返回

```json
{
  "success": true,
  "data": { ... }
}
```

字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `success` | `bool` | 是否成功 |
| `data` | `any` | 业务数据 |

#### 失败返回

```json
{
  "success": false,
  "error": {
    "code": "...",
    "message": "..."
  }
}
```

字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `success` | `bool` | 是否成功 |
| `error.code` | `string` | 错误码 |
| `error.message` | `string` | 错误描述 |

业务 API 使用上述 envelope。`/healthz` 和 `/api/healthz` 返回健康状态 JSON，`/swagger/*any` 返回 Swagger 资源，`/videos/*filepath` 与 `/knowledge-video-media/*` 可能返回原始媒体、Range/206 响应；这些入口不是统一业务 envelope。

### 5.2 健康检查与静态入口

#### `GET /healthz`
#### `GET /api/healthz`

- 无请求参数
- 返回 `{ "status": "ok" }`

#### `GET /swagger/*any`

- 路径参数：`*any`
- 作用：访问 Swagger 页面资源

#### `GET /api/system/metrics`

- 鉴权：管理员 JWT
- 无请求参数
- 作用：查询系统运行指标

#### `GET /videos/*filepath`

- 路径参数：`*filepath`
- 作用：代理访问对象存储中的视频资源
- 说明：实际媒体代理路由前缀由 `Storage.MediaRoutePrefix` 控制，默认是 `/videos`；如果配置成其他前缀，仍保留 `/videos/*filepath` 兼容路由

### 5.3 上传相关接口参数

本节所有接口均要求管理员 JWT，并从已验证的管理员身份派生上传者 ID；请求需发送 `Authorization: Bearer <access_token>`。

#### `POST /api/videos`

请求类型：`multipart/form-data`

说明：该接口用于普通 multipart 上传。大文件或需要断点续传时，建议使用后面的分片上传接口。

表单字段：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `file` | `file` | 是 | 上传的视频文件 |
| `title` | `string` | 否 | 视频标题 |
| `description` | `string` | 否 | 视频描述 |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `video_id` | `uint64` | 视频 ID |
| `task_id` | `string` | 转码任务 ID |
| `raw_url` | `string` | 原始视频访问地址 |
| `hls_url` | `string` | HLS 播放地址 |
| `file_name` | `string` | 存储后的文件名 |

#### `POST /api/videos/archive`

请求类型：`multipart/form-data`

说明：该接口用于兼容 ZIP 归档直传。后端会先把 ZIP 落盘再流式解包，避免整包读入内存；如果调用方需要断点续传，建议使用 ZIP 分片上传接口。

表单字段：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `file` | `file` | 是 | 包含视频文件的 zip 归档 |
| `description` | `string` | 否 | 批量上传时写入每个视频的描述 |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `total` | `int` | 归档中识别到的候选文件总数 |
| `uploaded` | `int` | 成功上传数量 |
| `failed` | `int` | 上传失败数量 |
| `skipped` | `int` | 跳过数量 |
| `videos` | `[]UploadVideoData` | 成功上传的视频列表 |
| `errors` | `[]UploadArchiveError` | 失败文件及错误信息 |
| `skipped_files` | `[]string` | 被跳过的文件名 |

#### `POST /api/videos/uploads`

请求类型：`application/json`

作用：创建普通视频分片上传会话。

请求字段：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `file_name` | `string` | 是 | 原始视频文件名 |
| `content_type` | `string` | 否 | 文件 MIME 类型 |
| `title` | `string` | 否 | 视频标题 |
| `description` | `string` | 否 | 视频描述 |
| `file_size` | `int64` | 是 | 原始文件总字节数，必须大于 0 |
| `chunk_size` | `int64` | 是 | 单分片字节数，必须大于 0 |
| `total_chunks` | `int` | 是 | 分片总数，必须等于 `ceil(file_size / chunk_size)` |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `upload_id` | `string` | 分片上传会话 ID |
| `file_name` | `string` | 原始文件名 |
| `file_size` | `int64` | 原始文件总字节数 |
| `chunk_size` | `int64` | 单分片字节数 |
| `total_chunks` | `int` | 分片总数 |
| `uploaded_chunks` | `[]int` | 已上传且大小校验通过的分片序号 |
| `completed` | `bool` | 是否所有分片已上传完成 |

#### `POST /api/videos/archive/uploads`

请求类型：`application/json`

作用：创建 ZIP 批量导入的分片上传会话。请求和响应字段与 `POST /api/videos/uploads` 基本一致，但要求 `file_name` 是 `.zip` 文件；`description` 会写入 ZIP 内每个成功导入的视频。

#### `PUT /api/videos/uploads/:uploadId/chunks/:chunkIndex`

请求类型：原始二进制请求体

作用：上传一个分片。普通视频和 ZIP 批量导入共用该接口。

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `uploadId` | `string` | 是 | 分片上传会话 ID |
| `chunkIndex` | `int` | 是 | 分片序号，从 `0` 开始 |

请求体：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| request body | `binary` | 是 | 当前分片内容 |

说明：除最后一个分片外，分片大小必须等于创建会话时的 `chunk_size`；最后一个分片大小必须等于文件剩余字节数。大小不正确的分片不会计入已上传状态。

响应字段：同 `ChunkedUploadData`，即 `upload_id`、`file_name`、`file_size`、`chunk_size`、`total_chunks`、`uploaded_chunks`、`completed`。

#### `GET /api/videos/uploads/:uploadId`

作用：查询分片上传状态。普通视频和 ZIP 批量导入共用该接口。

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `uploadId` | `string` | 是 | 分片上传会话 ID |

响应字段：同 `ChunkedUploadData`。

#### `GET /api/videos/archive/batches/:batchId/progress`

- 鉴权：管理员 JWT。
- 路径参数：`batchId`，正整数；归档处理批次 ID。
- 返回：`batch_id`、`status`、`total`、`processed`、`succeeded`、`failed`、`skipped`、`errors` 等归档处理进度字段；具体字段以当前 Swagger schema 为准。

#### `POST /api/videos/uploads/:uploadId/complete`

作用：完成普通视频分片上传。服务端会校验所有分片、合并本地文件、上传对象存储、创建视频记录并投递转码任务。

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `uploadId` | `string` | 是 | 分片上传会话 ID |

响应字段：同 `POST /api/videos`，即 `video_id`、`task_id`、`raw_url`、`hls_url`、`file_name`。

#### `POST /api/videos/archive/uploads/:uploadId/complete`

作用：完成 ZIP 批量分片上传。服务端会校验并合并 ZIP 文件，然后从本地 ZIP 文件流式解包导入视频。

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `uploadId` | `string` | 是 | 分片上传会话 ID |

响应字段：同 `POST /api/videos/archive`，即 `total`、`uploaded`、`failed`、`skipped`、`videos`、`errors`、`skipped_files`。

#### `POST /api/videos/:id/cover`

请求类型：`multipart/form-data`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

表单字段：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `file` | `file` | 是 | 上传的封面文件 |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `video_id` | `uint64` | 视频 ID |
| `cover_url` | `string` | 封面访问地址 |

### 5.4 视频管理接口参数

鉴权边界：`PATCH /api/videos/:id`、`DELETE /api/videos/:id`、`POST /api/videos/:id/publish` 和 `POST /api/videos/:id/recommend` 要求管理员 JWT；本节其余列表、播放和反馈接口保持公开。

#### `GET /api/videos`

Query 参数：

| 参数名 | 类型 | 必填 | 默认值 | 作用 |
|---|---|---|---|---|
| `type` | `string` | 否 | `ALL` | 列表过滤类型，可选 `ALL`、`RAW`、`HLS` |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `videos` | `[]VideoItem` | 视频列表 |
| `total` | `int` | 总数 |
| `type` | `string` | 当前过滤类型 |

#### `PATCH /api/videos/:id`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

JSON Body：

| 参数名 | 类型 | 必填 | 约束 | 作用 |
|---|---|---|---|---|
| `title` | `string` | 是 | `required,max=200` | 标题 |
| `description` | `string` | 否 | `max=5000` | 描述 |

#### `DELETE /api/videos/:id`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

#### `POST /api/videos/:id/publish`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

JSON Body：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `is_published` | `bool` | 是 | 是否发布 |

#### `POST /api/videos/:id/recommend`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

JSON Body：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `is_recommend` | `bool` | 是 | 是否设为推荐 |
| `user_id` | `uint64` | 否 | 操作人 ID |
| `recommend_level` | `int16` | 否 | 推荐等级 |
| `recommend_score` | `float64` | 否 | 推荐得分 |

#### `POST /api/videos/:id/reactions`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

JSON Body：

| 参数名 | 类型 | 必填 | 约束 | 作用 |
|---|---|---|---|---|
| `user_id` | `uint64` | 是 | `> 0` | 用户 ID |
| `reaction_type` | `string` | 是 | `like`、`double_like`、`dislike` | 视频反馈类型；重复提交同一反馈会取消 |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `video_id` | `uint64` | 视频 ID |
| `user_id` | `uint64` | 用户 ID |
| `reaction_type` | `string` | 本次反馈类型 |
| `active` | `bool` | 当前反馈是否处于激活状态 |
| `like_count` | `int64` | 点赞数 |
| `double_like_count` | `int64` | 双赞数 |
| `updated` | `bool` | 是否完成更新 |

#### `GET /api/videos/:id/reaction-counts`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `video_id` | `uint64` | 视频 ID |
| `like_count` | `int64` | 点赞数 |
| `double_like_count` | `int64` | 双赞数 |

#### `GET /api/video-segments/random-play`

Query 参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `user_id` | `uint64` | 否 | 传入时按 `Recommendation.Engine` 使用 RecBole、Gorse 或知识点召回；未传时 handler 使用默认用户 `6` 后走同一链路 |

作用：刷新返回一个可播放的视频片段。当前示例配置使用 `recbole`；未传 `user_id` 时会按默认用户 `6` 尝试个性化召回，缺少 active 模型、用户 embedding 或候选为空时才回退到随机可播放片段。`user_id` 非正整数或非法字符串会返回参数错误。

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `video_id` | `uint64` | 所属视频 ID |
| `segment_id` | `uint64` | 片段 ID |
| `start_time` | `float64` | 片段起始时间（秒） |
| `end_time` | `float64` | 片段结束时间（秒） |
| `text` | `string` | 片段文本内容 |
| `play_url` | `string` | 播放地址 |
| `video` | `VideoItem` | 所属视频信息 |

#### `POST /api/video-segments/:id/reactions`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频片段 ID |

JSON Body：

| 参数名 | 类型 | 必填 | 约束 | 作用 |
|---|---|---|---|---|
| `user_id` | `uint64` | 是 | `> 0` | 用户 ID |
| `reaction_type` | `string` | 是 | `like`、`double_like`、`dislike` | 反馈类型；重复提交同一反馈会取消 |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `segment_id` | `uint64` | 片段 ID |
| `user_id` | `uint64` | 用户 ID |
| `reaction_type` | `string` | 本次反馈类型 |
| `active` | `bool` | 当前反馈是否处于激活状态 |
| `like_count` | `int64` | 点赞数 |
| `double_like_count` | `int64` | 双赞数 |
| `updated` | `bool` | 是否完成更新 |

#### `GET /api/video-segments/:id/reaction-counts`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频片段 ID |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `segment_id` | `uint64` | 片段 ID |
| `like_count` | `int64` | 点赞数 |
| `double_like_count` | `int64` | 双赞数 |

### 5.5 播放与状态接口参数

#### `GET /api/videos/:id/play`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `play_url` | `string` | 实际播放地址 |
| `video` | `VideoItem` | 视频信息 |

#### `GET /api/videos/:id/similar`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

Query 参数：

| 参数名 | 类型 | 必填 | 默认值 | 作用 |
|---|---|---|---|---|
| `limit` | `int` | 否 | `6` | 返回相似视频数量 |

#### `GET /api/videos/:id/view-count`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 视频 ID |

#### `GET /api/transcode-tasks/:taskId`

- 鉴权：管理员 JWT

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `taskId` | `string` | 是 | 转码任务 ID |

响应字段：

| 字段 | 类型 | 作用 |
|---|---|---|
| `task_id` | `string` | 任务 ID |
| `status` | `string` | 当前状态 |
| `hls_url` | `string` | HLS 地址 |

### 5.6 推荐接口参数

#### `POST /api/recommendations/by-question`

JSON Body：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `question_id` | `uint64` | 否 | 题库中的题目 ID |
| `question_text` | `string` | 否，但当 `question_id` 缺失时必填 | 题目文本 |
| `user_id` | `uint64` | 否 | 用户 ID |
| `limit` | `int` | 否 | 推荐数量，handler 默认 `3`，service 内最终限制不超过 `50` |

#### `GET /api/recommendations`

Query 参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `question_id` | `uint64` | 是 | 题目 ID |
| `user_id` | `uint64` | 否 | 用户 ID |
| `limit` | `int` | 否 | 返回数量 |

#### `POST /api/watch-records`

JSON Body：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `question_id` | `uint64` | 否 | 关联题目 ID |
| `user_id` | `uint64` | 否 | 用户 ID |
| `video_segment_id` | `uint64` | 是 | 视频片段 ID |
| `is_watched` | `bool` | 否 | 是否看过 |
| `watch_duration` | `int` | 否 | 观看时长，要求 `>= 0` |

### 5.7 题库接口参数

#### `GET /api/questions`

Query 参数：

| 参数名 | 类型 | 必填 | 默认值 | 作用 |
|---|---|---|---|---|
| `page` | `int` | 否 | `1` | 页码 |
| `page_size` | `int` | 否 | `20` | 每页数量 |

#### `GET /api/questions/:id`

路径参数：

| 参数名 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `id` | `uint64` | 是 | 题目 ID |

### 5.8 鉴权与管理员会话

#### `POST /api/auth/login`

- 鉴权：公开登录入口。
- 请求类型：`application/json`。

| 字段 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `username` | `string` | 是 | `sys_user.username` |
| `password` | `string` | 是 | 管理员密码；仓库不提供默认密码 |

只接受 `user_type=3`、`status=1`、`deleted=0` 的管理员账号。成功返回 `data.access_token`、`data.expires_at` 和脱敏 `data.admin`；后续管理员请求使用 `Authorization: Bearer <access_token>`。本服务不提供管理员创建或密码重置接口，部署方必须先通过现有身份/数据库运维流程预置账号。

#### `GET /api/auth/me`

- 鉴权：管理员 JWT。
- 无请求参数。
- 返回当前管理员的 `id`、`username`、`real_name`；账号失效或 token 过期返回 `401`。

### 5.9 知识点视频接口

#### `POST /api/admin/knowledge-videos/batches`

- 鉴权：管理员 JWT；上传者 ID 从 token 中取得。
- 请求类型：`multipart/form-data`。

| 字段 | 类型 | 必填 | 作用 |
|---|---|---|---|
| `archive` | `file` | 是 | ZIP 视频归档 |
| `mapping` | `file` | 是 | XLSX 知识点映射 |

XLSX/ZIP 契约：

1. 读取 XLSX 的第一个 worksheet；第一行必须严格为 `id`、`name`、`video_name` 三列，不能有第四列，且至少有一行数据。
2. `id` 必须是已存在的正整数知识点 ID，`name` 必须与知识点字典名称完全一致；同一个知识点可以有多行视频。
3. `video_name` 必须在表内唯一，并按 basename 与 ZIP 中恰好一个视频文件匹配。ZIP 可以包含目录，但重复 basename、映射缺失视频或未被映射引用的额外视频都会拒绝整个批次。
4. 支持 `.mp4`、`.mov`、`.mkv`、`.avi`、`.webm`、`.m4v`；路径穿越、绝对路径、符号链接和其他扩展名会被拒绝。`__MACOSX`、`.DS_Store`、`._*` 元数据会被忽略。

成功返回 `202`，数据含 `batch_id`、`total_count`、`status`、`progress_url`。归档大小、解压总量、单条目大小和条目数受 `KnowledgeVideoStorage` 限制。

#### `GET /api/admin/knowledge-videos/batches/:batchId`

- 鉴权：管理员 JWT。
- 路径参数：`batchId` 正整数。
- 返回：`batch_id`、`total_count`、`ready_count`、`failed_count`、`status`、`videos[]`；每个视频含知识点、文件名、时长、状态和可选错误信息。

#### `GET /api/knowledge-videos/tree`

- 鉴权：公开。
- 无请求参数。
- 返回 `nodes[]` 树结构；节点含 `id`、`parent_id`、`name`、`children` 和关联 `videos`。

#### `GET /api/knowledge-points/:knowledgePointId/video`
#### `GET /api/knowledge-points/:knowledgePointId/videos`

- 鉴权：公开；前者是单数兼容路径，后者是标准复数路径。
- 路径参数：`knowledgePointId` 正整数。
- 返回 `knowledge_point_id`、`knowledge_point_name`、`videos[]`；视频项含 `knowledge_video_id`、源文件名、显示名、时长和 `playback_url`。兼容响应可能同时提供首个视频的 `video_id`、`video_name`、`duration`、`playback_url`。

#### `POST /api/knowledge-videos/:knowledgeVideoId/playbacks`

- 鉴权：公开；用于记录一次播放开始/播放记录。
- 路径参数：`knowledgeVideoId` 正整数。
- JSON body：`user_id`（`uint64`，必填且大于 0）。
- 成功返回空 `data` envelope；重复调用应按业务记录语义处理。

#### `PUT /api/knowledge-videos/:knowledgeVideoId/watch-sessions/:sessionId`

- 鉴权：公开；由播放器按会话重试上报。
- 路径参数：`knowledgeVideoId` 正整数；`sessionId` 由客户端为一次播放会话生成，必须匹配 `^[A-Za-z0-9_-]{16,64}$`，其幂等范围为同一个 `(user_id, knowledge_video_id, session_id)`。
- JSON body：`user_id`（必填且大于 0）、`watched_seconds`（必填，当前 session 的绝对观看秒数，不能为负数）。同一 session 重试只保留较大值；不同 session 的最大值按用户和视频求和，服务端把单次值与汇总值都封顶到视频时长。客户端不应发送增量值，并应为新的实际播放会话生成新的 `sessionId`。
- 返回 `session_id`、`session_watched_seconds`、`total_watched_seconds`、`duration_seconds`、`progress_ratio`、`effective_watch`。累计观看达到视频时长的 `60%` 时 `effective_watch=true`，该信号只进入 RecBole 训练期虚拟 item，不成为在线候选。

安全边界：上述播放记录和观看会话接口当前不使用 JWT，`user_id` 由调用方提供，服务只在观看会话接口确认该用户存在，不校验调用方是否拥有该身份。若流量来自不可信客户端，必须在上游网关/业务服务补充身份绑定、限流和防刷；否则任意有效用户 ID 都可能被冒用并污染 RecBole 训练信号。

#### `GET /knowledge-video-media/hls/:videoId/*filepath`

- 鉴权：公开媒体代理。
- 路径参数：`videoId` 和 `filepath`。
- 返回 HLS/媒体原始响应，可能支持 Range/206，不使用业务 JSON envelope。

### 5.10 管理推荐与内部候选接口

以下 `/api/admin/recommendation/*` 路径全部要求管理员 JWT；响应使用成功 envelope，错误使用通用错误结构。

#### `GET /api/admin/recommendation/overview`

- 无参数；返回当前引擎、Gorse/RecBole 状态和 Redis random-play 状态摘要。

#### `GET /api/admin/recommendation/diagnostics`

- Query：`days`（可选，默认 `14`）、`limit`（可选，最近请求数量）。返回健康检查、数据新鲜度、近期请求、策略效果和任务状态。

#### `GET /api/admin/recommendation/datasources`

- 无参数；返回视频/片段/曝光/观看/reaction/RecBole 数据源计数和比例。

#### `GET /api/admin/recommendation/effects`

- Query：`days`（可选，默认 `14`）；返回按日及按策略的曝光、观看、观看率、平均排名和平均分。

#### `GET /api/admin/recommendation/recbole/performance`

- Query：`metric`、`begin`、`end` 均必填；`begin/end` 使用 RFC3339，`metric` 为性能指标名（如 `Recall@20`、`NDCG@20`）。
- 返回可用指标列表和按模型版本/时间的点序列。

#### `GET /api/admin/recommendation/trace/random-play`

- Query：`user_id` 必填且为正整数，`limit` 可选。
- 返回随机播放链路阶段、候选 item、策略、模型版本和过滤原因。

#### `POST /api/admin/recommendation/trace/by-question`

- JSON body 与 `POST /api/recommendations/by-question` 的 `RecommendByQuestionRequest` 相同；返回题目推荐链路 trace。

#### `GET /api/admin/recommendation/redis-state`

- Query：`user_id` 必填且为正整数；返回该用户 random-play bucket/recent 的 TTL、数量、上限和片段 ID。

#### `GET /api/admin/recommendation/preview/random-play`

- Query：`user_id` 必填且为正整数，`limit` 可选；返回不写入曝光/推荐记录的预览候选列表。

#### `POST /api/admin/recommendation/preview/by-question`

- JSON body 与 `RecommendByQuestionRequest` 相同；返回 `preview_only=true` 的候选列表，不作为正常业务曝光。

#### `GET /api/internal/recommendations/external/recbole`

- 鉴权：应用层当前没有 JWT 或 API key 校验。该路径只供 Gorse external script 或受控内部调用，部署时必须通过反向代理 ACL、网络策略或防火墙限制访问；不是公开 Java/浏览器契约，路径中的 `internal` 本身不是安全边界。
- Query：`user_id` 必填且为正整数，`n` 可选，默认 `100`，最大 `500`。
- 成功返回普通数字 `video_segment_id` 数组（非业务 envelope），供 Gorse external script 使用；知识点视频虚拟 item 不会出现在列表。

## 6. Worker / 内部默认参数

这部分参数不是直接由外部接口传入，但会显著影响系统行为。

### 6.1 HTTP 运行时默认参数

#### HTTP / CORS / 路径默认值

文件：`internal/config/defaults.go`

| 参数 | 默认值 | 作用 |
|---|---|---|
| `HTTPAddr()` | `:8081` | HTTP 服务默认监听地址 |
| `HTTPShutdownTimeout()` | `30s` | HTTP 服务优雅关闭默认超时 |
| `HTTPLogDir()` | `logs` | 默认日志目录 |
| `HTTPSlowRequestThreshold()` | `1000ms` | 慢请求默认阈值 |
| `CORSAllowOrigin()` | `*` | 默认允许来源 |
| `CORSAllowMethods()` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` | 默认允许方法 |
| `CORSAllowHeaders()` | `Origin, Content-Type, Accept, Authorization, X-Requested-With` | 默认允许请求头 |
| `CORSExposeHeaders()` | `Content-Length, Content-Type` | 默认暴露响应头 |
| `CORSMaxAge()` | `86400` | 默认预检缓存时间 |
| `RawPath()` | `os.TempDir()/video-embedding/tmp/raw` | 原视频本地目录兜底值 |
| `HLSPath()` | `os.TempDir()/video-embedding/tmp/hls` | HLS 本地目录兜底值 |
| `VectorTempPath()` | `os.TempDir()/video-embedding/tmp/video_vectorize` | 向量化临时目录兜底值 |
| `MediaRoutePrefix()` | `/videos` | 媒体代理路由兜底值 |
| `RawURLPrefix()` | `/videos/raw` | 原视频 URL 前缀兜底值 |
| `HLSURLPrefix()` | `/videos/hls` | HLS URL 前缀兜底值 |
| `CoverURLPrefix()` | `/videos` | 封面 URL 前缀兜底值 |
| `EmbeddingDim()` | `1536` | embedding 维度兜底值 |
| `ASRWSURL()` | `wss://dashscope.aliyuncs.com/api-ws/v1/inference/` | ASR WebSocket 地址兜底值 |

#### Redis key 默认值

文件：`internal/config/defaults.go`

| 参数 | 默认值 | 作用 |
|---|---|---|
| `TranscodeQueueKey()` | `video:transcode:queue` | 转码队列 |
| `VectorizeQueueKey()` | `video:vectorize:queue` | 向量化队列 |
| `VideoReactionQueueKey()` | `video:reaction:queue` | 视频反馈队列 |
| `VideoReactionCountsPrefix()` | `video:reaction:counts:` | 视频反馈计数前缀 |
| `VideoReactionUserPrefix()` | `video:reaction:user:` | 用户反馈状态前缀 |
| `TranscodeStatusPrefix()` | `video:transcode:status:` | 转码状态前缀 |
| `RuntimeActiveCounterPrefix()` | `video:runtime:active:` | 运行中任务计数前缀 |
| `SegmentReactionQueueKey()` | `segment:reaction:queue` | 视频片段反馈队列 |
| `SegmentReactionCountsPrefix()` | `segment:reaction:counts:` | 视频片段反馈计数前缀 |
| `SegmentReactionUserPrefix()` | `segment:reaction:user:` | 用户片段反馈状态前缀 |

所有 Redis Stream 队列的死信流均为 `<队列 key>:dlq`。例如 `VectorCoarseQueueKey()` 对应的死信流是 `video:vector:coarse:dlq`。

### 6.2 转码 worker 默认参数

文件：

- `internal/worker/transcodeworker/app.go`
- `internal/application/videoapp/worker.go`

| 参数 | 默认值 | 作用 |
|---|---|---|
| `taskTimeout` | `6h` | 当 `Transcode.TaskTimeoutMinutes <= 0` 时的兜底超时 |
| `StatusTTL` | `24h` | 转码状态缓存 TTL |
| `LeaseTTL` | `1m` | worker 内部任务租约 TTL 默认值 |
| `maxRetryAttempts` | `5` | 转码任务最大重试预算 |
| `retryDelayBase` | `500ms` | Redis 短暂错误重试延迟基数 |

### 6.3 向量化 worker 默认参数

文件：

- `internal/worker/vectorworker/app.go`

| 参数 | 默认值 | 作用 |
|---|---|---|
| `maxASRWorkers` | `20` | 向量化 ASR worker 上限 |
| `normalizeASRWorkers()` 默认值 | `4` | 当 `ASRWorkers <= 0` 时兜底值 |
| `windowSec` | `60` | 当 `SegmentWindowSec <= 0` 时默认窗口长度 |
| `stepSec` | `windowSec` | 当 `SegmentStepSec <= 0` 时默认步长 |
| `coarseWorkers` fallback | `asrWorkers` | 当 `CoarseWorkers <= 0` 时的兜底值 |
| `embedBatch` | `64` | 当 `EmbedBatch <= 0` 时的兜底批量 |
| `sampleCount` | `3` | 当 `SampleCount <= 0` 时默认采样段数 |
| `sampleDurSec` | `10` | 当 `SampleDurSec <= 0` 时默认采样时长 |
| `coarseSegmentSec` | `15` | 当 `CoarseSegmentSec <= 0` 时默认 coarse 分段长度 |
| `refineMinSegmentSec` | `20` | 当 `RefineMinSegmentSec <= 0` 时默认最小细分段时长 |
| `refineMaxSegmentSec` | `180` | 当 `RefineMaxSegmentSec <= 0` 时默认最大细分段时长 |
| `llmModel` | `qwen-plus` | 当 `LLMModel` 为空时默认模型 |
| `llmTimeoutMinutes` | `3` | 当 `LLMTimeoutMinutes <= 0` 时默认超时 |
| `taskTimeout` | `3h` | 当 `VectorWorker.TaskTimeoutMinutes <= 0` 时任务超时 |
| `workerCount` | `1` | 当 `CoarseWorkers <= 0` 时视频级 worker 数兜底值 |
| `maxRetryTimes` | `3` | vectorize 任务最大重试次数 |
| `retryDelay` 初始值 | `5s` | vectorize 重试初始等待时间 |

### 6.4 hierarchical 内容分段相关内部参数

文件：

- `internal/worker/vectorworker/tasks/hierarchical.go`

| 参数 | 默认值 | 作用 |
|---|---|---|
| `defaultSegmentOverlapSec` | `3` | 默认相邻 segment 允许重叠秒数 |
| `maxSegmentOverlapSec` | `8` | 相邻 segment 最大允许重叠秒数 |
| `minValidLen` | `5` | `CalcUniformStats` 中统计有效分段的最小长度 |
| `binWidth` | `10` | uniform stats 分箱宽度 |
| `modeRatio` 阈值 | `0.6` | 判断是否“过于等距”的阈值 |
| `st.MaxLen-st.MinLen` 阈值 | `15` | 另一种等距判断阈值 |
| `continuationPrefixes` | `然后/所以/因为/接下来/也就是说/我们继续/继续` | 用于低置信度续接段合并的前缀词表 |

### 6.5 tail alignment 相关内部参数

文件：

- `internal/worker/vectorworker/tasks/tail_alignment.go`
- `internal/worker/vectorworker/tasks/boundary_alignment.go`

#### boundary alignment 窗口参数

| 参数 | 默认值 | 作用 |
|---|---|---|
| `boundaryStartLookBackSec` | `3` | 起点向前探测窗口 |
| `boundaryStartLookAheadSec` | `2` | 起点向后探测窗口 |
| `boundaryEndLookBackSec` | `2` | 终点向前探测窗口 |
| `boundaryEndLookAheadSec` | `4` | 终点向后探测窗口 |
| `maxRecommendedOverlapSec` | `3` | 边界修正后推荐的最大重叠 |

#### tail alignment 默认值

| 参数 | 默认值 | 作用 |
|---|---|---|
| `MaxExtendSec` | `3` | 结尾最多往后延几秒 |
| `ProbeStepSec` | `1` | 每次 probe 步长 |
| `MaxOverlapSec` | `6` | 与下一段允许最大重叠 |

#### 句子启发式词表

这些不是外部配置，但会影响内容边界判断：

1. `sentenceEndTokens`
2. `sentenceEndPhrases`
3. `trailingConnectors`
4. `sentenceStartPhrases`

它们共同决定：

1. `LooksLikeSentenceEnd(text)`
2. `LooksLikeSentenceStart(text)`
3. `NormalizeBoundaryConfidence(s)`
4. `NeedsTailExtension(text)`

## 7. 使用建议

### 7.1 如果你是开发者

建议优先关注：

1. `configs/video.yml`
2. `configs/video_prod.yml`
3. `internal/config/types.go`
4. `internal/http/router/router.go`
5. `internal/worker/vectorworker/app.go`

### 7.2 如果你是上游调用方

优先关注：

1. 上传接口参数
2. 推荐接口参数
3. 转码状态查询参数
4. 统一返回结构

### 7.3 如果你在排查 worker 问题

优先关注：

1. `Transcode.WorkerCount`
2. `VectorWorker.ASRWorkers`
3. `VectorWorker.CoarseWorkers`
4. `VectorWorker.Mode`
5. `VectorWorker.LLMTimeoutMinutes`
6. `TailAlignment*` 系列参数

### 7.4 DLQ 查看与重放工具

文件：

- `cmd/dlqctl/main.go`
- `internal/infrastructure/redis/dead_letter.go`

`cmd/dlqctl` 用于查看和显式重放 Redis Stream 死信队列。它复用 `CONFIG_FILE` / `VIDEO_CONFIG_FILE` 的配置加载规则，并从 `Redis`、`RedisKeys` 配置中确定 Redis 连接和主队列 key。

支持的队列名：

| 队列名 | 对应配置 |
|---|---|
| `transcode` | `RedisKeys.TranscodeQueue` |
| `vectorize` | `RedisKeys.VectorizeQueue` |
| `vector-prepare` | `RedisKeys.VectorPrepareQueue` |
| `vector-coarse` | `RedisKeys.VectorCoarseQueue` |
| `vector-refine` | `RedisKeys.VectorRefineQueue` |
| `vector-finalize` | `RedisKeys.VectorFinalizeQueue` |
| `video-reaction` | `RedisKeys.VideoReactionQueue` |
| `segment-reaction` | `RedisKeys.SegmentReactionQueue` |

知识点视频转码的终态失败会写入 `RedisKeys.KnowledgeVideoTranscodeQueue + ":dlq"`（默认 `knowledge_video:transcode:stream:dlq`），但当前 `cmd/dlqctl` 不包含该队列，`--queue all` 也不会查看或重放它。批次接口会显示失败状态和错误；仓库当前没有受支持的知识点视频 DLQ 恢复工具或 runbook。生产恢复必须由服务负责人同时核对并修复持久化失败状态、重试计数和 Redis payload；单纯把原 payload 重新插入队列会再次被判定为终态失败，不能直接套用下方通用命令。

常用命令：

```bash
cd video-service

go run ./cmd/dlqctl list --queue all --limit 20
go run ./cmd/dlqctl list --queue transcode --limit 20
go run ./cmd/dlqctl replay --queue vector-coarse --limit 10 --dry-run
go run ./cmd/dlqctl replay --queue transcode --id <dlq-message-id>
go run ./cmd/dlqctl replay --queue vector-coarse --limit 10 --keep-dlq
```

默认情况下，`replay` 会把 DLQ 消息中的 `payload` 写回原主队列，并删除原 DLQ 消息。加 `--keep-dlq` 可保留原死信消息；加 `--dry-run` 只展示将要重放的消息，不写回主队列。

使用建议：

1. 先执行 `list` 或 `replay --dry-run` 看失败原因和 payload 摘要。
2. 确认节点、对象存储、AI 服务或数据库等依赖已经恢复。
3. 对明确可恢复的任务按 id 重放，谨慎使用 `--queue all --limit` 批量重放。
4. 视频损坏、对象 key 不存在、参数非法等永久失败任务不应直接重放。

### 7.5 知识点视频 MinIO 初始化导入工具

文件：

- `cmd/knowledgevideo-import/main.go`
- `cmd/knowledgevideo-import/bulk.go`

`cmd/knowledgevideo-import` 是运维初始化工具：直接扫描源 MinIO 前缀，跳过 HTTP、ZIP 和浏览器鉴权，把源对象复制到知识视频对象存储并创建批次/视频记录，再投递转码任务。目标 PostgreSQL、Redis 和知识视频对象存储继续使用项目配置；源 MinIO 只通过 `SOURCE_MINIO_*` 环境变量传入。

XLSX 映射契约与 HTTP 批次接口一致：第一行必须严格为 `id`、`name`、`video_name` 三列表头；`video_name` 可以是源前缀下唯一的 basename，也可以是相对对象路径。数据库知识点名称是最终依据，`父级 / 叶子名称` 形式会在叶子名称精确匹配时自动规范化。同一个源对象可被多行引用并挂到多个知识点，每个唯一对象只流式复制一次。

#### 单批次模式 CLI 参数

| 参数 | 必填 | 作用 |
|---|---|---|
| `--mapping <path>` | 是 | XLSX 映射文件路径 |
| `--source-prefix <prefix>` | 是 | 允许的源 MinIO 对象前缀 |
| `--batch-key <key>` | 是 | 幂等 key；相同 key 已创建批次时直接跳过，格式 `minio:<key>` 写入 `zip_file_name` |
| `--upload-user-id <id>` | 是 | 批次归属用户 ID，正整数 |
| `--dry-run` | 否 | 只校验和统计（`validated rows / knowledge_points / unique_objects / bytes`），不写入 |
| `--list-only` | 否 | 只列出源前缀下的对象和字节数，不访问数据库 |
| `--inspect-object <key>` | 否 | 下载并打印单个 XLSX 的原始表头、解析行数和校验问题 |

校验通过后输出形如 `validated rows=N knowledge_points=N unique_objects=N ...`，正式执行时创建 `direct-import/<batch_id>/<hash>-<basename>` 对象、`manifests/<batch_id>/mapping.xlsx` 清单，并把任务写入 `RedisKeys.KnowledgeVideoTranscodeQueue`。

#### 批量模式 CLI 参数

| 参数 | 必填 | 作用 |
|---|---|---|
| `--all-prefix <prefix>` | 是（该模式下） | 处理前缀下所有 `batch-*.xlsx` + `batch-*.zip` 批次 |
| `--batch-key <key>` | 是 | 幂等 key，逐批以 `minio:<key>:<batch-name>` 去重 |
| `--upload-user-id <id>` | 是 | 批次归属用户 ID |
| `--report-dir <dir>` | 否 | 报告目录；默认系统临时目录下的 `knowledge-video-import-<batch-key>` |
| `--continue-on-error` | 否 | 单个批次失败后是否继续，默认 `true` |

批量报告文件：

| 文件 | 内容 |
|---|---|
| `summary.json` | 全部批次结果（`batch`、`status`、`rows`、`unique_objects`、`skipped_empty_rows`、`error`） |
| `valid-batches.csv` | `imported` / `skipped` / `dry-run` / `skipped-empty` 批次 |
| `invalid-batches.csv` | `failed` / `invalid` 批次及错误详情 |

常用失败模式：表头不精确（`row=1 field=id: header must be exactly id` 等三条）导致整批失败；数据行 `video_name` 为空（`row=N field=video_name: video name is required`）导致整批失败。报告与批次记录不会自动恢复，修正映射后重新执行（幂等 key 会阻止已创建批次重复入库）。

常用命令：

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

### 7.6 如果你要做下游服务化治理

建议补充关注：

1. 鉴权参数
2. 幂等参数
3. 回调参数
4. trace / source system 参数

这些在当前项目中还不是完整体系，但未来应该进入标准接口契约。
