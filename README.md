# video-embedding

视频向量化与推荐服务仓库，包含 Go HTTP 后端、异步转码/向量化 worker、Vue 调试前端、RecBole 推荐离线训练流水线、Gorse 可选推荐集成、知识点视频导入工具和服务器部署交付包。

这个仓库不是单一可运行应用。当前主工程在 [`embedding-video/video-service/`](embedding-video/video-service/)，前端调试工程在 [`embedding-video/hls-web/`](embedding-video/hls-web/)，离线训练工程在 [`embedding-video/recbole-training/`](embedding-video/recbole-training/)；历史双塔训练代码归档在 [`embedding-video/two-tower-training/`](embedding-video/two-tower-training/)，不作为新功能入口。

## 项目能做什么

- 上传视频、ZIP 归档视频和大文件分片断点续传。
- 使用 FFmpeg 转码为 HLS，并生成播放地址和封面。
- 对视频内容做 ASR、语义分段、摘要、embedding 向量化。
- 按题目文本召回相关视频片段。
- 记录曝光、观看和 reaction 行为，用于推荐和训练样本。
- 通过 RecBole user/item embedding 提供个性化视频片段推荐，Gorse 作为可选候选引擎。
- 知识点视频 ZIP + XLSX 批量导入、知识点树查询、多视频播放和观看会话上报。
- 提供 Vue 前端用于本地上传、播放、推荐和接口联调。

## 目录结构

```text
.
├── README.md                         # 当前入口文档
└── embedding-video/
    ├── README.md                     # 仓库内部更详细的说明
    ├── AGENTS.md                     # AI agent 行为准则
    ├── PROJECT_PARAMETERS.md         # 中文参数总览
    ├── PROJECT_PARAMETERS_EN.md      # English parameter reference
    ├── docker-compose.yml            # 根目录源码构建与联调编排
    ├── docker-compose.migrated.yml   # 隔离本地数据栈(Postgres/Redis/MinIO)
    ├── docker-compose.local.yml      # 复用本地依赖容器的覆盖配置
    ├── docker-compose.gorse.yml      # 暴露 Gorse 诊断端口的覆盖配置
    ├── docker-compose.cloud.yml      # 叠加在生产栈上的端口暴露覆盖
    ├── docker-compose.recbole.yml    # 历史 RecBole 编排(保留兼容,勿作新入口)
    ├── .env.local.example            # 本地环境变量模板
    ├── .env.deploy.example           # 部署环境变量模板
    ├── .env.migrated.example         # 隔离本地栈环境模板
    ├── video-service/                # Go HTTP API、worker、工具命令
    ├── hls-web/                      # Vue 3 + Vite 调试前端
    ├── recbole-training/             # RecBole 推荐训练流水线(当前方案)
    ├── two-tower-training/           # 历史双塔训练代码(已归档)
    ├── gorse/                        # Gorse 本地配置和镜像构建文件
    ├── deployment/                   # 服务器交付包(standalone/cloud/intranet)
    ├── scripts/                      # 隔离本地栈与仓库校验脚本
    └── docs/                         # 仓库级设计文档与迁移记录
```

## 环境要求

推荐先使用 Docker Compose 启动完整联调环境。

- Docker 和 Docker Compose
- Go 1.26.1 或兼容版本，本地直接运行后端时需要
- Node.js 22 和 npm，本地直接运行前端时需要
- Python 3，本地运行 RecBole 训练代码测试时需要；完整训练请使用 `recbole-training` 的容器镜像
- 可选：`DASHSCOPE_API_KEY`、`OPENAI_API_KEY`、`EMBEDDING_API_KEY`、`ASR_API_KEY`，启用向量化和 AI 能力时需要

## 快速启动

### 方式一：隔离本地栈(推荐)

`embedding-video/docker-compose.migrated.yml` 会启动独立的 PostgreSQL + pgvector、Redis 和 MinIO 容器(默认宿主机端口 `15432` / `16379` / `19000`)，与本机其他服务互不干扰：

```bash
cd embedding-video
./scripts/init-migrated-env.sh   # 生成 .env.migrated.local 并填入随机密钥
docker compose -f docker-compose.migrated.yml up -d
```

分别启动 API、worker 和前端(每个命令一个终端)：

```bash
./scripts/run-migrated-local.sh api
./scripts/run-migrated-local.sh worker
./scripts/run-migrated-local.sh frontend
```

启动后常用地址：

| 服务 | 地址 |
| --- | --- |
| 后端健康检查 | `http://localhost:18083/healthz` |
| Swagger | `http://localhost:18083/swagger/index.html` |
| 调试前端 | `http://localhost:15173` |
| MinIO Console | `http://localhost:19001` |

