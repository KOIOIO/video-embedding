# video-service 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `video-service/` 下构建一个独立的 HTTP 服务，尽量复用原有业务逻辑与接口能力，移除对 gRPC 运行链路的依赖，并提供给 Java 同事直接调用的 REST + OpenAPI 接口。

**Architecture:** 新项目以 `HTTP -> application` 为主链路，复用原项目的 `internal/application/videoapp`、`internal/infrastructure/*`、`internal/config`、`internal/lifecycle` 等核心逻辑；协议层新增 `internal/http/*` 负责 DTO、路由、错误映射与 Swagger，不再让 `internal/api/client`、`cmd/rpc`、`internal/rpc/service` 参与运行时请求路径。

**Tech Stack:** Go、Gin、GORM、Redis、Swagger/OpenAPI、PostgreSQL、RustFS/S3 兼容对象存储。

---

## 文件结构与职责

### 新建目录

- `video-service/go.mod`
  - 新 HTTP 工程模块定义；优先保持与原工程依赖版本接近，减少迁移噪音。
- `video-service/cmd/httpapi/main.go`
  - 新 HTTP 主入口，负责配置、日志、依赖初始化、路由注册和优雅退出。
- `video-service/internal/http/router/router.go`
  - 注册新 REST 路由、Swagger 路由和全局中间件。
- `video-service/internal/http/handler/*.go`
  - 直接调用 application service 的 HTTP handler。
- `video-service/internal/http/dto/*.go`
  - 对外稳定的 HTTP 请求/响应结构体。
- `video-service/internal/http/errors/errors.go`
  - 统一 `success/data/error` 响应封装和错误码映射。
- `video-service/internal/http/app/app.go`
  - 将数据库、Redis、对象存储、repo、queue、service 组装成 handler 可用依赖。
- `video-service/docs/swagger/*`
  - Swagger 生成产物。

### 从原项目复制并复用的目录

- `nlp-video-project/internal/application/videoapp/*`
- `nlp-video-project/internal/infrastructure/*`
- `nlp-video-project/internal/config/*`
- `nlp-video-project/internal/lifecycle/*`
- `nlp-video-project/internal/domain/*`
- `nlp-video-project/internal/model/*`
- `nlp-video-project/middleware/*`

### 参考但不纳入新运行链路的旧文件

- `nlp-video-project/cmd/api/main.go`
- `nlp-video-project/cmd/rpc/main.go`
- `nlp-video-project/internal/api/handler/impl/*.go`
- `nlp-video-project/internal/api/client/client.go`
- `nlp-video-project/internal/rpc/service/*.go`

---

### Task 1: 建立新工程骨架

**Files:**
- Create: `video-service/go.mod`
- Create: `video-service/cmd/httpapi/main.go`
- Create: `video-service/internal/http/router/router.go`
- Create: `video-service/internal/http/errors/errors.go`
- Create: `video-service/internal/http/dto/common.go`
- Create: `video-service/internal/http/app/app.go`
- Modify: `docs/superpowers/specs/2026-04-28-http-migration-design.md` only if implementation reality forces a spec correction
- Test: `video-service/cmd/httpapi/main_test.go` or package-level boot tests if needed

- [ ] **Step 1: 复制最小可运行的工程骨架到新目录**

创建新目录并复制以下内容，保留业务基础能力但不复制 gRPC 入口作为主链路：

```text
video-service/
  cmd/
  configs/
  internal/
  middleware/
```

复制时优先保留：

```text
internal/application/
internal/config/
internal/domain/
internal/infrastructure/
internal/lifecycle/
internal/model/
middleware/
```

不要把下面这些旧协议层代码接入新主流程：

```text
internal/api/client/
internal/rpc/service/
cmd/rpc/
```

- [ ] **Step 2: 初始化新模块并校正 import path**

`go.mod` 初版建议沿用原模块名风格，只把根目录改成新工程可自洽的模块路径，例如：

