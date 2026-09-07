# Eino 迁移设计

## 目标

在当前视频后端的 AI 子系统中引入字节 CloudWeGo Eino 框架，用于标准化 LLM Chat、Embedding 和后续 AI workflow 编排。

本设计的目标不是把服务整体改成 Eino 应用，而是在保留现有业务链路稳定性的前提下，逐步替换手写模型调用代码，并为后续 RAG、推荐理由、题目解释等能力预留清晰边界。

## 背景

仓库根目录包含多个项目。按当前项目约定，后端活跃服务是 `legacy-video-http/`，本设计的迁移对象也限定在该服务内。

当前 AI 相关能力主要分布在：

- `legacy-video-http/internal/worker/vectorworker/client_openai.go`
  - 手写 OpenAI-compatible ChatCompletions 请求。
  - 手写 OpenAI-compatible Embedding 请求。
  - 同时承载 DashScope WebSocket ASR 的配置与入口。
- `legacy-video-http/internal/worker/vectorworker/hierarchical_stage_processing.go`
  - 负责粗分段 ASR 后调用 LLM 生成细分段。
  - 包含 LLM 输出解析、分段质量检查、二次 retry 和入库。
- `legacy-video-http/internal/worker/vectorworker/tasks/asr.go`
  - 负责细分段二次 ASR、标题改写和 embedding。
- `legacy-video-http/internal/infrastructure/embedding/client.go`
  - HTTP API 推荐入口使用的单条 embedding 客户端。
- `legacy-video-http/internal/application/videoapp/recommendation/service.go`
  - 题目文本向量化、pgvector 相似度检索、推荐记录入库。

这些代码已经形成了稳定的业务能力，但模型调用层和 AI 编排层仍以手写 HTTP、手写 retry、手写输出解析为主。Eino 适合接管这些 AI 子步骤，而不是替代 FFmpeg、Redis Stream、GORM stage 记录或 HTTP handler。

## 范围与非目标

### 范围

- 引入 Eino adapter 层，统一 ChatModel 和 Embedding 的调用方式。
- 保持业务层依赖项目内小接口，而不是直接依赖 Eino 类型。
- 将 LLM 细分段、标题改写等 AI 子流程逐步抽象为 Eino Chain 或 Graph。
- 后续可按需求把 pgvector 查询封装为 Retriever，但不在第一阶段强制实施。

### 非目标

- 不整体重写 `vectorworker`。
- 不替换 DashScope WebSocket ASR 协议实现。
- 不替换 FFmpeg 音视频处理。
- 不替换 Redis Stream 分阶段 worker。
- 不替换 GORM repository、数据库表结构、HTTP API 路由。
- 不改变推荐、观看记录、stage 状态记录等业务语义。

## 方案对比

### 方案 A：仅做技术评估，不进入迁移设计

做法：只记录哪些模块适合 Eino，哪些模块不适合 Eino。

优点：

- 变更成本最低。
- 对当前代码没有任何影响。

缺点：

- 后续开发仍需重新补迁移边界、接口设计和测试策略。
- 不能直接指导实现。

结论：不采用。当前已有足够上下文，可以形成可执行设计。

### 方案 B：渐进式迁移 AI 子系统

做法：先用 Eino adapter 替换 Chat/Embedding 调用，再将 LLM 分段和标题改写抽成 Chain/Graph，最后按业务需求决定是否引入 Retriever/RAG。

优点：

- 改动集中在 AI 子系统，风险可控。
- 业务层继续依赖现有小接口，便于回滚。
- 能逐步减少手写协议代码和重复的 provider 适配逻辑。
- 后续可自然扩展到 RAG、推荐理由、题目解释。

缺点：

- 初期会同时存在旧 client 和 Eino adapter，需要明确装配边界。
- 需要补充 provider fake、adapter 单测和集成级流程测试。

结论：推荐采用。

### 方案 C：Eino-first 重构整个向量化 worker

做法：用 Eino Graph 表达完整向量化链路，包括下载、切片、ASR、LLM、embedding、入库。

优点：

- 长期架构表达更统一。

缺点：

- 改动范围过大。
- 会把 Eino 用到非 AI 基础设施上，收益低且风险高。
- 当前 Redis Stream stage 记录、幂等、重试和状态持久化已经比较贴合业务，不适合推翻。

结论：不采用。

## 选定方案

采用方案 B：渐进式迁移 AI 子系统。

核心原则：

1. Eino 只进入 AI 子系统，不进入非 AI 基础设施。
2. 业务层继续依赖项目内接口，例如 `Embed`、`ChatCompletionsWithTimeout`、`Transcribe`。
3. ASR WebSocket 保留现有实现，必要时只把它包装成自定义能力，不改协议细节。
4. LLM 输出的业务后处理继续保留在 `tasks` 层，不把业务规则藏进 provider adapter。
5. 每个迁移阶段都能独立测试和回滚。

