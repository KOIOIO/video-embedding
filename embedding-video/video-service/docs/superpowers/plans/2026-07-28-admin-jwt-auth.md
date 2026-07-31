# 管理员 JWT 鉴权实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为管理类 API 和 `hls-web` 控制台增加真实管理员 JWT 登录，并让所有视频上传者身份来自 JWT，同时保持播放、推荐、观看和互动接口公开且协议不变。

**Architecture:** 在应用层新增独立的 `adminauth` 服务，GORM 仓储读取 `sys_user`，JWT 服务负责签发和解析令牌。Gin 路由明确拆为公开路由与管理员路由，管理员中间件在每次请求中重新校验数据库账号状态并把管理员 ID 写入上下文。前端用单一认证会话模块替换本地写死登录，并只为管理请求和上传 XHR 注入 Bearer 令牌。

**Tech Stack:** Go 1.26、Gin、GORM、PostgreSQL、bcrypt、`github.com/golang-jwt/jwt/v5`、Vue 3、Vite、Vitest。

---

### Task 1: 认证配置与管理员仓储

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/config/types.go`
- Modify: `internal/config/loader.go`
- Modify: `internal/config/loader_test.go`
- Modify: `configs/video.yml`
- Modify: `configs/video_prod.yml`
- Create: `internal/application/adminauth/types.go`
- Create: `internal/application/adminauth/contracts.go`
- Create: `internal/infrastructure/persistence/gorm_admin_repository.go`
- Create: `internal/infrastructure/persistence/gorm_admin_repository_test.go`

- [ ] **Step 1: 写配置与仓储失败测试**

测试 `JWT_SECRET` 优先于 YAML，TTL 缺省为 8 小时；SQLite 测试表覆盖有效管理员、错误密码、`user_type != 3`、禁用和删除账号。仓储只返回 `id, username, password, real_name, user_type, status, deleted`。

```go
func TestGormAdminRepositoryFindActiveAdminByUsername(t *testing.T) {
    admin, found, err := repo.FindActiveAdminByUsername(context.Background(), "admin")
    if err != nil || !found || admin.ID != 7 || admin.UserType != 3 {
        t.Fatalf("admin=%+v found=%v err=%v", admin, found, err)
    }
}

func TestJWTSecretPrefersEnvironment(t *testing.T) {
    t.Setenv("JWT_SECRET", "environment-secret")
    if got := JWTSecret(Config{Auth: AuthConfig{JWTSecret: "yaml-secret"}}); got != "environment-secret" {
        t.Fatalf("JWTSecret()=%q", got)
    }
}
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/config ./internal/infrastructure/persistence -run 'TestJWT|TestGormAdmin'`

Expected: FAIL，缺少 `AuthConfig`、`JWTSecret` 和管理员仓储。

- [ ] **Step 3: 实现最小配置和仓储**

`Config` 增加：

```go
type AuthConfig struct {
    JWTSecret     string `yaml:"JWTSecret"`
    JWTExpireHour int    `yaml:"JWTExpireHour"`
}
```

配置助手从 `JWT_SECRET` 读取签名密钥，TTL 使用 YAML 或缺省 8 小时。仓储 SQL 必须包含：

```sql
WHERE username = ? AND user_type = 3 AND status = 1 AND deleted = 0
```

执行 `go get github.com/golang-jwt/jwt/v5`，并把 `golang.org/x/crypto` 提升为直接依赖。

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/config ./internal/infrastructure/persistence -run 'TestJWT|TestGormAdmin'`

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add go.mod go.sum internal/config configs internal/application/adminauth internal/infrastructure/persistence/gorm_admin_repository.go internal/infrastructure/persistence/gorm_admin_repository_test.go
git commit -m "feat: add administrator authentication repository"
```

### Task 2: 登录服务、JWT 与 HTTP 中间件

**Files:**
- Create: `internal/application/adminauth/service.go`
- Create: `internal/application/adminauth/service_test.go`
- Create: `internal/http/handler/admin_auth.go`
- Create: `internal/http/handler/admin_auth_test.go`
- Create: `middleware/admin_auth.go`
- Create: `middleware/admin_auth_test.go`
- Modify: `internal/http/dto/common.go`

- [ ] **Step 1: 写登录和令牌失败测试**

覆盖 bcrypt 成功、错误密码、非管理员统一拒绝、JWT subject 为管理员 ID、过期令牌拒绝，以及令牌签发后账号降权时中间件返回 401。

```go
func TestServiceLoginReturnsTokenForActiveAdmin(t *testing.T) {
    result, err := service.Login(context.Background(), "admin", "correct-password")
    if err != nil || result.Token == "" || result.Admin.ID != 7 {
        t.Fatalf("result=%+v err=%v", result, err)
    }
}

