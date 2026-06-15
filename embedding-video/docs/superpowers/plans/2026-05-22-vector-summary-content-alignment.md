# Vector Summary-Content Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure hierarchical video segments are semantically complete enough that each segment's `content_summary` is fully supported by its captured text, while keeping selective refine ASR within the new cost target.

**Architecture:** Strengthen the hierarchical segmentation prompts so the LLM prefers semantically closed knowledge units, then add lightweight summary-content mismatch heuristics in the task package to detect risky segments. Use those heuristics to drive merge-first or mismatch-driven selective refine decisions, keeping coarse transcript reuse as the default for healthy segments.

**Tech Stack:** Go, existing vector worker task pipeline, rule-based segment normalization, OpenAI-compatible LLM and ASR client, Go testing package.

---

## File Map

- Modify: `video-service/internal/worker/vectorworker/tasks/hierarchical.go`
  - Tighten prompt rules for `content_summary` and semantic closure.
- Modify: `video-service/internal/worker/vectorworker/tasks/hierarchical_test.go`
  - Add prompt-level tests for the new summary-support constraints.
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
  - Expand selective refine trigger logic beyond low confidence.
- Create: `video-service/internal/worker/vectorworker/tasks/summary_alignment.go`
  - Add mismatch heuristic helpers and merge/repair decision helpers.
