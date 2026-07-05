# LLM Boundary Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `embedding-video` 的 hierarchical 分段链路中新增一个可降级的 `Boundary Alignment` 层，同时提升 LLM 语义边界表达能力和 ASR 边界对齐能力，降低开头半句、结尾半句和知识点硬裁切问题。

**Architecture:** 保持现有 coarse ASR -> LLM segmentation -> refine ASR -> embedding 的主链路不变，在 `internal/worker/vectorworker/tasks/` 下新增独立的边界对齐模块。LLM prompt 和解析层只补充辅助边界字段；真正的边界吸附和相邻片段重叠协调都在 refine 阶段完成，并且任何增强失败都能安全回退到当前 tail alignment 行为。

**Tech Stack:** Go 1.26、标准库 `encoding/json`、现有 vector worker、Zap、GORM、现有 `go test` 测试链路。

---

## 文件结构与职责

- `embedding-video/internal/worker/vectorworker/tasks/db.go`
  - 扩展 `LLMSegment`，增加可选辅助边界字段，保持现有入库字段兼容。
- `embedding-video/internal/worker/vectorworker/tasks/hierarchical.go`
  - 更新主 prompt / retry prompt，对 LLM 提出 `boundary_reason`、`start_anchor_text`、`end_anchor_text`、`boundary_confidence` 要求。
- `embedding-video/internal/worker/vectorworker/tasks/hierarchical_test.go`
  - 新增 prompt 和 JSON normalization 的测试，约束新增字段和兼容解析行为。
- `embedding-video/internal/worker/vectorworker/tasks/tail_alignment.go`
  - 保留现有尾部规则，抽出共享句首/句尾判断辅助函数，供新边界对齐器复用。
- `embedding-video/internal/worker/vectorworker/tasks/boundary_alignment.go`
  - 新增边界对齐引擎：窗口计算、候选打分、起止点校正、相邻片段重叠限制。
- `embedding-video/internal/worker/vectorworker/tasks/boundary_alignment_test.go`
  - 新增规则级测试，覆盖开头半句、结尾半句、重叠限制、锚点命中和降级路径。
- `embedding-video/internal/worker/vectorworker/tasks/asr.go`
  - 在 refine 阶段接入新的边界对齐器，保留失败时回退到 `alignSegmentTail` 的路径。
- `embedding-video/internal/worker/vectorworker/tasks/asr_boundary_alignment_test.go`
  - 新增 refine 集成测试，约束校正结果和降级行为。
- `embedding-video/docs/superpowers/specs/2026-05-11-llm-boundary-alignment-design.md`
  - 仅当实现现实与 spec 不一致时修正文档，否则不改。

---

### Task 1: 扩展 LLM segment 契约并补齐 prompt 约束

**Files:**
- Modify: `embedding-video/internal/worker/vectorworker/tasks/db.go`
- Modify: `embedding-video/internal/worker/vectorworker/tasks/hierarchical.go`
- Create: `embedding-video/internal/worker/vectorworker/tasks/hierarchical_test.go`

- [ ] **Step 1: 先写 JSON normalization 兼容测试**

创建 `embedding-video/internal/worker/vectorworker/tasks/hierarchical_test.go`，先锁定“新增字段可解析、缺失字段不报错、未知字段被忽略”的行为：

