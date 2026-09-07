# short-video-web - 短视频客户端

抖音风格的短视频客户端前端：登录后通过上下滑动随机浏览视频片段，双击点赞、点爱心按钮点赞，播放 HLS 视频。

## 功能

- 账号密码登录（对接后端 `POST /api/auth/login`，使用登录管理员的 `id` 作为 `user_id`）
- 随机视频流：`GET /api/video-segments/random-play`，客户端按最近播放 key 去重
- 上下滑动切换视频（触摸 + 鼠标滚轮），预取下一条，播完自动切换
- 互动三按钮：点赞 / 双赞 / 踩，对接 `POST /api/video-segments/:id/reactions`（同一片段同一用户只保留一种互动，再点同类型取消，点不同类型切换）；双击屏幕点赞；激活的按钮变红，状态由后端按用户持久化
- 评论系统（登录后可用）：
  - 查看/发布评论与二级回复（回复之间互相评论，总层级固定两级）
  - 评论互动三按钮：点赞 / 双赞 / 踩，与视频互动语义一致（再点同类型取消、点不同类型切换），激活变红、按用户持久化，后端走 Redis Lua 原子切换 + worker 异步落库
  - 评论旁显示用户名与相对时间（刚刚 / N 分钟前 / N 小时前 / N 天前 / 日期）
  - 右侧评论按钮显示评论总数
- 视频默认有声播放（浏览器阻止自动播放时降级为静音并提示点击开启声音）
- TikTok 风格 UI：右侧互动栏、评论底部面板、音乐跑马灯、封面模糊背景、爱心爆裂动画、底部进度条

## 对接的接口

| 方法 | 路径 | 用途 |
|------|------|------|
| `POST` | `/api/auth/login` | 登录获取 token 和管理员信息 |
| `GET` | `/api/auth/me` | 校验会话 |
| `GET` | `/api/video-segments/random-play?user_id=` | 随机获取下一条视频片段 |
| `POST` | `/api/video-segments/:id/reactions` | 点赞/双赞/踩（切换语义） |
| `GET` | `/api/video-segments/:id/reaction-counts` | 查询互动计数 |
| `GET` | `/api/video-segments/:id/comments` | 评论列表（含二级回复首页） |
| `POST` | `/api/video-segments/:id/comments` | 发布评论（需登录 token） |
| `GET` | `/api/video-segments/:id/comment-counts` | 评论总数 |
| `GET` | `/api/comments/:id/replies` | 二级回复分页 |
| `POST` | `/api/comments/:id/replies` | 回复评论（需登录 token） |
| `POST` | `/api/comments/:id/reactions` | 评论点赞/双赞/踩（切换语义，需登录 token） |

## 技术栈

- Vue 3 (Composition API, `<script setup>`)
- Vite 8
- hls.js（HLS 播放，Safari 走原生 HLS）
- Vitest（单元测试）

## 快速启动

```bash
cd short-video-web
npm install
npm run dev
```

默认启动在 `http://localhost:5174`，通过 Vite proxy 将 `/api`、`/videos` 请求转发到后端（默认 `http://localhost:8081`，可通过 `VITE_PROXY_TARGET` 覆盖）。

登录使用后端管理员账号（如 `admin`），登录后 token 存于 `localStorage` 的 `short-video.session`。

## 可用命令

| 命令 | 说明 |
|------|------|
| `npm run dev` | 启动 Vite 开发服务器 (port 5174) |
| `npm run build` | 生产构建，输出到 `dist/` |
| `npm run preview` | 预览生产构建 |
| `npm test` | 运行 Vitest 单元测试 |

## 对接的接口

| 方法 | 路径 | 用途 |
|------|------|------|
| `POST` | `/api/auth/login` | 登录获取 token 和管理员信息 |
| `GET` | `/api/auth/me` | 校验会话 |
| `GET` | `/api/video-segments/random-play?user_id=` | 随机获取下一条视频片段 |
| `POST` | `/api/video-segments/:id/reactions` | 点赞/双赞/踩（切换语义） |
| `GET` | `/api/video-segments/:id/reaction-counts` | 查询互动计数 |
| `GET` | `/api/video-segments/:id/comments` | 评论列表（含二级回复首页） |
| `POST` | `/api/video-segments/:id/comments` | 发布评论（需登录 token） |
| `GET` | `/api/video-segments/:id/comment-counts` | 评论总数 |
| `GET` | `/api/comments/:id/replies` | 二级回复分页 |
| `POST` | `/api/comments/:id/replies` | 回复评论（需登录 token） |
| `POST` | `/api/comments/:id/reactions` | 评论点赞/双赞/踩（切换语义，需登录 token） |

## 目录结构

```text
src/
├── main.js
├── App.vue                 # 启动时校验会话，切换登录页/信息流
├── auth/                   # 登录与本地会话
├── feed/                   # random-play、互动、评论接口与滑动纯逻辑
└── components/
    ├── LoginView.vue       # 登录页
    ├── FeedView.vue        # 上下滑动信息流与预取
    ├── VideoCard.vue       # 单条视频卡片：播放、进度、互动、评论入口
    └── CommentsPanel.vue   # 评论区底部面板：列表、回复、点赞、相对时间
```