- Create: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`
  - Add unit tests for mismatch detection and merge/repair triggers.
- Modify: `video-service/internal/worker/vectorworker/task.go`
  - Thread mismatch-driven decisions into fresh-path refine selection without changing resume-path contracts.

### Task 1: Strengthen prompt constraints for summary support

**Files:**
- Modify: `video-service/internal/worker/vectorworker/tasks/hierarchical_test.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/hierarchical.go`
- Test: `video-service/internal/worker/vectorworker/tasks/hierarchical_test.go`

- [ ] **Step 1: Write the failing test for main prompt summary-support rules**

Add this test to `hierarchical_test.go`:

```go
func TestBuildHierarchicalSegmentationPromptRequiresSummaryToBeFullySupported(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "先讲定义，再解释它的适用条件。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationPrompt error = %v", err)
	}
	for _, needle := range []string{
		"content_summary 必须能被该分段内的实际讲解内容完整支撑",
		"如果某个知识点在本段没有讲完，不要把它总结成已经完整讲完",
		"优先保证知识单元完整，不要为了时长均匀硬切",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q", needle)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildHierarchicalSegmentationPromptRequiresSummaryToBeFullySupported`

Expected: FAIL with missing prompt text.

- [ ] **Step 3: Write the failing test for retry prompt summary-support rules**

Add this test to `hierarchical_test.go`:

```go
func TestBuildHierarchicalSegmentationRetryPromptRequiresSummaryToBeFullySupported(t *testing.T) {
	prompt, err := BuildHierarchicalSegmentationRetryPrompt(120, 60, 20, 180, []coarseItem{{Index: 0, StartSec: 0, EndSec: 60, Text: "先讲定义，再解释它的适用条件。"}})
	if err != nil {
		t.Fatalf("BuildHierarchicalSegmentationRetryPrompt error = %v", err)
	}
	for _, needle := range []string{
		"content_summary 必须能被该分段内的实际讲解内容完整支撑",
		"如果某个知识点在本段没有讲完，不要把它总结成已经完整讲完",
		"优先保证知识单元完整，不要为了时长均匀硬切",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("retry prompt missing %q", needle)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildHierarchicalSegmentationRetryPromptRequiresSummaryToBeFullySupported`

Expected: FAIL with missing retry prompt text.

- [ ] **Step 5: Add the minimal prompt wording to pass both tests**

In `hierarchical.go`, add lines shaped like this to both prompt builders near the existing `content_summary` guidance:

```go
b.WriteString("- content_summary 必须能被该分段内的实际讲解内容完整支撑，不能提前概括下一段内容\n")
b.WriteString("- 如果某个知识点、例题步骤、结论在本段没有讲完，不要把它总结成已经完整讲完\n")
b.WriteString("- 优先保证知识单元完整，不要为了时长均匀硬切到一句话说一半的位置\n")
```

- [ ] **Step 6: Run prompt tests to verify they pass**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestBuildHierarchicalSegmentationPromptRequiresSummaryToBeFullySupported|TestBuildHierarchicalSegmentationRetryPromptRequiresSummaryToBeFullySupported'`

Expected: PASS

### Task 2: Add failing tests for summary-content mismatch heuristics

**Files:**
- Create: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`
- Create: `video-service/internal/worker/vectorworker/tasks/summary_alignment.go`
- Test: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`

- [ ] **Step 1: Write the failing test for short segment oversized summary detection**

Create `summary_alignment_test.go` with this test:

```go
package tasks

import "testing"

func TestDetectSummaryContentMismatchFlagsShortSegmentWithBroadSummary(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   0,
		EndTimeSec:     18,
		ContentSummary: "完整讲解二次函数顶点式的定义和适用条件",
	}
	text := "先看二次函数。"
	if !detectSummaryContentMismatch(seg, text, "") {
		t.Fatal("expected mismatch for short segment with oversized summary")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestDetectSummaryContentMismatchFlagsShortSegmentWithBroadSummary`

Expected: FAIL with `undefined: detectSummaryContentMismatch`

- [ ] **Step 3: Write the failing test for obvious continuation detection**

Add this test to `summary_alignment_test.go`:

```go
func TestDetectSummaryContentMismatchFlagsHalfSentenceContinuation(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   30,
		EndTimeSec:     55,
		ContentSummary: "定义讲解完成",
	}
	text := "然后继续说明这个定义在题目里的用法"
	if !detectSummaryContentMismatch(seg, text, "接下来分析例题") {
		t.Fatal("expected mismatch for continuation-like incomplete text")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestDetectSummaryContentMismatchFlagsHalfSentenceContinuation`

Expected: FAIL with `undefined: detectSummaryContentMismatch`

- [ ] **Step 5: Write the failing test for merge-first repair decision**

Add this test to `summary_alignment_test.go`:

```go
func TestShouldMergeMismatchedSegmentWithNextWhenTopicContinues(t *testing.T) {
	current := LLMSegment{SegmentIndex: 0, StartTimeSec: 0, EndTimeSec: 25, ContentSummary: "定义说明"}
	next := LLMSegment{SegmentIndex: 1, StartTimeSec: 25, EndTimeSec: 50, ContentSummary: "继续说明定义在题目中的用法"}
	if !shouldMergeMismatchedSegment(current, next, "然后继续说明定义的用法") {
		t.Fatal("expected merge decision for continued topic")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestShouldMergeMismatchedSegmentWithNextWhenTopicContinues`

Expected: FAIL with `undefined: shouldMergeMismatchedSegment`

- [ ] **Step 7: Write minimal heuristic implementations**

Create `summary_alignment.go` with code shaped like this:

```go
package tasks

import "strings"

func detectSummaryContentMismatch(seg LLMSegment, text string, nextSummary string) bool {
	text = strings.TrimSpace(text)
	summary := strings.TrimSpace(seg.ContentSummary)
	if text == "" {
		return true
	}
	if seg.EndTimeSec-seg.StartTimeSec < 20 && len([]rune(summary)) > 12 {
		return true
	}
	if looksLikeContinuationText(text) {
		return true
	}
	if strings.TrimSpace(nextSummary) != "" && sharesTopic(summary, nextSummary) && !LooksLikeSentenceEnd(text) {
		return true
	}
	return false
}

func shouldMergeMismatchedSegment(current LLMSegment, next LLMSegment, currentText string) bool {
	if strings.TrimSpace(next.ContentSummary) == "" {
		return false
	}
	if !detectSummaryContentMismatch(current, currentText, next.ContentSummary) {
		return false
	}
	return sharesTopic(current.ContentSummary, next.ContentSummary) || looksLikeContinuationText(currentText)
}
```

- [ ] **Step 8: Run heuristic tests to verify they pass**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestDetectSummaryContentMismatchFlagsShortSegmentWithBroadSummary|TestDetectSummaryContentMismatchFlagsHalfSentenceContinuation|TestShouldMergeMismatchedSegmentWithNextWhenTopicContinues'`

Expected: PASS

### Task 3: Add failing tests for mismatch-driven refine triggers

**Files:**
- Modify: `video-service/internal/worker/vectorworker/tasks/asr_refine_input_test.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment.go`
- Test: `video-service/internal/worker/vectorworker/tasks/asr_refine_input_test.go`

- [ ] **Step 1: Write the failing test for high-confidence mismatch-driven refine**

Add this test to `asr_refine_input_test.go`:

```go
func TestBuildRefineSegmentInputUsesRefineASRWhenSummaryContentMismatchIsDetected(t *testing.T) {
	var calls int
	input, _, _, err := buildRefineSegmentInput(context.Background(), refineInputJob{
		StartSec:           40,
		EndSec:             60,
		NextStartSec:       80,
		Summary:            "完整讲解定义和适用条件",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 40, EndSec: 60, Text: "先看定义"}}, 120, func(context.Context, int, int) (string, error) {
		calls++
		return "完整讲解定义和适用条件。", nil
	})
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("ASR calls = %d, want 1", calls)
	}
	if input != "完整讲解定义和适用条件\n完整讲解定义和适用条件。" {
		t.Fatalf("input = %q", input)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildRefineSegmentInputUsesRefineASRWhenSummaryContentMismatchIsDetected`

Expected: FAIL because high-confidence segments currently do not refine.

- [ ] **Step 3: Write the failing test for healthy high-confidence segment staying on coarse path**

Add this test to `asr_refine_input_test.go`:

```go
func TestBuildRefineSegmentInputSkipsRefineForHealthyHighConfidenceSegment(t *testing.T) {
	var calls int
	input, _, _, err := buildRefineSegmentInput(context.Background(), refineInputJob{
		StartSec:           0,
		EndSec:             45,
		NextStartSec:       0,
		Summary:            "定义讲解",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 0, EndSec: 45, Text: "下面先看定义，这就是它的定义。"}}, 120, func(context.Context, int, int) (string, error) {
		calls++
		return "", nil
	})
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("ASR calls = %d, want 0", calls)
	}
	if input != "定义讲解\n下面先看定义，这就是它的定义。" {
		t.Fatalf("input = %q", input)
	}
}
```

- [ ] **Step 4: Run test to verify it fails only if implementation is too broad**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildRefineSegmentInputSkipsRefineForHealthyHighConfidenceSegment`

Expected: PASS now, and must remain PASS after the next change.

- [ ] **Step 5: Extend `buildRefineSegmentInput` with mismatch-aware triggering**

In `asr.go`, change `buildRefineSegmentInput` to compute coarse text first and then use logic shaped like this:

```go
	coarseText := buildTranscriptFromCoarseItems(coarseItems, job.StartSec, job.EndSec)
	shouldRefine := shouldUseRefineASRFallback(job.BoundaryConfidence) || detectSummaryContentMismatch(LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   job.StartSec,
		EndTimeSec:     job.EndSec,
		ContentSummary: job.Summary,
	}, coarseText, "")
	if !shouldRefine {
		return combine(coarseText), job.StartSec, job.EndSec, nil
	}
