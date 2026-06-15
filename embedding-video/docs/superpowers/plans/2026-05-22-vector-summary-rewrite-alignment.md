# Vector Summary Rewrite Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make hierarchical segment persistence summary-safe by validating summary-text alignment after text repair and rewriting `content_summary` when the final text no longer supports the original summary.

**Architecture:** Keep final segment text as the primary factual source, then add a mandatory summary-text alignment gate before persistence. Use deterministic summary rewrite for simple segments and LLM-based rewrite only for complex repaired segments, while preserving the current selective refine ASR strategy.

**Tech Stack:** Go, vector worker task pipeline, existing LLM/ASR client, heuristic text validators, Go testing package.

---

## File Map

- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment.go`
  - Add stable-text checks, summary alignment gate, deterministic rewrite helpers, and rewrite strategy selection.
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`
  - Add tests for stable-text summary mismatch and deterministic rewrite.
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
  - Route repaired text through the alignment gate and update summary after text repair.
- Modify: `video-service/internal/worker/vectorworker/tasks/hierarchical.go`
  - Add a prompt builder for summary rewrite requests.
- Modify: `video-service/internal/worker/vectorworker/task.go`
  - Ensure repaired summaries propagate through fresh-path persistence.

### Task 1: Add failing tests for summary alignment gate and deterministic rewrite

**Files:**
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment.go`
- Test: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`

- [ ] **Step 1: Write the failing test for stable text with oversized summary**

Add this test to `summary_alignment_test.go`:

```go
func TestSummaryNeedsRewriteWhenTextIsStableButSummaryOverreaches(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   0,
		EndTimeSec:     45,
		ContentSummary: "完整讲解定义与全部适用条件",
	}
	text := "下面先看定义，这就是它的定义。"
	if !summaryNeedsRewrite(seg, text) {
		t.Fatal("expected stable text with oversized summary to require rewrite")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestSummaryNeedsRewriteWhenTextIsStableButSummaryOverreaches`

Expected: FAIL with `undefined: summaryNeedsRewrite`

- [ ] **Step 3: Write the failing test for deterministic summary rewrite**

Add this test to `summary_alignment_test.go`:

```go
func TestRewriteSummaryFromTextBuildsShortTitleFromStableText(t *testing.T) {
	got := rewriteSummaryFromText("下面先看定义，这就是它的定义。")
	if got != "下面先看定义" {
		t.Fatalf("rewriteSummaryFromText() = %q, want %q", got, "下面先看定义")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestRewriteSummaryFromTextBuildsShortTitleFromStableText`

Expected: FAIL with `undefined: rewriteSummaryFromText`

- [ ] **Step 5: Write the failing test for rule-based rewrite selection**

Add this test to `summary_alignment_test.go`:

```go
func TestShouldUseLLMSummaryRewriteSkipsLLMForSimpleStableText(t *testing.T) {
	text := "下面先看定义，这就是它的定义。"
	if shouldUseLLMSummaryRewrite(text) {
		t.Fatal("did not expect simple stable text to require LLM rewrite")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestShouldUseLLMSummaryRewriteSkipsLLMForSimpleStableText`

Expected: FAIL with `undefined: shouldUseLLMSummaryRewrite`

- [ ] **Step 7: Implement minimal alignment gate and deterministic rewrite helpers**

In `summary_alignment.go`, add code shaped like this:

```go
func summaryNeedsRewrite(seg LLMSegment, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return detectSummaryContentMismatch(seg, text, "")
}

func rewriteSummaryFromText(text string) string {
	text = NormalizeText(text)
	if text == "" {
		return ""
	}
	for _, sep := range []string{"。", "，", "\n"} {
		if idx := strings.Index(text, sep); idx > 0 {
			return normalizeSegmentTitle(text[:idx])
		}
	}
	return normalizeSegmentTitle(text)
}

func shouldUseLLMSummaryRewrite(text string) bool {
	text = NormalizeText(text)
	return len([]rune(text)) > 60 || strings.Count(text, "。") >= 2 || strings.Count(text, "\n") >= 2
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestSummaryNeedsRewriteWhenTextIsStableButSummaryOverreaches|TestRewriteSummaryFromTextBuildsShortTitleFromStableText|TestShouldUseLLMSummaryRewriteSkipsLLMForSimpleStableText'`