```go
package tasks

import "testing"

func TestNormalizeLLMSegmentsKeepsOptionalBoundaryFields(t *testing.T) {
	llmOut := `{
	  "segments": [
	    {
	      "segment_index": 0,
	      "start_time": 0,
	      "end_time": 42,
	      "content_summary": "定义概念",
	      "knowledge_tags": ["定义"],
	      "boundary_reason": "从题目背景进入定义讲解",
	      "start_anchor_text": "下面先看定义",
	      "end_anchor_text": "这就是它的定义",
	      "boundary_confidence": "high",
	      "ignored_field": "ignored"
	    }
	  ]
	}`

	segs, err := NormalizeLLMSegments(llmOut, 120, 20, 180)
	if err != nil {
		t.Fatalf("NormalizeLLMSegments error = %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("len(segs) = %d, want 1", len(segs))
	}
	if segs[0].BoundaryReason != "从题目背景进入定义讲解" {
		t.Fatalf("BoundaryReason = %q", segs[0].BoundaryReason)
	}
	if segs[0].StartAnchorText != "下面先看定义" {
		t.Fatalf("StartAnchorText = %q", segs[0].StartAnchorText)
	}
	if segs[0].EndAnchorText != "这就是它的定义" {
		t.Fatalf("EndAnchorText = %q", segs[0].EndAnchorText)
	}
	if segs[0].BoundaryConfidence != "high" {
		t.Fatalf("BoundaryConfidence = %q", segs[0].BoundaryConfidence)
	}
}

func TestNormalizeLLMSegmentsAllowsMissingOptionalBoundaryFields(t *testing.T) {
	llmOut := `{
	  "segments": [
	    {
	      "segment_index": 0,
	      "start_time": 0,
	      "end_time": 42,
	      "content_summary": "定义概念",
	      "knowledge_tags": ["定义"]
	    }
	  ]
	}`

	segs, err := NormalizeLLMSegments(llmOut, 120, 20, 180)
	if err != nil {
		t.Fatalf("NormalizeLLMSegments error = %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("len(segs) = %d, want 1", len(segs))
	}
	if segs[0].BoundaryReason != "" || segs[0].StartAnchorText != "" || segs[0].EndAnchorText != "" || segs[0].BoundaryConfidence != "" {
		t.Fatalf("optional fields should stay empty: %+v", segs[0])
	}
}
```

- [ ] **Step 2: 运行测试，确认当前实现失败**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -run "TestNormalizeLLMSegments(KeepsOptionalBoundaryFields|AllowsMissingOptionalBoundaryFields)" -v`

Expected:

```text
FAIL
... segs[0].BoundaryReason undefined ...
```

- [ ] **Step 3: 扩展 `LLMSegment` 结构，增加可选辅助字段**

修改 `embedding-video/internal/worker/vectorworker/tasks/db.go`：

```go
type LLMSegment struct {
	SegmentIndex       int      `json:"segment_index"`
	StartTimeSec       int      `json:"start_time"`
	EndTimeSec         int      `json:"end_time"`
	ContentSummary     string   `json:"content_summary"`
	KnowledgeTags      []string `json:"knowledge_tags"`
	BoundaryReason     string   `json:"boundary_reason"`
	StartAnchorText    string   `json:"start_anchor_text"`
	EndAnchorText      string   `json:"end_anchor_text"`
	BoundaryConfidence string   `json:"boundary_confidence"`
}
```

要求：

1. 不修改现有 SQL upsert 字段。
2. 新字段只存在于内存契约和 prompt/align 阶段。

- [ ] **Step 4: 在 normalization 阶段清洗新增字段**

修改 `embedding-video/internal/worker/vectorworker/tasks/hierarchical.go` 的 `NormalizeLLMSegments` 循环，补充字符串 trim 和置信度规范化：

```go
	s.BoundaryReason = strings.TrimSpace(s.BoundaryReason)
	s.StartAnchorText = strings.TrimSpace(s.StartAnchorText)
	s.EndAnchorText = strings.TrimSpace(s.EndAnchorText)
	s.BoundaryConfidence = strings.ToLower(strings.TrimSpace(s.BoundaryConfidence))
	switch s.BoundaryConfidence {
	case "high", "medium", "low", "":
	default:
		s.BoundaryConfidence = ""
	}
```

- [ ] **Step 5: 先写 prompt 约束测试**

在同一个 `hierarchical_test.go` 里追加：

```go
func TestBuildHierarchicalSegmentationPromptMentionsBoundaryFields(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "下面先看定义，这就是它的定义。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationPrompt error = %v", err)
	}
	for _, needle := range []string{"boundary_reason", "start_anchor_text", "end_anchor_text", "boundary_confidence"} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q", needle)
		}
	}
}
```

需要补 import：

```go
import (
	"strings"
	"testing"
)
```

- [ ] **Step 6: 运行测试，确认 prompt 测试先失败**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -run "Test(BuildHierarchicalSegmentationPromptMentionsBoundaryFields|NormalizeLLMSegments)" -v`

Expected:

```text
FAIL
... prompt missing "start_anchor_text" ...
```

- [ ] **Step 7: 更新主 prompt 和 retry prompt 的 schema 与规则说明**

修改 `embedding-video/internal/worker/vectorworker/tasks/hierarchical.go`，在 `BuildHierarchicalSegmentationPrompt` 和 `BuildHierarchicalSegmentationRetryPrompt` 中新增这些约束文本：

