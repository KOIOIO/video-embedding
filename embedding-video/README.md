# 视频向量化视频服务仓库

## 项目总览

这个仓库是一个多项目容器，不是单一可运行应用。后续部署和 Java 系统对接时，推荐使用 `video-service/` 作为对外提供能力的后端服务。

当前主要项目：

- `video-service/`：推荐部署的 Go HTTP 视频服务，提供上传、转码、播放、推荐、观看记录、题库查询和异步 worker。
- `two-tower-training/`：双塔推荐离线训练代码、样本导出流水线和模型产物目录。
- `hls-web/`：Vue 3 + Vite 视频服务调试前端，用于本地联调上传、播放、反馈和推荐接口。
- `embedding-video/`：历史 Go 后端工程，当前不作为后续 Java 对接入口。
- `docs/`：仓库级设计文档、演示文稿等材料。

> Go 命令需要在 `video-service/` 目录下执行；仓库根目录没有 `go.mod`。

## 推荐部署目标

- 对外服务工程：`video-service/`
- Java 对接方式：HTTP REST API
- 标准对接入口：`video-service/cmd/httpapi`
- 异步处理入口：`video-service/cmd/worker`
- 双塔训练入口：`video-service/cmd/twotowertrainer`
- 本地默认监听地址：`:8081`
- 当前 `docker-compose.yml` 服务器部署端口：`8083`
- 健康检查接口：`GET /healthz`
- Swagger 文档入口：`GET /swagger/index.html`

## 仓库结构

```text
.
├── README.md
├── PROJECT_PARAMETERS.md            # 中文参数总览文档
├── PROJECT_PARAMETERS_EN.md         # 英文参数总览文档
├── AGENTS.md                        # AI agent 行为准则
├── docker-compose.yml               # 根目录便捷部署编排
├── hls-web.zip                      # 前端预构建压缩包
├── video-vectorization-cost-report.md
├── docs/                            # 仓库级设计文档、演示文稿等
├── video-service/      # 推荐部署的 HTTP 后端，供 Java 调用
├── two-tower-training/              # 双塔推荐训练代码、样本与模型产物
├── embedding-video/           # 历史 Go 后端主工程
└── hls-web/                         # Vue 3 + Vite 前端调试工程
```

## 文档导航

- HTTP 后端服务说明：[`video-service/README.md`](video-service/README.md)
- 双塔训练说明：[`two-tower-training/README.md`](two-tower-training/README.md)
- 前端调试工程说明：[`hls-web/README.md`](hls-web/README.md)
- 历史后端工程说明：[`embedding-video/README.md`](embedding-video/README.md)
- 仓库级文档说明：[`docs/README.md`](docs/README.md)
- 接口契约：[`video-service/docs/swagger/swagger.yaml`](video-service/docs/swagger/swagger.yaml)
- 双塔算法交接：[`two-tower-training/ALGORITHM_HANDOFF.md`](two-tower-training/ALGORITHM_HANDOFF.md)

## 根目录部署

根目录 `docker-compose.yml` 当前提供便捷部署形态：

- `backend_http`：从 `${VIDEO_PROJECT_ROOT}/video-service` 构建并运行 HTTP API 和统一 worker。
- `two_tower_trainer`：独立训练容器，镜像内包含 Go、Python 和 CPU PyTorch，按调度执行双塔训练发布流水线。
- `frontend_web`：`hls-web` 调试前端，默认代理到 `backend_http:8083`。

常用环境变量：

| 环境变量 | 作用 |
| --- | --- |
| `VIDEO_ENV_FILE` | compose 和直接 `go run` 加载的私有环境文件；本地 `.env` 可指向 `.env.local` |
| `VIDEO_PROJECT_ROOT` | 服务器上的仓库绝对路径，默认 `/home/debian/dev-ops/embedding-video` |
| `VIDEO_HTTP_PORT` | 宿主机暴露的后端 HTTP 端口，默认 `8083` |
| `VIDEO_WEB_PORT` | 宿主机暴露的调试前端端口，默认 `1325` |

启动示例：

```bash
cp .env.deploy.example .env.deploy
# 编辑 .env.deploy，填入服务器路径、数据库 DSN、对象存储、Gorse 和 AI API key
docker compose up -d
```

如果需要同时启动双塔训练容器：

```bash
docker compose --profile two_tower up -d
```

如果需要单独启动 Gorse 推荐引擎：

```bash
docker compose -f docker-compose.gorse.yml up -d
```

Gorse 默认使用宿主机 Redis 和主服务 PostgreSQL 的独立 schema，具体初始化、同步和回滚见
`video-service/docs/gorse-recommendation-runbook.md`。

当 `FFmpeg.UseDocker=true` 时，服务容器会通过宿主机 Docker daemon 启动 ffmpeg/ffprobe 子容器。`docker run -v` 的源路径由宿主机解析，因此 compose 会把项目目录挂载到容器内的同名绝对路径。如果服务器代码目录不是默认路径，启动前必须设置 `VIDEO_PROJECT_ROOT`。

正式生产环境如果需要独立扩缩容，仍建议把 HTTP API、worker、双塔训练调度拆成独立进程或容器。

## 优先查看

- `video-service/cmd/httpapi/main.go`
- `video-service/cmd/worker/main.go`
- `video-service/cmd/twotowertrainer/main.go`
- `video-service/internal/http/router/router.go`
- `video-service/docs/swagger/swagger.yaml`
- `two-tower-training/scripts/run_two_tower_pipeline.sh`
- `hls-web/vite.config.js`

## 补充说明

- `video-service/` 是当前推荐对接入口；新集成应优先使用标准 REST 路径，不要继续依赖历史兼容路径。
- 根目录 compose 更偏便捷部署和联调形态，不等同于完整生产编排。
- `embedding-video/` 是历史工程，除非明确需要维护历史链路，否则不建议作为新功能入口。