Expected: PASS

### Task 2: Add failing tests for summary repair in the text-first pipeline

**Files:**
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment.go`
- Test: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`

- [ ] **Step 1: Write the failing test for repairing summary after stable text**

Add this test to `summary_alignment_test.go`:

```go
func TestAlignSummaryToStableTextRewritesSummaryWhenNeeded(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   0,
		EndTimeSec:     45,
		ContentSummary: "完整讲解定义与全部适用条件",
	}
	rewritten, changed := alignSummaryToStableText(seg, "下面先看定义，这就是它的定义。")
	if !changed {
		t.Fatal("expected summary rewrite")
	}
	if rewritten.ContentSummary != "下面先看定义" {
		t.Fatalf("rewritten summary = %q", rewritten.ContentSummary)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestAlignSummaryToStableTextRewritesSummaryWhenNeeded`

Expected: FAIL with `undefined: alignSummaryToStableText`

- [ ] **Step 3: Write the failing test for preserving aligned summary**

Add this test to `summary_alignment_test.go`:

```go
func TestAlignSummaryToStableTextKeepsAlignedSummary(t *testing.T) {
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   0,
		EndTimeSec:     45,
		ContentSummary: "下面先看定义",
	}
	rewritten, changed := alignSummaryToStableText(seg, "下面先看定义，这就是它的定义。")
	if changed {
		t.Fatal("did not expect summary rewrite")
	}
	if rewritten.ContentSummary != "下面先看定义" {
		t.Fatalf("summary = %q", rewritten.ContentSummary)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestAlignSummaryToStableTextKeepsAlignedSummary`

Expected: FAIL with `undefined: alignSummaryToStableText`

- [ ] **Step 5: Implement minimal summary alignment repair helper**

In `summary_alignment.go`, add code shaped like this:

```go
func alignSummaryToStableText(seg LLMSegment, text string) (LLMSegment, bool) {
	if !summaryNeedsRewrite(seg, text) {
		return seg, false
	}
	rewritten := seg
	rewritten.ContentSummary = rewriteSummaryFromText(text)
	return rewritten, rewritten.ContentSummary != seg.ContentSummary
}
```

- [ ] **Step 6: Run alignment repair tests to verify they pass**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestAlignSummaryToStableTextRewritesSummaryWhenNeeded|TestAlignSummaryToStableTextKeepsAlignedSummary'`

Expected: PASS

### Task 3: Add prompt builder and selection tests for complex-segment LLM summary rewrite

**Files:**
- Modify: `video-service/internal/worker/vectorworker/tasks/hierarchical_test.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/hierarchical.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment_test.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment.go`

- [ ] **Step 1: Write the failing test for complex-text LLM rewrite selection**

Add this test to `summary_alignment_test.go`:

```go
func TestShouldUseLLMSummaryRewriteUsesLLMForComplexStableText(t *testing.T) {
	text := "先定义概念。然后解释适用条件。最后补充题目中的使用方式。"
	if !shouldUseLLMSummaryRewrite(text) {
		t.Fatal("expected complex stable text to require LLM summary rewrite")
	}
}
```

- [ ] **Step 2: Run test to verify it fails if current threshold is too weak**

Run: `go test ./internal/worker/vectorworker/tasks -run TestShouldUseLLMSummaryRewriteUsesLLMForComplexStableText`

Expected: PASS if helper already satisfies it; otherwise FAIL and then tune the helper.

- [ ] **Step 3: Write the failing test for summary rewrite prompt constraints**

Add this test to `hierarchical_test.go`:

```go
func TestBuildSummaryRewritePromptRestrictsOutputToSupportedTitle(t *testing.T) {
	prompt := BuildSummaryRewritePrompt("下面先看定义，这就是它的定义。")
	for _, needle := range []string{
		"只根据提供的正文生成标题",
		"不要引入正文里没有出现的概念",
		"输出短标题",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("summary rewrite prompt missing %q", needle)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildSummaryRewritePromptRestrictsOutputToSupportedTitle`