## 目标架构

### Adapter 层

新增一个 Eino adapter 层，建议放在：

- `legacy-video-http/internal/infrastructure/ai/eino/`

该层负责把 Eino 的 ChatModel、Embedding 能力适配成当前项目内部接口。

建议接口分为三类：

```go
type ChatClient interface {
	ChatCompletionsWithTimeout(ctx context.Context, model string, prompt string, timeoutMinutes int) (string, error)
}

type BatchEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
```

`Transcribe(ctx, audioPath)` 不由 Eino adapter 接管，继续由现有 DashScope WebSocket client 实现。

### Worker 装配

`vectorworker.Register` 继续负责装配 worker 依赖。

第一阶段可以保留 `openAICompatClient` 的 ASR 能力，并把 Chat/Embedding 委托给 Eino adapter。也可以新增组合型 client：

- `ASRClient`：保留现有 DashScope WebSocket 实现。
- `EinoChatClient`：负责 LLM Chat。
- `EinoEmbeddingClient`：负责批量 embedding。
- `VectorAIClient`：组合以上能力，满足现有 `tasks.openAICompatClient` 接口。

这样可以避免一次性修改 `tasks/asr.go`、`hierarchical_stage_processing.go` 的调用方式。

### 业务后处理

以下逻辑保持在现有业务层，不迁移到 provider adapter：

- `BuildHierarchicalSegmentationPrompt`
- `BuildHierarchicalSegmentationRetryPrompt`
- `NormalizeLLMSegments`
- `RepairMismatchedSegments`
- `IsUniformSegments`
- `BuildSummaryRewritePrompt`
- `NormalizeEmbeddingDim`

这些函数表达的是教育视频分段和推荐业务规则，不属于通用模型 provider 能力。

## 阶段性迁移计划

### 第一阶段：替换 Chat 和 Embedding 调用层

目标：

- 用 Eino adapter 替代手写 ChatCompletions 和 Embedding HTTP 请求。
- 保留现有业务接口和调用点。
- 保留旧 client 作为回滚路径。

涉及位置：

- `legacy-video-http/internal/worker/vectorworker/client_openai.go`
- `legacy-video-http/internal/infrastructure/embedding/client.go`
- `legacy-video-http/internal/http/app/app.go`
- `legacy-video-http/internal/worker/vectorworker/app.go`

建议做法：

1. 新增 Eino Chat adapter，实现 `ChatCompletionsWithTimeout` 等价能力。
2. 新增 Eino batch embedding adapter，实现 `Embed(ctx, []string)`。
3. 新增 Eino text embedding adapter，实现 `Embed(ctx, string)`。
4. 保留现有 `FallbackEmbedder`，只替换其中的 primary embedder。
5. 通过配置或构造函数控制使用 Eino adapter 还是旧 HTTP client。

验证重点：

- Chat adapter 能正确传递 system/user prompt、model、temperature 和 timeout。
- Embedding adapter 保持返回顺序与输入顺序一致。
- 维度归一化仍由现有业务逻辑处理。
- provider 失败时 fallback 和错误返回语义不变。

### 第二阶段：抽象 LLM 分段和标题改写 workflow

目标：

- 将 LLM 细分段和标题改写从“手写流程代码”收敛为 Eino Chain 或 Graph。
- 保留现有后处理函数和质量判断。

LLM 细分段建议表达为：

```text
BuildSegmentationPrompt
-> ChatModel
-> ExtractAndNormalizeSegments
-> RepairMismatchedSegments
-> UniformityCheck
-> OptionalRetry
-> PersistSegments
```

标题改写建议表达为：

```text
BuildSummaryRewritePrompt
-> ChatModel
-> TrimAndValidateTitle
```

涉及位置：

- `legacy-video-http/internal/worker/vectorworker/hierarchical_stage_processing.go`
- `legacy-video-http/internal/worker/vectorworker/tasks/asr.go`
- `legacy-video-http/internal/worker/vectorworker/tasks/hierarchical.go`

注意事项：

- `PersistSegments` 仍然可以留在业务流程中，不一定放入 Eino Graph。
- retry 条件仍基于 `IsUniformSegments` 等现有业务判断。
- Eino Chain/Graph 的输入输出结构要显式定义，避免传递大而散的 `map[string]any`。

验证重点：

- LLM 输出合法时，细分段结果与旧流程一致或更稳定。
- LLM 输出非法时，错误信息包含可排查 preview。
- uniform 分段触发 retry 的条件不变。
- 标题改写只在现有 `shouldUseLLMSummaryRewrite` 条件成立时触发。

