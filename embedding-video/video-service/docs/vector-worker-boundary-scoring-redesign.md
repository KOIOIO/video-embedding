# Vector Worker 边界判定重构设计文档

## 1. 背景

当前 `video-service` 中，`vector_worker` 在 `hierarchical` 模式下已经具备：

1. coarse ASR
2. LLM 细分段
3. `NormalizeLLMSegments()` 规范化
4. refine ASR
5. tail alignment / boundary alignment

但当前边界判定逻辑中，仍然存在一类明显的设计问题：

1. 句首、句尾、续接判断大量依赖固定短语表
2. 短语表直接参与主判断逻辑，而不是弱特征或辅助特征
3. 规则对中文表达的覆盖面极低
4. 规则与“教学讲解口吻”高度耦合，泛化能力差

典型问题包括：

1. `sentenceEndPhrases`
2. `trailingConnectors`
3. `sentenceStartPhrases`
4. `continuationPrefixes`

这些规则虽然能覆盖少量高频表达，但无法适配中文教学语料里成千上万种开始、结束和承接表达方式。

因此，当前设计的根本问题不是“词表还不够长”，而是：

**边界判定本身不应该以短语表命中作为主决策机制。**

## 2. 目标

本次重构的目标不是简单替换几个词表，而是把边界判定从：

- phrase list matching

迁移为：

- multi-signal boundary scoring

具体目标：

1. 不再让固定短语表主导句首/句尾/续接判定
2. 引入多信号打分机制，提升泛化能力
3. 让 LLM 输出的 `content_summary`、`boundary_reason`、`start_anchor_text`、`end_anchor_text`、`boundary_confidence` 成为主要信号来源
4. 将当前二元布尔判断逻辑，升级为结构化边界评分与决策逻辑
5. 让 normalize 阶段具备更强的可解释性和可调试性

## 3. 非目标

本次设计不直接包含以下内容：

1. 不引入完整的 VAD 模型或声学边界模型
2. 不在第一阶段接入词级时间戳
3. 不改变 LLM 基本输出格式为完全新 schema
4. 不重构整个 coarse -> refine pipeline
5. 不要求一次性替换全部旧逻辑为新逻辑，可允许渐进迁移

## 4. 当前设计的核心问题

## 4.1 语言覆盖面不足

固定短语表只能覆盖极少量表达，例如：

- “讲到这里”
- “下面先看”
- “接下来”

但真实中文口语表达中，开始、结束、承接和总结方式极其丰富，不可能穷举。

因此，短语表的扩展方向天然不可持续。

## 4.2 场景耦合严重

当前短语表明显更适配“老师讲题、老师讲概念”的教学口吻。

一旦视频来源变化，例如：

1. 讲座
2. 讨论
3. 访谈
4. 不同地区或不同风格表达

短语表就会迅速失效。

## 4.3 误判不可控

诸如：

- “然后”
- “因为”
- “接下来”

这些词本身并不一定意味着“坏边界”或“续接关系”。关键在于：

1. 它们出现在什么位置
2. 它们和前后内容的语义关系是什么
3. 它们后面是不是新知识点展开

因此，直接 `HasPrefix` 或 `HasSuffix` 属于过度简化。

## 4.4 当前判断层级过低

系统真正要判断的不是：

- 有没有某几个词

而是：

1. 这里是不是一个自然的内容开始
2. 这里是不是一个自然的内容结束
3. 当前 segment 是新内容，还是上一段的延续
4. 当前 segment 是否构成完整知识单元

也就是说，当前问题的粒度应该提升到“边界合理性”和“内容关系建模”，而不是停留在“短语命中”。

## 5. 重构方向概述

建议将现有设计从：

- phrase-driven boolean rules

演进为：

- feature-driven scoring + structured decision

推荐分三层：

1. LLM 语义输出层
2. 边界评分层
3. normalize 决策层

## 6. 设计方案

## 6.1 第一层：LLM 语义输出层