```

Keep the existing single-shot refine behavior for the refine path.

- [ ] **Step 6: Run the targeted refine tests**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestBuildRefineSegmentInputUsesRefineASRWhenSummaryContentMismatchIsDetected|TestBuildRefineSegmentInputSkipsRefineForHealthyHighConfidenceSegment|TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow|TestBuildRefineSegmentInputUsesSingleShotASRForLowConfidence|TestBuildRefineSegmentInputFallsBackToCoarseTextWhenLowConfidenceASRFails|TestBuildRefineSegmentInputUsesContentSummaryOnlyWhenCoarseTextIsEmpty'`

Expected: PASS

### Task 4: Apply merge-first repair before selective refine selection

**Files:**
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment.go`
- Modify: `video-service/internal/worker/vectorworker/task.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/hierarchical.go`
- Test: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`
- Test: `video-service/internal/worker/vectorworker/tasks/hierarchical_test.go`

- [ ] **Step 1: Write the failing test for merge-first repair application**

Add this test to `summary_alignment_test.go`:

```go
func TestRepairMismatchedSegmentsMergesContinuationWithNext(t *testing.T) {
	segs := []LLMSegment{
		{SegmentIndex: 0, StartTimeSec: 0, EndTimeSec: 20, ContentSummary: "定义说明"},
		{SegmentIndex: 1, StartTimeSec: 20, EndTimeSec: 50, ContentSummary: "继续说明定义的适用条件"},
	}
	coarse := []CoarseItem{{StartSec: 0, EndSec: 50, Text: "下面先看定义，然后继续说明定义的适用条件。"}}
	repaired := repairMismatchedSegments(segs, coarse)
	if len(repaired) != 1 {
		t.Fatalf("len(repaired) = %d, want 1", len(repaired))
	}
	if repaired[0].StartTimeSec != 0 || repaired[0].EndTimeSec != 50 {
		t.Fatalf("merged segment = [%d, %d]", repaired[0].StartTimeSec, repaired[0].EndTimeSec)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestRepairMismatchedSegmentsMergesContinuationWithNext`

