# LLM Boundary Alignment Design

## Summary

当前 hierarchical 分段链路已经具备以下能力：

- 基于 coarse ASR 文本让 LLM 生成结构化分段
- 对过于等距的 LLM 分段自动触发一次 retry
- 在 refine 阶段对片段尾部做轻量 ASR 对齐，减少结尾硬截断

但现有方案仍然存在两个核心问题：

- 片段开头和结尾都可能落在半句话中间，播放体验不自然
- LLM 给出的知识点边界与自然语句边界并不总是重合，导致“语义切对了但听感很硬”，或“听感完整但知识点切得不准”

本设计引入一个新的 `Boundary Alignment` 层，把“语义分段”和“可播放边界对齐”拆成两个职责清晰的阶段：

- LLM 负责给出知识点、步骤、定义、总结等语义边界
- ASR 驱动的边界校正器负责把边界吸附到更自然的起句点和句尾点

目标是同时降低以下问题：

- 片段开头半句
- 片段结尾半句
- 主题切换处的硬裁切

约束如下：

- 策略选择为“平衡型”：兼顾知识点边界和句子完整性
- 允许相邻片段存在 1 到 3 秒少量重叠
- 成本允许小幅增加，但不接受明显增加多轮 AI 调用带来的耗时和费用失控
- 任一增强环节失败时，链路必须安全降级到现有行为，不得让结果明显劣化

## Problem Statement

当前问题不是单一的“片段尾部补几秒”能够解决的，而是边界决策本身需要分层处理：

1. LLM 更适合判断“这一段讲的还是不是同一个知识点”
2. ASR 更适合判断“这一句话在什么时间点自然开始/结束”

把两件事混在一个阶段内，会产生以下副作用：

- 过度依赖 LLM 时间戳，导致句子完整性不稳定
- 仅靠尾部延长，无法修正开头半句
- 相邻片段独立修正，导致这一段结尾修好了，下一段开头仍然是半句
- 为了贴近语义切换点而硬裁切，播放时用户感知明显生硬

本设计的核心思想是：

- 让 LLM 输出“语义意图”而不是“完美播放边界”
- 让局部 ASR 在一个受控小窗口内完成最终边界吸附
- 让相邻片段协同决策重叠，而不是各修各的

## Goals

- 降低 hierarchical 分段结果中的开头半句率和结尾半句率
- 保持或提升知识点边界与 `content_summary` 的一致性
- 在相邻分段之间引入可控的 1 到 3 秒重叠，用于承接转折句
- 保持现有向量化、embedding 和落库主流程不被大范围改写
- 为后续调参与排障提供可解释的日志与指标

## Non-Goals

- 不重写 coarse segmentation 阶段
- 不引入复杂声学模型或额外音频级别的停顿检测服务
- 不修改当前数据库表结构作为本阶段前提
- 不让前端播放器承担新的纠偏逻辑
- 不追求“所有边界都完全等于真实人工剪辑结果”

## Existing Context

当前代码和最近改动中，已存在以下基础：

- `internal/worker/vectorworker/tasks/hierarchical.go`
  - 负责构造 hierarchical prompt、retry prompt，以及 LLM 分段结果规范化
- `internal/worker/vectorworker/task.go`
  - 负责 hierarchical 主流程、uniform segmentation 检测、LLM retry 和分段落库
- `internal/worker/vectorworker/tasks/asr.go`
  - 负责 refine 阶段 ASR、embedding 和当前尾部延长逻辑接入
- `internal/worker/vectorworker/tasks/tail_alignment.go`
  - 负责当前的尾部句尾判定与逐步延长策略

已有提交 `6ade1ae` 已经证明“仅做尾部轻量延长”能缓解一部分结尾硬截断问题，但对于开头半句和知识点边界偏移仍然不够。

## Proposed Architecture

新的 hierarchical refine 链路拆成三个明确阶段：

1. `LLM Semantic Segmentation`
2. `Boundary Alignment`
3. `Embedding and Persistence`

### Stage 1: LLM Semantic Segmentation

输入保持不变：coarse ASR 文本。

输出保持兼容现有 `segments` 结构，但语义职责调整为：

- 优先识别知识点、定义、步骤、总结、举例切换等语义边界
- 不要求 `start_time/end_time` 已经是最终播放边界
- 允许边界附近保留 1 到 3 秒可校正空间

这意味着 LLM 不再被要求“既判知识点又精确卡到句尾”。它只需要给出合理的语义切分意图，以及供后处理定位的边界锚点。

### Stage 2: Boundary Alignment

这是本次设计的核心新增层。它基于每个 segment 附近的局部 ASR 文本，在一个受控小窗口内重新评估边界。

边界校正器同时负责：

- 判断当前 `start_time` 是否落在承接上句的中间
- 判断当前 `end_time` 是否落在未结束的半句中
- 判断相邻分段之间是应该同步后移、还是采用少量重叠承接转折句