```go
module nlp-video-project/video-service

go 1.23
```

然后统一修正新工程内 import，例如：

```go
import (
    "nlp-video-project/video-service/internal/application/videoapp"
    "nlp-video-project/video-service/internal/config"
)
```

- [ ] **Step 3: 写一个最小启动失败测试，约束新入口不会再依赖 gRPC client 初始化**

测试目标：新 `cmd/httpapi` 启动依赖初始化时，不应调用 `internal/api/client.InitVideoClient`，即使环境中没有 `RPC_ADDR` 也不应该以“gRPC 初始化失败”形式报错。

示例测试思路：

```go
func TestHTTPAPIBootstrapDoesNotRequireRPCAddr(t *testing.T) {
    t.Setenv("RPC_ADDR", "")
    err := validateBootstrapConfigWithoutRPCDependency()
    if err != nil && strings.Contains(err.Error(), "gRPC") {
        t.Fatalf("bootstrap should not depend on gRPC: %v", err)
    }
}
```

- [ ] **Step 4: 实现新的应用装配器 `internal/http/app/app.go`**

将旧 `cmd/rpc/main.go` 里的业务组装逻辑抽到单独 builder，例如：

```go
type App struct {
    VideoService *videoapp.Service
    DB           *gorm.DB
    Store        objectstorage.Storage
    Redis        *redis.Client
}

func Build(ctx context.Context, cfg *config.Config) (*App, func(context.Context) error, error) {
    // 初始化 DB/Redis/Object Storage/Repo/Queue/Embedder
}
```

要求：

1. 组装逻辑来源于旧 `cmd/rpc/main.go`。
2. 不启动 gRPC server。
3. 关闭逻辑集中返回。

- [ ] **Step 5: 实现最小 HTTP 主入口**

`cmd/httpapi/main.go` 最小形态应类似：

```go
func main() {
    config.EnsureProjectRoot()
    cfg := config.MustLoadDefault()
    middleware.InitFileLogger("httpapi")

    lc := lifecycle.New("httpapi", 30*time.Second)
    app, closeFn, err := httpapp.Build(lc.Context(), &cfg)
    if err != nil {
        zap.L().Fatal("http_app_build_failed", zap.Error(err))
    }
    lc.AddCloser(closeFn)

    r := router.Setup(app)
    srv := &http.Server{Addr: ":8081", Handler: r}
    lc.AddCloser(func(ctx context.Context) error { return srv.Shutdown(ctx) })
    _ = lc.Run(func(ctx context.Context) error { return srv.ListenAndServe() })
}
```

- [ ] **Step 6: 运行最小编译验证**

Run: `go test ./video-service/...`

Expected:

```text
所有包成功编译；如果有失败，应该只来自尚未迁移的 handler/router 引用，而不是 gRPC 初始化链路
```

- [ ] **Step 7: Commit**

```bash
git add video-service docs/superpowers/plans/2026-04-28-http-migration-plan.md
git commit -m "feat(http): bootstrap standalone http project"
```

### Task 2: 定义统一 HTTP 响应与错误模型

**Files:**
- Create: `video-service/internal/http/dto/common.go`
- Create: `video-service/internal/http/errors/errors.go`
- Test: `video-service/internal/http/errors/errors_test.go`

- [ ] **Step 1: 先写错误映射测试**

```go
func TestWriteError_InvalidArgument(t *testing.T) {
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)

    httperrors.Write(c, httperrors.InvalidArgument("question_text 不能为空"))

    if w.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", w.Code)
    }
    if !strings.Contains(w.Body.String(), `"code":"invalid_argument"`) {
        t.Fatalf("unexpected body: %s", w.Body.String())
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./video-service/internal/http/errors -run TestWriteError_InvalidArgument -v`

Expected:

```text
FAIL，提示 httperrors 包或方法未定义
```

- [ ] **Step 3: 实现统一 envelope 与错误写出函数**

