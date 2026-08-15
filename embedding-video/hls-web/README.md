# hls-web - 视频与推荐统一控制台

该 Vue 3 + Vite 工程是 `video-service/` 的统一联调前端。登录后可通过页面顶部的工作区切换器，在“视频调试台”“推荐控制台”和“知识点视频”之间切换。

它是调试控制台，不是独立后端或账号管理系统；服务配置、API 契约和部署入口以 `../video-service/README.md` 为准。

控制台使用后端管理员 JWT 鉴权。登录账号来自后端 `sys_user`，仓库不提供或记录默认管理员密码；生产环境必须设置独立的 `JWT_SECRET` 并按后端规则维护管理员账号。

## 三个工作区

- **视频调试台**：保留上传、播放、视频管理、题库、反馈和观看追踪等视频服务联调能力。
- **推荐控制台**：集中展示推荐链路诊断、运行状态、数据源、RecBole 性能趋势、业务效果指标、链路追踪、Redis 状态和结果预览。
- **知识点视频**：批量导入 ZIP 视频包与 XLSX 映射表，查看处理进度、筛选知识点树，并预览同一知识点下的多个 HLS 视频。

三个工作区共用一个管理员会话和顶部应用栏，同一时刻只挂载当前选中的工作区。浏览器会保存 access token、管理员资料、当前工作区和推荐控制台的当前栏目。

## 技术栈

- Vue 3 (Composition API, `<script setup>`)
- Vite 8
- hls.js (HLS 播放)
- KaTeX (数学公式渲染)
- Vitest (单元测试)

## 快速启动

```bash
cd hls-web
npm install
npm run dev
```

默认启动在 `http://localhost:5173`，通过 Vite proxy 将 `/api`、`/videos`、`/swagger` 和 `/knowledge-video-media` 请求转发到后端服务（默认 `http://localhost:8081`，可通过 `VITE_PROXY_TARGET` 环境变量覆盖）。

## 可用命令

| 命令 | 说明 |
|------|------|
| `npm run dev` | 启动 Vite 开发服务器 (port 5173) |
| `npm run build` | 生产构建，输出到 `dist/` |
| `npm run preview` | 预览生产构建 |
| `npm test` | 运行 Vitest 单元测试 |

## 管理员登录

1. 登录表单调用 `POST /api/auth/login`，提交 `username` 和 `password`。
2. 成功响应中的 `access_token` 与管理员资料保存在浏览器本地会话中。
3. 页面恢复已有会话时调用 `GET /api/auth/me` 重新校验账号；受保护请求统一附带 `Authorization: Bearer <token>`。
4. 任一受保护请求返回 `401` 或 `403` 时，前端清除会话并要求重新登录；用户也可主动退出。

控制台不创建或重置账号，当前后端服务也没有对应 API；管理员必须由现有身份/数据库运维流程预置到 `sys_user`。密码校验、管理员状态和令牌有效期由后端负责。浏览器本地保存只用于维持会话，不能替代后端授权检查。

## 视频调试台

### 视频上传与调试

- 普通 multipart 上传调试
- 分片断点续传上传调试 (`src/chunkedUpload.js`)
- ZIP 批量导入调试

### 视频播放

- HLS 播放器组件 (`src/components/HlsPlayer.vue`)
- 视频列表、播放、删除、发布管理

### 反馈与互动

- 视频反馈（like / double_like / dislike）
- 视频片段反馈 (`src/segmentReaction.js`)
- 反馈计数查询

### 观看追踪

- 观看记录上报
- 随机片段播放 (`src/randomSegment.js`)
- 观看进度追踪 (`src/watchProgress.js`)

## 推荐控制台

推荐控制台包含七个模块：

1. **诊断中心**：查看健康检查、数据新鲜度、最近推荐请求以及训练与同步状态。
2. **概览**：查看推荐引擎、Gorse、RecBole 和 Redis 的运行摘要。
3. **数据源**：查看视频与片段池、曝光与观看、用户信号等推荐输入数据。
4. **命中效果**：按日期和指标查看 RecBole 离线评估趋势，并按时间和策略查看业务曝光、观看及命中率变化。
5. **链路追踪**：追踪 random-play 和按题推荐的候选、过滤及排序阶段。
6. **Redis 状态**：查看指定用户的 random-play 播放桶和最近播放去重状态。
7. **预览调试**：预览 random-play 和按题推荐结果，不写入正式推荐反馈。

## 推荐管理接口

推荐控制台使用的管理接口统一位于 `/api/admin/recommendation/*`：

| 方法 | 路径 | 用途 |
|------|------|------|
| `GET` | `/api/admin/recommendation/overview` | 推荐运行概览 |
| `GET` | `/api/admin/recommendation/diagnostics` | 健康、数据新鲜度、请求与任务诊断 |
| `GET` | `/api/admin/recommendation/datasources` | 推荐数据源统计 |
| `GET` | `/api/admin/recommendation/effects` | 推荐效果指标 |
| `GET` | `/api/admin/recommendation/recbole/performance` | RecBole 离线评估指标与模型版本时间序列 |
| `GET` | `/api/admin/recommendation/trace/random-play` | random-play 链路追踪 |
| `POST` | `/api/admin/recommendation/trace/by-question` | 按题推荐链路追踪 |
| `GET` | `/api/admin/recommendation/redis-state` | 用户播放桶与最近播放状态 |
| `GET` | `/api/admin/recommendation/preview/random-play` | random-play 结果预览 |
| `POST` | `/api/admin/recommendation/preview/by-question` | 按题推荐结果预览 |