func TestRequireAdminRejectsAdminDisabledAfterTokenIssued(t *testing.T) {
    req.Header.Set("Authorization", "Bearer "+token)
    repo.active = false
    router.ServeHTTP(recorder, req)
    if recorder.Code != http.StatusUnauthorized {
        t.Fatalf("status=%d", recorder.Code)
    }
}
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/application/adminauth ./internal/http/handler ./middleware -run 'Test(ServiceLogin|AdminAuth|RequireAdmin)'`

Expected: FAIL，认证服务、Handler 和中间件尚不存在。

- [ ] **Step 3: 实现认证服务和 Handler**

公开登录契约：

```json
POST /api/auth/login
{"username":"admin","password":"secret"}

{"success":true,"data":{"access_token":"...","expires_at":"...","admin":{"id":1,"username":"admin","real_name":"系统管理员"}}}
```

实现 `POST /api/auth/login` 与受保护的 `GET /api/auth/me`。所有账号或密码错误统一返回 `401 invalid username or password`。JWT 只接受 HMAC 指定算法，claims 至少包含 `sub`, `iat`, `exp`。

- [ ] **Step 4: 实现管理员中间件上下文契约**

```go
const AdminContextKey = "authenticated_admin"

func AdminID(c *gin.Context) (uint64, bool) {
    admin, ok := c.Get(AdminContextKey)
    if !ok { return 0, false }
    return admin.(adminauth.Admin).ID, true
}
```

缺少 Bearer、格式错误、签名错误、过期、账号失效均返回 401，并终止 Handler。

- [ ] **Step 5: 运行测试并确认通过**

Run: `go test ./internal/application/adminauth ./internal/http/handler ./middleware -run 'Test(ServiceLogin|AdminAuth|RequireAdmin)'`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/application/adminauth internal/http/handler/admin_auth* middleware/admin_auth*
git commit -m "feat: add jwt administrator login"
```

### Task 3: 应用装配与公开/管理路由分组

**Files:**
- Modify: `internal/http/app/app.go`
- Modify: `internal/http/app/app_test.go`
- Modify: `internal/http/router/router.go`
- Modify: `internal/http/router/swagger_test.go`
- Create: `internal/http/router/auth_routes_test.go`
- Modify: `cmd/httpapi/main_test.go` if startup config tests construct the app

- [ ] **Step 1: 写路由边界失败测试**

表驱动覆盖每一组路由：健康、播放、推荐、观看、互动接口无令牌仍进入 Handler；上传、修改、删除、发布、人工推荐、状态监控、Swagger 与 `/api/admin/**` 无令牌返回 401；有效管理员令牌不被中间件拦截。

```go
tests := []struct { method, path string; protected bool }{
    {http.MethodGet, "/api/videos/1/play", false},
    {http.MethodPost, "/api/watch-records", false},
    {http.MethodPost, "/api/videos", true},
    {http.MethodDelete, "/api/videos/1", true},
    {http.MethodGet, "/api/system/metrics", true},
}
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/http/router -run 'Test(AdminRoutes|PublicRoutes|AuthRoutes)'`

Expected: FAIL，当前所有路由均公开。

- [ ] **Step 3: 装配认证服务并重组路由**

`app.App` 持有 `AdminAuth *adminauth.Service`；`app.New` 在 JWT 密钥为空时返回明确配置错误。路由结构采用：

```go
public := r.Group("")
admin := r.Group("")
admin.Use(middleware.RequireAdmin(httpApp.AdminAuth))

public.POST("/api/auth/login", authHandler.Login)
admin.GET("/api/auth/me", authHandler.Me)
```

按已确认清单逐条迁移，所有历史兼容路径与 REST 路径必须位于相同路由组。OPTIONS 预检由 CORS 在鉴权前处理。

- [ ] **Step 4: 运行路由和启动测试**

