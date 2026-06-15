# Vector Worker 边界判定重构实施计划

## 1. 目标

将当前 `vector_worker` 中依赖固定短语表的边界判定逻辑，逐步重构为“多信号评分 + 结构化决策”的方式，提升内容边界识别的泛化能力、可解释性和可维护性。

本计划对应设计文档：

- `video-service/docs/vector-worker-boundary-scoring-redesign.md`

## 2. 实施原则

1. 渐进迁移，不做一次性大爆炸式替换
2. 第一阶段保留现有外部函数签名，优先重构内部语义
3. 先让短语表失去“主决策权”，再逐步引入评分器和结构化决策
4. 所有核心逻辑调整都必须配套单元测试和诊断日志

## 3. 文件范围

### 核心改造文件

1. `video-service/internal/worker/vectorworker/tasks/tail_alignment.go`
2. `video-service/internal/worker/vectorworker/tasks/boundary_alignment.go`
3. `video-service/internal/worker/vectorworker/tasks/hierarchical.go`
4. `video-service/internal/worker/vectorworker/tasks/asr.go`

### 建议新增文件

1. `video-service/internal/worker/vectorworker/tasks/boundary_scoring.go`
2. `video-service/internal/worker/vectorworker/tasks/boundary_scoring_test.go`

### 现有测试文件

1. `video-service/internal/worker/vectorworker/tasks/boundary_alignment_test.go`
2. `video-service/internal/worker/vectorworker/tasks/asr_boundary_alignment_test.go`
3. `video-service/internal/worker/vectorworker/tasks/hierarchical_test.go`
4. `video-service/internal/worker/vectorworker/tasks/tail_alignment_test.go`

## 4. 总体分阶段路线

## 阶段 1：去短语表主导化

### 目标

1. 让短语表从“强规则”降级为“弱特征”
2. 保留原有函数对外接口，内部改为评分思维
3. 先重构最容易误判的 `shouldMergeLowConfidenceContinuation(...)`

### 任务清单

#### 任务 1.1：新增边界评分核心结构

新增文件：

- `internal/worker/vectorworker/tasks/boundary_scoring.go`

新增结构：

```go
type BoundaryScore struct {
	Score      float64
	Confidence string
	Reasons    []string
}

type SegmentBoundaryDecision struct {
	Action     string
	Score      float64
	Confidence string
	Reasons    []string
}
```

说明：

1. `Action` 第一阶段可以只支持 `keep` 和 `merge`
2. `recut` 可以先预留，但不一定立刻落地到主流程

#### 任务 1.2：实现基础评分函数

新增函数建议：

```go
func EvaluateStartBoundary(curr LLMSegment) BoundaryScore
func EvaluateEndBoundary(curr LLMSegment) BoundaryScore
func EvaluateContinuation(prev LLMSegment, curr LLMSegment) BoundaryScore
func EvaluateSeparation(prev LLMSegment, curr LLMSegment) BoundaryScore
func EvaluateSegmentBoundary(prev LLMSegment, curr LLMSegment) SegmentBoundaryDecision
```

第一阶段要求：

1. 可以继续使用标点、长度、部分连接词等弱特征
2. 不允许任何单一词命中直接决定结果
3. 必须把 `content_summary`、`boundary_reason`、`start_anchor_text`、`end_anchor_text`、`boundary_confidence` 纳入判断依据

#### 任务 1.3：重构 `shouldMergeLowConfidenceContinuation(...)`

当前状态：

1. 主要依赖 `continuationPrefixes`
2. 仍然是布尔判断

重构方式：

1. 保留函数名以减少调用面变化
2. 内部改为调用 `EvaluateSegmentBoundary(...)`
3. 只有当 `decision.Action == "merge"` 时返回 `true`

这样做的好处：

1. 对外调用面不变
2. 内部已经完成从短语表判断到评分决策的迁移

#### 任务 1.4：补日志

建议新增：

1. `boundary_score_start`
2. `boundary_score_end`
3. `boundary_score_continuation`
4. `segment_boundary_decision`

日志字段至少包含：

1. `score`
2. `confidence`
3. `action`
4. `reasons`
5. `prev_summary`
6. `curr_summary`
7. `prev_boundary_confidence`
8. `curr_boundary_confidence`

## 阶段 2：弱化旧 `LooksLike...` 函数的统治地位

### 目标

1. `LooksLikeSentenceStart()` / `LooksLikeSentenceEnd()` 继续存在，但只作为弱特征函数
2. 不允许它们单独决定边界

### 任务清单

#### 任务 2.1：修改 `tail_alignment.go`

当前问题：

1. `sentenceEndPhrases`
2. `trailingConnectors`
3. `sentenceStartPhrases`
4. `continuationPrefixes`

都更像硬编码规则，而不是评分特征。

第一阶段处理方式：

1. 保留标点类弱特征
2. 大幅弱化 phrase list 在主流程中的影响
3. 将 phrase list 从“直接判定”改为“附加正负分”使用

#### 任务 2.2：迁移 `boundary_alignment.go`

当前 `scoreBoundaryCandidate(...)` 仍然高度依赖：

