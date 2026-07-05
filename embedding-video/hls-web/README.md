# hls-web - 视频服务调试前端

基于 Vue 3 + Vite 构建的视频服务前端调试/测试工程，用于联调和验证 `hengshui-tablet-video-http/` 后端服务。

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

## 主要功能模块

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

## 测试

测试文件与对应源文件同目录，使用 Vitest：

- `src/chunkedUpload.test.js` — 分片上传逻辑测试
- `src/randomSegment.test.js` — 随机片段测试
- `src/segmentReaction.test.js` — 片段反馈测试
- `src/watchProgress.test.js` — 观看进度测试

```bash
npm test
```

## Docker 部署

根目录 `docker-compose.yml` 中包含 `frontend_web` 服务，使用 Node 22 镜像，启动后监听 `1325` 端口并通过环境变量 `VITE_PROXY_TARGET=http://backend_http:8083` 代理到后端。