当前 LLM 已经输出如下字段：

1. `content_summary`
2. `boundary_reason`
3. `start_anchor_text`
4. `end_anchor_text`
5. `boundary_confidence`

这些字段相比固定词表，更接近“模型对内容边界的语义解释”。

因此在重构后，应当明确：

1. 这些字段是边界判定的主要信号
2. 固定短语只作为弱特征，而不是主规则

中长期还可考虑让 LLM 增加结构化输出字段，例如：

1. `segment_role`
   - `definition`
   - `explanation`
   - `example`
   - `step`
   - `summary`
2. `boundary_type`
   - `topic_switch`
   - `step_switch`
   - `example_switch`
   - `summary_end`
   - `weak_boundary`

但这不是本次第一阶段必须落地的内容。

## 6.2 第二层：边界评分层

边界评分层负责从多种信号中生成结构化打分结果，而不是简单返回 `true/false`。

建议新增以下核心结构：

```go
type BoundaryScore struct {
	Score      float64
	Confidence string
	Reasons    []string
}
```

建议新增以下核心函数：

```go
func EvaluateStartBoundary(curr LLMSegment) BoundaryScore
func EvaluateEndBoundary(curr LLMSegment) BoundaryScore
func EvaluateContinuation(prev LLMSegment, curr LLMSegment) BoundaryScore
func EvaluateSeparation(prev LLMSegment, curr LLMSegment) BoundaryScore
```

### 6.2.1 文本完整性特征

用于判断段首、段尾是否像完整内容边界。

#### 段首特征

关注：

1. 开头是否像完整陈述开头
2. 是否是残句
3. 是否缺少主语/动作目标
4. 是否明显只是上一句的续接

#### 段尾特征

关注：

1. 是否有完整收束感
2. 是否以句末标点结束
3. 是否存在未完成展开
4. 是否明显停在从句或转折前半句

这类特征允许继续使用标点、句法片段、文本长度等通用信号，但不应依赖少量固定短语表直接得出结论。

### 6.2.2 内容单元完整性特征

这是当前最需要补的信号层。

重点判断：

1. 当前 segment 是否像一个完整知识单元
2. 是否把同一个定义/步骤/例题阶段切碎了
3. 是否把两个不同知识点硬拼在一起

这里建议优先依赖：

1. `content_summary`
2. `boundary_reason`
3. `start_anchor_text`
4. `end_anchor_text`
5. 相邻 segment 的 summary 对比

### 6.2.3 相邻段关系特征

这部分用于替代当前 `continuationPrefixes` 的主逻辑。

重点不是看有没有“然后”“接下来”，而是看：

1. 当前段是不是前一段的继续解释
2. 当前段是不是对前一段的补充、举例或收尾
3. 当前段是否已经开始了新的目标、新的对象或新的步骤

因此，建议将“续接判断”从词匹配改为相邻段关系打分。

### 6.2.4 置信度融合特征

`boundary_confidence` 应从“单独判断条件”改为“融合权重”。

例如可以采用类似下面的思路：

```text
FinalBoundaryScore =
    0.35 * ContentUnitCompleteness
  + 0.25 * TextCompleteness
  + 0.25 * NeighborSeparation
  + 0.15 * LLMConfidence
```

这里的比例只是概念示例，不要求第一版严格实现为浮点权重模型，但要体现出：

1. `boundary_confidence` 不是唯一依据
2. `boundary_confidence` 也不应该一票否决其他信号

## 6.3 第三层：normalize 决策层

在评分层之上，normalize 阶段不再做简单布尔判断，而是输出更结构化的决策。

建议新增结构：

```go
type SegmentBoundaryDecision struct {
	Action     string   // keep / merge / recut
	Score      float64
	Confidence string
	Reasons    []string
}
```

建议新增函数：

```go
func EvaluateSegmentBoundary(prev LLMSegment, curr LLMSegment) SegmentBoundaryDecision
```

### 三种动作