`dto/common.go` 建议包含：

```go
type SuccessResponse[T any] struct {
    Success bool `json:"success"`
    Data    T    `json:"data"`
}

type ErrorBody struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

type ErrorResponse struct {
    Success bool      `json:"success"`
    Error   ErrorBody `json:"error"`
}
```

`errors/errors.go` 建议包含：

```go
type APIError struct {
    Status  int
    Code    string
    Message string
}

func InvalidArgument(msg string) *APIError { ... }
func NotFound(code, msg string) *APIError { ... }
func Internal(msg string) *APIError { ... }
func Write(c *gin.Context, err *APIError) { ... }
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./video-service/internal/http/errors -v`

Expected:

```text
PASS
```

- [ ] **Step 5: Commit**

```bash
git add video-service/internal/http/dto video-service/internal/http/errors
git commit -m "feat(http): add api response and error model"
```

### Task 3: 迁移视频列表、详情操作和状态接口

**Files:**
- Create: `video-service/internal/http/dto/video.go`
- Create: `video-service/internal/http/handler/video.go`
- Modify: `video-service/internal/http/router/router.go`
- Reference: `nlp-video-project/internal/api/handler/impl/video.go`
- Test: `video-service/internal/http/handler/video_test.go`

- [ ] **Step 1: 为列表接口写失败测试**

```go
func TestListVideos_UsesApplicationService(t *testing.T) {
    svc := &stubVideoService{
        listVideosResp: videoapp.ListVideosOutput{Total: 1},
    }
    h := handler.NewVideoHandler(svc)

    w := httptest.NewRecorder()
    c, r := gin.CreateTestContext(w)
    req := httptest.NewRequest(http.MethodGet, "/api/videos?type=ALL", nil)
    c.Request = req

    h.ListVideos(c)

    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", w.Code)
    }
}
```

- [ ] **Step 2: 实现 `video.go` DTO 与 handler 基础结构**

建议 DTO：

```go
type VideoItem struct {
    VideoID      uint64 `json:"video_id"`
    Title        string `json:"title"`
    Description  string `json:"description"`
    RawURL       string `json:"raw_url"`
    HLSURL       string `json:"hls_url"`
    IsPublished  bool   `json:"is_published"`
    IsRecommend  bool   `json:"is_recommend"`
    ViewCount    int64  `json:"view_count"`
    CoverURL     string `json:"cover_url"`
    CreatedAtUnix int64 `json:"created_at_unix"`
    UpdatedAtUnix int64 `json:"updated_at_unix"`
}
```

handler 结构：

```go
type VideoApp interface {
    ListVideos(ctx context.Context, in videoapp.ListVideosInput) (videoapp.ListVideosOutput, error)
    UpdateVideoMetadata(ctx context.Context, in videoapp.UpdateVideoMetadataInput) (videoapp.UpdateVideoMetadataOutput, error)
    DeleteVideo(ctx context.Context, in videoapp.DeleteVideoInput) error
    PlayVideo(ctx context.Context, in videoapp.PlayVideoInput) (videoapp.PlayVideoOutput, error)
    GetSimilarVideos(ctx context.Context, in videoapp.GetSimilarVideosInput) (videoapp.GetSimilarVideosOutput, error)
    GetViewCount(ctx context.Context, videoID uint64) (int64, error)
    ListRecommendPoolVideos(ctx context.Context) (videoapp.ListVideosOutput, error)
    SetVideoPublished(ctx context.Context, in videoapp.SetVideoPublishedInput) error
    SetVideoRecommend(ctx context.Context, in videoapp.SetVideoRecommendInput) error
    GetTranscodeStatus(ctx context.Context, taskID string) (videoapp.TranscodeStatusOutput, error)
}
```

- [ ] **Step 3: 实现以下接口，直接走 application，不再经由 gRPC client**

需要完成：

