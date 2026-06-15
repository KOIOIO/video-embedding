# Vector Worker 内容切分优化执行文档

## 1. 背景

当前 `video-service` 下的 `vector_worker` 在 `hierarchical` 模式中，已经具备以下能力：

1. 按固定 `coarseSegmentSec` 对整段视频做粗分段
2. 对每个粗分段执行 ASR，形成 `coarseItems`
3. 将粗分段 ASR 文本交给 LLM 生成细分段结果
4. 通过 `normalizeLLMSegments()` 对 LLM 结果做时间合法性清洗
5. 对细分段执行 refine ASR 和 embedding

但目前实际问题不是“最后几百毫秒硬裁切不准”，而是更上游的“内容切分不准”，主要表现为：

- 同一个知识点被切碎
- 一个完整步骤被拆成两段
- 上一段总结和下一段新知识点被混在一起
- 分段时间看起来合法，但内容单元不自然

因此，本次优化不优先修改 `tail_alignment` 或音频微调层，而是优先优化：

1. `hierarchical` prompt
2. retry prompt
3. `normalizeLLMSegments()` 的内容合理性治理能力

## 2. 目标

本次优化目标是：

1. 让 LLM 更稳定地按“知识单元”而不是按“句子长度/时间长度”切分视频内容
2. 减少同一知识点被切碎的情况
3. 减少两个不同知识点被错误合并到同一 segment 的情况
4. 在不改动整体 pipeline 的前提下，以最小改造提升内容切分质量

## 3. 非目标

本次优化暂不包含以下内容：

1. 不修改 coarse ASR 的基础并发模型
2. 不引入新的 VAD 模型或声学边界模型
3. 不重构 refine ASR 后处理链路
4. 不新增数据库表结构
5. 不重做对象存储、队列或 worker 生命周期管理

## 4. 当前代码入口

本次改动聚焦以下文件：

1. `video-service/internal/worker/vectorworker/tasks/hierarchical.go`
2. `video-service/internal/worker/vectorworker/tasks/hierarchical_test.go`

涉及的关键函数：

1. `BuildHierarchicalSegmentationPrompt(...)`
2. `BuildHierarchicalSegmentationRetryPrompt(...)`
3. `NormalizeLLMSegments(...)`
4. `IsUniformSegments(...)`

上游调用链路在：

`internal/worker/vectorworker/task.go`

关键调用顺序如下：

1. `buildHierarchicalSegmentationPrompt(...)`
2. `client.ChatCompletionsWithTimeout(...)`
3. `normalizeLLMSegments(...)`
4. `isUniformSegments(...)`
5. 可选 `buildHierarchicalSegmentationRetryPrompt(...)`
6. `upsertHierarchicalSegments(...)`
7. `refineSegmentsASRAndEmbed(...)`

## 5. 优化策略概述

本次优化采用三步策略。

### 5.1 第一步：增强主 prompt 的内容单元表达

现有 prompt 已经强调：

- 不要等距切分
- 优先完整句
- 允许少量重叠
- 输出 `boundary_reason`、`start_anchor_text`、`end_anchor_text`

但目前还没有足够强地定义：

- 什么叫“一个合理的内容段”
- 什么叫“同一个知识单元”
- 哪些切法虽然时长合法，但内容上不合理

因此需要在主 prompt 中新增以下规则：

1. 一个分段应尽量对应一个完整知识单元，例如：定义、定理、连续推导、完整解题步骤、完整例题阶段、总结结论
2. 如果同一段内容仍围绕同一个知识点展开，不要仅因为时长接近上限就切开
3. 如果已经进入新的定义、步骤、例子、结论或分析目标，应优先切分
4. 不要把“定义”和其紧随其后的关键解释拆开
5. 不要把“解题步骤进行中”的内容拆成前后两段
6. 不要把“上一段总结”和“下一段新知识点引入”混在同一段
7. 分段优先级应为：先保证知识单元完整，再保证句意完整，最后才考虑时长均衡

### 5.2 第二步：增强 retry prompt 的错误识别能力

现有 retry prompt 主要纠正“过于等距/规整”的问题，但无法明确纠正以下内容级问题：

1. 同一知识点被拆成多个小段
2. 一个完整步骤被切成前后两半
3. 一个 segment 里同时包含上一知识点收尾和下一知识点开头
4. 分段长度看起来不完全均匀，但本质仍按时间切分