Run: `go test ./internal/http/router ./internal/http/app ./cmd/httpapi`

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/http/app internal/http/router cmd/httpapi
git commit -m "feat: protect administrator routes"
```

### Task 4: 普通视频上传使用 JWT 管理员 ID

**Files:**
- Modify: `internal/http/handler/uploads/handler.go`
- Modify: `internal/http/handler/upload.go`
- Modify: `internal/http/handler/upload_test.go`
- Modify: `internal/http/dto/upload.go`
- Modify: `internal/application/videoapp/upload.go`
- Modify: `internal/application/videoapp/chunked_upload.go`
- Modify: `internal/application/videoapp/upload_http_test.go`
- Modify: `internal/application/videoapp/chunked_upload_test.go`
- Modify: `internal/infrastructure/persistence/gorm_video_repository.go`
- Modify: `internal/infrastructure/persistence/gorm_video_repository_test.go`

- [ ] **Step 1: 写上传身份失败测试**

测试 multipart 即使提交 `user_id=999` 也把上下文管理员 ID 7 传给应用服务；分片初始化 DTO 不再解析 `user_id`；服务端 session 保存管理员 7，并在 complete 时仍使用 7。上传权限仓储只允许有效 `user_type=3` 管理员。

```go
req := multipartRequest(map[string]string{"user_id": "999"})
req = withAdmin(req, adminauth.Admin{ID: 7})
router.ServeHTTP(recorder, req)
if app.input.UserID != 7 { t.Fatalf("UserID=%d", app.input.UserID) }
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/http/handler ./internal/application/videoapp ./internal/infrastructure/persistence -run 'Test.*Upload.*(Admin|UserID|Permission)'`

Expected: FAIL，Handler 仍读取客户端字段且默认 ID 为 1。

- [ ] **Step 3: 从上下文注入管理员 ID**

所有普通上传 Handler 使用 `middleware.AdminID(c)`，删除 `parseOptionalPositiveUintForm(c, "user_id")` 和 `InitiateChunkedUploadRequest.UserID`。应用服务要求 `UserID > 0`，不再在管理上传入口回退 `DefaultUploadUserID`。分片 session 继续持久化初始化管理员 ID。

- [ ] **Step 4: 收紧数据库上传许可并运行测试**

`CanUploadVideo` 条件改为：

```sql
id = ? AND user_type = 3 AND status = 1 AND deleted = 0
```

Run: `go test ./internal/http/handler ./internal/application/videoapp ./internal/infrastructure/persistence`

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/http/handler internal/http/dto/upload.go internal/application/videoapp internal/infrastructure/persistence/gorm_video_repository*
git commit -m "feat: derive video uploader from jwt"
```

### Task 5: 知识点视频导入使用 JWT 管理员 ID

**Files:**
- Modify: `internal/http/handler/knowledgevideos/handler.go`
- Modify: `internal/http/handler/knowledgevideos/handler_test.go`
- Modify: `internal/application/knowledgevideo/import.go`
- Modify: `internal/application/knowledgevideo/import_test.go`

- [ ] **Step 1: 写知识点导入失败测试**

构造带管理员上下文 ID 7、同时携带恶意 `upload_user_id=999` 的 multipart 请求，断言 `ImportInput.UploadUserID == 7`。另测缺少管理员上下文不会静默写入 0。

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/http/handler/knowledgevideos ./internal/application/knowledgevideo -run 'Test.*Import.*(Admin|UploadUser)'`

Expected: FAIL，Handler 仍解析表单字段。

- [ ] **Step 3: 实现上下文身份注入**

删除 `upload_user_id` 表单解析和 Swagger 参数声明，使用管理员上下文 ID 构造 `knowledgevideo.ImportInput`。应用服务保留正整数防御性校验，批次继续写入 `upload_user_id` 数据库列。

- [ ] **Step 4: 运行测试并确认通过**

Run: `go test ./internal/http/handler/knowledgevideos ./internal/application/knowledgevideo`

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/http/handler/knowledgevideos internal/application/knowledgevideo
git commit -m "feat: derive knowledge video uploader from jwt"
```

### Task 6: 前端真实登录与管理请求鉴权

**Files:**
- Replace: `hls-web/src/config/consoleSession.js`
- Modify: `hls-web/src/config/consoleSession.test.js`
- Create: `hls-web/src/auth/api.js`
- Create: `hls-web/src/auth/api.test.js`
- Create: `hls-web/src/auth/session.js`
- Create: `hls-web/src/auth/session.test.js`
- Modify: `hls-web/src/App.vue`
- Modify: `hls-web/src/recommendation/api/http.js`
- Modify: `hls-web/src/recommendation/api/http.test.js`
- Modify: `hls-web/src/workspaces/VideoWorkspace.vue`

- [ ] **Step 1: 写认证会话与请求失败测试**

覆盖登录调用、JWT/管理员持久化、恢复 `/api/auth/me`、管理 fetch 添加 Bearer、401/403 清理会话并触发未授权事件。不得把密码写入 storage。

```js
expect(fetchImpl).toHaveBeenCalledWith('/api/auth/me', expect.objectContaining({
  headers: expect.objectContaining({ Authorization: 'Bearer token-1' }),
}))
expect(storage.getItem('video-admin-token')).toBe('token-1')
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `npm test -- src/auth src/config/consoleSession.test.js src/recommendation/api/http.test.js`

Expected: FAIL，当前仅有写死账号的本地 UI 解锁。

- [ ] **Step 3: 实现认证模块并接入 App**

登录表单异步调用 `/api/auth/login`，显示提交中和服务端错误；启动时有 token 则调用 `/api/auth/me`；工具栏显示后端返回的 `real_name || username`；退出清理 token。统一管理请求助手合并现有 headers 后添加 Authorization，不修改公开请求 payload。

- [ ] **Step 4: 运行测试并确认通过**

Run: `npm test -- src/auth src/config/consoleSession.test.js src/recommendation/api/http.test.js`

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add hls-web/src/App.vue hls-web/src/auth hls-web/src/config hls-web/src/recommendation/api/http*
git commit -m "feat: connect console administrator login"
```