```go
	b.WriteString("- 如果主题切换点出现在一句话中间，先给出语义边界意图，最终句子收尾由后处理完成\n")
	b.WriteString("- 每个分段必须给出 boundary_reason，说明为什么从这里开始/上一段为什么在这里结束，并引用 ASR 关键词或短句\n")
	b.WriteString("- 每个分段必须给出 start_anchor_text 和 end_anchor_text，要求短、具体、可在 ASR 中定位\n")
	b.WriteString("- 每个分段必须给出 boundary_confidence，取值为 high、medium、low\n")
```

并把 JSON schema 扩展为：

```go
	b.WriteString("      \"knowledge_tags\": [\"tag1\", \"tag2\"],\n")
	b.WriteString("      \"boundary_reason\": \"上一句在这里完整收束，后面开始进入新的知识点\",\n")
	b.WriteString("      \"start_anchor_text\": \"下面先看定义\",\n")
	b.WriteString("      \"end_anchor_text\": \"这就是它的定义\",\n")
	b.WriteString("      \"boundary_confidence\": \"high\"\n")
```

- [ ] **Step 8: 运行测试，确认契约与 prompt 通过**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -run "Test(BuildHierarchicalSegmentationPromptMentionsBoundaryFields|NormalizeLLMSegments)" -v`

Expected:

```text
PASS
```

- [ ] **Step 9: Commit**

```bash
git add embedding-video/internal/worker/vectorworker/tasks/db.go embedding-video/internal/worker/vectorworker/tasks/hierarchical.go embedding-video/internal/worker/vectorworker/tasks/hierarchical_test.go
git commit -m "feat(vector-worker): extend llm boundary contract"
```

### Task 2: 提取可复用的句首/句尾规则并为边界对齐器打基础

**Files:**
- Modify: `embedding-video/internal/worker/vectorworker/tasks/tail_alignment.go`
- Modify: `embedding-video/internal/worker/vectorworker/tasks/tail_alignment_test.go`

- [ ] **Step 1: 先写句首/句尾规则测试**

在 `embedding-video/internal/worker/vectorworker/tasks/tail_alignment_test.go` 追加：

```go
func TestLooksLikeSentenceStart(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "definition lead", text: "下面先看定义", want: true},
		{name: "step lead", text: "第一步我们先列式", want: true},
		{name: "connector fragment", text: "所以", want: false},
		{name: "carry over fragment", text: "然后再", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LooksLikeSentenceStart(tt.text); got != tt.want {
				t.Fatalf("LooksLikeSentenceStart(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestNormalizeBoundaryConfidence(t *testing.T) {
	if got := NormalizeBoundaryConfidence(" HIGH "); got != "high" {
		t.Fatalf("NormalizeBoundaryConfidence = %q", got)
	}
	if got := NormalizeBoundaryConfidence("maybe"); got != "" {
		t.Fatalf("NormalizeBoundaryConfidence invalid = %q", got)
	}
}
```

- [ ] **Step 2: 运行测试，确认新函数还不存在**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -run "Test(LooksLikeSentenceStart|NormalizeBoundaryConfidence)" -v`

Expected:

```text
FAIL
... undefined: LooksLikeSentenceStart
```

- [ ] **Step 3: 在 `tail_alignment.go` 中提取共享辅助函数**

将 `tail_alignment.go` 改造成“边界规则公共层”，补充这些内容：

```go
var sentenceStartPhrases = []string{
	"下面先看",
	"我们先看",
	"第一步",
	"接下来我们看",
	"先来看",
}

func LooksLikeSentenceStart(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, phrase := range trailingConnectors {
		if text == phrase {
			return false
		}
	}
	for _, phrase := range sentenceStartPhrases {
		if strings.HasPrefix(text, phrase) {
			return true
		}
	}
	if strings.HasPrefix(text, "因为") || strings.HasPrefix(text, "所以") || strings.HasPrefix(text, "然后") {
		return false
	}
	return len([]rune(text)) >= 4
}

func NormalizeBoundaryConfidence(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "high", "medium", "low":
		return s
	default:
		return ""
	}
}
```

要求：

1. 不改 `LooksLikeSentenceEnd` 现有行为。
2. 不删除 `NeedsTailExtension` 和 `NextAlignedEndSec`。

- [ ] **Step 4: 让 `NormalizeLLMSegments` 复用新的置信度归一化函数**

把 Task 1 中 `hierarchical.go` 的这段：

```go
	s.BoundaryConfidence = strings.ToLower(strings.TrimSpace(s.BoundaryConfidence))
	switch s.BoundaryConfidence {
	case "high", "medium", "low", "":
	default:
		s.BoundaryConfidence = ""
	}
```

替换为：

```go
	s.BoundaryConfidence = NormalizeBoundaryConfidence(s.BoundaryConfidence)
```

- [ ] **Step 5: 运行测试，确认规则层通过且旧测试不回归**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -run "Test(NormalizeTailAlignmentConfigDefaults|LooksLikeSentenceEnd|NeedsTailExtension|NextAlignedEndSec|LooksLikeSentenceStart|NormalizeBoundaryConfidence)" -v`

Expected:

```text
PASS
```

- [ ] **Step 6: Commit**

```bash
git add embedding-video/internal/worker/vectorworker/tasks/tail_alignment.go embedding-video/internal/worker/vectorworker/tasks/tail_alignment_test.go embedding-video/internal/worker/vectorworker/tasks/hierarchical.go
git commit -m "refactor(vector-worker): share boundary rule helpers"
```

### Task 3: 新增 `Boundary Alignment` 引擎和规则测试

**Files:**
- Create: `embedding-video/internal/worker/vectorworker/tasks/boundary_alignment.go`
- Create: `embedding-video/internal/worker/vectorworker/tasks/boundary_alignment_test.go`

- [ ] **Step 1: 先写规则级测试，约束窗口、重叠和锚点行为**

创建 `embedding-video/internal/worker/vectorworker/tasks/boundary_alignment_test.go`：

```go
package tasks

import "testing"

func TestBuildBoundaryWindows(t *testing.T) {
	startMin, startMax, endMin, endMax := buildBoundaryWindows(30, 70, 120)
	if startMin != 27 || startMax != 32 || endMin != 68 || endMax != 74 {
		t.Fatalf("unexpected windows: %d %d %d %d", startMin, startMax, endMin, endMax)
	}
}

func TestAlignSegmentBoundariesPrefersNaturalSentenceAndCapsOverlap(t *testing.T) {
	current := LLMSegment{SegmentIndex: 0, StartTimeSec: 30, EndTimeSec: 70, StartAnchorText: "下面先看定义", EndAnchorText: "这就是它的定义", BoundaryConfidence: "high"}
	next := &LLMSegment{SegmentIndex: 1, StartTimeSec: 70, EndTimeSec: 100, StartAnchorText: "接下来我们看例题"}

	aligned := alignSegmentBoundaries(current, next, boundaryAlignmentSnapshot{
		StartCandidates: []boundaryCandidate{{Sec: 29, Text: "所以", Score: -2}, {Sec: 30, Text: "下面先看定义", Score: 5}},
		EndCandidates:   []boundaryCandidate{{Sec: 71, Text: "这就是它的定义", Score: 4}, {Sec: 74, Text: "这就是它的定义。", Score: 8}},
	})

	if aligned.StartTimeSec != 30 {
		t.Fatalf("StartTimeSec = %d, want 30", aligned.StartTimeSec)
	}
	if aligned.EndTimeSec != 74 {
		t.Fatalf("EndTimeSec = %d, want 74", aligned.EndTimeSec)
	}
	if next.StartTimeSec != 71 {
		t.Fatalf("next.StartTimeSec = %d, want 71 to cap overlap at 3s", next.StartTimeSec)
	}
}

func TestScoreBoundaryCandidateUsesAnchorAndConfidence(t *testing.T) {
	seg := LLMSegment{StartAnchorText: "下面先看定义", BoundaryConfidence: "high"}
	score := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 30, Text: "下面先看定义"}, true)
	if score <= 0 {
		t.Fatalf("score = %d, want positive anchor score", score)
	}
}
```

- [ ] **Step 2: 运行测试，确认新文件函数全部缺失**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -run "Test(BuildBoundaryWindows|AlignSegmentBoundariesPrefersNaturalSentenceAndCapsOverlap|ScoreBoundaryCandidateUsesAnchorAndConfidence)" -v`

