# 问题向量检索推荐分段 + 片段化播放（进度条仅显示片段）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用户在 API 仅提交问题字符串，后端生成 embedding 并在 pgvector 中检索最相近的 3 个视频分段，返回“列表展示所需字段 + 可播放 URL + 片段 start/end”，前端按片段范围播放（到 end 强制停止、进度条只显示片段区间）。

**Architecture:** 复用 RPC 的 `RecommendByQuestion`（embedding + pgvector 检索）能力，扩展其返回分段 `start_time/end_time` 并将列表标题/封面按业务要求映射；API 新增 `POST /api/video/recommend_by_question`，请求体只包含 `question_text`，服务端固定 `limit=3` 调用 gRPC，并对每条结果补齐 `play_url`（调用 `PlayVideo(video_id)`，按 `video_id` 去重缓存）。

**Tech Stack:** Go (Gin, gRPC, GORM), PostgreSQL + pgvector, RustFS/S3 代理 Range 支持（MP4 渐进式），HLS(m3u8)。

---

## Files & Responsibilities

**Modify**
- `video/video_service.proto`：给推荐结果补充分段时间字段（用于片段播放）。
- `video/video_service.pb.go`、`video/video_service_grpc.pb.go`：重新生成 protobuf 代码。
- `internal/rpc/service/recommend_logic.go`：推荐查询 SQL 补 `start_time/end_time/content_summary/cover_url`，并按要求映射返回字段。
- `internal/api/router/router.go`：新增 API 路由 `POST /api/video/recommend_by_question`。
- `internal/api/handler/handler.go`：新增 handler：只收 `question_text`，调用 gRPC 获取 top3，并补齐 `play_url`。
- `docs/superpowers/specs/2026-04-15-question-embedding-recommendation-playback-design.md`：更新说明：API 请求仅传问题字符串，列表字段来源明确。

---

### Task 1: 更新 Spec（对齐“API 只传问题字符串”与列表字段）

**Files:**
- Modify: `docs/superpowers/specs/2026-04-15-question-embedding-recommendation-playback-design.md`

- [ ] **Step 1: 更新 API 请求体字段描述（仅 question_text）**

把“请求体（JSON）”改为仅包含：

```markdown
请求体（JSON）：
- `question_text`：string，必填
```

并说明 API 内部固定参数：

```markdown
说明：API 侧固定 `limit=3`，并在 gRPC 请求中不传 `user_id/question_id`（均为 0），由 RPC 使用默认行为处理。
```

- [ ] **Step 2: 提交文档变更**

Run:

```bash
git add docs/superpowers/specs/2026-04-15-question-embedding-recommendation-playback-design.md
git commit -m "docs(spec): API仅接收问题字符串并明确列表字段来源"
```

---

### Task 2: 扩展 proto（推荐分段补 start/end 秒）

**Files:**
- Modify: `video/video_service.proto`
- Modify: `video/video_service.pb.go`
- Modify: `video/video_service_grpc.pb.go`

- [ ] **Step 1: 修改 proto 的 RecommendItem**

在 `message RecommendItem` 中增加分段时间字段（使用新的 field number，避免破坏兼容性）：

```proto
message RecommendItem {
  uint64 question_id = 1;
  uint64 video_id = 2;
  uint64 video_segment_id = 3;
  double recommend_score = 4;
  bool is_watched = 5;
  int32 watch_duration = 6;
  VideoInfo video = 7;

  int32 start_time_sec = 8;
  int32 end_time_sec = 9;
}
```

- [ ] **Step 2: 重新生成 pb 代码**

在 `nlp-video-project/` 工程目录（包含 `video/` 子目录）执行：

```bash
protoc --go_out=. --go-grpc_out=. video/video_service.proto
```

期望：
- `video/video_service.pb.go`、`video/video_service_grpc.pb.go` 发生更新

- [ ] **Step 3: 编译检查（不运行）**

```bash
go build ./...
```

- [ ] **Step 4: 提交**

```bash
git add video/video_service.proto video/video_service.pb.go video/video_service_grpc.pb.go
git commit -m "feat(proto): 推荐结果补充分段start/end秒"
```

---

### Task 3: RPC 推荐查询补字段并按业务映射（title=content_summary，cover=video cover）