### Task 7: 前端上传移除上传者 ID 并支持鉴权 XHR

**Files:**
- Modify: `hls-web/src/chunkedUpload.js`
- Modify: `hls-web/src/chunkedUpload.test.js`
- Modify: `hls-web/src/knowledgeVideo/api.js`
- Modify: `hls-web/src/knowledgeVideo/api.test.js`
- Modify: `hls-web/src/workspaces/VideoWorkspace.vue`
- Modify: `hls-web/src/workspaces/KnowledgeVideoWorkspace.vue`
- Modify: related workspace tests if present

- [ ] **Step 1: 写上传请求失败测试**

普通分片初始化请求体不得含 `user_id`；分片 PUT、complete、知识点导入 XHR 必须设置 Authorization；知识点 FormData 不得含 `upload_user_id`。公开的 `recordKnowledgePlayback` 仍必须发送行为用户 `user_id`。

```js
expect(JSON.parse(init.body)).not.toHaveProperty('user_id')
expect(xhr.setRequestHeader).toHaveBeenCalledWith('Authorization', 'Bearer token-1')
expect(form.has('upload_user_id')).toBe(false)
expect(JSON.parse(playbackInit.body)).toEqual({ user_id: 6 })
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `npm test -- src/chunkedUpload.test.js src/knowledgeVideo/api.test.js`

Expected: FAIL，上传请求仍提交上传者 ID，XHR 未携带 JWT。

- [ ] **Step 3: 修改上传 API 与界面**

删除 `uploadVideoInChunks` 的 `userId` 参数和初始化 JSON 字段；让 XHR 上传函数接收 token 并调用 `setRequestHeader`。删除两个工作区的“上传用户 ID”输入。知识点工作区为公开的播放记录保留独立的行为测试用户 ID 常量，不再复用上传者状态。

- [ ] **Step 4: 运行前端测试和构建**

Run: `npm test`

Expected: PASS。

Run: `npm run build`

Expected: exit 0，Vite 生成 `dist`。

- [ ] **Step 5: 提交**

```bash
git add hls-web/src
git commit -m "feat: authenticate console upload requests"
```

### Task 8: Swagger、文档与全量验证

**Files:**
- Modify: Swagger 注释所在的 `internal/http/handler/*.go`
- Regenerate: `docs/swagger/docs.go`
- Regenerate: `docs/swagger/swagger.json`
- Regenerate: `docs/swagger/swagger.yaml`
- Modify: `README.md`
- Modify: `../docker-compose.yml`

- [ ] **Step 1: 写 Swagger 安全声明失败测试**

扩展 `internal/http/router/swagger_test.go`：登录接口存在；管理接口声明 BearerAuth；公开播放、观看和互动接口不声明安全要求；上传文档不再出现上传者 ID 参数。

- [ ] **Step 2: 运行测试并确认失败**

Run: `go test ./internal/http/router -run TestSwagger`

Expected: FAIL，生成文档尚无 JWT 安全定义。

- [ ] **Step 3: 更新注释、部署配置和 README**

增加 Swagger `BearerAuth` security definition，管理 Handler 使用 `@Security BearerAuth`。Docker Compose 给 API 注入 `JWT_SECRET`，README 记录必须设置不少于 32 字节的随机密钥、默认 8 小时有效期和公开/管理接口边界。不要把真实密钥写入仓库。

- [ ] **Step 4: 重新生成 Swagger**

Run: `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/httpapi/main.go -o docs/swagger`

Expected: exit 0，三个生成文件更新。

- [ ] **Step 5: 运行完整验证**

Run from `video-service/`: `go test ./...`

Expected: PASS，0 failures。

Run from `hls-web/`: `npm test`

Expected: PASS，0 failures。

Run from `hls-web/`: `npm run build`

Expected: exit 0。

- [ ] **Step 6: 检查上传者字段与公开用户字段**

Run:

```bash
rg -n "upload_user_id|上传用户 ID" hls-web/src internal/http/handler
rg -n "user_id" hls-web/src/watchProgress.js hls-web/src/segmentReaction.js hls-web/src/knowledgeVideo/api.js
```

Expected: 第一条没有上传表单/UI 命中；第二条仍显示公开观看和互动接口的行为用户字段。

- [ ] **Step 7: 提交**

```bash
git add video-service/docs/swagger video-service/README.md video-service/internal/http docker-compose.yml
git commit -m "docs: document administrator jwt security"
```