Expected:

```text
FAIL
... undefined: buildBoundaryWindows
```

- [ ] **Step 3: 新增 `boundary_alignment.go` 的最小结构**

创建 `embedding-video/internal/worker/vectorworker/tasks/boundary_alignment.go`：

```go
package tasks

import "strings"

const (
	boundaryStartLookBackSec  = 3
	boundaryStartLookAheadSec = 2
	boundaryEndLookBackSec    = 2
	boundaryEndLookAheadSec   = 4
	maxRecommendedOverlapSec  = 3
)

type boundaryCandidate struct {
	Sec   int
	Text  string
	Score int
}

type boundaryAlignmentSnapshot struct {
	StartCandidates []boundaryCandidate
	EndCandidates   []boundaryCandidate
}

func buildBoundaryWindows(startSec int, endSec int, durationSec int) (int, int, int, int) {
	startMin := startSec - boundaryStartLookBackSec
	if startMin < 0 {
		startMin = 0
	}
	startMax := startSec + boundaryStartLookAheadSec
	if durationSec > 0 && startMax > durationSec {
		startMax = durationSec
	}
	endMin := endSec - boundaryEndLookBackSec
	if endMin < 0 {
		endMin = 0
	}
	endMax := endSec + boundaryEndLookAheadSec
	if durationSec > 0 && endMax > durationSec {
		endMax = durationSec
	}
	return startMin, startMax, endMin, endMax
}

func scoreBoundaryCandidate(seg LLMSegment, candidate boundaryCandidate, isStart bool) int {
	score := candidate.Score
	text := strings.TrimSpace(candidate.Text)
	if isStart {
		if LooksLikeSentenceStart(text) {
			score += 3
		}
		if seg.StartAnchorText != "" && strings.Contains(text, seg.StartAnchorText) {
			score += 4
		}
	} else {
		if LooksLikeSentenceEnd(text) {
			score += 3
		}
		if seg.EndAnchorText != "" && strings.Contains(text, seg.EndAnchorText) {
			score += 4
		}
	}
	if seg.BoundaryConfidence == "high" {
		score += 1
	}
	return score
}

func alignSegmentBoundaries(current LLMSegment, next *LLMSegment, snap boundaryAlignmentSnapshot) LLMSegment {
	bestStart := current.StartTimeSec
	bestStartScore := -1 << 30
	for _, c := range snap.StartCandidates {
		score := scoreBoundaryCandidate(current, c, true)
		if score > bestStartScore {
			bestStartScore = score
			bestStart = c.Sec
		}
	}
	bestEnd := current.EndTimeSec
	bestEndScore := -1 << 30
	for _, c := range snap.EndCandidates {
		score := scoreBoundaryCandidate(current, c, false)
		if score > bestEndScore {
			bestEndScore = score
			bestEnd = c.Sec
		}
	}
	current.StartTimeSec = bestStart
	current.EndTimeSec = bestEnd
	if next != nil && current.EndTimeSec-next.StartTimeSec > maxRecommendedOverlapSec {
		next.StartTimeSec = current.EndTimeSec - maxRecommendedOverlapSec
	}
	if current.EndTimeSec <= current.StartTimeSec {
		current.EndTimeSec = current.StartTimeSec + 1
	}
	return current
}
```