```text
GET    /api/videos
PATCH  /api/videos/:id
DELETE /api/videos/:id
GET    /api/videos/:id/play
GET    /api/videos/:id/similar
GET    /api/videos/:id/view-count
POST   /api/videos/:id/publish
POST   /api/videos/:id/recommend
GET    /api/transcode-tasks/:taskId
```

输入校验规则沿用旧语义：

1. `id` 必须为合法正整数。
2. `question_text` 一类的必填逻辑保留到对应接口。
3. `type` 默认 `ALL`。

- [ ] **Step 4: 注册路由并写路由级测试**

`router/router.go` 期望至少包含：

```go
func Setup(app *httpapp.App) *gin.Engine {
    r := gin.New()
    r.Use(gin.Recovery())
    r.Use(middleware.AccessLogMiddleware())

    h := handler.NewVideoHandler(app.VideoService)

    r.GET("/api/videos", h.ListVideos)
    r.PATCH("/api/videos/:id", h.UpdateVideoMetadata)
    r.DELETE("/api/videos/:id", h.DeleteVideo)
    r.GET("/api/videos/:id/play", h.PlayVideo)
    r.GET("/api/videos/:id/similar", h.GetSimilarVideos)
    r.GET("/api/videos/:id/view-count", h.GetViewCount)
    r.POST("/api/videos/:id/publish", h.SetVideoPublished)
    r.POST("/api/videos/:id/recommend", h.SetVideoRecommend)
    r.GET("/api/transcode-tasks/:taskId", h.GetTranscodeStatus)
    return r
}
```

- [ ] **Step 5: 运行相关测试**

Run: `go test ./video-service/internal/http/... -run "Test(ListVideos|UpdateVideoMetadata|DeleteVideo|PlayVideo|GetSimilarVideos|GetViewCount|SetVideoPublished|SetVideoRecommend|GetTranscodeStatus)" -v`

Expected:

```text
PASS
```

- [ ] **Step 6: Commit**

```bash
git add video-service/internal/http
git commit -m "feat(http): migrate core video endpoints"
```

### Task 4: 迁移上传与封面上传接口

**Files:**
- Create: `video-service/internal/http/handler/upload.go`
- Create: `video-service/internal/http/dto/upload.go`
- Modify: `video-service/internal/http/router/router.go`
- Reference: `nlp-video-project/internal/api/handler/impl/upload.go`
- Reference: `nlp-video-project/internal/api/handler/impl/cover.go`
- Test: `video-service/internal/http/handler/upload_test.go`

- [ ] **Step 1: 为 multipart 上传写失败测试**

```go
func TestUploadVideo_RequiresFile(t *testing.T) {
    h := handler.NewUploadHandler(stubUploadApp{})
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    req := httptest.NewRequest(http.MethodPost, "/api/videos", &bytes.Buffer{})
    req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
    c.Request = req

    h.UploadVideo(c)

    if w.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", w.Code)
    }
}
```

- [ ] **Step 2: 从旧 gRPC 流式上传逻辑中剥离纯业务所需输入**

这里不要再复用：

```go
client.GetVideoClient().UploadVideo(...)
```

而是要把旧上传 handler 里真正有价值的部分收敛成 HTTP 直连需要的输入：

```go
type UploadVideoInput struct {
    FileName    string
    ContentType string
    Title       string
    Description string
    Reader      io.Reader
}
```

如果 `videoapp` 现有接口不接受流式 reader，则在新项目内新增一个最小适配函数，把 multipart 文件落到业务层当前能接受的形式，但不要引回 gRPC。

- [ ] **Step 3: 实现上传与封面上传 handler**

完成：

```text
POST /api/videos
POST /api/videos/:id/cover
```

返回统一 envelope，例如：

```json
{
  "success": true,
  "data": {
    "video_id": 1,
    "task_id": "task-123",
    "raw_url": "/videos/raw/xxx.mp4",
    "hls_url": "/videos/hls/xxx/index.m3u8"
  }
}
```

