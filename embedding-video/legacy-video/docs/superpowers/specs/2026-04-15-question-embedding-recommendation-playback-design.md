# 按问题向量检索最相近视频分段并可流式播放（强制片段结束）设计

## 目标

用户输入一个问题字符串，服务端将该文本做 embedding，基于 pgvector 从数据库中检索最相近的 3 个视频分段（允许来自同一视频），并返回可播放信息。播放器需要支持边加载边播放，并且在达到分段 `end_time` 时强制停止。

## 现状与复用

- 已有 gRPC：`RecommendByQuestion`（RPC 层）可对 `question_text` 生成 embedding，并使用 `edu_video_segment.embedding <=> query_vector` 进行相似检索，返回分段与视频基本信息。
- 已有 gRPC：`PlayVideo(video_id)` 返回可播放 URL。
- API 层已有对象存储代理 `/videos/*filepath`，支持 `Range`（206），可用于 MP4 的边下边播；HLS（m3u8）天然流式。

## 需求澄清（已确认）

- Top3 允许来自同一个 `video_id`（不做去重）。
- 播放必须“按片段范围”控制：seek 到 `start_time` 开始播放，到 `end_time` 强制停止。
- 前端进度条与拖动进度必须只反映该片段区间，看起来像“只播放一段被截取的视频”，但不要求后端实际拆分文件。
- 推荐结果展示为列表：
  - 封面使用视频封面（`edu_video_resource.cover_url`）
  - 标题使用分段内容摘要（`edu_video_segment.content_summary`）

## 方案概述（选型）

采用 API 聚合方案：

1) RPC 返回推荐分段时同时返回该分段的 `start_time_sec/end_time_sec`。  
2) API 新增 HTTP 接口：调用 gRPC `RecommendByQuestion(limit=3)` 获取推荐列表；再对每个 `video_id` 调用 gRPC `PlayVideo` 拿到 `play_url`，合并到返回中。  
3) 前端播放器按 `start_time_sec/end_time_sec` 做“片段播放控制”，以 HLS 或支持 Range 的 MP4 实现边加载边播放。

## 接口设计

### gRPC（proto）变更

对 `RecommendItem` 增加字段：

- `int32 start_time_sec`
- `int32 end_time_sec`

语义：

- `start_time_sec/end_time_sec` 来自 `edu_video_segment.start_time/end_time`（秒）
- API/前端使用该范围进行片段定位播放与强制停止

### HTTP API 新增接口

`POST /api/video/recommend_by_question`

请求体（JSON）：

- `question_text`：string，必填

说明：API 侧固定 `limit=3`，请求体只允许传 `question_text`；API 调 gRPC 时不传 `user_id/question_id`（均为 0），由 RPC 使用默认行为处理。

响应体（JSON）：

- `success`：bool
- `total`：int
- `items[]`：
  - `question_id`
  - `video_id`
  - `video_segment_id`
  - `recommend_score`
  - `start_time_sec`
  - `end_time_sec`
  - `title`：分段标题（取 `content_summary`）
  - `cover_url`：视频封面（取 `cover_url`）
  - `play_url`
  - `video`：可选（如需复用现有字段，可继续透传 `VideoInfo`；但列表展示以 `title/cover_url` 为准）

## 数据流

1) 前端提交 `question_text` 到 API
2) API 调用 gRPC `RecommendByQuestion`：
   - RPC 侧调用 embedding 服务生成 query vector
   - SQL：从 `edu_video_segment`（`deleted=0 AND status=1`）按 `<=>` 排序取 TopN
   - 返回时包含 `start_time/end_time`，并返回展示字段：
     - `title = edu_video_segment.content_summary`
     - `cover_url = edu_video_resource.cover_url`
3) API 对 TopN 做 `PlayVideo(video_id)`（按 `video_id` 做简单去重缓存），得到 `play_url`
4) API 返回聚合后的 items（包含 `play_url + start/end`）
5) 前端播放器：
   - 载入 `play_url`，Ready 后 seek 到 `start_time_sec` 播放
   - 监听播放进度，达到 `end_time_sec` 强制 `pause` 并将 `currentTime` 固定在 `end_time_sec`

## 播放与“边加载边播”

- HLS（m3u8）：浏览器/播放器按分片请求，天然边下边播
- MP4：依赖浏览器 Range 请求；API 代理已支持 `Range`，可实现渐进式下载播放

## 前端播放控制（片段进度条与强制停止）

由于 HTML5 `<video>` 的原生 controls 进度条使用的是媒体的真实 `duration`（不可直接改写），要实现“进度条只显示 start~end 这一段”的效果，需要关闭原生 controls，使用自定义进度条与拖动逻辑：

- 初始化：媒体 ready 后 `video.currentTime = start_time_sec` 并 `video.play()`
- 进度显示：
  - `segmentDuration = end_time_sec - start_time_sec`
  - `segmentCurrent = clamp(video.currentTime - start_time_sec, 0, segmentDuration)`
  - UI 进度百分比：`segmentCurrent / segmentDuration`
- 拖动 seek：
  - 用户拖到 `p(0..1)`，执行 `video.currentTime = start_time_sec + p*segmentDuration`
- 强制停止：
  - 监听 `timeupdate`（必要时加 `setInterval` 兜底），若 `video.currentTime >= end_time_sec`：
    - `video.pause()`
    - `video.currentTime = end_time_sec`

## 错误处理与降级

- `question_text` 为空：HTTP 400 / gRPC InvalidArgument
- embedding 服务失败：HTTP 500 / gRPC Internal
- 推荐结果为空：HTTP 200，`items=[]`
- `PlayVideo(video_id)` 失败：
  - 保留 item 但 `play_url=""`，便于排查；前端对空 `play_url` 做不可播放提示

## 性能与约束

- Top3 场景下 API 额外 gRPC 调用最多 3 次；使用 `video_id -> play_url` 缓存避免重复调用。
- SQL 使用 `ORDER BY s.embedding <=> ? LIMIT ?`，依赖 pgvector 索引（ivfflat）以提升检索效率。

## 兼容性

- 不改变现有 `RecommendByQuestion` 的核心行为，仅扩展返回字段。
- 新增 API 不影响现有 `/api/video/play/:id` 等接口。

## 验收标准

- 输入任意问题可返回最多 3 条推荐分段，包含 `video_segment_id`、`start_time_sec/end_time_sec`、`play_url`。
- 使用返回结果可流式播放（HLS/MP4 均可边加载边播放）。
- 播放到 `end_time_sec` 必须强制停止，不继续播放后续内容。