- [ ] **Step 4: 扩充测试，覆盖开头半句和无锚点降级**

在 `boundary_alignment_test.go` 追加：

```go
func TestScoreBoundaryCandidatePenalizesSentenceFragmentStart(t *testing.T) {
	seg := LLMSegment{}
	fragment := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 10, Text: "所以"}, true)
	natural := scoreBoundaryCandidate(seg, boundaryCandidate{Sec: 10, Text: "下面先看定义"}, true)
	if fragment >= natural {
		t.Fatalf("fragment score = %d, natural score = %d", fragment, natural)
	}
}

func TestAlignSegmentBoundariesWithoutAnchorKeepsLegalRange(t *testing.T) {
	current := LLMSegment{SegmentIndex: 0, StartTimeSec: 10, EndTimeSec: 20}
	aligned := alignSegmentBoundaries(current, nil, boundaryAlignmentSnapshot{
		StartCandidates: []boundaryCandidate{{Sec: 10, Text: "因为", Score: 0}},
		EndCandidates:   []boundaryCandidate{{Sec: 20, Text: "然后", Score: 0}},
	})
	if aligned.StartTimeSec < 0 || aligned.EndTimeSec <= aligned.StartTimeSec {
		t.Fatalf("invalid aligned segment: %+v", aligned)
	}
}
```

