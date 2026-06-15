# LLM 规整分段自动重试 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 当 LLM 给出的结构化分段出现“过于等距/规整”（例如大量 120s 一段）时，自动触发一次更强约束的二次 LLM 分段，以提升 `start_time/end_time` 与 `content_summary` 的一致性。

**Architecture:** 在 hierarchical 流程中：LLM 第一次输出 → 解析校验 → 计算“规整度”指标 → 若命中阈值则构造 retry prompt 再调用一次 LLM → 用第二次结果替换并继续落库/二次 ASR+embedding。

**Tech Stack:** Go + DashScope(OpenAI compatible Chat) + json 解析 + 现有 normalizeLLMSegments/Prompt 体系

---

## 规整度判定规则（可配置为常量，先写死即可）

对 `segments` 计算每段长度 `len_i = end_i - start_i`，过滤掉长度 < 5s 的异常段后：

- **规则 A（主判定）：** 若“众数长度”占比 >= 0.6 且 分段数 >= 3，则判定为规整分段。  
  - 允许容差：把长度按 5s 或 10s 做桶（比如 118~122 视为同一桶）。
- **规则 B（辅助判定）：** 若 `max(len_i) - min(len_i) <= 15s` 且 分段数 >= 3，则判定为规整分段。

命中任一规则即触发二次 LLM。

---

## Prompt 设计（第二次更强约束）

在原 prompt 基础上新增强约束：
- 必须依据“主题转折点/关键句变化”切分
- 禁止等距切分，明确禁止“固定每 N 秒一段”
- 要求给出每段的 **boundary_reason**（一句话），来自输入 ASR 的关键句或关键词（仅用于 LLM 自检，不落库也可）

输出 schema（第二次）：
```json
{
  "segments": [
    {
      "segment_index": 0,
      "start_time": 0,
      "end_time": 75,
      "content_summary": "...",
      "knowledge_tags": ["..."],
      "boundary_reason": "从“xxx”转到“yyy”"
    }
  ]
}
```

实现上解析时忽略 `boundary_reason`（不影响现有表结构）。

---

## Task 1：实现“规整度检测”

**Files:**
- Modify: `c:\Users\xiaoy\Desktop\nlp-video-project\nlp-video-project\cmd\vector_worker\task.go`

- [ ] Step 1: 增加函数 `isUniformSegments(segs []llmSegment) bool`
  - 统计每段时长（桶化，桶宽建议 10s）
  - 计算众数占比、max-min
  - 返回是否命中规整分段

- [ ] Step 2: 增加日志
  - 打印：segments_count、mode_bucket、mode_ratio、min/max

---

## Task 2：实现二次 LLM 重分段

**Files:**
- Modify: `...\cmd\vector_worker\task.go`

- [ ] Step 1: 抽出一个生成 retry prompt 的函数
  - `buildHierarchicalSegmentationRetryPrompt(...) string`
  - 复用 coarseItems 输入，但新增“必须给出 boundary_reason”的 schema

- [ ] Step 2: 在 hierarchical 流程中接入
  - 第一次 LLM → normalizeLLMSegments → 若 `isUniformSegments==true`：
    - 记录日志：触发重试
    - 第二次 LLM 调用（同 `LLMTimeoutMinutes`）
    - 第二次输出 normalizeLLMSegments
    - 若第二次失败：回退使用第一次结果（但记录 warning）
    - 若第二次成功：用第二次结果继续后续落库/二次 ASR+embedding

---

## Task 3：验证

- [ ] Step 1: 编译检查
```bash
go build ./...
```
Expected: build success

- [ ] Step 2: 运行观察日志
  - 当分段过于规整时，日志应出现 “uniform segmentation detected, retrying LLM”
  - 重试后 segments 的时长分布应更不均匀（更贴近内容边界）