Expected: FAIL with `undefined: BuildSummaryRewritePrompt`

- [ ] **Step 5: Add minimal LLM rewrite prompt builder**

In `hierarchical.go`, add code shaped like this:

```go
func BuildSummaryRewritePrompt(text string) string {
	text = NormalizeText(text)
	var b strings.Builder
	b.WriteString("请只根据提供的正文生成标题。\n")
	b.WriteString("要求：\n")
	b.WriteString("- 只根据提供的正文生成标题\n")
	b.WriteString("- 不要引入正文里没有出现的概念\n")
	b.WriteString("- 输出短标题\n")
	b.WriteString("- 不要输出解释、不要输出多句\n")
	b.WriteString("正文：\n")
	b.WriteString(text)
	return b.String()
}
```

- [ ] **Step 6: Run LLM selection and prompt tests**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestShouldUseLLMSummaryRewriteUsesLLMForComplexStableText|TestBuildSummaryRewritePromptRestrictsOutputToSupportedTitle'`

Expected: PASS

### Task 4: Apply summary alignment gate in the refine path

**Files:**
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/summary_alignment.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/asr_refine_input_test.go`

- [ ] **Step 1: Write the failing test for post-text summary rewrite in refine input flow**

Add this test to `asr_refine_input_test.go`:

```go
func TestBuildRefineSegmentInputRewritesSummaryFromStableCoarseText(t *testing.T) {
	input, _, _, err := buildRefineSegmentInput(context.Background(), refineInputJob{
		StartSec:           0,
		EndSec:             45,
		NextStartSec:       0,
		Summary:            "完整讲解定义与全部适用条件",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 0, EndSec: 45, Text: "下面先看定义，这就是它的定义。"}}, 120, func(context.Context, int, int) (string, error) {
		return "", nil
	})
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if input != "下面先看定义\n下面先看定义，这就是它的定义。" {
		t.Fatalf("input = %q", input)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildRefineSegmentInputRewritesSummaryFromStableCoarseText`

Expected: FAIL because the current flow still uses the original summary.

- [ ] **Step 3: Update `buildRefineSegmentInput` to align summary after text stabilizes**

In `asr.go`, replace the current local `combine`-only logic with code shaped like this:

```go
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   job.StartSec,
		EndTimeSec:     job.EndSec,
		ContentSummary: job.Summary,
	}
	combine := func(summary string, text string) string {
		summary = strings.TrimSpace(summary)
		text = strings.TrimSpace(text)
		if summary == "" {
			return text
		}
		if text == "" {
			return summary
		}
		return summary + "\n" + text
	}
```

Then, after choosing the final text candidate, run:

```go
	alignedSeg, _ := alignSummaryToStableText(seg, finalText)
	return combine(alignedSeg.ContentSummary, finalText), finalStart, finalEnd, nil
```

Apply this to both coarse-text and refine-ASR success paths.

- [ ] **Step 4: Run summary rewrite refine tests**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestBuildRefineSegmentInputRewritesSummaryFromStableCoarseText|TestBuildRefineSegmentInputUsesRefineASRWhenSummaryContentMismatchIsDetected|TestBuildRefineSegmentInputSkipsRefineForHealthyHighConfidenceSegment|TestBuildRefineSegmentInputUsesContentSummaryOnlyWhenCoarseTextIsEmpty'`

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
  - summary-text gate: covered by Tasks 1, 2, and 4
  - text-first then summary rewrite: covered by Tasks 2 and 4
  - rule-based summary rewrite: covered by Task 1
  - LLM summary rewrite prompt and selection: covered by Task 3
  - final aligned pair before persistence: covered by Task 4 and verified in Task 5
- Placeholder scan: no `TODO`, `TBD`, or implied code steps remain.
- Type consistency:
  - helper names are consistent across tests and implementation tasks
  - prompt builder, rewrite helpers, and refine flow changes align with existing package layout