- [ ] **Step 5: 运行测试，确认引擎行为通过**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -run "Test(BuildBoundaryWindows|AlignSegmentBoundaries|ScoreBoundaryCandidate)" -v`

Expected:

```text
PASS
```

- [ ] **Step 6: Commit**

```bash
git add embedding-video/internal/worker/vectorworker/tasks/boundary_alignment.go embedding-video/internal/worker/vectorworker/tasks/boundary_alignment_test.go
git commit -m "feat(vector-worker): add boundary alignment engine"
```

### Task 4: 在 refine ASR 流程中接入边界对齐并保留安全降级

**Files:**
- Modify: `embedding-video/internal/worker/vectorworker/tasks/asr.go`
- Create: `embedding-video/internal/worker/vectorworker/tasks/asr_boundary_alignment_test.go`
- Keep: `embedding-video/internal/worker/vectorworker/tasks/asr_tail_alignment_test.go`

- [ ] **Step 1: 先写 refine 集成测试，约束“优先新对齐器，失败回退旧 tail alignment”**

创建 `embedding-video/internal/worker/vectorworker/tasks/asr_boundary_alignment_test.go`：

```go
package tasks

import (
	"context"
	"errors"
	"testing"
)

func TestAlignSegmentForRefinePrefersBoundaryAlignmentResult(t *testing.T) {
	seg := LLMSegment{SegmentIndex: 0, StartTimeSec: 30, EndTimeSec: 70, StartAnchorText: "下面先看定义", EndAnchorText: "这就是它的定义", BoundaryConfidence: "high"}

	start, end, text, err := alignSegmentForRefine(context.Background(), seg, nil, 120,
		func(startSec int, endSec int) (string, error) {
			if endSec == 74 {
				return "这就是它的定义。", nil
			}
			if startSec == 30 {
				return "下面先看定义", nil
			}
			return "", nil
		},
		func(context.Context(), TailAlignmentConfig, int, int, int, int, func(int) (string, error)) (int, string, error) {
			return 70, "fallback", nil
		},
	)
	if err != nil {
		t.Fatalf("alignSegmentForRefine error = %v", err)
	}
	if start != 30 || end != 74 {
		t.Fatalf("got start=%d end=%d", start, end)
	}
	if text == "fallback" {
		t.Fatal("should prefer boundary alignment result")
	}
}

func TestAlignSegmentForRefineFallsBackToTailAlignment(t *testing.T) {
	seg := LLMSegment{SegmentIndex: 0, StartTimeSec: 30, EndTimeSec: 70}
	_, end, text, err := alignSegmentForRefine(context.Background(), seg, nil, 120,
		func(int, int) (string, error) {
			return "", errors.New("probe failed")
		},
		func(context.Context(), TailAlignmentConfig, int, int, int, int, func(int) (string, error)) (int, string, error) {
			return 72, "fallback text。", nil
		},
	)
	if err != nil {
		t.Fatalf("alignSegmentForRefine error = %v", err)
	}
	if end != 72 || text != "fallback text。" {
		t.Fatalf("fallback result = %d %q", end, text)
	}
}
```

- [ ] **Step 2: 运行测试，确认集成函数尚不存在**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -run "TestAlignSegmentForRefine" -v`

Expected:

```text
FAIL
... undefined: alignSegmentForRefine
```

- [ ] **Step 3: 在 `asr.go` 中抽出 `alignSegmentForRefine`，先接入新对齐器，再回退旧 tail alignment**

在 `embedding-video/internal/worker/vectorworker/tasks/asr.go` 增加一个小型协调函数：