**Files:**
- Modify: `internal/rpc/service/recommend_logic.go`

- [ ] **Step 1: 扩展 SQL 查询字段**

将当前 SQL 改为同时取出：
- `s.start_time`、`s.end_time`
- `s.content_summary`（作为列表标题）
- `r.cover_url`（作为列表封面）

示例（只展示核心差异，按现有 Raw SQL 格式替换）：

```sql
SELECT
  s.id AS video_segment_id,
  s.video_id AS video_id,
  s.start_time AS start_time_sec,
  s.end_time AS end_time_sec,
  (s.embedding <=> ?) AS distance,
  s.content_summary AS segment_title,
  r.cover_url AS cover_url,
  r.video_url AS video_url,
  r.is_published AS is_published,
  r.is_recommend AS is_recommend,
  r.view_count AS view_count,
  r.create_time AS create_time,
  r.update_time AS update_time
FROM edu_video_segment s
JOIN edu_video_resource r ON r.id = s.video_id
WHERE s.deleted = 0 AND s.status = 1 AND r.deleted = 0
ORDER BY s.embedding <=> ?
LIMIT ?;
```

- [ ] **Step 2: 扩展 row struct 并映射到 proto**

把 `row` 增加字段，并在构造 `RecommendItem` 时填充：
- `StartTimeSec/EndTimeSec`
- `Video.Name` 用 `content_summary`（即 `segment_title`）
- `Video.CoverUrl` 用 `cover_url`

示例（保持现有字段风格）：

```go
type row struct {
    VideoSegmentID uint64  `gorm:"column:video_segment_id"`
    VideoID        uint64  `gorm:"column:video_id"`
    StartTimeSec   int     `gorm:"column:start_time_sec"`
    EndTimeSec     int     `gorm:"column:end_time_sec"`
    Distance       float64 `gorm:"column:distance"`
    SegmentTitle   string  `gorm:"column:segment_title"`
    VideoURL       string  `gorm:"column:video_url"`
    CoverURL       string  `gorm:"column:cover_url"`
    IsPublished    bool    `gorm:"column:is_published"`
    IsRecommend    bool    `gorm:"column:is_recommend"`
    ViewCount      int     `gorm:"column:view_count"`
    CreateTime     time.Time `gorm:"column:create_time"`
    UpdateTime     time.Time `gorm:"column:update_time"`
}
```

映射时：

```go
items = append(items, &video.RecommendItem{
    QuestionId:     questionID,
    VideoId:        r.VideoID,
    VideoSegmentId: r.VideoSegmentID,
    RecommendScore: score,
    StartTimeSec:   int32(r.StartTimeSec),
    EndTimeSec:     int32(r.EndTimeSec),
    Video: &video.VideoInfo{
        Name:     r.SegmentTitle,
        RawUrl:   r.VideoURL,
        CoverUrl: r.CoverURL,
        // 其它字段保持现状
    },
})
```

- [ ] **Step 3: 编译检查（不运行）**

```bash
go build ./...
```

- [ ] **Step 4: 提交**

```bash
git add internal/rpc/service/recommend_logic.go
git commit -m "feat(rpc): 推荐分段补start/end并用content_summary作为标题"
```

---

### Task 4: 新增 API：只接收 question_text，返回 Top3 分段列表 + play_url + start/end

**Files:**
- Modify: `internal/api/router/router.go`
- Modify: `internal/api/handler/handler.go`

- [ ] **Step 1: 增加路由**

在 `SetupRouter()` 的 `/api/video` 分组下添加：

```go
videoGroup.POST("/recommend_by_question", handler.RecommendByQuestion)
```

- [ ] **Step 2: 实现 handler（请求体只包含 question_text）**

在 `handler.go` 新增：

