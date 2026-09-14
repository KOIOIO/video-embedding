# 衡桃学堂视频服务仓库（hengshui-tablet-video-rpc）

## 项目总览

本仓库是短视频推荐与社交平台的多项目容器，不是单一可运行应用。对外能力统一由 `video-service/` 提供 REST API。

当前主要项目：

- `video-service/`：Go HTTP 视频服务，提供用户/视频/社交/推荐/消息/通知 API 与统一异步 worker（转码 + 向量化 + RecBole 训练调度）。
- `short-video-web/`：抖音风格短视频客户端（Vue 3 + Vite, `:5174`），面向最终用户的产品前端。
- `hls-web/`：视频与推荐统一联调控制台（Vue 3 + Vite, `:5173`）。
- `recbole-training/`：RecBole 推荐离线训练（现役推荐引擎）。
- `two-tower-training/`：历史双塔训练，已由 RecBole 替代（归档）。
- `legacy-video/`：历史 Go 工程，不作为新功能入口。
- `docs/`：仓库级设计文档、演示文稿等。
- `deployment/`：服务器交付包（standalone / cloud / intranet 三种拓扑）。

> Go 命令需在 `video-service/` 目录下执行；仓库根目录没有 `go.mod`。

## 推荐部署目标

- 对外服务工程：`video-service/`
- 对接方式：HTTP REST API（JSON）
- HTTP 入口：`video-service/cmd/httpapi`（默认 `:8081`）
- 异步处理入口：`video-service/cmd/worker`（转码 + 向量化 + 知识点视频 + RecBole 训练调度）
- RecBole 训练入口：`video-service/cmd/recboletrainer`（独立训练调度容器）
- 健康检查：`GET /healthz`
- Swagger：`GET /swagger/index.html`

## 仓库结构

```text
.
├── README.md
├── PROJECT_PARAMETERS.md            # 中文参数总览文档
├── PROJECT_PARAMETERS_EN.md         # 英文参数总览文档
├── AGENTS.md                        # AI agent 行为准则
├── docker-compose*.yml              # 源码构建 / 联调 / 云 / RecBole / 迁移编排
├── deployment/                      # 服务器部署包、拓扑与运维脚本
├── docs/                            # 仓库级设计文档、演示文稿
├── video-service/                   # Go HTTP 后端 + worker（推荐部署）
├── short-video-web/                 # 抖音风格短视频客户端（:5174）
├── hls-web/                         # 视频与推荐统一联调控制台（:5173）
├── recbole-training/                # RecBole 推荐训练代码、数据与模型产物
├── two-tower-training/              # 历史双塔训练（已归档）
├── gorse/                           # Gorse 可选集成配置
└── legacy-video/                    # 历史 Go 工程（不建议新接入）
```

## 核心能力

- **用户体系**：注册 / 登录（JWT）、资料与头像、主页访问统计、关注 / 取关、用户搜索。
- **视频发布**：上传（含分片断点续传）→ 转码 HLS → 封面；向量化按内容逻辑分段 + 摘要 + embedding；播放进度条分段标记（`GET /api/videos/:id/segments`）。
- **社交互动**：点赞 / 双赞 / 踩（Redis 原子切换 + worker 落库）、两级评论 + @提及通知、私信会话与未读数、消息/通知红点总数、主页喜欢列表同步。
- **推荐引擎**：社交召回 + 热门加权 + 新视频扶持 + 用户发布纳入推荐池；RecBole 离线训练 + 发布门禁；AI provider 异常降级与向量化异步补偿。

## 文档导航

- HTTP 后端服务说明：[`video-service/README.md`](video-service/README.md)
- 抖音风格客户端：[`short-video-web/README.md`](short-video-web/README.md)
- 联调控制台：[`hls-web/README.md`](hls-web/README.md)
- RecBole 训练说明：[`recbole-training/README.md`](recbole-training/README.md)
- 中文参数参考：[`PROJECT_PARAMETERS.md`](PROJECT_PARAMETERS.md)
- English parameter reference: [`PROJECT_PARAMETERS_EN.md`](PROJECT_PARAMETERS_EN.md)
- 历史双塔训练说明：[`two-tower-training/README.md`](two-tower-training/README.md)
- 仓库级文档说明：[`docs/README.md`](docs/README.md)
- 接口契约：[`video-service/docs/swagger/swagger.yaml`](video-service/docs/swagger/swagger.yaml)
- RecBole 算法交接：[`recbole-training/ALGORITHM_HANDOFF.md`](recbole-training/ALGORITHM_HANDOFF.md)
- 服务器部署手册：[`deployment/DEPLOYMENT.md`](deployment/DEPLOYMENT.md)

## 根目录开发与联调部署

根目录 `docker-compose.yml` 是源码构建和联调编排：

- `api`：对外 HTTP API，单独健康检查和重启。
- `worker`：独立消费通用视频转码、向量化、知识点视频转码和 Gorse 同步任务。
- `recbole_trainer`：独立训练调度容器，按计划执行 RecBole 训练、门禁和发布。
- `frontend`：构建后的静态前端（Nginx 代理 `/api`、`/videos`、`/swagger` 等到 `api`）。
- `gorse`：可选推荐服务，默认不暴露宿主机端口。

常用环境变量：

| 环境变量 | 作用 |
| --- | --- |
| `VIDEO_APP_ENV_FILE` | Compose 使用的私有环境文件，本地默认 `.env.local` |
| `VIDEO_APP_HTTP_PORT` | 宿主机暴露的后端 HTTP 端口，默认 `8083` |
| `VIDEO_APP_WEB_PORT` | 宿主机暴露的调试前端端口，默认 `1325` |
| `GORSE_DASHBOARD_USERNAME` / `GORSE_DASHBOARD_PASSWORD` | 可选 Gorse 诊断 Dashboard 登录信息 |

启动示例：

```bash
cp .env.deploy.example .env.deploy
# 编辑 .env.deploy，填入数据库 DSN、对象存储、AI API key
docker compose up -d
```

查看状态：

```bash
docker compose ps
docker compose logs -f api worker
```

## 优先查看

- `video-service/cmd/httpapi/main.go`
- `video-service/cmd/worker/main.go`
- `video-service/cmd/recboletrainer/main.go`
- `video-service/internal/http/router/router.go`
- `video-service/internal/application/videoapp/`
- `video-service/docs/swagger/swagger.yaml`
- `short-video-web/src/App.vue`、`short-video-web/src/components/VideoCard.vue`
- `recbole-training/scripts/run_recbole_pipeline.sh`
- `hls-web/vite.config.js`

## 补充说明

- `video-service/` 是当前唯一推荐对接入口；新集成应使用标准 REST 路径，不要依赖历史兼容路径。
- 向量化链路为 `hierarchical` 模式：prepare → coarse（ASR）→ refine（LLM 逻辑分段 + embedding）→ finalize，通过 Redis Stream 四阶段队列驱动。
- 配置密钥不写入 YAML / 配置文件；API Key 只放 git 忽略的本地 `.env.local`，通过 `VIDEO_APP_ENV_FILE` 加载。
- `legacy-video/`、`two-tower-training/` 是历史工程，除非需要维护历史链路，否则不建议作为新功能入口。