```go
func alignSegmentForRefine(
	ctx context.Context,
	seg LLMSegment,
	next *LLMSegment,
	videoDurationSec int,
	probeRange func(startSec int, endSec int) (string, error),
	fallbackTail func(context.Context, TailAlignmentConfig, int, int, int, int, func(int) (string, error)) (int, string, error),
) (int, int, string, error) {
	startMin, startMax, endMin, endMax := buildBoundaryWindows(seg.StartTimeSec, seg.EndTimeSec, videoDurationSec)
	startCandidates := make([]boundaryCandidate, 0, startMax-startMin+1)
	for sec := startMin; sec <= startMax; sec++ {
		text, err := probeRange(sec, sec)
		if err == nil {
			startCandidates = append(startCandidates, boundaryCandidate{Sec: sec, Text: text})
		}
	}
	endCandidates := make([]boundaryCandidate, 0, endMax-endMin+1)
	for sec := endMin; sec <= endMax; sec++ {
		text, err := probeRange(seg.StartTimeSec, sec)
		if err == nil {
			endCandidates = append(endCandidates, boundaryCandidate{Sec: sec, Text: text})
		}
	}
	if len(startCandidates) > 0 && len(endCandidates) > 0 {
		aligned := alignSegmentBoundaries(seg, next, boundaryAlignmentSnapshot{StartCandidates: startCandidates, EndCandidates: endCandidates})
		text, err := probeRange(aligned.StartTimeSec, aligned.EndTimeSec)
		if err == nil && strings.TrimSpace(text) != "" {
			return aligned.StartTimeSec, aligned.EndTimeSec, normalizeText(text), nil
		}
	}
	endSec, text, err := fallbackTail(ctx, NormalizeTailAlignmentConfig(TailAlignmentConfig{Enabled: true}), seg.StartTimeSec, seg.EndTimeSec, 0, videoDurationSec, func(endSec int) (string, error) {
		return probeRange(seg.StartTimeSec, endSec)
	})
	if err != nil {
		return seg.StartTimeSec, seg.EndTimeSec, "", err
	}
	return seg.StartTimeSec, endSec, normalizeText(text), nil
}
```

要求：

1. 先只抽成独立函数，不在 worker 主循环里大面积改写逻辑。
2. 失败时必须回退到当前 `alignSegmentTail`。

- [ ] **Step 4: 在 worker refine 主循环里替换原有单点调用**

将 `asr.go` 里这段：

```go
	alignedEndSec, text, err := alignSegmentTail(asrCtx, tailCfg, j.StartSec, j.EndSec, j.NextStartSec, videoDurationSec, probe)
```

改成：

```go
	seg := LLMSegment{
		SegmentIndex:       j.JobIndex,
		StartTimeSec:       j.StartSec,
		EndTimeSec:         j.EndSec,
		ContentSummary:     j.Summary,
		StartAnchorText:    j.StartAnchorText,
		EndAnchorText:      j.EndAnchorText,
		BoundaryConfidence: j.BoundaryConfidence,
	}
	var next *LLMSegment
	if j.NextStartSec > 0 {
		next = &LLMSegment{StartTimeSec: j.NextStartSec}
	}
	alignedStartSec, alignedEndSec, text, err := alignSegmentForRefine(asrCtx, seg, next, videoDurationSec, probeRange, alignSegmentTail)
```

同时把后续日志补全：

```go
	zap.Int("old_start_sec", j.StartSec),
	zap.Int("new_start_sec", alignedStartSec),
```

并确保最终 `Input` 与数据库更新逻辑使用新的 `alignedStartSec` / `alignedEndSec`。

- [ ] **Step 5: 扩展 refine job 结构，带上 LLM 辅助字段**

如果现有 `jobList` 或数据库读取结构体里没有这些字段，最小补充：

```go
type job struct {
	...
	StartAnchorText    string
	EndAnchorText      string
	BoundaryConfidence string
}
```

以及读取 segment 草稿时把 `content_summary` 继续作为 `Summary`，辅助字段从当前 `LLMSegment` 切片或临时 map 中传入；若暂时无法从数据库恢复，就在本次进程内通过 `segment_index -> LLMSegment` map 传递，不额外改表。

- [ ] **Step 6: 运行新旧边界测试**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -run "Test(AlignSegmentForRefine|AlignSegmentTail)" -v`

Expected:

```text
PASS
```

- [ ] **Step 7: Commit**

```bash
git add embedding-video/internal/worker/vectorworker/tasks/asr.go embedding-video/internal/worker/vectorworker/tasks/asr_boundary_alignment_test.go
git commit -m "feat(vector-worker): integrate boundary alignment into refine"
```

### Task 5: 补齐日志、回归测试并完成整包验证

**Files:**
- Modify: `embedding-video/internal/worker/vectorworker/tasks/asr.go`
- Modify: `embedding-video/internal/worker/vectorworker/tasks/boundary_alignment.go`
- Modify: `embedding-video/internal/worker/vectorworker/tasks/boundary_alignment_test.go`
- Modify: `embedding-video/docs/superpowers/specs/2026-05-11-llm-boundary-alignment-design.md` only if implementation reality diverges

- [ ] **Step 1: 给新对齐器补充结构化日志字段**

在 `asr.go` 成功和降级分支中增加这些字段：

```go
	zap.Int("old_start_sec", j.StartSec),
	zap.Int("new_start_sec", alignedStartSec),
	zap.Int("old_end_sec", j.EndSec),
	zap.Int("new_end_sec", alignedEndSec),
	zap.Bool("used_boundary_alignment", usedBoundaryAlignment),
	zap.Bool("used_tail_fallback", usedTailFallback),
	zap.String("start_anchor_text", seg.StartAnchorText),
	zap.String("end_anchor_text", seg.EndAnchorText),
	zap.String("boundary_confidence", seg.BoundaryConfidence),
