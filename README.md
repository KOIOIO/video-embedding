# video-embedding（hengshui-tablet-video-rpc）

短视频推荐与社交平台仓库：Go HTTP 后端 + 异步转码/向量化 worker + 抖音风格 Vue 客户端 + 联调控制台 + RecBole 推荐离线训练。

主工程统一在 [`embedding-video/`](embedding-video/) 下：

- [`embedding-video/video-service/`](embedding-video/video-service/)：Go HTTP 后端（httpapi `:8081`）、统一 worker（转码 + 向量化 + RecBole 训练调度）。
- [`embedding-video/short-video-web/`](embedding-video/short-video-web/)：抖音风格短视频客户端（`:5174`），面向最终用户的完整产品前端。
- [`embedding-video/hls-web/`](embedding-video/hls-web/)：视频与推荐统一联调控制台（`:5173`）。
- [`embedding-video/recbole-training/`](embedding-video/recbole-training/)：RecBole 推荐离线训练（现役推荐引擎）。
- [`embedding-video/two-tower-training/`](embedding-video/two-tower-training/)：历史双塔训练（已归档，由 RecBole 替代）。

## 项目能做什么

**视频内容处理**

- 用户视频发布：上传 → FFmpeg 转码 HLS → 生成封面与播放地址。
- 向量化（hierarchical 模式）：ASR 转写 → 按内容逻辑分段（LLM）→ 片段摘要 + embedding 向量 → 落库 `edu_video_segment`。
- 播放进度条分段标记：`GET /api/videos/:id/segments` 返回每个片段的时间区间与摘要，前端在进度条上标记并可点击跳转。
- 大文件分片断点续传、ZIP 批量导入、知识点视频 ZIP + XLSX 批量导入。

**用户与社交体系**

- 用户注册 / 登录（JWT）、资料与头像上传、个人主页访问统计。
- 关注 / 取关、好友关系、用户搜索。
- 私信会话 + 未读数 + 轮询，消息/通知红点显示总数。
- 评论两级体系（Redis 缓冲 + worker 异步落库），点赞 / 双赞 / 踩，@提及搜索与通知跳转。
- 点赞后主页「喜欢」列表同步显示对应视频，作品 / 喜欢列表支持 HLS 连续播放。

**推荐引擎**

- 社交召回通道（关注 / 点赞 / 评论 / 观看行为）+ 热门加权 + 新视频扶持 + 用户发布纳入推荐池。
- RecBole 离线训练（BPR 等模型），社交行为纳入训练数据，发布门禁 + active model_version 管理。
- 推荐链路具备 AI provider 异常降级与向量化异步补偿。

## 目录结构

```text
.
├── README.md                    # 当前入口文档
└── embedding-video/
    ├── README.md                # 仓库内部详细说明
    ├── docker-compose*.yml      # 本地/联调/云/RecBole/迁移 Docker 编排
    ├── .env.local.example       # 本地环境变量模板（私有 .env.local 不入库）
    ├── video-service/           # Go HTTP API、统一 worker、训练调度、工具命令
    ├── short-video-web/         # 抖音风格短视频客户端（Vue 3 + Vite, :5174）
    ├── hls-web/                 # 视频与推荐统一联调控制台（Vue 3 + Vite, :5173）
    ├── recbole-training/        # RecBole 推荐离线训练流水线
    ├── two-tower-training/      # 历史双塔训练（已归档）
    ├── gorse/                   # Gorse 可选集成配置（当前推荐引擎为 RecBole）
    ├── docs/                    # 仓库级设计文档
    ├── deployment/              # 服务器部署包（standalone / cloud / intranet）
    └── legacy-video/            # 历史 Go 工程，不建议作为新入口
```

## 环境要求

- Docker 与 Docker Compose（推荐方式启动基础设施）
- Go 1.26.1（本地运行后端）
- Node.js 22 + npm（本地运行前端）
- Python 3 + PyTorch + RecBole（仅 RecBole 训练需要）
- 可选：`DASHSCOPE_API_KEY`（ASR / Embedding / LLM 统一使用百炼标准端点，向量化能力必需）

## Clone 后快速启动（Docker Compose）

