# hls-web — 视频服务调试前端

基于 Vue 3 + Vite 构建的视频服务前端调试工程，用于联调和验证 `video-service/` 后端服务。

## 技术栈

- Vue 3 (Composition API)
- Vite 8
- hls.js (HLS 播放)
- Vitest (单元测试)

## 快速启动

```bash
npm install
npm run dev
```

默认启动在 `http://localhost:5173`，通过 Vite proxy 将 `/api`、`/videos`、`/swagger` 转发到后端（默认 `http://localhost:8081`，可通过 `VITE_PROXY_TARGET` 覆盖）。

## 可用命令

| 命令 | 说明 |
|------|------|
| `npm run dev` | 启动 Vite 开发服务器 |
| `npm run build` | 生产构建，输出到 `dist/` |
| `npm run preview` | 预览生产构建 |
| `npm test` | 运行 Vitest 单元测试 |

## 主要功能

### 视频上传

- 普通 multipart 上传
- 分片断点续传上传
- ZIP 批量导入

### 视频播放

- HLS 播放器组件
- 视频列表、播放、删除

### 反馈与互动

- 视频反馈（like / dislike）
- 视频片段反馈
- 反馈计数查询

### 其他

- 观看记录上报
- 随机片段播放
- 系统运行指标展示

## Docker 部署

根目录 `docker-compose.yml` 中包含 `frontend_web` 服务，使用 Node 22 镜像，启动后监听 `1325` 端口并通过环境变量 `VITE_PROXY_TARGET=http://backend_http:8083` 代理到后端。
