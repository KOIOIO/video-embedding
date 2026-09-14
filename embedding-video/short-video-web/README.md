# short-video-web - 抖音风格短视频客户端

面向最终用户的短视频产品前端：注册 / 登录后，在抖音风格界面中上下滑动浏览视频，支持点赞 / 双赞 / 踩、评论（@提及）、关注、私信、发布视频、作品 / 喜欢列表、播放进度条分段标记等完整社交功能。

## 功能

**浏览与播放**

- 随机视频信息流：`GET /api/video-segments/random-play`，按用户去重、预取下一条、播完自动切换
- 上下滑动切换（触摸 + 鼠标滚轮），滑动累积锁定避免误滑
- 视频整片 HLS 播放；播放进度条上按内容分段标记（时间区间 + 摘要），点击可跳转对应时间点
- 作品 / 喜欢列表 HLS 连续播放，刷完为止

**互动**

- 点赞 / 双赞 / 踩：同一视频同一用户只保留一种互动（再点同类型取消、点不同类型切换），双击屏幕点赞，激活按钮变红，状态按用户持久化
- 两级评论：查看 / 发布评论与二级回复，评论旁显示昵称、头像与相对时间，@提及用户搜索（好友优先）与高亮
- 评论点赞 / 双赞 / 踩，语义与视频互动一致，Redis Lua 原子切换 + worker 异步落库

**用户与社交**

- 注册（自动登录）/ 登录，JWT 会话
- 个人资料编辑、头像上传
- 关注 / 取关、粉丝 / 关注列表
- 私信会话 + 未读数轮询 + 已读回执
- 底部消息 Tab 红点显示消息与通知总数
- 个人主页：作品 / 喜欢列表 Tab，点击直接播放；主页访问统计
- 发布视频：上传 → 后端转码 + 向量化，发布后主页「作品」与「喜欢」同步展示

**UI**

- TikTok 风格：底部五 Tab 导航、右侧互动栏、评论底部面板、音乐跑马灯、封面模糊背景、爱心爆裂动画

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
| `POST` | `/api/auth/register` | 注册（自动登录） |
| `POST` | `/api/auth/login` | 登录获取 token |
| `GET` | `/api/auth/me` | 校验会话 |
| `GET` / `PATCH` | `/api/me/profile` | 查看 / 修改个人资料 |
| `POST` | `/api/me/avatar` | 上传头像 |
| `GET` | `/api/me/videos` | 我发布的视频 |
| `GET` | `/api/me/visits` | 主页访问统计 |
| `GET` | `/api/users/:id/profile` | 用户主页信息 |
| `GET` | `/api/users/:id/videos` | 用户作品列表 |
| `GET` | `/api/users/:id/liked-videos` | 用户喜欢列表 |
| `POST` | `/api/users/:id/follow` | 关注 / 取关 |
| `GET` | `/api/users/:id/followers` | 粉丝列表 |
| `GET` | `/api/users/:id/following` | 关注列表 |
| `GET` | `/api/users/search` | 用户搜索（@提及） |
| `GET` | `/api/messages/conversations` | 私信会话列表 |
| `GET` | `/api/messages/:userId` | 与某用户私信 |
| `POST` | `/api/messages/:userId/read` | 标记已读 |
| `GET` | `/api/messages/unread-count` | 私信未读数 |
| `GET` | `/api/notifications` | 通知列表 |
| `GET` | `/api/notifications/unread-count` | 通知未读数 |
| `POST` | `/api/notifications/read-all` | 全部已读 |
| `GET` | `/api/video-segments/random-play?user_id=` | 随机下一条视频片段 |
| `POST` | `/api/video-segments/:id/reactions` | 点赞/双赞/踩（切换语义） |
| `GET` | `/api/video-segments/:id/reaction-counts` | 互动计数 |
| `GET` | `/api/videos/:id/segments` | 视频分段标记（进度条展示） |
| `GET` | `/api/video-segments/:id/comments` | 评论列表（含二级回复首页） |
| `POST` | `/api/video-segments/:id/comments` | 发布评论 |
| `GET` | `/api/video-segments/:id/comment-counts` | 评论总数 |
| `GET` | `/api/comments/:id/replies` | 二级回复分页 |
| `POST` | `/api/comments/:id/replies` | 回复评论 |
| `POST` | `/api/comments/:id/reactions` | 评论点赞/双赞/踩 |

## 目录结构

```text
src/
├── main.js
├── App.vue                 # 启动校验会话，登录/注册/信息流路由
├── style.css               # 全局样式
├── auth/                   # 登录、注册与本地会话
├── feed/                   # random-play、互动、评论接口与滑动纯逻辑
├── user/                   # 用户资料、视频发布、作品/喜欢接口
└── components/
    ├── LoginView.vue       # 登录页
    ├── RegisterView.vue    # 注册页
    ├── FeedView.vue        # 上下滑动信息流与预取
    ├── VideoCard.vue       # 视频卡片：HLS 播放、进度条分段标记、互动、评论入口
    ├── VideoView.vue       # 全屏视频播放视图
    ├── CommentsPanel.vue   # 评论底部面板：列表、回复、@提及、点赞
    ├── BottomNav.vue       # 底部五 Tab 导航（首页/关注/发布/消息/我的）
    ├── ProfileView.vue     # 个人主页：作品/喜欢列表
    ├── ProfilePlaylistView.vue # 作品/喜欢连续播放列表
    ├── ProfileEditView.vue # 资料编辑
    ├── PublishView.vue     # 发布视频
    ├── ChatView.vue        # 私信聊天
    ├── MessageListView.vue # 消息列表（红点总数）
    ├── FollowButton.vue    # 关注按钮
    └── FollowListView.vue  # 粉丝/关注列表
```
