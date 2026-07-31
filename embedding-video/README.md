# 衡桃学堂视频服务仓库

## 项目总览

这个仓库是一个多项目容器，不是单一可运行应用。后续部署和 Java 系统对接时，推荐使用 `video-service/` 作为对外提供能力的后端服务。

当前主要项目：

- `video-service/`：推荐部署的 Go HTTP 视频服务，提供上传、转码、播放、推荐、观看记录、题库查询和异步 worker。
- `recbole-training/`：RecBole 推荐离线训练代码、atomic 数据流水线和模型产物目录。
- `hls-web/`：Vue 3 + Vite 联调控制台，包含视频调试、推荐诊断和知识点视频三个工作区。
- `video-embedding/`：历史 Go 后端工程，当前不作为后续 Java 对接入口。
- `docs/`：仓库级设计文档、演示文稿等材料。
- `deployment/`：服务器交付包，包含 standalone、cloud、intranet 三种部署拓扑及打包、校验脚本。

> Go 命令需要在 `video-service/` 目录下执行；仓库根目录没有 `go.mod`。

## 推荐部署目标

- 对外服务工程：`video-service/`
- Java 对接方式：HTTP REST API
- 标准对接入口：`video-service/cmd/httpapi`
- 异步处理入口：`video-service/cmd/worker`
- RecBole 训练入口：`video-service/cmd/recboletrainer`
- 知识点视频处理：由 `cmd/worker` 中的独立 Redis Stream worker 消费
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
├── docker-compose.local.yml         # 复用本地 Postgres/Redis/MinIO 的覆盖配置
├── deployment/                      # 服务器部署包、拓扑和运维脚本
├── hls-web.zip                      # 前端预构建压缩包
├── video-vectorization-cost-report.md
├── docs/                            # 仓库级设计文档、演示文稿等
├── video-service/      # 推荐部署的 HTTP 后端，供 Java 调用
├── recbole-training/                # RecBole 推荐训练代码、数据与模型产物
├── video-embedding/           # 历史 Go 后端主工程
└── hls-web/                         # Vue 3 + Vite 前端调试工程
```

## 文档导航

- HTTP 后端服务说明：[`video-service/README.md`](video-service/README.md)
- 中文参数参考：[`PROJECT_PARAMETERS.md`](PROJECT_PARAMETERS.md)
- English parameter reference: [`PROJECT_PARAMETERS_EN.md`](PROJECT_PARAMETERS_EN.md)
- RecBole 训练说明：[`recbole-training/README.md`](recbole-training/README.md)
- 前端调试工程说明：[`hls-web/README.md`](hls-web/README.md)
- 历史后端工程说明：[`video-embedding/README.md`](video-embedding/README.md)
- 仓库级文档说明：[`docs/README.md`](docs/README.md)
- 接口契约：[`video-service/docs/swagger/swagger.yaml`](video-service/docs/swagger/swagger.yaml)
- RecBole 算法交接：[`recbole-training/ALGORITHM_HANDOFF.md`](recbole-training/ALGORITHM_HANDOFF.md)
- Gorse 可选集成运行手册：[`video-service/docs/gorse-recommendation-runbook.md`](video-service/docs/gorse-recommendation-runbook.md)
- 服务器部署手册：[`deployment/DEPLOYMENT.md`](deployment/DEPLOYMENT.md)

## 根目录开发与联调部署

根目录 `docker-compose.yml` 是源码构建和联调编排：

- `api`：对外 HTTP API，单独健康检查和重启。
- `worker`：独立消费通用视频转码、向量化、知识点视频转码和 Gorse 同步任务。
- `recbole_trainer`：独立训练调度容器，按计划执行 RecBole 训练、门禁和发布。
- `frontend`：构建后的静态前端，由 Nginx 代理 `/api`、`/videos`、`/swagger`、`/knowledge-video-media` 到 `api`。
- `gorse`：官方 `gorse-in-one` 推荐服务，默认不向宿主机暴露端口。

常用环境变量：

| 环境变量 | 作用 |
| --- | --- |
| `VIDEO_APP_ENV_FILE` | Compose 使用的私有环境文件，默认 `.env.deploy` |
| `VIDEO_APP_HTTP_PORT` | 宿主机暴露的后端 HTTP 端口，默认 `8083` |
| `VIDEO_APP_WEB_PORT` | 宿主机暴露的调试前端端口，默认 `1325` |
| `GORSE_DASHBOARD_USERNAME` | 可选诊断 Dashboard 登录用户，不供 Go API 使用 |
| `GORSE_DASHBOARD_PASSWORD` | 可选诊断 Dashboard 登录密码，生产环境必须替换示例值 |

启动示例：

```bash
cp .env.deploy.example .env.deploy
# 编辑 .env.deploy，填入数据库 DSN、对象存储、Gorse 和 AI API key
docker compose up -d
```

查看联调栈状态：

```bash
docker compose ps
docker compose logs -f api worker
```

如需将 Gorse 诊断端口暴露到宿主机，叠加可选 override：

```bash
docker compose -f docker-compose.yml -f docker-compose.gorse.yml up -d
```

Gorse 默认使用宿主机 Redis 和主服务 PostgreSQL 的独立 schema，具体初始化、同步和回滚见
`video-service/docs/gorse-recommendation-runbook.md`。

推荐控制台的“命中效果”页通过 API 展示 PostgreSQL 中保存的 RecBole 离线评估趋势，包括 Recall@20、NDCG@20、Hit@20 和 Precision@20。

生产后端镜像内置 FFmpeg，`configs/video_prod.yml` 使用原生 FFmpeg，不需要挂载宿主 Docker socket。API 和 worker 共享受控具名卷作为本地转码暂存区，持久对象仍由对象存储管理。

若要复用已加入 `video-embedding_default` 网络的本地 Postgres、Redis 和 MinIO，叠加本地覆盖配置：

```bash
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d
```

## 服务器部署包

`deployment/` 是当前服务器交付入口，不依赖根 Compose。它提供：

- `standalone`：API、worker、RecBole trainer 和前端部署在同一台服务器。
- `cloud`：仅部署 API，连接已有 PostgreSQL 和 Redis。
- `intranet`：部署 worker、RecBole trainer 和前端，前端反向代理云端 API。

先阅读 [`deployment/DEPLOYMENT.md`](deployment/DEPLOYMENT.md)，再使用 `deployment/scripts/init-env.sh` 和 `deployment/scripts/compose.sh`。可通过 `deployment/scripts/package.sh source|images|all` 生成源码包或 `linux/amd64` 离线镜像包。

## 优先查看

- `video-service/cmd/httpapi/main.go`
- `video-service/cmd/worker/main.go`
- `video-service/cmd/recboletrainer/main.go`
- `video-service/internal/http/router/router.go`
- `video-service/docs/swagger/swagger.yaml`
- `recbole-training/scripts/run_recbole_pipeline.sh`
- `hls-web/vite.config.js`

## 补充说明

- `video-service/` 是当前推荐对接入口；新集成应优先使用标准 REST 路径，不要继续依赖历史兼容路径。
- 根目录 compose 更偏便捷部署和联调形态，不等同于完整生产编排。
- `video-embedding/` 是历史工程，除非明确需要维护历史链路，否则不建议作为新功能入口。
