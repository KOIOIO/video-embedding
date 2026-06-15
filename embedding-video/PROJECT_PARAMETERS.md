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
- [4. 环境变量参数](#4-环境变量参数)
- [5. HTTP API 参数](#5-http-api-参数)
  - [5.1 通用返回结构](#51-通用返回结构)
  - [5.2 健康检查与静态入口](#52-健康检查与静态入口)
  - [5.3 上传相关接口参数](#53-上传相关接口参数)
  - [5.4 视频管理接口参数](#54-视频管理接口参数)
  - [5.5 播放与状态接口参数](#55-播放与状态接口参数)
  - [5.6 推荐接口参数](#56-推荐接口参数)
  - [5.7 题库接口参数](#57-题库接口参数)
- [6. Worker / 内部默认参数](#6-worker--内部默认参数)
  - [6.1 HTTP 运行时默认参数](#61-http-运行时默认参数)
  - [6.2 转码 worker 默认参数](#62-转码-worker-默认参数)
  - [6.3 向量化 worker 默认参数](#63-向量化-worker-默认参数)
  - [6.4 hierarchical 内容分段相关内部参数](#64-hierarchical-内容分段相关内部参数)
  - [6.5 tail alignment 相关内部参数](#65-tail-alignment-相关内部参数)
- [7. 使用建议](#7-使用建议)

## 1. 文档说明

本文档总结当前仓库中，尤其是 `video-service/` 主项目里可见的主要参数，覆盖三类来源：

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

### 3.1 顶层配置 Config

| 参数名 | 类型 | 示例 | 作用 |
|---|---|---|---|
| `Name` | `string` | `video-rpc` | 服务名称标识，主要用于日志与配置描述 |
| `Host` | `string` | `localhost` | 历史主机配置字段，当前 HTTP 主服务监听不直接依赖它 |
| `Port` | `int` | `9090` | 历史端口配置字段，当前 HTTP 主服务监听不直接依赖它 |
| `HTTP` | `HTTPConfig` | 见下文 | HTTP API 监听、日志、慢请求和 CORS 配置 |
| `GRPC` | `GRPCConfig` | 见下文 | gRPC 相关配置，主要兼容历史工程 |
| `Video` | `VideoConfig` | 见下文 | 本地视频目录配置 |
| `FFmpeg` | `FFmpegConfig` | 见下文 | 转码、截图、音频提取配置 |
| `Storage` | `StorageConfig` | 见下文 | 对象存储 key、对外 URL 前缀、媒体代理路由和向量化临时目录配置 |
| `Redis` | `RedisConfig` | 见下文 | Redis 连接配置 |
| `RedisKeys` | `RedisKeysConfig` | 见下文 | Redis Stream 队列、状态和运行计数 key 配置 |
| `Postgres` | `PostgresConfig` | 见下文 | PostgreSQL 连接与连接池配置 |
| `RustFS` | `RustFSConfig` | 见下文 | 对象存储连接配置 |
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
| `RawPath` | `string` | `./storage/videos/raw` | 系统临时目录下的 `nlp-video-project/tmp/raw` | 原始视频本地目录配置 |
| `HlsPath` | `string` | `./storage/videos/hls` | 系统临时目录下的 `nlp-video-project/tmp/hls` | HLS 文件本地目录配置 |

### 3.6 StorageConfig

| 参数名 | 类型 | 示例 | 默认值 | 作用 |
|---|---|---|---|---|
| `RawObjectPrefix` | `string` | `raw` | `raw` | 原视频对象存储 key 前缀 |
| `HLSObjectPrefix` | `string` | `hls` | `hls` | HLS 产物对象存储 key 前缀 |
| `MediaRoutePrefix` | `string` | `/videos` | `/videos` | 对象存储代理路由前缀；如果改成其他值，仍保留 `/videos` 兼容路由 |
| `RawURLPrefix` | `string` | `/videos/raw` | `/videos/raw` | 返回给调用方的原视频 URL 前缀 |
| `HLSURLPrefix` | `string` | `/videos/hls` | `/videos/hls` | 返回给调用方的 HLS URL 前缀 |
| `CoverURLPrefix` | `string` | `/videos` | `/videos` | 返回给调用方的封面 URL 前缀 |
| `VectorTempPath` | `string` | `./storage/tmp/video_vectorize` | 系统临时目录下的 `nlp-video-project/tmp/video_vectorize` | 向量化 worker 临时文件目录 |

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
| `VectorizeQueue` | `string` | `video:vectorize:queue` | `video:vectorize:queue` | 向量化任务 Redis Stream key |
| `VectorPrepareQueue` | `string` | `video:vector:prepare` | `video:vector:prepare` | hierarchical 向量化 prepare 阶段 Redis Stream key |
| `VectorCoarseQueue` | `string` | `video:vector:coarse` | `video:vector:coarse` | hierarchical 向量化 coarse 阶段 Redis Stream key |
| `VectorRefineQueue` | `string` | `video:vector:refine` | `video:vector:refine` | hierarchical 向量化 refine 阶段 Redis Stream key |
| `VectorFinalizeQueue` | `string` | `video:vector:finalize` | `video:vector:finalize` | hierarchical 向量化 finalize 阶段 Redis Stream key |
| `VideoReactionQueue` | `string` | `video:reaction:queue` | `video:reaction:queue` | 视频反馈异步队列 key |
| `VideoReactionCounts` | `string` | `video:reaction:counts:` | `video:reaction:counts:` | 视频反馈计数 key 前缀 |
| `VideoReactionUser` | `string` | `video:reaction:user:` | `video:reaction:user:` | 用户视频反馈状态 key 前缀 |
| `TranscodeStatus` | `string` | `video:transcode:status:` | `video:transcode:status:` | 转码任务状态 key 前缀 |
| `RuntimeActiveCounter` | `string` | `video:runtime:active:` | `video:runtime:active:` | 运行中任务计数 key 前缀 |

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
| `Endpoint` | `string` | `localhost:9000` | 对象存储地址 |
| `AccessKey` | `string` | `minioadmin` | AccessKey |
| `SecretKey` | `string` | `minioadmin` | SecretKey |
| `Bucket` | `string` | `hengshui-tablet-cloud-drive` | 存储桶名称 |
| `UseSSL` | `bool` | `false` | 是否走 HTTPS |

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
| `LLMModel` | `string` | `qwen-plus` | 用于 hierarchical 内容分段的模型 |
| `LLMTimeoutMinutes` | `int` | `2` | LLM 调用超时时间 |

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
| `ASRWorkers` | `int` | `30` | ASR worker 数 |
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

## 4. 环境变量参数

项目中当前明确使用到的环境变量包括：

| 环境变量 | 来源位置 | 作用 |
|---|---|---|
| `HTTP_ADDR` | `internal/http/app/app.go` | 覆盖 HTTP 服务监听地址，默认 `:8081` |
| `CONFIG_FILE` | `internal/config/loader.go` | 覆盖默认配置文件路径，优先级高于 `VIDEO_CONFIG_FILE` |
| `VIDEO_CONFIG_FILE` | `internal/config/loader.go` | 覆盖默认配置文件路径 |
| `RUSTFS_ACCESS_KEY` | `internal/http/app/app.go`、worker 初始化 | 当配置文件未提供时兜底 AccessKey |
| `RUSTFS_SECRET_KEY` | `internal/http/app/app.go`、worker 初始化 | 当配置文件未提供时兜底 SecretKey |
| `DASHSCOPE_API_KEY` | embedding 客户端、vector worker AI client | DashScope / 百炼兼容接口 API Key |
| `OPENAI_API_KEY` | embedding 客户端、vector worker AI client | OpenAI 兼容接口 API Key 兜底 |
| `EMBEDDING_API_KEY` | embedding 客户端 | 推荐链路 embedding API Key 兜底 |
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
- 作用：作为对象存储访问凭证的兜底来源
- 使用场景：配置文件未填时，由环境变量注入敏感信息

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

### 5.2 健康检查与静态入口

#### `GET /healthz`
#### `GET /api/healthz`

- 无请求参数
- 返回 `{ "status": "ok" }`

#### `GET /swagger/*any`

- 路径参数：`*any`
- 作用：访问 Swagger 页面资源

#### `GET /api/system/metrics`

- 无请求参数
- 作用：查询系统运行指标

#### `GET /videos/*filepath`

- 路径参数：`*filepath`
- 作用：代理访问对象存储中的视频资源
- 说明：实际媒体代理路由前缀由 `Storage.MediaRoutePrefix` 控制，默认是 `/videos`；如果配置成其他前缀，仍保留 `/videos/*filepath` 兼容路由

### 5.3 上传相关接口参数

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
| `RawPath()` | `os.TempDir()/nlp-video-project/tmp/raw` | 原视频本地目录兜底值 |
| `HLSPath()` | `os.TempDir()/nlp-video-project/tmp/hls` | HLS 本地目录兜底值 |
| `VectorTempPath()` | `os.TempDir()/nlp-video-project/tmp/video_vectorize` | 向量化临时目录兜底值 |
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

### 7.4 如果你要做下游服务化治理

建议补充关注：

1. 鉴权参数
2. 幂等参数
3. 回调参数
4. trace / source system 参数

这些在当前项目中还不是完整体系，但未来应该进入标准接口契约。
