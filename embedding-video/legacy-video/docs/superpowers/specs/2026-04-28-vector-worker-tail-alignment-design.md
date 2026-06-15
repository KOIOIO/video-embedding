# 向量化分段尾部保守对齐设计

## 目标

修复当前 `hierarchical` 向量化链路中“内容摘要正确但视频片段经常在一句话尚未讲完时提前结束”的问题。

本次设计的目标不是让分段更激进地贴紧语义边界，而是遵循已确认的产品取舍：宁可片段尾部多保留 1-3 秒，也要尽量避免把一句话截断在半句处。

## 范围与非目标

### 范围

- 仅修改后端 `worker` 内部算法与配置。
- 仅针对 `vector worker` 的 `hierarchical` 模式分段链路。
- 允许新增 worker 内部辅助函数、配置项、单元测试和日志。

### 非目标

- 不修改前端页面与播放逻辑。
- 不修改 gRPC / HTTP 接口协议。
- 不修改数据库表结构。
- 不重做整套分段策略，不替换 LLM 为其他主分段器。

## 当前实现与问题定位

### 当前分段链路

当前 `hierarchical` 模式的主流程位于：

- `nlp-video-project/internal/worker/vectorworker/task.go`
- `nlp-video-project/internal/worker/vectorworker/tasks/hierarchical.go`
- `nlp-video-project/internal/worker/vectorworker/tasks/asr.go`

真实链路是：

1. 整段视频先按固定粗分段时长切成多个粗片段。
2. 每个粗片段做 ASR，得到粗粒度文本。
3. LLM 基于粗粒度 ASR 文本输出细分段 `start_time/end_time/content_summary/knowledge_tags`。
4. `NormalizeLLMSegments` 对 LLM 输出做合法化、排序、限制重叠、合并短段。
5. 细分段草稿写入 `edu_video_segment`。
6. `RefineSegmentsASRAndEmbed` 再按这些细分段时间去截音频、做更精确的二次 ASR，并生成 embedding。

### 问题根因

当前真正决定裁切边界的是第 3-4 步，但这两个步骤只有粗粒度文本，没有直接利用最终片段级别的 ASR 结果去校准结束点。

因此会出现：

- LLM 对“讲的是哪个知识点”判断基本正确。
- 但 `end_time` 仍然可能落在一句话尚未说完的位置。
- `RefineSegmentsASRAndEmbed` 虽然拿到了更接近最终片段的 ASR 文本，但它只用来补全文本和生成 embedding，没有反向修正边界。

这个问题本质上不是 FFmpeg 裁切精度不够，而是“最终裁切时间在语义上不够保守”。

## 已有机制与不足

`nlp-video-project/internal/worker/vectorworker/tasks/hierarchical.go` 里已经做过一轮优化：

- prompt 明确要求优先在完整句结束处切分。
- 允许相邻分段保留少量 overlap。
- 对过于规整的分段结果会触发 retry prompt。

这些优化能减少明显的等距分段问题，但仍然属于“让 LLM 猜边界”，不足以稳定解决“半句截断”。

原因是：

- LLM 看到的是粗分段 ASR 文本，不是最终片段级音频。
- 句子结束点经常落在粗分段文本内部，而不是粗分段边缘。
- 即便 prompt 写得更强，也无法保证每次都把 `end_time` 放到自然收尾位置。

## 方案对比

### 方案 A：固定尾部补偿

做法：对所有细分段统一把 `end_time` 延后固定 1-3 秒。

优点：

- 实现最简单。
- 不需要新增太多判断逻辑。

缺点：

- 只能缓解，不能识别句子是否真的讲完。
- 有些分段仍会截半句。
- 有些分段会多出明显无关内容。

结论：不作为本次主方案。

### 方案 B：基于二次 ASR 的尾部保守校准