因此需要在 retry prompt 中新增“自检项”和“重切原则”，要求模型在重新输出前检查：

1. 是否把同一知识单元切碎
2. 是否把定义与解释拆开
3. 是否把步骤切断
4. 是否只是做了假改进，即仅仅把时间改得不那么整齐，但内容边界并未真正改善

### 5.3 第三步：给 NormalizeLLMSegments 增加最小内容级治理

当前 `NormalizeLLMSegments()` 的强项是：

1. 时间越界清洗
2. 相邻重叠限制
3. 过短段合并
4. 重新编号

它的不足是：

1. 只判断时间是否合法，不判断内容是否自然
2. 过短段只按时长并前，不区分是否是独立总结句或定义句
3. 无法识别“明显续接关系”的相邻段

本次只做最小增强，不引入复杂内容分析，仅补两类规则：

1. 如果后一段开头是明显续接词，且 `boundary_confidence` 较低，则优先考虑与前一段合并
2. 如果相邻两段都为低置信度，且后一段本质上像前一段的继续，则优先合并

## 6. 文件级改动清单

## 6.1 修改 `BuildHierarchicalSegmentationPrompt(...)`

文件：

`video-service/internal/worker/vectorworker/tasks/hierarchical.go`

建议在现有规则中追加以下文案：

```text
- 一个分段应尽量对应一个完整的知识单元，例如：一个定义、一个定理、一组连续推导、一个完整解题步骤、一个完整例题阶段、一个总结结论
- 如果同一段内容仍在围绕同一个知识点展开解释，不要仅因为时长接近上限就切开
- 如果一个知识点已经讲完，并且开始进入新的定义、步骤、例子、结论或分析目标，应优先在这里切分
- 不要把“定义”和它紧随其后的关键解释强行拆开
- 不要把“解题步骤进行中”的内容切成前后两段
- 不要把“上一段的结论/总结”和“下一段的新知识点引入”混在同一个分段里
- 分段优先级从高到低：先保证知识单元完整，再保证句意完整，最后再考虑时长均衡
- 如果知识单元完整与时长均衡冲突，优先保证知识单元完整
```

建议再新增一段正反例说明：

```text
- 错误切分示例：一个定义刚说出解释还没讲完就切开；一个解题步骤进行到一半就切到下一段；上一段总结和下一段新主题开头混在一起
- 正确切分示例：定义及其必要解释保留在同一段；完整步骤结束后再切到下一步骤；总结收束后再进入新知识点
```

## 6.2 修改 `BuildHierarchicalSegmentationRetryPrompt(...)`

文件：

`video-service/internal/worker/vectorworker/tasks/hierarchical.go`

建议调整开头文案，把 focus 从“过于等距”扩展到“内容边界错误”：

```text
你上一次的分段结果存在内容边界不合理的问题，可能表现为过于等距/规整，也可能表现为把同一个知识点切碎，或把两个不同知识点混在一起。请重新分段，并严格只输出 JSON。
```

建议新增以下自检与重切原则：

```text
- 重新分段前，请先自检上一版结果是否存在以下问题：同一个知识点被拆成多个过碎小段；定义和关键解释被切开；完整解题步骤被切成前后两半；上一段结论和下一段新主题混在一起；分段表面不等距但本质仍按时间切分
- 如果发现某个分段只是上一段内容的延续，应优先合并或后移边界
- 如果发现某个分段同时包含上一知识点收尾和下一知识点起始，应优先把边界移动到两者之间更自然的位置
- 如果某个分段虽然时长合规，但内容上仍然是半个定义、半个步骤或半个例题阶段，也视为不合格分段
- 不要只通过微调时间让长度看起来不一致；只有当知识点边界或步骤边界更清晰时，才算更好的重分段
- 分段优先级从高到低：先保证知识单元完整，再保证句意完整，最后再考虑时长均衡
```

## 6.3 最小增强 `NormalizeLLMSegments(...)`

文件：

`video-service/internal/worker/vectorworker/tasks/hierarchical.go`

在不引入复杂 NLP 或模型分析的情况下，增加一个轻量级内容收口阶段。

建议新增一个局部 helper，例如：

```go
func shouldMergeLowConfidenceContinuation(prev llmSegment, curr llmSegment) bool
```

建议规则：