## 知识点视频工作区

批量导入需要一个 ZIP 视频包和一个 `.xlsx` 映射表。映射表第一个 worksheet 的首行必须严格为 `id`、`name`、`video_name`，视频名按 ZIP entry 的 basename 匹配；完整校验规则见后端 README 的“知识点视频导入文件约定”。服务从管理员 JWT 获取上传用户 ID，验证文件后异步转码；页面轮询批次状态并在完成后刷新知识点树。同一知识点可关联并同时展示多个可播放视频。

| 方法 | 路径 | 用途 |
|------|------|------|
| `POST` | `/api/admin/knowledge-videos/batches` | 携带管理员 JWT 上传 ZIP 和 XLSX，创建导入批次 |
| `GET` | `/api/admin/knowledge-videos/batches/:batchId` | 查询批次和逐视频处理状态 |
| `GET` | `/api/knowledge-videos/tree` | 获取知识点树及其视频状态 |
| `GET` | `/api/knowledge-points/:knowledgePointId/videos` | 获取知识点下全部可播放视频 |
| `POST` | `/api/knowledge-videos/:knowledgeVideoId/playbacks` | 记录用户实际播放 |
| `PUT` | `/api/knowledge-videos/:knowledgeVideoId/watch-sessions/:sessionId` | 幂等上报当前会话的实际累计观看秒数 |
| `GET` | `/knowledge-video-media/hls/:videoId/*filepath` | 代理知识点视频 HLS 资源 |

播放器使用 `crypto.randomUUID()` 为每次播放生成符合后端 `[A-Za-z0-9_-]{16,64}` 约束的 session ID，只在实际播放期间累计墙钟秒数，并按绝对累计值周期上报；暂停、结束和组件卸载时也会尽力刷新。客户端重试不会重复累计，服务端按会话保留最大值并汇总多个会话。累计达到视频时长 60% 后，该行为可作为 RecBole 的训练期辅助信号，但知识点视频不会成为线上推荐候选。

播放记录和观看会话接口本身是公开路由，`user_id` 由调用方提供，后端没有把它绑定到管理员 JWT 或浏览器身份。生产接入必须由上游业务服务/网关完成身份绑定、限流和防刷，避免冒用用户并污染训练数据。

## 测试

测试文件与对应源文件同目录，使用 Vitest：

统一控制台与推荐工作区新增测试：

- `src/appShell.test.js` - 统一登录、工作区切换和样式隔离
- `src/auth/api.test.js` - 后端登录、Bearer token 和会话失效处理
- `src/auth/session.test.js` - 管理员 token/资料的本地持久化与清理
- `src/config/consoleSession.test.js` - 工作区与推荐栏目持久化
- `src/recommendation/api/recommendationConsole.test.js` - 推荐管理 API 请求构造
- `src/recommendation/recbolePerformance.test.js` - RecBole 时间序列归一化与 SVG 几何计算
- `src/recommendation/components/RecBolePerformanceChart.test.js` - 趋势图控件、状态和页面位置契约
- `src/recommendation/config/sectionSession.test.js` - 推荐栏目状态持久化
- `src/workspaceBoundary.test.js` - 视频与推荐工作区边界
- `src/knowledgeVideo/api.test.js` - 知识点视频上传、轮询和多视频响应归一化
- `src/knowledgeVideo/proxyConfig.test.js` - 本地代理路径契约
- `src/knowledgeVideo/watchSession.test.js` - 观看会话串行重试、绝对时长和确认进度
- `src/knowledgeVideo/workspace.test.js` - 知识点视频工作区交互契约

保留的现有视频调试测试：

- `src/adminLayout.test.js` - 视频调试台布局约束
- `src/archiveProgressStorage.test.js` - 批量导入进度持久化
- `src/chunkedUpload.test.js` - 分片上传逻辑
- `src/randomSegment.test.js` - 随机片段
- `src/segmentReaction.test.js` - 片段反馈
- `src/watchProgress.test.js` - 观看进度

```bash
npm test
```

## Docker 部署

根目录 `docker-compose.yml` 中的 `frontend` 服务会先构建静态资源，再由 Nginx 监听容器 `8080` 端口；宿主机端口由 `VIDEO_APP_WEB_PORT` 控制，默认 `1325`。Nginx 将 `/api`、`/videos`、`/swagger` 和 `/knowledge-video-media` 代理到 `API_UPSTREAM`，根 Compose 中默认是 `api:8081`。

RecBole 趋势由 `api` 服务从 `recsys.recommend_model_version.metrics_json` 读取，浏览器不直接访问训练产物目录。
