# RPC 层幂等性（API 生成 Key）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 对 RPC 的写接口提供统一的幂等性保障：API 生成 `idempotency-key` 并通过 gRPC metadata 透传；RPC 使用 Redis 做“processing/done + response cache”；重复提交返回同一结果，防止重复写库/重复触发任务。

**Architecture:** API → gRPC 时注入 metadata（`idempotency-key` + `idempotency-fingerprint`）；RPC 侧在 UnaryInterceptor 中完成：占位锁（SET NX EX）、fingerprint 校验、执行 handler、缓存响应（proto bytes）与错误（code/message），后续同 key 直接返回缓存或提示处理中。UploadVideo（stream）后续单独扩展。

**Tech Stack:** Go, gRPC interceptors, Redis (go-redis/v8), protobuf (proto.Marshal / proto.Unmarshal), sha256 fingerprint.

---

## Files & Responsibilities

**Modify**
- `nlp-video-project/middleware/rpc_log.go`：新增 `UnaryIdempotencyInterceptor(rdb)`（放在现有 middleware 包，避免新增文件）。
- `nlp-video-project/cmd/rpc/main.go`：将 gRPC server interceptor 从单个改为 chain，挂载幂等拦截器。
- `nlp-video-project/internal/api/handler/handler.go`：对写接口 gRPC 调用注入 metadata（API 生成 key）。

---

## Key Decisions

- **幂等 key 来源：** API 生成（如果 HTTP 请求头 `Idempotency-Key` 存在则复用，否则生成 UUID）。
- **存储介质：** Redis（RPC 侧已有 rdb，可复用）。
- **一致性：** 同 key 但不同 payload → 直接返回 `InvalidArgument`（防滥用）。
- **返回策略：**
  - 已完成（done）→ 返回缓存 response（成功/失败都一致）
  - 处理中（processing）→ 返回 `Aborted`（客户端可重试/短轮询）
- **TTL：** 默认 10 分钟（processing/done 都用同 TTL；可后续做配置化）

---

### Task 1: API 侧生成并透传 idempotency metadata（先覆盖核心写接口）

**Files:**
- Modify: `nlp-video-project/internal/api/handler/handler.go`

- [ ] **Step 1: 增加 helper：为 gRPC ctx 注入幂等 metadata**

在 `handler.go` 增加函数（示例代码，按项目 import 风格落地）：

```go
func withIdempotency(c *gin.Context, method string, body []byte) context.Context {
    key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
    if key == "" {
        key = uuid.NewString()
    }
    fp := sha256.Sum256(append([]byte(method+"|"), body...))
    return metadata.AppendToOutgoingContext(c,
        "idempotency-key", key,
        "idempotency-fingerprint", hex.EncodeToString(fp[:]),
    )
}
```

- [ ] **Step 2: 在以下 handler 的 gRPC 调用处使用 withIdempotency**

覆盖范围（最小闭环）：
- `SetVideoRecommend`
- `SetVideoPublished`
- `UploadVideoCover`
- `RecommendByQuestion`（如果存在写推荐记录，属于有副作用）

做法：在构造 pb request 后，对 pb 做序列化（protojson 或 json）作为 body，传入 helper。

- [ ] **Step 3: 手工验证 headers 透传**

用 curl 指定同一个 `Idempotency-Key` 重复请求两次，确保：
- 服务端第二次不会重复写入
- 返回一致

---

### Task 2: RPC Unary 幂等拦截器（Redis + response cache）

**Files:**
- Modify: `nlp-video-project/middleware/rpc_log.go`

- [ ] **Step 1: 新增 interceptor 定义**

新增函数签名：

```go
func UnaryIdempotencyInterceptor(rdb *redis.Client) grpc.UnaryServerInterceptor
```

- [ ] **Step 2: 读取 metadata + fingerprint 校验**

读取：
- `idempotency-key`
- `idempotency-fingerprint`（可选）

fingerprint 计算建议：
- 若 req 是 `proto.Message`，使用 `protojson.Marshal(req)` 做稳定序列化
- `sha256(fullMethod + "|" + jsonBytes)`

校验规则：
- 若缓存里 fingerprint 不一致 → `codes.InvalidArgument`

- [ ] **Step 3: Redis 记录结构与操作**

key 建议：
- `rpc:idmp:<fullMethod>:<idempotency-key>`

value（JSON）字段：
- `status`: "processing" | "done"
- `fingerprint`: string
- `grpc_code`: int
- `grpc_message`: string
- `resp_b64`: string (proto bytes base64，成功时有)

流程：
1) `SET key {"status":"processing","fingerprint":"..."} NX EX ttl`
2) 若失败：GET 解析
   - done：如果 `grpc_code==0` → 反序列化 resp 返回；否则返回 `status.Error(code,msg)`
   - processing：返回 `codes.Aborted`
3) handler 执行后：SET done（成功写 resp_b64；失败写 code/msg），EX ttl

- [ ] **Step 4: 仅对写接口启用（避免影响 GET 类查询）**

通过 `info.FullMethod` 白名单控制，例如：
- `/video.VideoService/SetVideoRecommend`
- `/video.VideoService/SetVideoPublished`
- `/video.VideoService/SetVideoCover`
- `/video.VideoService/RecommendByQuestion`
- `/video.VideoService/DeleteVideo`

（ListVideos / PlayVideo / GetViewCount 等读接口不启用）

---

### Task 3: 将幂等拦截器挂到 rpc server

**Files:**
- Modify: `nlp-video-project/cmd/rpc/main.go`

- [ ] **Step 1: 改用 ChainUnaryInterceptor**

将：

```go
grpc.UnaryInterceptor(middleware.UnaryAccessLogInterceptor()),
```

改为：

```go
grpc.ChainUnaryInterceptor(
    middleware.UnaryIdempotencyInterceptor(rdb),
    middleware.UnaryAccessLogInterceptor(),
),
```

顺序说明：先幂等再 access log，避免重复请求每次都真正进入业务逻辑。

- [ ] **Step 2: go build 验证**

```bash
go build ./...
```

---

### Task 4: Stream UploadVideo 的幂等（后续扩展）

**Files:**
- (pending)

- [ ] **Step 1: 约定 upload 首帧 meta 必带 idempotency-key**
- [ ] **Step 2: RPC 收到 meta 立即做 processing/done 判定**
- [ ] **Step 3: done 直接返回上次 UploadVideoResponse，processing 拒绝**

---

## Self-Review

- Spec coverage：RPC 层幂等、API 生成并透传、重复请求返回同结果、fingerprint 防滥用、processing 拦截并发。
- Placeholder scan：无 TODO/TBD。
- Type consistency：metadata key 固定为 `idempotency-key` 与 `idempotency-fingerprint`。