边界校正器的输出是新的 `start_time/end_time`。只有这组校正后的边界才会进入后续 embedding 和最终落库。

### Stage 3: Embedding and Persistence

refine 阶段生成的文本输入、embedding 和最终 `end_time/start_time` 持久化都以校正结果为准。

这样推荐、播放、相似搜索读取到的边界是统一的、稳定的，不会出现“embedding 用的是一个时间段，播放用的是另一个时间段”的分裂状态。

## Boundary Alignment Design

### Window Strategy

对每个 LLM segment，以原始边界为中心构造局部探测窗口：

- 起点窗口：`start_time - 3s` 到 `start_time + 2s`
- 终点窗口：`end_time - 2s` 到 `end_time + 4s`

窗口需要满足两个约束：

- 足够小，避免成本失控
- 足够大，允许把明显半句修回自然边界

首版不追求自适应窗口，先使用固定窗口，便于验证和调参。

### Candidate Generation

边界候选不以“纯秒级枚举”为主，而是优先基于以下线索构造：

- ASR 文本中的句子边界
- 明显停顿点或短句切换点
- LLM 提供的锚点文本附近位置
- 原始 `start_time/end_time` 自身

这样可以减少无意义候选，并让评分结果更可解释。

### Start Boundary Heuristics

好的片段起点应满足：

- 尽量不是上一句话的中间片段
- 从该点开始后，后续几秒能形成完整主句或完整陈述
- 不以明显承接词、连接词、语气残句作为句首
- 若无法完全避免承接，可接受从上一句尾部前 1 到 2 秒开始，形成轻量重叠

首版规则建议包含：

- 命中明显连接词开头时减分，例如“所以”“那么”“然后”“接下来我们看”
- 命中更像定义句、步骤句、讲解起句的表达时加分
- 过度偏离 LLM 原始起点时小幅减分，避免语义边界漂移过大

### End Boundary Heuristics

好的片段终点应满足：

- 尽量落在完整句尾
- 定义、步骤、举例或总结已经自然收束
- 若语义切换发生在句中，优先让当前段把该句说完
- 允许下一段通过 1 到 3 秒重叠承接转折

首版规则建议包含：

- 命中标点或结束短语加分
- 命中明显残句、连接词或悬空收尾减分
- 若终点明显切断一个仍在继续的解释句，重罚

### Neighbor Coordination

边界校正不得“每段独立决策”。相邻分段必须联动处理。

典型规则如下：

- 若当前段结尾向后延了 2 秒，则下一段起点不能机械保持原值
- 若下一段调整到自然起句点后仍会损失语义衔接，则允许保留 1 到 3 秒重叠
- 若重叠超过上限，则优先减少重叠，必要时接受次优句子边界

这部分的目标不是让两个 segment 完全对称，而是保证“上一段结尾自然、下一段点开也能直接听懂”。

### Scoring Model

首版使用可解释规则评分，不引入新的模型判分。

建议分值来源包括：

- 句尾标点或结束短语命中加分
- 起点命中完整句首、定义句首、步骤句首加分
- 起点落在连接词或残句中减分
- 终点看起来像半句时减分
- 重叠超过 3 秒重罚
- 偏离 LLM 原始边界过大时小幅减分
- 命中 LLM 锚点文本时加分

候选组合最终按总分排序，选择最佳 `start/end` 结果。

## LLM Output Contract

为了让边界校正器具备更稳的语义参考，建议在现有 segment schema 上增加增强字段，但保持兼容和可降级。

### Proposed Optional Fields

- `boundary_reason`
- `start_anchor_text`
- `end_anchor_text`
- `boundary_confidence`

字段语义如下：

- `boundary_reason`
  - 用一句话说明为什么在这里切分，要求引用输入 ASR 的关键词或短句
- `start_anchor_text`
  - 该 segment 开头附近应听到的短语，要求尽量短、可定位
- `end_anchor_text`
  - 该 segment 结尾附近应听到的短语，要求尽量短、可定位
- `boundary_confidence`
  - LLM 对边界判断的主观置信度，取值建议为 `high`、`medium`、`low`

### Contract Principles

- 这些字段是“辅助校正字段”，不是最终对外业务字段
- 即使字段缺失，也不能导致链路失败
- schema 解析需要忽略未知字段，保持对已有模型输出兼容

### Prompt Requirements

prompt 需要明确约束 LLM：

- 按知识点、步骤、定义、举例、总结等语义转折切分
- 禁止等距分段
- 若语义切换点落在句中，先表达语义切换意图，最终句子收尾交给后处理
- `start_anchor_text` 和 `end_anchor_text` 必须短、具体、可在 ASR 中定位
- `boundary_reason` 不能写成空泛描述，如“自然过渡”或“内容变化”

## Failure Handling and Safe Degradation

### Missing LLM Auxiliary Fields

若 LLM 未输出 `boundary_reason`、`start_anchor_text`、`end_anchor_text`，或内容明显无效：

- 不报错
- 直接回退到“原始边界 + ASR 窗口规则打分”模式