### 第三阶段：按需求引入 Retriever/RAG

目标：

- 只有当产品需要“题目解释、推荐理由、问答式学习助手、跨视频知识点回答”时，才封装 Retriever/RAG。
- 当前推荐列表不因引入 Eino 而改变业务行为。

可选方向：

- 将 `edu_video_segment` 的 pgvector 查询封装为 Eino Retriever。
- 将题目内容、相似片段、视频标题、知识标签组合为 RAG 上下文。
- 用 ChatModel 生成推荐理由或题目讲解。

暂不迁移：

- `RecommendByQuestion` 的 SQL 检索排序。
- `SaveUserVideoRecommendation` 入库逻辑。
- `ReportWatch` 和播放统计逻辑。

## 适合替换的代码点

### ChatCompletions

当前位置：

- `legacy-video-http/internal/worker/vectorworker/client_openai.go`

替换价值：

- 减少手写 HTTP payload、response parsing、retry 逻辑。
- 后续切换 OpenAI-compatible、Ark、DashScope 等 provider 更容易。
- 更容易统一 callbacks、trace 和模型调用指标。

### Embedding

当前位置：

- `legacy-video-http/internal/worker/vectorworker/client_openai.go`
- `legacy-video-http/internal/infrastructure/embedding/client.go`

替换价值：

- 消除单条 embedding 和批量 embedding 两套重复 HTTP 逻辑。
- 统一模型配置、API key、base URL 和错误处理。
- 便于后续接入多 provider fallback。

### LLM 细分段 workflow

当前位置：

- `legacy-video-http/internal/worker/vectorworker/hierarchical_stage_processing.go`

替换价值：

- 当前流程天然是 prompt -> model -> parse -> validate -> retry。
- 用 Chain/Graph 能让流程边界更清晰，便于测试每个节点。
- 后续增加 callbacks、trace、token 统计更自然。

### 标题改写

当前位置：

- `legacy-video-http/internal/worker/vectorworker/tasks/asr.go`

替换价值：

- 子流程短小，适合作为低风险 Eino workflow 试点。
- 能验证 Eino Chat adapter 在真实 worker 链路中的行为。

## 不适合替换的代码点

### DashScope WebSocket ASR

当前位置：

- `legacy-video-http/internal/worker/vectorworker/client_openai.go`
- `legacy-video-http/internal/worker/vectorworker/dashscope_ws.go`

不替换原因：

- 这是实时音频 WebSocket 协议，不是标准 ChatModel 或 Embedding 调用。
- 现有实现包含音频流速控制、task lifecycle、模型 fallback 和 quota 判断。
- Eino 不应直接接管这种厂商流协议细节。

### FFmpeg、对象存储、Redis Stream、GORM

不替换原因：

- 这些是确定性基础设施或业务状态管理。
- 当前逻辑与视频处理、幂等、stage 记录强绑定。
- 使用 Eino 替代不会减少复杂度，反而会扩大依赖面。

### 推荐业务入库逻辑

当前位置：

- `legacy-video-http/internal/application/videoapp/recommendation/service.go`
- `legacy-video-http/internal/infrastructure/persistence/gorm_video_repository.go`

不替换原因：

- 推荐结果不仅是向量检索，还包括用户、题目、视频片段、观看状态和得分入库。
- 这些是业务规则，不应隐藏在 Retriever adapter 内。

## 配置与兼容性

建议新增或复用配置时遵守以下原则：

- 继续兼容现有 `DASHSCOPE_API_KEY`、`OPENAI_API_KEY`、`EMBEDDING_API_KEY` 等环境变量。
- 继续兼容现有 `embedding.base-url`、`embedding.options.model`、`VectorWorker.LLMModel`。
- 如需新增 provider 选择项，应放在 AI 相关配置下，并提供旧 client 回退选项。
- 默认行为应尽量保持与当前配置一致，避免部署后模型 provider 被意外切换。

建议新增配置示例：

```yaml
AI:
  Provider: "legacy" # legacy 或 eino
```

如果只需要代码层灰度，也可以先不新增 YAML 字段，在构造函数或测试中注入 adapter，等第一阶段验证稳定后再补配置开关。

## 测试策略

### 单元测试

新增 Eino adapter 单测：

- Chat 请求成功时返回模型文本。
- Chat timeout 时返回 context deadline 相关错误。
- Embedding 返回数量与输入数量一致。
- Embedding 返回数量不一致时报错。
- provider 失败时错误能被现有 retry/fallback 逻辑识别。

现有业务测试保持覆盖：