```bash
git clone <repo-url> video-embedding
cd video-embedding/embedding-video

cp .env.local.example .env.local
printf 'VIDEO_APP_ENV_FILE=.env.local\n' > .env
```

编辑 `.env.local`，至少确认：

```dotenv
VIDEO_PROJECT_ROOT=/你的绝对路径/video-embedding/embedding-video
POSTGRES_PASSWORD=postgres
REDIS_PASSWORD=redis123
POSTGRES_DSN=host=postgres user=postgres password=postgres dbname=video_embedding port=5432 sslmode=disable TimeZone=Asia/Shanghai
RUSTFS_ACCESS_KEY=minioadmin
RUSTFS_SECRET_KEY=minioadmin
```

向量化能力需要补充 AI Key（百炼标准端点）：

```dotenv
DASHSCOPE_API_KEY=
```

启动基础设施与后端：

```bash
docker compose --env-file .env.local up -d postgres redis minio
docker compose --env-file .env.local up -d api worker
```

启动前端（可选）：

```bash
docker compose --env-file .env.local up -d frontend
```

常用地址：

| 服务 | 地址 |
| --- | --- |
| 后端健康检查 | `http://localhost:8081/healthz` |
| Swagger | `http://localhost:8081/swagger/index.html` |
| 抖音风格客户端 | `http://localhost:5174` |
| 联调控制台 | `http://localhost:5173` |
| MinIO Console | `http://localhost:9001` |

## 本地分进程启动（开发调试）

用 Docker 只启动依赖，本地分别运行各服务：

```bash
cd embedding-video
docker compose --env-file .env.local up -d postgres redis minio
# 本地 .env.local 中 POSTGRES_DSN 改用 host=localhost 与本机暴露端口
```

1. HTTP API：

```bash
cd video-service
go run ./cmd/httpapi          # 默认 :8081，自动加载 .env -> .env.local
```

2. Worker（转码 + 向量化 + 训练调度）：

```bash
cd video-service
go run ./cmd/worker
```

3. 抖音风格客户端：

```bash
cd short-video-web
npm install && npm run dev    # http://localhost:5174
```

4. 联调控制台：

```bash
cd hls-web
npm install && npm run dev    # http://localhost:5173
```

两个前端的 Vite proxy 默认把 `/api`、`/videos` 等请求转发到 `http://localhost:8081`。

## 常用命令

```bash
# 后端测试
cd embedding-video/video-service && go test ./...

# 前端测试
cd embedding-video/short-video-web && npm test
cd embedding-video/hls-web && npm test

# 手动跑 RecBole 训练流水线
cd embedding-video/recbole-training
CONFIG_FILE=../video-service/configs/video.yml ./scripts/run_recbole_pipeline.sh

# 死信队列查看与重放
cd embedding-video/video-service
go run ./cmd/dlqctl list --queue all --limit 20
```

## 重要文档

- 项目内部总览：[`embedding-video/README.md`](embedding-video/README.md)
- Go 后端说明：[`embedding-video/video-service/README.md`](embedding-video/video-service/README.md)
- 抖音风格客户端：[`embedding-video/short-video-web/README.md`](embedding-video/short-video-web/README.md)
- 联调控制台：[`embedding-video/hls-web/README.md`](embedding-video/hls-web/README.md)
- RecBole 训练：[`embedding-video/recbole-training/README.md`](embedding-video/recbole-training/README.md)
- 服务器部署手册：[`embedding-video/deployment/DEPLOYMENT.md`](embedding-video/deployment/DEPLOYMENT.md)

## 注意事项

- Go 命令在 `embedding-video/video-service/` 下执行；Compose 命令在 `embedding-video/` 下执行。
- `.env` 与 `.env.local` 是本地私有文件，禁止提交真实密钥（API Key 只放 git 忽略的本地 `.env.local`）。
- 向量化依赖百炼标准端点（ASR WS / compatible-mode HTTP）；LLM 逻辑分段耗时较长，`LLMTimeoutMinutes` 默认 15 分钟。
- `legacy-video/`、`two-tower-training/` 为历史工程，新功能优先使用 `video-service/` REST API 与 RecBole 训练。
