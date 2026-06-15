# `internal/application/videoapp` 测试文档

## 1. 范围

本文档用于梳理 `video-service/internal/application/videoapp/` 目录下各文件的测试现状，并为当前尚未覆盖的文件提供测试设计建议。

本次目录扫描结果：

- 已有测试文件：`ai_resilience.go`、`recommend.go`、`runtime_counters.go`、`worker.go`
- 无需重复编写：`ai_resilience_test.go`、`recommend_test.go`、`runtime_counters_test.go`、`worker_retry_test.go`
- 本文档重点覆盖：其余暂未看到对应 `_test.go` 的文件

## 2. 已有测试覆盖

以下文件已经存在测试，可继续增补，但不属于本次“从零补文档”的重点：

### 2.1 `ai_resilience.go`

已有测试点：

- AI 重试错误与终态错误区分
- AI provider unavailable 判断
- 重试退避时长递增

### 2.2 `recommend.go`

已有测试点：

- 推荐结果保留片段起止时间
- 按自由文本推荐时持久化历史记录
- 按题目 ID 推荐时持久化历史记录
- `ResolvePlaybackURL()` 对已完成视频推导 HLS 地址

### 2.3 `runtime_counters.go`

已有测试点：

- 计数器增减与快照

### 2.4 `worker.go`

已有测试点：

- 可重试失败时重新入队
- 终态失败进入 dead letter
- 成功任务 ACK
- 视频不存在时跳过
- 默认重试策略与临时存储错误识别

## 3. 待补测试文件与建议场景

## 3.1 `errors.go`

职责：

- 定义 `ValidationError`
- 提供 `InvalidArgumentError()` 构造器

建议测试点：

1. `ValidationError.Error()` 应返回原始 `Message`
2. `InvalidArgumentError()` 返回值应为 `ValidationError`
3. `errors.As()` 应能把 `InvalidArgumentError()` 结果识别为 `ValidationError`

优先级：低

原因：逻辑简单，但这是参数校验错误在 handler 层正确映射为 4xx 的基础。

## 3.2 `play.go`

职责：

- 播放前读取视频
- 增加播放次数
- 根据缓存状态或数据库状态推导播放地址
- 读取转码状态与播放计数

建议测试点：

1. `PlayVideo()` 在 `Repo.GetByID()` 首次返回错误时，直接透传错误
2. `PlayVideo()` 在视频不存在时，应返回 `ok=false`
3. `PlayVideo()` 在 `IncrementViewCount()` 失败时，应返回错误且不继续读取状态缓存
4. `PlayVideo()` 在缓存里命中 `StatusDone + HLSURL` 时，应优先返回缓存中的 `HLSURL`
5. `PlayVideo()` 在缓存命中完成态且数据库状态不是 `done` 时，应调用 `Repo.UpdateStatusByID()` 修正状态
6. `PlayVideo()` 在缓存缺失但数据库状态为 `done` 时，应调用 `deriveHLSURLFromRaw()` 返回 HLS 地址
7. `PlayVideo()` 在数据库状态非 `done` 时，应返回原始 `VideoURL`
8. `ResolvePlaybackURL()` 在缓存命中完成态时，应优先返回缓存 HLS 地址
9. `ResolvePlaybackURL()` 在缓存缺失且视频状态为 `done` 时，应回退推导 HLS 地址
10. `ResolvePlaybackURL()` 在非完成态视频上，应返回原始 `VideoURL`
11. `GetViewCount()` 应把 repo 返回的 `int` 转成 `int64`
12. `GetTranscodeStatus()` 应直接透传 `StatusStore.Get()` 结果
13. `deriveHLSURLFromRaw()` 在空路径、非法路径、正常 raw 路径上的行为

优先级：高

原因：播放地址解析是用户直接可见行为，缓存优先级、状态修正和 raw/hls 切换都容易产生回归。

## 3.3 `question.go`

职责：

- 题目列表分页参数清洗
- 单题读取参数校验

建议测试点：

1. `ListQuestions()` 当 `page < 1` 时，应回退为 `1`
2. `ListQuestions()` 当 `pageSize < 1` 时，应回退为 `20`
3. `ListQuestions()` 当 `pageSize > 100` 时，应回退为 `20`
4. `ListQuestions()` 在合法分页参数时，应原样透传给 repo
5. `GetQuestion()` 在 `id == 0` 时，应返回 `InvalidArgumentError("question_id is required")`
6. `GetQuestion()` 在合法 ID 时，应直接透传 repo 返回值

优先级：中

原因：逻辑不复杂，但分页边界和参数校验非常适合做稳定的表驱动测试。

## 3.4 `service.go`

职责：

- 构造 `Service`
- 注入默认值

建议测试点：

1. `NewService()` 应正确保存传入依赖
2. `NewService()` 默认 `Now` 不为 nil
3. `NewService()` 默认 `StatusTTL == 24*time.Hour`
4. `NewService()` 默认 `DeleteLocal == true`