- `tasks/hierarchical_test.go`
- `tasks/asr_refine_input_test.go`
- `tasks/utils_test.go`
- `application/videoapp/recommendation/service_test.go`

### 集成级测试

使用 fake ChatModel / fake Embedding provider 验证：

- LLM 细分段 workflow 能从固定 JSON 输出生成稳定分段。
- uniform 分段仍触发 retry。
- 标题改写只在复杂文本或弱支撑摘要场景触发。
- 推荐入口在 embedding provider 不可用时仍按现有降级策略返回。

### 回归验证

每阶段完成后至少运行：

```text
cd legacy-video-http
go test ./internal/worker/vectorworker ./internal/worker/vectorworker/tasks ./internal/infrastructure/ai ./internal/infrastructure/embedding ./internal/application/videoapp/recommendation
```

必要时再运行：

```text
cd legacy-video-http
go test ./...
```

## 回滚策略

第一阶段必须保留旧 HTTP client 或等价 fallback：

- 如果 Eino adapter 出现 provider 兼容问题，装配回旧 `openAICompatClient` 的 Chat/Embedding 实现。
- 如果只影响 HTTP 推荐入口，装配回旧 `internal/infrastructure/embedding/client.go`。
- 如果 workflow 化后的 LLM 分段不稳定，保留旧的手写流程入口，通过构造函数或配置切回。

回滚不应要求修改数据库 schema、Redis key 或 HTTP API。

## 风险与缓解

### provider 行为差异

风险：Eino adapter 传参、默认 temperature、message 结构或 base URL 处理与旧 client 不一致。

缓解：

- 第一阶段先保持 system prompt、temperature、model、timeout 与旧 client 一致。
- 用 fake provider 验证请求语义。
- 对真实 provider 做小批量 worker 验证。

### JSON 输出稳定性

风险：ChatModel 接入方式变化后，模型输出格式可能略有变化。

缓解：

- 继续使用现有 `ExtractFirstJSONObject` 和 `NormalizeLLMSegments`。
- 保留 preview 错误信息。
- 不把 JSON parse 逻辑下沉到 provider adapter。

### 依赖版本风险

风险：Eino 和 eino-ext 版本变化可能影响 provider 行为。

缓解：

- 在 `go.mod` 固定版本。
- adapter 层隔离 Eino 类型，业务层不直接引用。
- 升级 Eino 时只需要重点回归 adapter 和 workflow 测试。

### 过度迁移风险

风险：为了使用 Eino，把 FFmpeg、Redis、GORM 等确定性流程也纳入 Graph，导致调试和回滚复杂。

缓解：

- 明确 Eino 只用于 AI 子步骤。
- stage worker、持久化、对象存储继续保持现有实现。

## 验收标准

第一阶段验收：

- Chat 和 Embedding 调用可以通过 Eino adapter 完成。
- 旧接口调用方无需感知 Eino 类型。
- 现有向量化和推荐相关单测通过。
- provider 失败时 fallback 或错误语义与旧实现一致。

第二阶段验收：

- LLM 细分段流程可通过 Eino Chain/Graph 表达。
- 现有分段后处理规则不丢失。
- uniform retry、非法 JSON、标题改写等关键场景有测试覆盖。

第三阶段验收：

- 仅在有 RAG/推荐理由等明确需求时启动。
- Retriever 封装不改变当前推荐列表排序、入库和观看记录语义。

## 建议实施顺序

1. 新增 Eino Chat/Embedding adapter，并用 fake provider 测试。
2. 修改 worker 和 HTTP app 装配，让现有业务接口可以使用 Eino adapter。
3. 小范围验证 embedding 和 LLM 分段链路。
4. 将标题改写抽成最小 Eino Chain，作为 workflow 试点。
5. 将 LLM 细分段抽成 Eino Chain/Graph，并保留旧流程回滚入口。
6. 等产品需要 RAG 能力时，再设计 Retriever/RAG 阶段。

## Implementation Notes

- `AI.Provider` 控制是否选择 Eino-backed adapters，默认值为 `legacy`。
- Eino Chat 和 Embedding adapters 隔离在 `legacy-video-http/internal/infrastructure/ai/eino/`。
- 当前 `eino-ext v0.0.1-alpha` 没有可用 provider 包，因此本次实现使用 Eino 组件接口加项目内 OpenAI-compatible HTTP adapter。
- Vector worker ASR 继续使用现有 DashScope WebSocket 实现，只将 Chat 和 Embedding 能力切到可组合 adapter。
- HTTP 推荐入口的 embedding 仍由 `FallbackEmbedder` 保护，Eino adapter 初始化失败时回落到 legacy embedding client。