Expected: FAIL with `undefined: repairMismatchedSegments`

- [ ] **Step 3: Implement minimal merge-first repair helper**

In `summary_alignment.go`, add code shaped like this:

```go
func repairMismatchedSegments(segs []LLMSegment, coarseItems []CoarseItem) []LLMSegment {
	if len(segs) < 2 {
		return segs
	}
	out := make([]LLMSegment, 0, len(segs))
	for i := 0; i < len(segs); i++ {
		current := segs[i]
		if i+1 < len(segs) {
			currentText := buildTranscriptFromCoarseItems(coarseItems, current.StartTimeSec, current.EndTimeSec)
			next := segs[i+1]
			if shouldMergeMismatchedSegment(current, next, currentText) {
				current.EndTimeSec = next.EndTimeSec
				current.ContentSummary = strings.TrimSpace(current.ContentSummary + "\n" + next.ContentSummary)
				current.KnowledgeTags = MergeTags(current.KnowledgeTags, next.KnowledgeTags)
				out = append(out, current)
				i++
				continue
			}
		}
		out = append(out, current)
	}
	for i := range out {
		out[i].SegmentIndex = i
	}
	return out
}
```

- [ ] **Step 4: Apply the repair helper before writing fresh-path segments**

In `task.go`, after `normalizeLLMSegments(...)` succeeds and before `upsertHierarchicalSegments(...)`, add code shaped like this:

```go
	segs = []llmSegment(tasks.RepairMismatchedSegments([]tasks.LLMSegment(segs), []tasks.CoarseItem(coarseItems)))
```

Export the helper from `summary_alignment.go` if needed:

```go
func RepairMismatchedSegments(segs []LLMSegment, coarseItems []CoarseItem) []LLMSegment {
	return repairMismatchedSegments(segs, coarseItems)
}
```

- [ ] **Step 5: Run merge and prompt tests**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestRepairMismatchedSegmentsMergesContinuationWithNext|TestBuildHierarchicalSegmentationPromptRequiresSummaryToBeFullySupported|TestBuildHierarchicalSegmentationRetryPromptRequiresSummaryToBeFullySupported'`

Expected: PASS

### Task 5: Verify package and module behavior

**Files:**
- Modify: `video-service/internal/worker/vectorworker/tasks/...`
- Modify: `video-service/internal/worker/vectorworker/task.go`
- Test: `video-service/internal/worker/vectorworker/tasks/...`
- Test: `video-service/internal/worker/vectorworker/...`

- [ ] **Step 1: Run focused tasks package verification**

Run: `go test ./internal/worker/vectorworker/tasks`

Expected: PASS

- [ ] **Step 2: Run vectorworker package verification**

Run: `go test ./internal/worker/vectorworker/...`

Expected: PASS

- [ ] **Step 3: Run full service verification**

Run: `go test ./...`

Expected: PASS, or if an unrelated failure appears, stop and report the exact package and output.

## Self-Review Notes

- Spec coverage:
  - prompt strengthening: covered by Task 1
  - mismatch heuristics: covered by Task 2
  - merge-first repair: covered by Task 4
  - mismatch-driven selective refine: covered by Task 3
  - ASR cost control via selective repairs: preserved by Tasks 3 and 5
- Placeholder scan: no `TODO`, `TBD`, or implied test work remains.
- Type consistency:
  - helper names are consistent across tests and implementation tasks
  - prompt builders, refine helpers, and repair helpers all reference existing package structures