优先级：低

原因：属于构造器测试，主要防止以后调整默认值时出现无意回归。

## 3.5 `system_metrics.go`

职责：

- 采集系统 CPU / 内存 / 进程内存 / goroutine 数量
- 拼装 `dto.SystemMetricsData`

建议测试点：

1. `mem.VirtualMemoryWithContext()` 返回错误时，应直接返回错误
2. `cpu.PercentWithContext()` 返回错误时，应直接返回错误
3. `cpu.PercentWithContext()` 返回空切片时，`CPUPercent` 应为 `0`
4. 正常情况下返回值应包含：
   - `MemoryUsedBytes`
   - `MemoryTotalBytes`
   - `MemoryUsedPercent`
   - `ProcessMemoryBytes`
   - `Goroutines`
   - `ActiveCounts`
   - 合法 RFC3339 时间戳

备注：

- 当前实现直接依赖 `gopsutil` 和 `runtime` 包，没有显式注入 seam
- 如果要单元测试错误分支，建议后续把 `mem` / `cpu` 调用包一层可替换函数变量或 interface

优先级：中

原因：这个文件适合先补“集成风格”测试或在重构后补纯单测。

## 3.6 `transcode_runtime.go`

职责：

- 定义转码运行时消息与 lease 结构
- 默认重试策略

现状：

- `DefaultRetryPolicy()` 与 `isTemporaryStorageError()` 已在 `worker_retry_test.go` 中覆盖

建议：

- 这里无需单独新增测试文件
- 但可以在文档里注明：覆盖落在 `worker_retry_test.go`，不要误判为未测试

优先级：无新增需求

## 3.7 `types.go`

职责：

- 定义应用层接口、输入输出结构、上传输入辅助方法

建议测试点：

1. `UploadVideoInput.ReadAll()` 在 `Reader == nil` 时返回 `nil, nil`
2. `UploadVideoInput.ReadAll()` 在 `Reader` 有内容时能完整读出内容
3. `UploadCoverInput.ReadAll()` 在 `Reader == nil` 时返回 `nil, nil`
4. `UploadCoverInput.ReadAll()` 在 `Reader` 有内容时能完整读出内容

其余 interface / struct 定义通常不需要专门测试。

优先级：低

原因：只有两个 helper 方法有行为，其余是类型定义。

## 3.8 `upload.go`

职责：

- 生成上传规划
- 打开本地写入器
- 完成视频上传后的对象存储、建库、转码入队、向量化入队、清理本地文件

建议测试点：

### `BuildUploadPlan()`

1. 空文件名时返回错误
2. 固定 `Now()` 后，应生成稳定的：
   - `StoredFileName`
   - `DatePath`
   - `RawAbsPath`
   - `RawObjectKey`
   - `RawURL`
   - `HLSAbsDir`
   - `HLSObjectPrefix`
   - `HLSURL`
3. 无扩展名文件名时，`StoredFileName` 仍能生成
4. `RawUploaded` 初始值应为 `false`

### `OpenUploadWriter()`

5. 应先对 `plan.RawAbsPath` 的父目录调用 `MkdirAll()`
6. `MkdirAll()` 失败时直接返回错误
7. `Create()` 失败时直接返回错误

### `FinalizeUpload()`

8. `meta.Title` 为空时，应回退到 `plan.OriginalFileName`
9. `plan.RawUploaded == false` 时，应调用 `Store.PutFile()` 上传原视频
10. `plan.RawUploaded == true` 时，不应重复调用 `Store.PutFile()`
11. `Repo.Create()` 失败时，应终止并返回错误
12. `StatusStore.Set()` 失败时，应终止并返回错误
13. `Queue.Enqueue()` 失败时，应终止并返回错误
14. `VectorQueue == nil` 时，不应报错
15. `VectorQueue.Enqueue()` 失败时，不应影响主流程返回成功
16. `DeleteLocal == true` 时，应尝试删除本地 raw 文件
17. `DeleteLocal == false` 时，不应删除本地 raw 文件
18. 成功返回值应包含：
   - `VideoID`
   - `TaskID`
   - `RawURL`
   - `HLSURL`
   - `Name`

优先级：高

原因：这是上传主链路核心文件，副作用多、依赖多、最需要用 stub/fake 做细粒度断言。

## 3.9 `upload_http.go`

职责：

- 把协议层上传输入桥接到应用层上传流程
- 封面上传并更新 `cover_url`
- 回滚失败时已上传对象

建议测试点：

### `UploadVideo()`