### Local ASR Alignment Failure

若局部窗口内 ASR 无法稳定定位，或所有候选评分都很差：

- 不做激进修正
- 回退到当前已有的轻量尾部补偿策略
- 再不行则保留 LLM 原始边界

### Neighbor Conflict

若当前段延后会导致下一段时长过短、重叠超过上限，或校正后出现非法边界：

- 优先保证 segment 仍然合法可用
- 再尽量避免硬截断
- 最后才追求最优重叠效果

明确优先级为：

1. 语义分段合法可用
2. 无明显硬截断
3. 重叠不超过上限
4. 尽量贴近原始 LLM 边界

### Cost Guardrails

为控制成本，首版加以下硬限制：

- 每个 segment 最多进行一次局部边界校正流程
- 边界探测窗口固定，不做无限扩张
- 相邻片段总重叠不超过 3 秒
- 不新增独立全量 ASR 流程，只复用 refine 阶段已有能力和局部探测

## Data Flow

推荐的数据流如下：

1. coarse segmentation 生成 coarse ASR 文本
2. LLM 输出结构化语义 segments 和辅助边界字段
3. `NormalizeLLMSegments` 对时间范围、顺序、重叠、最小时长做基础规范化
4. 对每个 segment 执行 `Boundary Alignment`
5. 相邻分段协同修正 `start_time/end_time`
6. 生成最终 refine 文本输入
7. 计算 embedding
8. 落库校正后的边界和 embedding

## Components

建议新增或调整的职责边界如下：

### 1. Prompt Builder Updates

位置：`internal/worker/vectorworker/tasks/hierarchical.go`

职责：

- 为主 prompt 和 retry prompt 增加辅助边界字段要求
- 明确“语义切分”和“句子收尾后处理”的分工

### 2. Alignment Contract Parsing

位置：仍在 `tasks` 范围内，与 LLM segment normalization 紧邻

职责：

- 解析可选增强字段
- 对无效或缺失字段安全忽略

### 3. Boundary Alignment Engine

位置：建议在 `internal/worker/vectorworker/tasks/` 下新增独立模块

职责：

- 构造起止点窗口
- 生成候选边界
- 对候选打分
- 联动协调相邻 segment 的边界与重叠

### 4. Refine Pipeline Integration

位置：`internal/worker/vectorworker/tasks/asr.go`

职责：

- 用新的边界校正器替换“仅尾部延长”的单点逻辑
- 保留必要降级路径
- 将最终校正后的边界用于 embedding 和状态更新

## Logging and Observability

需要新增结构化日志，至少覆盖：

- 原始 LLM 边界与校正后边界
- 起点/终点分别偏移了多少秒
- 是否命中锚点文本
- 是否触发重叠，以及重叠秒数
- 候选评分摘要
- 是否走了降级分支，以及原因

目标是让线上排障时能快速判断问题属于：

- LLM 语义边界给错
- ASR 锚点定位失败
- 规则权重不合理
- 成本保护过于保守

## Verification Strategy

### Automated Checks

新增或扩展单元测试，覆盖：

- 开头半句识别
- 结尾半句识别
- 相邻段协同调整
- 重叠上限限制
- LLM 辅助字段缺失时的降级
- 校正失败时回退到现有路径
- 校正后不产生非法时间段

### Offline Metrics

建议增加统计指标：

- 起点疑似半句率
- 终点疑似半句率
- 平均边界偏移秒数
- 平均相邻重叠秒数
- 锚点命中率
- 边界校正触发率
- 边界校正成功率

目标不是单纯提高校正触发率，而是降低半句率，并把偏移和重叠维持在可控范围内。

### Human Evaluation

抽样试听重点覆盖：

- 主题切换处
- 长解释句
- 定义讲解段
- 步骤切换段

重点观察：

- 片段点开是否能直接听懂
- 片段结尾是否自然收束
- 知识点是否仍然集中且不跑偏

## Rollout Guidance

建议分阶段上线：

1. 先在日志中记录候选边界和推荐边界，但不实际改写结果
2. 验证规则评分和人工试听结果
3. 再开启真实边界替换
4. 保留开关，允许快速回退到现有 tail alignment 方案

这样可以先验证规则是否选出了“人耳认可”的边界，再承担线上结果变更风险。

## Final Recommendation

推荐采用“语义定界 + ASR 对齐双阶段”方案，并引入一个可插拔的 `Boundary Alignment` 层。

原因如下：

- 只增强 LLM，无法稳定解决开头半句和结尾半句
- 只增强后处理，无法稳定修复知识点边界偏差
- 双阶段职责清晰，便于调参、灰度和排障
- 允许小幅成本上升的前提下，这是质量和风险最平衡的路线

本设计的成功标准是：

- 推荐片段的开头和结尾明显更自然
- 知识点摘要与播放内容的对应关系更稳
- 相邻片段重叠受控，不影响前端播放和推荐体验
- 任一增强环节失败时，结果不比当前链路更差