- [ ] **Step 4: 运行上传相关测试**

Run: `go test ./video-service/internal/http/handler -run "Test(UploadVideo|UploadVideoCover)" -v`

Expected:

```text
PASS
```

- [ ] **Step 5: Commit**

```bash
git add video-service/internal/http/handler/upload.go video-service/internal/http/dto/upload.go video-service/internal/http/router/router.go
git commit -m "feat(http): migrate upload endpoints"
```

### Task 5: 迁移推荐、观看记录与题库接口

**Files:**
- Create: `video-service/internal/http/handler/recommend.go`
- Create: `video-service/internal/http/handler/question.go`
- Create: `video-service/internal/http/dto/recommend.go`
- Modify: `video-service/internal/http/router/router.go`
- Reference: `nlp-video-project/internal/api/handler/impl/recommend.go`
- Reference: `nlp-video-project/internal/api/handler/impl/watch.go`
- Reference: `nlp-video-project/internal/api/handler/impl/question.go`
- Test: `video-service/internal/http/handler/recommend_test.go`
- Test: `video-service/internal/http/handler/question_test.go`

- [ ] **Step 1: 为推荐接口先写参数校验测试**

```go
func TestRecommendByQuestion_EmptyQuestionText(t *testing.T) {
    h := handler.NewRecommendHandler(stubRecommendApp{})
    body := strings.NewReader(`{"question_id":1,"question_text":"   ","user_id":2}`)

    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = httptest.NewRequest(http.MethodPost, "/api/recommendations/by-question", body)
    c.Request.Header.Set("Content-Type", "application/json")

    h.RecommendByQuestion(c)

    if w.Code != http.StatusBadRequest {
        t.Fatalf("expected 400, got %d", w.Code)
    }
}
```

- [ ] **Step 2: 迁移并实现以下接口**

```text
POST /api/recommendations/by-question
GET  /api/recommendations
POST /api/watch-records
GET  /api/questions
GET  /api/questions/:id
```

要求：

1. `question_text` 去空格后不能为空。
2. `limit <= 0` 时保留旧默认值语义。
3. 返回字段保留 `video_id`、`video_segment_id`、`start_time_sec`、`end_time_sec` 等现有语义。
4. `play_url` 的补充逻辑如果旧实现是通过再次调 `PlayVideo` 获得，则在新实现中改为直接复用 application 或同一 handler 内服务调用，不再走 gRPC。

- [ ] **Step 3: 运行推荐与题库测试**

Run: `go test ./video-service/internal/http/handler -run "Test(RecommendByQuestion|ListRecommendations|ReportWatch|ListQuestions|GetQuestion)" -v`

Expected:

```text
PASS
```

- [ ] **Step 4: Commit**

```bash
git add video-service/internal/http/handler/recommend.go video-service/internal/http/handler/question.go video-service/internal/http/dto/recommend.go video-service/internal/http/router/router.go
git commit -m "feat(http): migrate recommendation and question endpoints"
```

### Task 6: 挂载 Swagger/OpenAPI 文档

**Files:**
- Modify: `video-service/cmd/httpapi/main.go`
- Modify: `video-service/internal/http/router/router.go`
- Create: `video-service/docs/swagger/docs.go`
- Create: `video-service/docs/swagger/swagger.json`
- Create: `video-service/docs/swagger/swagger.yaml`
- Test: `video-service/internal/http/router/swagger_test.go`

- [ ] **Step 1: 为 swagger 路由先写测试**

```go
func TestSwaggerRouteRegistered(t *testing.T) {
    r := router.Setup(stubApp())
    req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
    w := httptest.NewRecorder()

    r.ServeHTTP(w, req)

    if w.Code != http.StatusOK && w.Code != http.StatusMovedPermanently {
        t.Fatalf("expected swagger route available, got %d", w.Code)
    }
}
```

- [ ] **Step 2: 为 handler 和 DTO 增加 Swagger 注释**

