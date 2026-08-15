# legacy-video（历史后端）

这个目录是早期 Go 后端，保留 gRPC 服务、联调 API 和合并 worker。它不是当前推荐的部署或 Java 集成入口；新调用方请使用 [`../video-service/README.md`](../video-service/README.md) 及根目录参数文档。

## 安全边界

该历史 API 只安装 Recovery 和访问日志中间件，没有管理员认证或授权。上传、删除、发布和推荐池修改等写接口都可以直接调用，因此只能放在受控内网或临时维护环境，禁止直接暴露到公网。

## 目录提示

- `cmd/`：历史进程入口。
- `configs/`：历史 YAML 配置。
- `internal/`：历史业务、gRPC、HTTP、worker 和基础设施实现。
- `video/`：gRPC proto 及生成代码。
- `middleware/`：访问日志、幂等和 gRPC 中间件。

## 进程入口

从本目录执行命令。推荐先启动 RPC，再启动合并 worker，最后启动 API：

```bash
CONFIG_FILE=configs/video.yml go run ./cmd/rpc
CONFIG_FILE=configs/video.yml go run ./cmd/worker
CONFIG_FILE=configs/video.yml API_ADDR=:8081 RPC_ADDR=localhost:9090 go run ./cmd/api
```

当前三个主入口的职责如下：

| 入口 | 默认监听/目标 | 职责 |
| --- | --- | --- |
| `cmd/rpc` | gRPC `:Port`，示例为 `:9090` | PostgreSQL/pgvector、Redis、RustFS、转码队列、向量化和 gRPC 服务；启动时执行迁移与索引初始化 |
| `cmd/worker` | 无监听端口 | 通过 `internal/worker/combined` 同时启动通用转码和向量化 worker |
| `cmd/api` | HTTP `API_ADDR`，默认 `:8081` | 历史 Gin API、题库查询和对象存储媒体代理；通过 `RPC_ADDR` 调用 RPC |

`cmd/transcodeworker` 和 `cmd/vector_worker` 是兼容别名，二者同样调用合并 worker；不要与 `cmd/worker` 或彼此同时启动，否则会重复消费队列。

## 配置与环境变量

历史模块的默认配置规则与当前 HTTP 服务不同：

- Windows 默认 `configs/video.yml`。
- macOS、Linux 等其他系统默认 `configs/video_prod.yml`；该文件含内网/生产示例地址，不能当作本地开发配置直接运行。
- `CONFIG_FILE` 优先于 `VIDEO_CONFIG_FILE`，二者都优先于系统默认选择。
- 启动会向上查找根 `.env`，再读取 `VIDEO_APP_ENV_FILE` 指定的环境文件；已有进程环境变量不会被文件覆盖。
- `API_ADDR` 控制 API HTTP 监听地址，默认 `:8081`；这里不要套用新 HTTP 服务的 `HTTP_ADDR`。
- `RPC_ADDR` 控制 API 连接的 RPC 目标；未设置时使用 YAML 的 `Host:Port`，再回退到 `localhost:9090`。RPC 服务端实际只监听 `:Port`，不绑定 `Host`。

常用覆盖项：

| 变量 | 作用 |
| --- | --- |
| `POSTGRES_DSN` | 覆盖 PostgreSQL DSN；RPC 和 worker 必填，API 仅在需要题库时使用 |
| `REDIS_PASSWORD` | 覆盖 Redis 密码；RPC/worker 启动时会 Ping Redis |
| `COS_SECRET_ID` / `COS_SECRET_KEY` | 覆盖 `RustFS` 凭据 |
| `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY` | 未设置 COS 变量时的对象存储凭据 |
| `DASHSCOPE_API_KEY` / `OPENAI_API_KEY` | 向量化 worker 的 ASR、LLM 和 Embedding 凭据回退来源 |
| `ASR_API_KEY` / `EMBEDDING_API_KEY` | 分别覆盖 ASR 与 Embedding 凭据 |

本地维护时显式选配置并注入私有凭据：

```bash
CONFIG_FILE=configs/video.yml \
POSTGRES_DSN='host=localhost user=postgres dbname=video-app sslmode=disable' \
RUSTFS_ACCESS_KEY='change-me' RUSTFS_SECRET_KEY='change-me' \
go run ./cmd/rpc
```

## 启动依赖与副作用

| 进程 | 必需依赖 | 启动副作用/注意事项 |
| --- | --- | --- |
| RPC | PostgreSQL（启用 pgvector）、Redis、S3 兼容对象存储、FFmpeg 或可用 Docker FFmpeg 镜像 | 创建 `vector` 扩展、AutoMigrate、视频索引，检查 Redis 并确保对象存储 Bucket；Postgres DSN 为空会直接退出 |
| worker | PostgreSQL、Redis、对象存储、FFmpeg；向量化还需要 ASR/Embedding/LLM API key | 转码和向量化都在同一进程注册；各自连接并迁移数据库，AI 客户端初始化失败会退出 |
| API | RPC、对象存储；PostgreSQL 可选 | RustFS 初始化/Bucket 检查失败会退出；未配置可用 PostgreSQL 时题库接口不可用；RPC Ping 失败只记录警告，但业务调用仍会失败 |

转码优先使用本机 `ffmpeg`/`ffprobe`；找不到时仅在 `FFmpeg.UseDocker=true` 且 Docker 可用时调用配置的镜像。本目录没有 Dockerfile、Compose 或服务器启动脚本，基础设施需由维护者另行提供。

## 历史 HTTP 路由

这些路径只属于 legacy API，不是新 REST 契约：

| 路径组 | 主要路径 |
| --- | --- |
| 健康/媒体 | `GET /api/healthz`（仅 liveness）、`GET /videos/*filepath`（支持单 Range/206） |
| 视频 | `/api/video/upload`、`/api/video/list`、`/api/video/:id`、`/api/video/play/:id`、`/api/video/similar/:id`、`/api/video/view_count/:id` |
| 管理动作 | `/api/video/recommend_pool`、`/api/video/recommend/:id`、`/api/video/cover/:id`、`/api/video/publish/:id`、`/api/video/status/:taskId` |
| 行为/题库 | `/api/video/recommend_by_question`、`/api/video/report_watch`、`/api/question/list`、`/api/question/:id` |

完整 HTTP handler 以 [`internal/api/router/router.go`](internal/api/router/router.go) 为准；gRPC 方法和消息以 [`video/video_service.proto`](video/video_service.proto) 为准。不要把这些路径与现行服务的 `/api/videos`、`/api/recommendations` 等标准 REST 路径混用。

## 配置陷阱

- `Name`、`Video.RawPath`、`Video.HlsPath` 和 `Transcode.QueueSize` 在当前 legacy 运行路径中不是可靠的实际控制面；RPC/worker 使用系统临时目录下的 `legacy-video/tmp/`。
- `VectorWorker.ASRWorkers` 和相关 pool 会在启动时归一化，ASR worker 上限为 20；不要把 YAML 中的更大值当作实际并发。
- `configs/video_prod.yml` 的内网地址和空凭据只是样例，显式配置本地文件和秘密后再启动。

## 运行与验证

只在维护历史链路时从本目录执行：

```bash
go test ./...
```

测试可能按测试包工作目录创建 `logs/` 下的访问日志；验证后确认这些运行产物没有被加入 Git。根目录没有 Go module，现行 HTTP 服务的启动、配置、worker 和部署命令不适用于本目录。