1. 空文件名时返回 `InvalidArgumentError("file is required")`
2. `Reader == nil` 时返回 `InvalidArgumentError("file is required")`
3. `BuildUploadPlan()` 失败时统一映射为 `InvalidArgumentError("file is required")`
4. `OpenUploadWriter()` 失败时直接返回错误
5. `io.Copy()` 失败时直接返回错误，并在 defer 中删除临时文件
6. `writer.Close()` 失败时直接返回错误
7. `FinalizeUpload()` 成功时返回上传结果
8. `FinalizeUpload()` 失败时应删除 `plan.RawAbsPath`

### `UploadVideoCover()`

9. `videoID == 0` 时返回 `InvalidArgumentError("video_id is required")`
10. 空文件名时返回 `InvalidArgumentError("file is required")`
11. `Reader == nil` 时返回 `InvalidArgumentError("file is required")`
12. `Repo.GetByID()` 返回错误时应透传
13. 视频不存在时返回 `updated=false, err=nil`
14. 无扩展名文件时应默认 `.jpg`
15. 未传 `ContentType` 时应按扩展名推导类型
16. `Store.Put()` 失败时直接返回错误
17. `SetVideoCover()` 返回错误时应触发对象回滚删除
18. `SetVideoCover()` 返回 `updated=false` 时应触发对象回滚删除
19. 成功时返回 `/videos/{objectKey}` 格式的 `coverURL`

### `rollbackObject()`

20. `store == nil` 时直接跳过
21. `objectKey` 为空时直接跳过
22. 正常情况下应调用 `store.Delete()`

### `contentTypeFromExtension()`

23. `.jpg/.jpeg` -> `image/jpeg`
24. `.png` -> `image/png`
25. `.webp` -> `image/webp`
26. `.gif` -> `image/gif`
27. 未知扩展名 -> `application/octet-stream`

优先级：高

原因：上传接口是协议层直接入口，错误回滚和内容类型推导都很容易在回归时出问题。

## 3.10 `video.go`

职责：

- 视频列表、删除、元数据更新、发布状态、推荐状态、相似视频、封面更新

建议测试点：

1. `ListVideos()` 应直接透传 repo 结果
2. `ListRecommendPoolVideos()` 应直接透传 repo 结果
3. `DeleteVideo()` 在 `videoID == 0` 时返回 `InvalidArgumentError("video_id is required")`
4. `DeleteVideo()` 在合法 ID 时调用 `Repo.DeleteByID()`
5. `UpdateVideoMetadata()` 在 `videoID == 0` 时返回 `InvalidArgumentError("video_id is required")`
6. `UpdateVideoMetadata()` 在 `title` 为空白时返回 `InvalidArgumentError("title is required")`
7. `UpdateVideoMetadata()` 应对 `title` 做 `TrimSpace()` 后再传给 repo
8. `SetVideoPublished()` 应直接透传 `Repo.UpdatePublished()`
9. `SetVideoRecommend()` 应直接透传 `Repo.UpdateRecommend()`
10. `GetSimilarVideos()` 应直接透传 `Repo.FindSimilar()`
11. `SetVideoCover()` 在 `coverURL` 为空白时返回 `InvalidArgumentError("cover_url is required")`
12. `SetVideoCover()` 在合法参数下应调用 `Repo.UpdateCoverByID()`

优先级：中

原因：这组方法以参数校验和 repo 透传为主，适合批量用表驱动测试补齐。

## 4. 建议补测优先级

如果要分批补测试，建议顺序如下：

1. `upload.go`
2. `upload_http.go`
3. `play.go`
4. `video.go`
5. `question.go`
6. `errors.go`
7. `types.go`
8. `service.go`
9. `system_metrics.go`

原因：

- 上传和播放链路最靠近用户主流程
- 副作用最多，最容易出现真实回归
- 参数校验类文件后补成本较低
- 指标采集类文件更适合在抽象 seam 后增强测试

## 5. 建议的测试组织方式

建议新增以下测试文件：

- `errors_test.go`
- `play_test.go`
- `question_test.go`
- `service_test.go`
- `system_metrics_test.go`
- `types_test.go`
- `upload_test.go`
- `upload_http_test.go`
- `video_test.go`

建议实践：

1. 对纯参数校验逻辑优先使用表驱动测试
2. 对上传、播放这类副作用流程使用 stub/fake repository、status store、object store、fs、queue
3. 对 `Now()` 固定时间，避免时间相关字段导致断言不稳定
4. 对 URL、路径、object key 的断言尽量写完整字符串，不只断言部分字段
5. 对“失败后是否回滚/删除/不继续调用下游”做显式断言，而不是只断言返回错误

## 6. 结论

当前 `videoapp/` 目录已经对推荐、AI 重试、运行时计数器、转码重试策略有一定测试覆盖，但上传、播放、视频管理、题目查询等应用层核心入口仍缺少系统化测试。

其中最值得优先补齐的是：

- `upload.go`
- `upload_http.go`
- `play.go`

这三类文件一旦补上测试，能显著提升主流程稳定性，并降低上传后转码、播放地址推导、封面处理等场景的回归风险。