至少覆盖：

```go
// @Summary 上传视频
// @Tags videos
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "视频文件"
// @Success 200 {object} dto.SuccessResponse[dto.UploadVideoResponse]
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/videos [post]
```

每个主接口都要补齐：

1. Summary
2. Tags
3. Param
4. Success
5. Failure
6. Router

- [ ] **Step 3: 集成 swagger 路由并生成产物**

路由期望：

```go
r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
```

生成命令期望类似：

```bash
swag init -g cmd/httpapi/main.go -o docs/swagger
```

- [ ] **Step 4: 运行 Swagger 验证**

Run: `go test ./video-service/internal/http/router -run TestSwaggerRouteRegistered -v`

Expected:

```text
PASS
```

- [ ] **Step 5: Commit**

```bash
git add video-service/cmd/httpapi/main.go video-service/internal/http/router/router.go video-service/docs/swagger
git commit -m "feat(http): add swagger documentation"
```

### Task 7: 完成端到端清理、兼容别名与验证

**Files:**
- Modify: `video-service/internal/http/router/router.go`
- Modify: `video-service/README.md` or equivalent local docs if the new project needs run instructions
- Test: `video-service/internal/http/router/router_test.go`

- [ ] **Step 1: 检查是否需要保留少量旧路径别名**

如确需平滑迁移，可保留少量 alias，例如：

```text
POST /api/video/recommend_by_question -> POST /api/recommendations/by-question
POST /api/video/report_watch -> POST /api/watch-records
GET  /api/video/list -> GET /api/videos
```

约束：

1. Swagger 只主推新 REST 路径。
2. 兼容别名必须复用同一 handler，不能复制业务逻辑。

- [ ] **Step 2: 搜索并移除新工程里的 gRPC 运行时依赖**

Run: `rg "grpc|internal/api/client|NewVideoServiceClient|RegisterVideoServiceServer" video-service`

Expected:

```text
只允许出现在历史参考文件、注释或未迁移说明里；不能出现在新 HTTP 主链路代码中
```

- [ ] **Step 3: 跑新工程完整测试与编译验证**

Run: `go test ./video-service/...`

Expected:

```text
PASS
```

如有前置生成依赖，再补跑：

```bash
go test ./video-service/... && go test ./nlp-video-project/... 
```

目标：

1. 新工程可编译可测试。
2. 旧工程不因复制/抽离而被破坏。

- [ ] **Step 4: 本地启动验证 HTTP 服务与 Swagger**

Run: `go run ./video-service/cmd/httpapi`

验证点：

1. 服务能启动。
2. `GET /swagger/index.html` 可访问。
3. `GET /api/videos` 至少能进入 handler 并返回业务结果或明确错误。

- [ ] **Step 5: Commit**

```bash
git add video-service
git commit -m "refactor(http): finalize standalone http migration"
```

## 自检结论

### Spec 覆盖检查

已覆盖的 spec 要点：

1. 新目录 `video-service/`：Task 1
2. 直接 HTTP 通信替换 gRPC 外部链路：Task 1、3、4、5、7
3. 最大化复用原有逻辑：Task 1、3、4、5
4. REST 风格接口：Task 3、4、5
5. 统一 `success/data/error` 响应：Task 2
6. OpenAPI/Swagger：Task 6
7. 不做认证：全计划未引入 auth 任务
8. 渐进迁移和最终清理：Task 7

没有发现 spec 漏项。

### 占位符检查

本计划未使用 `TODO`、`TBD`、`implement later` 之类占位语。

### 类型与命名一致性检查

计划中统一使用以下命名：

1. `success/data/error` 为统一 envelope
2. `video_id`、`task_id`、`video_segment_id`、`start_time_sec`、`end_time_sec` 保留现有业务语义
3. 新入口统一命名为 `cmd/httpapi`
4. 新协议层统一落在 `internal/http/*`