```go
type recommendByQuestionReq struct {
    QuestionText string `json:"question_text"`
}

func RecommendByQuestion(c *gin.Context) {
    var req recommendByQuestionReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
        return
    }
    qt := strings.TrimSpace(req.QuestionText)
    if qt == "" {
        c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "question_text 不能为空"})
        return
    }

    // 固定 top3；API 不接收/不透传 user_id/question_id
    resp, err := client.GetVideoClient().RecommendByQuestion(c, &video.RecommendByQuestionRequest{
        QuestionText: qt,
        Limit:        3,
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
        return
    }

    playURLCache := make(map[uint64]string, 8)
    items := make([]gin.H, 0, len(resp.Items))
    for _, it := range resp.Items {
        vid := it.GetVideoId()
        playURL, ok := playURLCache[vid]
        if !ok {
            pr, err := client.GetVideoClient().PlayVideo(c, &video.PlayVideoRequest{VideoId: vid})
            if err == nil {
                playURL = pr.GetPlayUrl()
            }
            playURLCache[vid] = playURL
        }

        title := ""
        coverURL := ""
        if it.GetVideo() != nil {
            title = it.GetVideo().GetName()
            coverURL = it.GetVideo().GetCoverUrl()
        }

        items = append(items, gin.H{
            "video_id":         vid,
            "video_segment_id": it.GetVideoSegmentId(),
            "recommend_score":  it.GetRecommendScore(),
            "start_time_sec":   it.GetStartTimeSec(),
            "end_time_sec":     it.GetEndTimeSec(),
            "title":            title,    // content_summary
            "cover_url":        coverURL, // video cover
            "play_url":         playURL,
        })
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "total":   len(items),
        "items":   items,
    })
}
```

注意：上面用到了 `strings`，需要补充 import。

- [ ] **Step 3: 编译检查（不运行）**

```bash
go build ./...
```

- [ ] **Step 4: 提交**

```bash
git add internal/api/router/router.go internal/api/handler/handler.go
git commit -m "feat(api): 按问题字符串返回top3相近分段并补齐play_url"
```

---

### Task 5: 手工验证（HTTP + 片段播放行为说明）

**Files:**
- (no repo changes required)

- [ ] **Step 1: 启动 rpc 与 api（建议用 go build 避免 go run 被策略拦截）**

```bash
cd nlp-video-project

go build -o .\bin\rpc.exe .\cmd\rpc
go build -o .\bin\api.exe .\cmd\api

.\bin\rpc.exe
```

另开一个终端：

```bash
.\bin\api.exe
```

- [ ] **Step 2: 调用 API 验证返回字段**

```bash
curl -X POST "http://127.0.0.1:8081/api/video/recommend_by_question" ^
  -H "Content-Type: application/json" ^
  -d "{\"question_text\":\"什么是一次函数\"}"
```

期望返回字段：
- `items[0].title` 为分段 `content_summary`
- `items[0].cover_url` 为视频封面
- `items[0].play_url` 有值（可用 `/api/video/play/:id` 的同源 URL 或直链）
- `start_time_sec/end_time_sec` 为分段时间范围

- [ ] **Step 3: 前端片段播放要点（供外部前端实现）**

由于原生 `<video controls>` 进度条无法改写 duration，前端必须自定义进度条：

```js
const start = item.start_time_sec
const end = item.end_time_sec
const segDur = Math.max(0.1, end - start)

video.src = item.play_url
video.addEventListener('loadedmetadata', () => {
  video.currentTime = start
  video.play()
})

video.addEventListener('timeupdate', () => {
  if (video.currentTime >= end) {
    video.pause()
    video.currentTime = end
  }
  const segCurrent = Math.min(segDur, Math.max(0, video.currentTime - start))
  progress.value = segCurrent / segDur
})

progress.addEventListener('input', () => {
  const p = Number(progress.value) // 0..1
  video.currentTime = start + p * segDur
})
```

---

## Self-Review

- Spec coverage：
  - “API 只传问题字符串”：Task 1 + Task 4
  - “Top3 相近分段”：Task 4 固定 limit=3
  - “标题=content_summary、封面=cover_url”：Task 3 映射 + Task 4 输出
  - “片段播放 + 强制停止 + 片段进度条”：Task 2/3 补 start/end + Task 5 给出前端实现要点
- Placeholder scan：无 TBD/TODO
- Type consistency：proto 字段名 `start_time_sec/end_time_sec` 在 RPC、API 一致使用

---

Plan complete and saved to `docs/superpowers/plans/2026-04-15-question-recommendation-segment-playback.md`. Two execution options:

1. Subagent-Driven (recommended) - I dispatch a fresh subagent per task, review between tasks, fast iteration
2. Inline Execution - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