做法：保留现有 LLM 分段结果，在细分段二次 ASR 之前增加“尾部校准”步骤。对每个 segment 以当前 `end_time` 为基准，向后试探最多 3 秒；如果当前文本看起来像半句，则逐步延长，直到命中更自然的句尾或达到上限。

优点：

- 直接针对当前体验问题。
- 不改前端、接口、表结构。
- 比固定补偿更稳，因为延长是“按文本证据触发”，不是盲目统一补偿。
- 能严格控制额外冗余时长上限，符合“宁可多 1-3 秒”的产品偏好。

缺点：

- worker 逻辑会增加一个边界校准层。
- 会增加一定的音频截取和 ASR 成本。

结论：推荐采用。

### 方案 C：重做更细粒度的 LLM 时间锚点分段

做法：先生成比当前更细的时间锚点文本，再让 LLM 直接按细粒度锚点输出边界。

优点：

- 理论上边界可更准。

缺点：

- 改动范围明显更大。
- 需要更复杂的 ASR 时间信息与更大验证成本。
- 不适合作为本次“最小、可落地修复”。

结论：不进入本次范围。

## 选定方案

采用方案 B：`LLM 负责语义边界，worker 负责句尾保护`。

### 核心思路

1. 保留当前粗分段 -> LLM 细分段的主链路，不推翻现有分段策略。
2. 在最终细分段二次 ASR 阶段前，为每个 segment 增加一个“尾部保守校准”步骤。
3. 如果当前 segment 的尾部看起来不像一句完整收尾，则允许在最多 3 秒范围内逐步延长 `end_time`。
4. 校准后的时间继续用于最终二次 ASR、embedding 与状态回写。

## 文件分解

### 需要修改的现有文件

- `nlp-video-project/internal/config/types.go`
  - 给 `VectorWorkerConfig` 增加尾部对齐配置项。
- `nlp-video-project/internal/worker/vectorworker/app.go`
  - 读取新配置、填默认值、记录启动日志，并将参数传给任务处理逻辑。
- `nlp-video-project/internal/worker/vectorworker/task.go`
  - 扩展 `handleVectorizeTask` 和 `refineSegmentsASRAndEmbed` 调用参数，把尾部对齐配置传入细分段补处理链路。
- `nlp-video-project/internal/worker/vectorworker/tasks/asr.go`
  - 增加尾部保守校准主逻辑，并在二次 ASR 前应用校准结果。

### 建议新增的文件

- `nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment.go`
  - 负责尾部对齐的纯逻辑：配置结构、句尾判断、延长决策、下一个候选结束时间计算。
- `nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment_test.go`
  - 负责句尾判定与时间延长决策的单元测试。
- `nlp-video-project/internal/worker/vectorworker/tasks/asr_tail_alignment_test.go`
  - 负责带 fake ffmpeg / fake transcribe 的集成式单测，验证细分段尾部会按规则被延长或停止延长。

如果实际代码组织上更适合把测试合并进已有测试文件，也可以保持最小文件数，但逻辑职责需要不变。

## 详细设计

### 1. 新增配置项

在 `VectorWorkerConfig` 中新增以下配置：

- `TailAlignmentEnabled bool`
  - 默认 `true`
  - 关闭时完全走当前旧逻辑，便于回滚和 A/B 对比。
- `TailAlignmentMaxExtendSec int`
  - 默认 `3`
  - 限制单个 segment 最多向后延长多少秒。
- `TailAlignmentProbeStepSec int`
  - 默认 `1`
  - 每次向后试探的步长。
- `TailAlignmentMaxOverlapSec int`
  - 默认 `6`
  - 校准后允许与下一段保留的最大重叠秒数。

为什么需要 `TailAlignmentMaxOverlapSec`：

- 当前 `NormalizeLLMSegments` 会限制初始 LLM 分段的 overlap。
- 但尾部校准发生在 LLM 之后，可能让当前段往后长一点。
- 本次产品取舍明确偏向“当前段讲完优先”，因此需要一个单独的、可控的对齐期 overlap 上限，而不是无条件禁止侵入下一段。