1. `LooksLikeSentenceStart(text)`
2. `LooksLikeSentenceEnd(text)`
3. anchor text 包含关系
4. `BoundaryConfidence`

建议重构：

1. 将 `scoreBoundaryCandidate(...)` 改成调用新的评分器
2. `LooksLikeSentenceStart/End` 只给少量加分或减分
3. anchor text 与 boundary reason 在总评分中的权重更高

## 阶段 3：让 normalize 决策层从“布尔合并”升级为“结构化决策”

### 目标

将当前：

```go
if shouldMergeLowConfidenceContinuation(*prev, s) {
    ...
}
```

逐步演进为：

```go
decision := EvaluateSegmentBoundary(*prev, s)
switch decision.Action {
case "merge":
    ...
case "keep":
    ...
case "recut":
    ...
}
```

### 任务清单

#### 任务 3.1：先支持 `keep / merge`

不建议一开始就把 `recut` 也接入生产逻辑。

第一步建议：

1. 先只支持 `keep`
2. 先只支持 `merge`
3. `recut` 先用于日志和分析，不进入生产处理分支

这样更稳。

#### 任务 3.2：完善 `NormalizeLLMSegments()` 中的 explainability

当前已经有：

1. `vectorize_hierarchical_llm_segment_raw`
2. `vectorize_hierarchical_llm_segments_merge_continuation`
3. `vectorize_hierarchical_llm_segment_normalized`

建议进一步补充：

1. `decision_score`
2. `decision_action`
3. `decision_reasons`

让 normalize 每次合并都有明确依据。

## 阶段 4：中期增强项

这些不建议第一轮就上，但要在计划里保留。

### 任务 4.1：让 LLM 输出更结构化标签

未来可以要求 LLM 输出：

1. `segment_role`
2. `boundary_type`

例如：

```json
"segment_role": "definition",
"boundary_type": "topic_switch"
```

这样评分器不必完全靠文本猜语义角色。

### 任务 4.2：引入 `recut` 后处理策略

对于某些边界：

1. 不适合 keep
2. 也不适合 merge

这时就需要 `recut`。

但 `recut` 不是第一阶段必须的，因为它需要更复杂的后续动作，例如：

1. 再次触发局部边界重算
2. 或交给 refine 阶段进一步修正

## 5. 测试计划

## 5.1 新增测试文件

建议新增：

- `internal/worker/vectorworker/tasks/boundary_scoring_test.go`

覆盖内容：

1. `EvaluateStartBoundary(...)`
2. `EvaluateEndBoundary(...)`
3. `EvaluateContinuation(...)`
4. `EvaluateSegmentBoundary(...)`

## 5.2 现有测试增强方向

### `hierarchical_test.go`

补充：

1. 低置信度且明显延续的段应判为 `merge`
2. 虽然有连接词，但明显进入新知识点的段不应被直接合并

### `boundary_alignment_test.go`

补充：

1. anchor 命中但文本不完整时，不应只靠 anchor 获高分
2. boundary confidence 为 low 时，应更依赖内容与相邻段关系

### `tail_alignment_test.go`

补充：

1. `LooksLikeSentenceStart/End` 不再是硬判定依据
2. 词表变化不应直接导致行为大幅摆动

## 6. 验证方式

## 6.1 单元测试验证

执行：

```bash
go test ./video-service/internal/worker/vectorworker/tasks/...
```

目标：

1. 所有新增评分器测试通过
2. 现有 hierarchical / boundary / tail alignment 测试无回归

## 6.2 日志验证

挑选 5 到 10 个坏 case，重点看：

1. `segment_boundary_decision`
2. `boundary_score_continuation`
3. `vectorize_hierarchical_llm_segments_merge_continuation`

验证：

1. 新逻辑是否能清楚解释为什么 merge
2. 是否还存在“只因为命中某个词就 merge”的情况

## 6.3 人工内容验证

按以下维度打分：

1. 是否把同一知识点切碎
2. 是否把完整步骤切断
3. 是否把上一段收尾和下一段起始混在一起
4. merge 是否有合理语义依据

## 7. 第一阶段建议的最小落地范围

如果要控制风险，建议第一轮只做这些：

1. 新增 `boundary_scoring.go`
2. 新增 `BoundaryScore` / `SegmentBoundaryDecision`
3. 重构 `shouldMergeLowConfidenceContinuation(...)`
4. 让 phrase list 从强规则降级为弱特征
5. 补充评分日志
6. 补充测试

不要第一轮就做：

1. 引入词级时间戳
2. 引入完整 VAD
3. 大规模改 LLM schema
4. 接 `recut` 实际重切流程

## 8. 最终交付标准

完成这份计划后，系统至少应达到：

1. 短语表不再主导边界决策
2. merge / keep 决策可解释
3. 评分器对相邻段关系有真正建模
4. 调试时能看见 why，而不是只看见 what
5. 后续新增 bad case 时，不再优先想到“补短语表”

## 9. 一句话总结

这次实施计划的核心，不是“把规则写得更复杂”，而是：

**把当前基于固定短语命中的边界判定，升级成基于语义摘要、边界解释、相邻段关系和弱文本特征共同驱动的结构化评分决策系统。**