1. `curr.BoundaryConfidence == "low"` 或 `prev.BoundaryConfidence == "low"`
2. `curr.StartAnchorText` 为空，或以明显续接词开头
3. `curr.ContentSummary` 明显像上一段延续，而不是新知识点引入

明显续接词第一版可以先用静态词表，例如：

- 然后
- 所以
- 因为
- 接下来
- 也就是说
- 我们继续

建议处理位置：

1. 在 `normalized` 生成之后
2. 在 `merged` 生成之前，或与短段合并逻辑放在同一阶段

第一版规则应尽量保守，只合并“很明显的延续段”，避免误伤真正的新知识点。

## 7. 推荐的实施顺序

建议按以下顺序逐步落地，而不是一次性全改：

### 阶段 1：只改 prompt

改动：

1. `BuildHierarchicalSegmentationPrompt(...)`
2. `BuildHierarchicalSegmentationRetryPrompt(...)`

目标：

1. 先观察 LLM 输出的 segment 质量是否明显提升
2. 不引入后处理副作用

### 阶段 2：增加 segment 级诊断日志

建议新增日志：

1. `vectorize_hierarchical_llm_segments_raw`
2. `vectorize_hierarchical_llm_segments_normalized_detail`
3. `vectorize_hierarchical_llm_low_confidence_summary`

每条 segment 建议记录：

1. `idx`
2. `start_sec`
3. `end_sec`
4. `boundary_reason`
5. `start_anchor_text`
6. `end_anchor_text`
7. `boundary_confidence`
8. `content_summary`

目标：

1. 判断问题是模型理解错，还是 normalize 收口不够
2. 积累坏 case 样本

### 阶段 3：增加最小内容后处理规则

改动：

1. `NormalizeLLMSegments(...)`
2. 新增 `shouldMergeLowConfidenceContinuation(...)`

目标：

1. 把明显错切的续接段收口掉
2. 不改变整体架构

## 8. 测试与验证建议

## 8.1 单元测试

文件：

`video-service/internal/worker/vectorworker/tasks/hierarchical_test.go`

建议新增测试方向：

1. prompt 中是否包含“知识单元完整”的约束
2. retry prompt 中是否包含“切碎知识点”的自检约束
3. `NormalizeLLMSegments(...)` 在低置信度续接段场景下是否会触发合并

建议新增测试名称示例：

1. `TestBuildHierarchicalSegmentationPromptMentionsKnowledgeUnitRules`
2. `TestBuildHierarchicalSegmentationRetryPromptMentionsContentBoundaryChecks`
3. `TestNormalizeLLMSegmentsMergesLowConfidenceContinuation`

## 8.2 日志验证

挑选 5 到 10 个“内容切分不准”的真实坏 case，重点看：

1. LLM 原始输出是不是本来就错
2. retry 后有没有改善
3. normalize 是否把本来还行的结果收坏了
4. 哪些 segment 的 `boundary_confidence` 总是低

## 8.3 人工质检维度

每个样本建议从下面 4 个维度打分：

1. 是否把一个定义切碎
2. 是否把一个步骤切断
3. 是否把两个不同知识点混在一起
4. 是否比优化前更像一个自然的知识单元

## 9. 成功标准

本轮优化成功，不要求一步做到完美，但至少应满足：

1. 等距切分的倾向继续受控
2. “同一知识点被切碎”的问题明显下降
3. “上一段收尾 + 下一段起始混合”的问题明显下降
4. segment 的 `boundary_reason` 和 `content_summary` 更容易被人类理解和接受

## 10. 后续可选增强项

如果本轮 prompt + normalize 优化后仍然有明显问题，下一步再考虑：

1. 提升 coarse ASR 输入质量
2. 对 coarse item 增加更强上下文提示
3. 引入更强的内容一致性后处理规则
4. 如果后续确认问题已经从“内容切分”下沉为“边界落点不自然”，再补音频级微调层

## 11. 一句话执行建议

先不要急着改 refine 或 tail alignment。当前最小、最稳、最值得做的优化路径是：

1. 先把 LLM prompt 从“避免等距切分”升级到“按知识单元切分”
2. 再用日志验证模型到底是怎么切的
3. 最后只对最明显的低置信度续接错切做最小收口

这样可以在不推翻现有架构的情况下，优先解决“内容裁不准”的主问题。