### 2. 新增尾部对齐判定逻辑

新增一个纯逻辑模块，用于判断“当前 segment 是否值得继续向后试探”。

推荐拆成以下职责：

- `NeedsTailExtension(text string) bool`
  - 判断当前文本末尾是否像半句。
- `LooksLikeSentenceEnd(text string) bool`
  - 判断文本是否看起来在自然句尾结束。
- `NextAlignedEndSec(currentEndSec int, probeStepSec int, maxEndSec int) int`
  - 计算下一次试探的结束时间。

启发式规则以“稳定、简单、可测”为主，不引入重 NLP：

判定“像完整收尾”的信号：

- 末尾是句号类标点：`。` `！` `？` `.` `!` `?`
- 末尾是常见的中文收束短语，例如：`就是这样`、`到这里`、`讲完了`、`总结一下`
- 文本整体长度足够，且最后一句没有明显未完成迹象

判定“像半句未完”的信号：

- 末尾没有句末标点
- 末尾是明显连接词或承接词，例如：`所以`、`然后`、`但是`、`因为`、`如果`、`接下来`
- 末尾是短残句，例如最后几个字明显像被硬切断
- 扩展后文本比原文本多出的尾巴部分出现了更自然的句尾结束

这里的规则目标不是语法学正确，而是稳定拦住最糟糕的“半句截断”。

### 3. 尾部对齐在链路中的位置

尾部对齐发生在 `RefineSegmentsASRAndEmbed` 内部、最终二次 ASR/embedding 之前。

原因：

- 当前 `RefineSegmentsASRAndEmbed` 已经逐段处理 segment，并能直接访问 `start_time/end_time`。
- 这是最接近最终落库片段内容的一层，不需要额外改数据库协议。
- 可以在这里把“校准边界”和“最终使用校准后片段做二次 ASR”串起来，避免重复保存中间态。

### 4. 尾部对齐执行流程

对每个待补处理的 segment，执行以下流程：

1. 先按数据库中的 `start_time/end_time` 抽取当前片段音频并做一次 ASR。
2. 对得到的文本做句尾判定。
3. 如果判定已经是自然收尾：
   - 直接使用当前 `end_time`。
4. 如果判定像半句：
   - 以 `probeStepSec` 为步长向后试探。
   - 每次试探时，把结束时间延长到 `min(current_end + step, original_end + max_extend, next_segment_start + max_overlap, video_duration)`。
   - 对试探后的更长音频再做 ASR。
   - 如果新文本出现更自然的句尾，则采纳这个更长的 `end_time`。
   - 如果达到 `max_extend` 仍未命中自然句尾，则停止延长，使用最后一次试探结果或原结果。

### 5. 与下一段边界的关系

尾部对齐允许有限侵入下一段起点，但要受控。

规则如下：

- 如果当前段延长后仍未超过下一段起点，直接允许。
- 如果超过下一段起点，但 overlap 不超过 `TailAlignmentMaxOverlapSec`，允许。
- 如果超过上限，则把当前段可延长上限截断在 `next_segment_start + TailAlignmentMaxOverlapSec`。

这样做的理由：

- 你当前痛点是“当前段太早结束”。
- 本次产品选择明确偏向“宁可多留 1-3 秒，也别截半句”。
- 因此允许小幅 overlap 是合理代价，但必须有上限，避免一个 segment 吞掉太多下一段内容。

### 6. 对数据库记录的更新策略

如果尾部对齐最终决定延长某个 segment 的结束时间，则在写 embedding 成功前，把该 segment 的 `end_time` 一并更新为校准后的值。

要求：

- 校准后的 `end_time` 必须只影响当前未完成 segment。
- 已经 `status=1` 的老 segment 不回写。
- 只在当前任务的细分段补处理事务范围内更新，避免产生部分更新状态。

