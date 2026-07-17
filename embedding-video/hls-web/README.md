# hls-web - 视频与推荐统一控制台

该 Vue 3 + Vite 工程是 `video-service/` 的统一联调前端。登录后可通过页面顶部的工作区切换器，在“视频调试台”和“推荐控制台”之间切换。

它是调试控制台，不是独立后端或生产鉴权边界；服务配置、API 契约和部署入口以 `../video-service/README.md` 为准。

> 登录账号 `aaddmmiinn`、密码 `admin123` 仅在浏览器内校验，用于避免误操作，不是安全鉴权。后端管理 API 在受可信网关或真实鉴权保护前，不得暴露到不受信任网络。

## 双工作区

- **视频调试台**：保留上传、播放、视频管理、题库、反馈和观看追踪等视频服务联调能力。
- **推荐控制台**：集中展示推荐链路诊断、运行状态、数据源、Gorse 性能趋势、业务效果指标、链路追踪、Redis 状态和结果预览。

两个工作区共用一个登录门禁和顶部应用栏，同一时刻只挂载当前选中的工作区。浏览器会保存 UI 解锁状态、当前工作区和推荐控制台的当前栏目。

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

默认启动在 `http://localhost:5173`，通过 Vite proxy 将 `/api`、`/videos`、`/swagger` 请求转发到后端服务（默认 `http://localhost:8081`，可通过 `VITE_PROXY_TARGET` 环境变量覆盖）。

## 可用命令

| 命令 | 说明 |
|------|------|
| `npm run dev` | 启动 Vite 开发服务器 (port 5173) |
| `npm run build` | 生产构建，输出到 `dist/` |
| `npm run preview` | 预览生产构建 |
| `npm test` | 运行 Vitest 单元测试 |

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
4. **命中效果**：按日期和指标查看 Gorse 原生推荐性能趋势，并按时间和策略查看业务曝光、观看及命中率变化。
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
| `GET` | `/api/admin/recommendation/gorse/performance` | Gorse 推荐性能指标列表与时间序列 |
| `GET` | `/api/admin/recommendation/trace/random-play` | random-play 链路追踪 |
| `POST` | `/api/admin/recommendation/trace/by-question` | 按题推荐链路追踪 |
| `GET` | `/api/admin/recommendation/redis-state` | 用户播放桶与最近播放状态 |
| `GET` | `/api/admin/recommendation/preview/random-play` | random-play 结果预览 |
| `POST` | `/api/admin/recommendation/preview/by-question` | 按题推荐结果预览 |

## 测试

测试文件与对应源文件同目录，使用 Vitest：

统一控制台与推荐工作区新增测试：

- `src/appShell.test.js` - 统一登录、工作区切换和样式隔离
- `src/config/consoleSession.test.js` - UI 会话、固定登录和工作区持久化
- `src/recommendation/api/recommendationConsole.test.js` - 推荐管理 API 请求构造
- `src/recommendation/gorsePerformance.test.js` - Gorse 时间序列归一化与 SVG 几何计算
- `src/recommendation/components/GorsePerformanceChart.test.js` - 趋势图控件、状态和页面位置契约
- `src/recommendation/config/sectionSession.test.js` - 推荐栏目状态持久化
- `src/workspaceBoundary.test.js` - 视频与推荐工作区边界

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

根目录 `docker-compose.yml` 中的 `frontend` 服务会先构建静态资源，再由 Nginx 监听容器 `8080` 端口；宿主机端口由 `VIDEO_WEB_PORT` 控制，默认 `1325`。Nginx 在内部网络将 `/api`、`/videos`、`/swagger` 代理到 `api:8081`。

Gorse 趋势由 `api` 服务使用 Dashboard 会话读取，浏览器不直接访问 Gorse，也不会接触 `GORSE_DASHBOARD_USERNAME` / `GORSE_DASHBOARD_PASSWORD`。