1. `keep`
   - 当前边界合理，保留
2. `merge`
   - 当前边界不合理，当前段更像上一段延续，应合并
3. `recut`
   - 当前边界不适合直接 keep 或 merge，需要后续重切或重判

相比当前：

```go
func shouldMergeLowConfidenceContinuation(prev, curr llmSegment) bool
```

这种结构化决策更容易：

1. 打日志
2. 调试
3. 统计分布
4. 后续演进

## 7. 短语表的新角色

本次设计不是要求“任何词表都不能存在”，而是要求：

1. **短语表不再担任主决策逻辑**
2. 短语表只能作为弱特征存在
3. 即使命中短语，也只能影响分数，不应直接决定 keep / merge / recut

例如：

1. 标点列表仍然是合理的弱特征
2. 极少量强结构词也可以保留为弱负特征
3. 但不能继续用 `HasPrefix("接下来")` 这种逻辑直接推断“续接”

## 8. 推荐实施顺序

## 8.1 第一阶段：去短语表主导化

目标：

1. 保留现有函数签名，降低改动面
2. 将旧逻辑从“强规则”改为“弱特征”
3. 引入评分思维，而不是词表扩充思维

建议做法：

1. 保持 `NormalizeLLMSegments()` 对外签名不变
2. 将 `shouldMergeLowConfidenceContinuation(...)` 改造成评分式判断
3. 把 `LooksLikeSentenceStart / End` 的词表逻辑降级为弱特征

## 8.2 第二阶段：增加结构化决策日志

建议新增日志：

1. `boundary_score_start`
2. `boundary_score_end`
3. `boundary_score_continuation`
4. `segment_boundary_decision`

每次决策至少记录：

1. `action`
2. `score`
3. `confidence`
4. `reasons`
5. `prev_summary`
6. `curr_summary`

目标：

1. 让“为什么 keep / merge / recut”可观察
2. 让后续调参和坏 case 分析有依据

## 8.3 第三阶段：引入更结构化的 LLM 边界标签

在需要进一步增强时，再扩展 LLM 输出：

1. `segment_role`
2. `boundary_type`

但这属于中期优化，不是第一阶段必要条件。

## 9. 对现有函数的迁移建议

## 9.1 现有函数保留，但内部语义改变

第一阶段建议保留外部调用点不变，内部逐步迁移。

例如：

1. `NormalizeLLMSegments()`
   - 保留
2. `LooksLikeSentenceStart()`
   - 保留，但降级为弱信号生成器
3. `LooksLikeSentenceEnd()`
   - 保留，但降级为弱信号生成器
4. `shouldMergeLowConfidenceContinuation()`
   - 重写为评分式或决策式逻辑

## 9.2 不建议继续做的事

以下方向不建议继续投入：

1. 持续往短语表里补 case
2. 通过增加几十个“开头词”“结尾词”试图解决泛化问题
3. 让单个词命中直接控制边界决策

这些会让系统进一步变成难维护的规则黑洞。

## 10. 成功标准

本次重构完成后，至少应满足：

1. 边界决策不再由固定短语表主导
2. keep / merge / recut 有结构化决策依据
3. LLM 输出字段在边界判定中的权重提升
4. 坏 case 调试时，可以从日志里清楚看出：
   - 用了哪些信号
   - 为什么做出这个边界决策
5. 后续新增 case 时，优先通过评分器调整解决，而不是继续补词表

## 11. 结论

当前 `vector_worker` 的边界判定问题，本质上不是“词表还不够长”，而是“判断方法层级太低”。

推荐的重构方向是：

1. 从短语表驱动，迁移到多信号评分驱动
2. 从布尔判断，迁移到结构化边界决策
3. 从词命中，迁移到内容单元、相邻段关系和 LLM 语义信息融合

一句话总结：

**不要再做短语表扩充，而要把边界判定升级为语义摘要、边界解释、相邻段关系共同驱动的评分决策器。**