这能保证后续推荐、播放、再次重试任务时，读到的是校准后的稳定边界，而不是一次性的运行时临时结果。

### 7. 日志与可观测性

新增结构化日志，至少覆盖：

- `tail_alignment_start`
  - `video_id` `task_id` `seg_id` `start_sec` `end_sec`
- `tail_alignment_probe`
  - `seg_id` `from_end_sec` `to_end_sec` `attempt` `reason`
- `tail_alignment_extended`
  - `seg_id` `old_end_sec` `new_end_sec` `probe_count`
- `tail_alignment_skipped`
  - `seg_id` `reason`，例如 `already_sentence_end`、`disabled`、`max_extend_reached`

这些日志主要用于排查：

- 为什么某段没有被延长
- 为什么某段被延长了 2-3 秒
- 当前规则是否过于保守或过于激进

## 测试设计

### 单元测试：纯逻辑

针对 `tail_alignment.go` 的纯函数补测试，覆盖：

1. 完整句结尾不应延长
2. 无句末标点的半句应判定为需要延长
3. 以连接词结尾应判定为需要延长
4. 探测步长、最大延长秒数、最大 overlap 的组合边界
5. 已达到最大结束时间时不再继续试探

### 单元测试：ASR 补处理链路

针对 `RefineSegmentsASRAndEmbed` 附近新增带 fake 依赖的测试，覆盖：

1. 原片段 ASR 是半句，`+1s` 仍半句，`+2s` 收尾完整，应延长 2 秒
2. 原片段已是完整句，不应额外延长
3. 即使想继续延长，也不能超过 `TailAlignmentMaxExtendSec`
4. 即使想继续延长，也不能超过 `next_segment_start + TailAlignmentMaxOverlapSec`
5. 尾部对齐关闭时，逻辑退回旧行为

### 回归测试

确认以下旧行为不被破坏：

- `NormalizeLLMSegments` 现有合法化逻辑仍成立
- `hierarchical` 模式任务重试与 resume 逻辑仍可工作
- embedding 结果数量与 segment 对应关系不乱序

### 工程级验证

执行：

- `go test ./nlp-video-project/...`

本次只要求后端测试通过，不需要前端构建。

## 风险与取舍

### 风险 1：ASR 标点不稳定

某些 ASR 返回的文本可能标点很少，若只依赖句号判断会误判。

应对：

- 句尾判断必须采用“标点 + 承接词 + 残句特征”的组合规则。
- 单测中要覆盖“无标点但像完整收尾”和“无标点且明显半句”两类样本。

### 风险 2：ASR 成本上升

尾部试探会增加额外的裁音频和转写次数。

应对：

- 默认最多延长 3 秒，最多 3 次探测。
- 仅在判定为“可能半句”时触发，不对所有 segment 无脑多跑。

### 风险 3：相邻 segment overlap 增加

为了避免截半句，当前段可能侵入下一段开头。

应对：

- 用单独配置限制最大 overlap。
- 明确把“当前段讲完优先”作为本次产品取舍写入代码与测试。

## 验收标准

满足以下条件即可认为本次设计达标：

1. `hierarchical` 模式下，明显半句截断的 segment 数量显著减少。
2. 单个 segment 的尾部额外保留时长默认不超过 3 秒。
3. 对已经自然收尾的 segment，不产生不必要延长。
4. 不修改前端、接口协议、数据库表结构。
5. 后端测试通过：`go test ./nlp-video-project/...`

## 实施建议

实施时应保持最小改动原则：

- 不要重写整条向量化链路。
- 先把尾部对齐封装成可测的纯逻辑，再接入 `RefineSegmentsASRAndEmbed`。
- 优先补足测试，再接主逻辑。
- 每完成一个小块就运行对应测试，避免把边界判断、ASR 编排、数据库回写一次性揉在一起。
