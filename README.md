# video-embedding

视频向量化与推荐服务仓库，包含 Go HTTP 后端、异步转码/向量化 worker、Vue 调试前端、Gorse 推荐引擎配置和双塔推荐离线训练代码。

这个仓库不是单一可运行应用。当前主工程在 [`embedding-video/video-service/`](embedding-video/video-service/)，前端调试工程在 [`embedding-video/hls-web/`](embedding-video/hls-web/)，离线训练工程在 [`embedding-video/two-tower-training/`](embedding-video/two-tower-training/)。

## 项目能做什么

- 上传视频、ZIP 归档视频和大文件分片上传。
- 使用 FFmpeg 转码为 HLS，并生成播放地址和封面。
- 对视频内容做 ASR、分段、摘要、embedding 向量化。
- 按题目文本召回相关视频片段。
- 记录曝光、观看和 reaction 行为，用于推荐和训练样本。
- 通过 Gorse 和双塔 embedding 提供个性化视频片段推荐。
- 提供 Vue 前端用于本地上传、播放、推荐和接口联调。

## 目录结构

```text
.
├── README.md                         # 当前入口文档
└── embedding-video/
    ├── README.md                     # 仓库内部更详细的说明
    ├── docker-compose.yml            # 本地/联调 Docker 编排
    ├── .env.local.example            # 本地环境变量模板
    ├── .env.deploy.example           # 部署环境变量模板
    ├── video-service/                # Go HTTP API、worker、工具命令
    ├── hls-web/                      # Vue 3 + Vite 调试前端
    ├── two-tower-training/           # 双塔推荐训练流水线
    ├── gorse/                        # Gorse 本地配置和镜像构建文件
    ├── docs/                         # 仓库级设计文档
    └── legacy-video/                 # 历史 Go 工程，不建议作为新入口
```

## 环境要求

推荐先使用 Docker Compose 启动完整联调环境。

- Docker 和 Docker Compose
- Go 1.26.1 或兼容版本，本地直接运行后端时需要
- Node.js 22 和 npm，本地直接运行前端时需要
- Python 3，本地运行双塔训练时需要
- 可选：`DASHSCOPE_API_KEY`、`OPENAI_API_KEY`、`EMBEDDING_API_KEY`、`ASR_API_KEY`，启用向量化和 AI 能力时需要

## Clone 后快速启动

下面方式会启动 PostgreSQL + pgvector、Redis、MinIO、Gorse、Go 后端和可选前端。

```bash
git clone <repo-url> video-embedding
cd video-embedding/embedding-video

cp .env.local.example .env.local
printf 'HSTV_ENV_FILE=.env.local\n' > .env
```

编辑 `.env.local`，至少建议确认这些值：

```dotenv
VIDEO_PROJECT_ROOT=/你的绝对路径/video-embedding/embedding-video
POSTGRES_PASSWORD=postgres
REDIS_PASSWORD=redis123
POSTGRES_DSN=host=postgres user=postgres password=postgres dbname=video_embedding port=5432 sslmode=disable TimeZone=Asia/Shanghai
RUSTFS_ACCESS_KEY=minioadmin
RUSTFS_SECRET_KEY=minioadmin
```

如果要完整跑向量化 worker，再补充 AI API Key：

```dotenv
DASHSCOPE_API_KEY=
OPENAI_API_KEY=
EMBEDDING_API_KEY=
ASR_API_KEY=
```

启动基础设施和后端：

```bash
docker compose --env-file .env.local up -d postgres redis minio gorse_master gorse_server gorse_worker backend
```

启动调试前端：

```bash
docker compose --env-file .env.local --profile frontend up -d frontend
```

启动后常用地址：

| 服务 | 地址 |
| --- | --- |
| 后端健康检查 | `http://localhost:8083/healthz` |
| Swagger | `http://localhost:8083/swagger/index.html` |
| 调试前端 | `http://localhost:1325` |
| MinIO Console | `http://localhost:9001` |
| Gorse Dashboard | `http://localhost:8088` |

查看日志：

```bash
docker compose logs -f backend
docker compose logs -f frontend
```

停止服务：

```bash
docker compose --profile frontend down
```

如果要同时清理本地容器卷数据：

```bash
docker compose --profile frontend down -v
```

## 本地分进程启动

如果你想用本机 Go 和 Node 调试代码，可以只用 Docker 启动依赖服务，然后分别运行后端、worker 和前端。

1. 准备环境变量：

```bash
cd video-embedding/embedding-video
cp .env.local.example .env.local
printf 'HSTV_ENV_FILE=.env.local\n' > .env
```

确认 `.env.local` 中 `REDIS_PASSWORD=redis123`，并按需填写 AI API Key。

本地 `go run` 连接的是宿主机暴露端口，所以需要把 `.env.local` 中的数据库 DSN 改成本机地址：

```dotenv
POSTGRES_DSN=host=localhost user=postgres password=postgres dbname=video_embedding port=5432 sslmode=disable TimeZone=Asia/Shanghai
```

2. 启动依赖服务：

```bash
docker compose --env-file .env.local up -d postgres redis minio gorse_master gorse_server gorse_worker
```

3. 启动 HTTP API：

```bash
cd video-service
CONFIG_FILE=configs/video.yml HTTP_ADDR=:8081 go run ./cmd/httpapi
```

4. 新开终端启动 worker：

```bash
cd video-embedding/embedding-video/video-service
CONFIG_FILE=configs/video.yml go run ./cmd/worker
```

5. 新开终端启动前端：

```bash
cd video-embedding/embedding-video/hls-web
npm install
npm run dev
```

本地分进程启动时，前端默认访问 `http://localhost:5173`，并把 `/api`、`/videos`、`/swagger` 代理到 `http://localhost:8081`。

## 常用命令

后端测试：

```bash
cd embedding-video/video-service
go test ./...
```

前端测试：

```bash
cd embedding-video/hls-web
npm test
```

前端构建：

```bash
cd embedding-video/hls-web
npm run build
```

手动启动双塔训练流水线：

```bash
cd embedding-video/two-tower-training
CONFIG_FILE=../video-service/configs/video.yml ./scripts/run_two_tower_pipeline.sh
```

## 重要文档

- 项目内部总览：[`embedding-video/README.md`](embedding-video/README.md)
- HTTP 后端说明：[`embedding-video/video-service/README.md`](embedding-video/video-service/README.md)
- 前端说明：[`embedding-video/hls-web/README.md`](embedding-video/hls-web/README.md)
- 双塔训练说明：[`embedding-video/two-tower-training/README.md`](embedding-video/two-tower-training/README.md)
- Swagger YAML：[`embedding-video/video-service/docs/swagger/swagger.yaml`](embedding-video/video-service/docs/swagger/swagger.yaml)
- Gorse 运行手册：[`embedding-video/video-service/docs/gorse-recommendation-runbook.md`](embedding-video/video-service/docs/gorse-recommendation-runbook.md)

## 注意事项

- Go 命令需要在 `embedding-video/video-service/` 下执行，因为 `go.mod` 在这个目录。
- Docker Compose 命令需要在 `embedding-video/` 下执行，因为 `docker-compose.yml` 在这个目录。
- `.env` 和 `.env.local` 是本地私有文件，不要提交真实密钥。
- 向量化 worker 没有 AI API Key 时会限制或跳过相关能力；普通健康检查、上传、基础接口仍可用于联调。
- `legacy-video/` 是历史工程，新功能和新对接优先使用 `video-service/` 的 HTTP REST API。