需要迁移旧库数据时，先编辑 `.env.migrated.local` 中的 `MIGRATION_SOURCE_*` 再运行 `./scripts/migrate-local-data.sh`。

### 方式二：根 Compose 源码构建

```bash
cd embedding-video
cp .env.deploy.example .env.deploy
# 编辑 .env.deploy：数据库 DSN、JWT_SECRET、对象存储、Gorse 和 AI API key
docker compose up -d
```

根 Compose 会把 `api`、`worker`、`recbole_trainer`、`frontend` 和 `gorse` 构建为独立服务；`VIDEO_APP_HTTP_PORT`(默认 `8083`)和 `VIDEO_APP_WEB_PORT`(默认 `1325`)控制宿主机端口。

| 服务 | 地址 |
| --- | --- |
| 后端健康检查 | `http://localhost:8083/healthz` |
| Swagger | `http://localhost:8083/swagger/index.html` |
| 调试前端 | `http://localhost:1325` |

查看日志与停止服务：

```bash
docker compose logs -f api worker
docker compose down
```

## 本地分进程启动

如果想用本机 Go 和 Node 调试代码，先启动依赖服务(见上面方式一)，然后分别运行后端、worker 和前端。

环境文件约定：启动命令会向上查找 `.env`，再按其中的 `VIDEO_APP_ENV_FILE` 加载私有环境文件。仓库根目录和 `video-service/` 下的 `.env` 示例：

```bash
# 仓库根目录 .env
VIDEO_APP_ENV_FILE=embedding-video/.env.local

# video-service/.env
VIDEO_APP_ENV_FILE=.env.local
```

连接隔离本地栈时，确认 `.env.local` 中的地址指向宿主机端口(DSN 示例见 `.env.migrated.example`)。

1. 启动 HTTP API：

```bash
cd embedding-video/video-service
CONFIG_FILE=configs/video.yml HTTP_ADDR=:8081 go run ./cmd/httpapi
```

2. 新开终端启动 worker：

```bash
cd embedding-video/video-service
CONFIG_FILE=configs/video.yml go run ./cmd/worker
```

3. 新开终端启动前端：

```bash
cd embedding-video/hls-web
npm install
npm run dev
```

本地分进程启动时，前端默认访问 `http://localhost:5173`，并把 `/api`、`/videos`、`/swagger`、`/knowledge-video-media` 代理到 `http://localhost:8081`。

## 常用命令

后端测试与构建：

```bash
cd embedding-video/video-service
go test ./...
go build ./...
```

前端测试与构建：

```bash
cd embedding-video/hls-web
npm test
npm run build
```

手动运行 RecBole 训练流水线：

```bash
cd embedding-video/recbole-training
CONFIG_FILE=../video-service/configs/video.yml ./scripts/run_recbole_pipeline.sh
```

仓库命名与安全校验：

```bash
cd embedding-video
./scripts/validate-repository.sh
```

## 重要文档

- 项目内部总览：[`embedding-video/README.md`](embedding-video/README.md)
- HTTP 后端说明：[`embedding-video/video-service/README.md`](embedding-video/video-service/README.md)
- 前端说明：[`embedding-video/hls-web/README.md`](embedding-video/hls-web/README.md)
- RecBole 训练说明：[`embedding-video/recbole-training/README.md`](embedding-video/recbole-training/README.md)
- 服务器部署手册：[`embedding-video/deployment/DEPLOYMENT.md`](embedding-video/deployment/DEPLOYMENT.md)
- Swagger YAML：[`embedding-video/video-service/docs/swagger/swagger.yaml`](embedding-video/video-service/docs/swagger/swagger.yaml)
- Gorse 运行手册：[`embedding-video/video-service/docs/gorse-recommendation-runbook.md`](embedding-video/video-service/docs/gorse-recommendation-runbook.md)
- 迁移验证记录：[`embedding-video/docs/migration/2026-07-31-verification.md`](embedding-video/docs/migration/2026-07-31-verification.md)

## 注意事项

- Go 命令需要在 `embedding-video/video-service/` 下执行，因为 `go.mod` 在这个目录。
- Docker Compose 命令需要在 `embedding-video/` 下执行，因为 `docker-compose*.yml` 在这个目录。
- `.env`、`.env.local`、`.env.deploy`、`.env.migrated.local` 是本地私有文件，不要提交真实密钥。
- 向量化 worker 没有 AI API Key 时会限制或跳过相关能力；普通健康检查、上传、基础接口仍可用于联调。
- `legacy-video/` 是历史 Go 工程(gRPC 时代)，新功能和新对接优先使用 `video-service/` 的 HTTP REST API。
- `two-tower-training/` 是已归档的历史双塔训练代码，当前推荐引擎为 RecBole，不要用旧脚本生成新模型版本。