```

降级日志至少应像：

```go
	zap.L().Warn("boundary_alignment_fallback_to_tail",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Uint64("seg_id", j.ID),
		zap.Error(err),
	)
```

- [ ] **Step 2: 补一个“重叠不超过 3 秒”的测试**

在 `boundary_alignment_test.go` 追加：

```go
func TestAlignSegmentBoundariesCapsOverlapAtThreeSeconds(t *testing.T) {
	current := LLMSegment{SegmentIndex: 0, StartTimeSec: 30, EndTimeSec: 70}
	next := &LLMSegment{SegmentIndex: 1, StartTimeSec: 68, EndTimeSec: 100}
	aligned := alignSegmentBoundaries(current, next, boundaryAlignmentSnapshot{
		StartCandidates: []boundaryCandidate{{Sec: 30, Text: "下面先看定义", Score: 5}},
		EndCandidates:   []boundaryCandidate{{Sec: 74, Text: "这就是它的定义。", Score: 8}},
	})
	if aligned.EndTimeSec-next.StartTimeSec > 3 {
		t.Fatalf("overlap = %d, want <= 3", aligned.EndTimeSec-next.StartTimeSec)
	}
}
```

- [ ] **Step 3: 跑 tasks 包全量测试**

Run: `go test ./embedding-video/internal/worker/vectorworker/tasks -v`

Expected:

```text
PASS
```

- [ ] **Step 4: 跑后端全量回归测试**

Run: `go test ./embedding-video/...`

Expected:

```text
PASS
```

如果失败：

1. 先修测试或签名不一致。
2. 不允许通过删测试规避。
3. 若发现实现与 spec 冲突，先更新 spec 再继续。

- [ ] **Step 5: 人工检查日志字段命名和降级路径是否一致**

重点检查：

```text
boundary_alignment_fallback_to_tail
tail_alignment_extended
vectorize_hierarchical_refine_asr_one_done
```

要求：

1. 能从日志看出是否使用新边界对齐器。
2. 能看出 start/end 各自偏移多少秒。
3. 能区分新对齐成功与旧 tail fallback。

- [ ] **Step 6: Commit**

```bash
git add embedding-video/internal/worker/vectorworker/tasks/asr.go embedding-video/internal/worker/vectorworker/tasks/boundary_alignment.go embedding-video/internal/worker/vectorworker/tasks/boundary_alignment_test.go embedding-video/internal/worker/vectorworker/tasks/asr_boundary_alignment_test.go
git commit -m "test(vector-worker): verify boundary alignment rollout"
```

## Spec Coverage Check

- LLM 输出契约增强：Task 1
- 共享句首/句尾规则：Task 2
- 新增 `Boundary Alignment` 引擎：Task 3
- refine 阶段接入与安全降级：Task 4
- 日志、重叠限制、全量验证：Task 5

未覆盖项：无。当前 spec 中的 architecture、data flow、failure handling、verification strategy 均已映射到任务。

## Placeholder Scan

已检查以下风险词并确认没有保留为未定义动作：

- `TODO`
- `TBD`
- `implement later`
- `add appropriate error handling`
- `write tests for the above`

## Type Consistency Check

- 新增边界字段统一使用：`BoundaryReason`、`StartAnchorText`、`EndAnchorText`、`BoundaryConfidence`
- 新增评分结构统一使用：`boundaryCandidate`
- 新增引擎入口统一使用：`alignSegmentBoundaries`
- refine 集成协调函数统一使用：`alignSegmentForRefine`

如果实现时需要改名，必须在同一提交内同步更新测试、调用点和 plan 注释。
