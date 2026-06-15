# Vector Summary Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist the refined `content_summary` produced by the hierarchical vector worker so stored segment titles match the final transcript used for embedding.

**Architecture:** Keep the existing hierarchical pipeline and selective refine ASR policy. Replace the refine input helper's implicit string contract with a structured result carrying `Input`, `Summary`, `StartSec`, `EndSec`, and `SummaryMode`, then propagate the final summary into the existing transactional segment update.

**Tech Stack:** Go, GORM, pgvector-go, existing vector worker task package, standard `go test`.

---

## File Structure

- Modify: `internal/worker/vectorworker/tasks/asr.go`
  - Add a small internal `refineSegmentInputResult` type.
  - Change `buildRefineSegmentInput` and `buildRefineSegmentInputWithSummaryRewrite` to return the structured result.
  - Propagate result summaries through `RefineSegmentsASRAndEmbed`.
  - Add `content_summary` to the final update map.
- Modify: `internal/worker/vectorworker/tasks/asr_refine_input_test.go`
  - Update existing helper tests to assert `result.Input`, `result.Summary`, `result.StartSec`, and `result.EndSec`.
  - Preserve ASR cost assertions, especially no-call and single-call tests.
- Optional Modify: `internal/worker/vectorworker/tasks/summary_alignment_test.go`
  - Only touch if implementation reveals a better home for summary-specific assertions. Prefer keeping the contract tests in `asr_refine_input_test.go`.

No database migration, config, prompt, queue, or frontend files should change.

## Task 1: Make Refine Input Return the Final Summary Explicitly

**Files:**
- Modify: `internal/worker/vectorworker/tasks/asr.go`
- Modify: `internal/worker/vectorworker/tasks/asr_refine_input_test.go`

- [ ] **Step 1: Update the first coarse-text test to expect a structured result**

In `internal/worker/vectorworker/tasks/asr_refine_input_test.go`, change `TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow` to this shape:

```go
func TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow(t *testing.T) {
	called := false
	result, err := buildRefineSegmentInput(context.Background(), refineInputJob{
		StartSec:           60,
		EndSec:             120,
		NextStartSec:       0,
		Summary:            "定义",
		BoundaryConfidence: "high",
	}, []CoarseItem{{StartSec: 60, EndSec: 120, Text: "这一段在讲定义。"}}, 180, func(context.Context, int, int) (string, error) {
		called = true
		return "", nil
	})
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if called {
		t.Fatal("expected no refine ASR call")
	}
	if result.StartSec != 60 || result.EndSec != 120 {
		t.Fatalf("window = (%d, %d), want (60, 120)", result.StartSec, result.EndSec)
	}
	if result.Summary != "定义" {
		t.Fatalf("summary = %q, want %q", result.Summary, "定义")
	}
	if result.SummaryMode != "original" {
		t.Fatalf("summary mode = %q, want original", result.SummaryMode)
	}
	if result.Input != "定义\n这一段在讲定义。" {
		t.Fatalf("input = %q", result.Input)
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

Run:

```bash
go test ./internal/worker/vectorworker/tasks -run TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow
```

Expected: FAIL at compile time because `buildRefineSegmentInput` still returns `(string, int, int, error)` instead of `(refineSegmentInputResult, error)`.

- [ ] **Step 3: Add the structured result type and update helper signatures**

In `internal/worker/vectorworker/tasks/asr.go`, place this type near `refineInputJob`:

```go
type refineSegmentInputResult struct {
	Input       string
	Summary     string
	StartSec    int
	EndSec      int
	SummaryMode string
}
```

Change the wrapper signature:

```go
func buildRefineSegmentInput(ctx context.Context, job refineInputJob, coarseItems []CoarseItem, videoDurationSec int, transcribeRange func(context.Context, int, int) (string, error)) (refineSegmentInputResult, error) {
	return buildRefineSegmentInputWithSummaryRewrite(ctx, job, coarseItems, videoDurationSec, transcribeRange, nil)
}
```

Change the rewrite helper signature:

```go
func buildRefineSegmentInputWithSummaryRewrite(ctx context.Context, job refineInputJob, coarseItems []CoarseItem, videoDurationSec int, transcribeRange func(context.Context, int, int) (string, error), rewriteSummary summaryRewriter) (refineSegmentInputResult, error) {
```

Inside `buildRefineSegmentInputWithSummaryRewrite`, add a helper that builds the result:

```go
buildResult := func(summary string, text string, startSec int, endSec int, summaryMode string) refineSegmentInputResult {
	summary = strings.TrimSpace(summary)
	text = strings.TrimSpace(text)
	input := text
	if summary != "" && text != "" {
		input = summary + "\n" + text
	} else if summary != "" {
		input = summary
	}
	return refineSegmentInputResult{
		Input:       input,
		Summary:     summary,
		StartSec:    startSec,
		EndSec:      endSec,
		SummaryMode: summaryMode,
	}
}
```

Remove the old `combine` helper. Replace each old return with a structured result:

```go
return buildResult(alignedSeg.ContentSummary, coarseText, job.StartSec, job.EndSec, summaryMode), nil
```

```go
return buildResult(alignedSeg.ContentSummary, finalText, start, end, summaryMode), nil
```

- [ ] **Step 4: Run the focused test and verify it passes**

Run:

```bash
go test ./internal/worker/vectorworker/tasks -run TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow
```

Expected: PASS.

- [ ] **Step 5: Update remaining refine input tests to use the structured result**

In `internal/worker/vectorworker/tasks/asr_refine_input_test.go`, update each remaining test with this pattern:

```go
result, err := buildRefineSegmentInput(...)
```

or:

```go
result, err := buildRefineSegmentInputWithSummaryRewrite(...)
```

Then replace:

```go
input
start
end
```

with:

```go
result.Input
result.StartSec
result.EndSec
```

For tests where the summary is meaningful, also assert `result.Summary`.

Use these exact expectations:

```go
if result.Summary != "例题" {
	t.Fatalf("summary = %q, want %q", result.Summary, "例题")
}
```

```go
if result.Summary != "下面先看定义" {
	t.Fatalf("summary = %q, want %q", result.Summary, "下面先看定义")
}
```

```go
if result.Summary != "定义及适用条件" {
	t.Fatalf("summary = %q, want %q", result.Summary, "定义及适用条件")
}
```

Keep the existing ASR call assertions unchanged:

```go
if calls != 0 {
	t.Fatalf("ASR calls = %d, want 0", calls)
}
```

```go
if calls != 1 {
	t.Fatalf("ASR calls = %d, want 1", calls)
}
```

- [ ] **Step 6: Run all refine input tests**

Run:

```bash
go test ./internal/worker/vectorworker/tasks -run 'TestBuildRefineSegmentInput'
```

Expected: PASS.

- [ ] **Step 7: Update selective refine call site to consume the structured result**

In `internal/worker/vectorworker/tasks/asr.go`, inside the `if useSelectiveRefine` branch in `RefineSegmentsASRAndEmbed`, replace:

```go
input, newStartSec, newEndSec, err := buildRefineSegmentInputWithSummaryRewrite(...)
```

with:

```go
refineResult, err := buildRefineSegmentInputWithSummaryRewrite(...)
input := refineResult.Input
newStartSec := refineResult.StartSec
newEndSec := refineResult.EndSec
summaryMode = refineResult.SummaryMode
```

Delete the old block that infers `summaryMode` by splitting `input`:

```go
if strings.Contains(input, "\n") {
	parts := strings.SplitN(input, "\n", 2)
	if len(parts) == 2 {
		oldSummary := strings.TrimSpace(j.Summary)
		newSummary := strings.TrimSpace(parts[0])
		switch {
		case newSummary == oldSummary:
			summaryMode = "original"
		case shouldUseLLMSummaryRewrite(parts[1]):
			summaryMode = "llm_rewrite"
		default:
			summaryMode = "rule_rewrite"
		}
	}
}
```

When sending the result, include the final summary after Task 2 adds the `Summary` field. For now, keep:

```go
_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, StartSec: newStartSec, EndSec: newEndSec, Input: input, SummaryMode: summaryMode, Err: nil})
```

- [ ] **Step 8: Run vector worker task tests**

Run:

```bash
go test ./internal/worker/vectorworker/tasks
```

Expected: PASS.

- [ ] **Step 9: Commit task 1 if you are committing locally**

Only do this if the user has asked for commits:

```bash
git add internal/worker/vectorworker/tasks/asr.go internal/worker/vectorworker/tasks/asr_refine_input_test.go
git commit -m "refactor(vector-worker): return structured refine input"
```

## Task 2: Persist Refined Summary With Embedding Updates

**Files:**
- Modify: `internal/worker/vectorworker/tasks/asr.go`
- Modify: `internal/worker/vectorworker/tasks/asr_refine_input_test.go`

- [ ] **Step 1: Add a unit test for the final update payload helper**

In `internal/worker/vectorworker/tasks/asr_refine_input_test.go`, add this test near the refine input tests:

```go
func TestBuildSegmentEmbeddingUpdateValuesIncludesContentSummary(t *testing.T) {
	values := buildSegmentEmbeddingUpdateValues(10, 45, "  下面先看定义  ", pgvector.NewVector([]float32{0.5}))

	if values["start_time"] != 10 {
		t.Fatalf("start_time = %v, want 10", values["start_time"])
	}
	if values["end_time"] != 45 {
		t.Fatalf("end_time = %v, want 45", values["end_time"])
	}
	if values["content_summary"] != "下面先看定义" {
		t.Fatalf("content_summary = %v, want 下面先看定义", values["content_summary"])
	}
	if values["status"] != int16(1) {
		t.Fatalf("status = %v, want %v", values["status"], int16(1))
	}
	if _, ok := values["embedding"]; !ok {
		t.Fatal("expected embedding key")
	}
}
```

Add this import to the existing import block in `internal/worker/vectorworker/tasks/asr_refine_input_test.go`:

```go
	"github.com/pgvector/pgvector-go"
```

This helper does not exist yet; the test should fail before implementation.

- [ ] **Step 2: Run the new test and verify it fails**

Run:

```bash
go test ./internal/worker/vectorworker/tasks -run TestBuildSegmentEmbeddingUpdateValuesIncludesContentSummary
```

Expected: FAIL with `undefined: buildSegmentEmbeddingUpdateValues`.

- [ ] **Step 3: Add the update payload helper**

In `internal/worker/vectorworker/tasks/asr.go`, near the embedding update code or near helper functions, add:

```go
func buildSegmentEmbeddingUpdateValues(startSec int, endSec int, summary string, embedding pgvector.Vector) map[string]any {
	return map[string]any{
		"start_time":       startSec,
		"end_time":         endSec,
		"content_summary":  normalizeSegmentTitle(summary),
		"embedding":        embedding,
		"status":           int16(1),
	}
}
```

- [ ] **Step 4: Run the new test and verify it passes**

Run:

```bash
go test ./internal/worker/vectorworker/tasks -run TestBuildSegmentEmbeddingUpdateValuesIncludesContentSummary
```

Expected: PASS.

- [ ] **Step 5: Add summary to worker result and ordered arrays**

In `internal/worker/vectorworker/tasks/asr.go`, update the local `result` struct inside `RefineSegmentsASRAndEmbed`:

```go
type result struct {
	JobIndex    int
	ID          uint64
	StartSec    int
	EndSec      int
	Input       string
	Summary     string
	SummaryMode string
	Err         error
}
```

Initialize ordered summaries after `orderedInputs`:

```go
orderedSummaries := make([]string, len(jobList))
```

Inside the loop that initializes ordered IDs and start/end values, add:

```go
orderedSummaries[i] = strings.TrimSpace(jobList[i].Summary)
```

Inside the results collection block, add:

```go
orderedSummaries[r.JobIndex] = r.Summary
```

Keep that assignment inside the existing `if r.JobIndex >= 0 && r.JobIndex < len(orderedInputs)` guard.

- [ ] **Step 6: Send final summary from the selective refine branch**

In the `useSelectiveRefine` branch, update `sendResult` to include `refineResult.Summary`:

```go
_ = sendResult(result{
	JobIndex:    j.JobIndex,
	ID:          j.ID,
	StartSec:    newStartSec,
	EndSec:      newEndSec,
	Input:       input,
	Summary:     refineResult.Summary,
	SummaryMode: summaryMode,
	Err:         nil,
})
```

For selective refine error results, keep the existing zero-value summary behavior:

```go
_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, StartSec: j.StartSec, EndSec: j.EndSec, Err: err})
```

- [ ] **Step 7: Send summary from the resume fallback branch**

In the non-selective branch near the existing `combined` input creation, update the result send to carry the existing base summary:

```go
_ = sendResult(result{
	JobIndex: j.JobIndex,
	ID:       j.ID,
	StartSec: alignedStartSec,
	EndSec:   alignedEndSec,
	Input:    combined,
	Summary:  base,
	Err:      nil,
})
```

This preserves resume behavior without adding new summary rewrite logic.

- [ ] **Step 8: Carry summaries through embedding pairs**

In `RefineSegmentsASRAndEmbed`, after:

```go
pairsIDs := make([]uint64, 0, len(orderedInputs))
pairsInputs := make([]string, 0, len(orderedInputs))
```

add:

```go
pairsSummaries := make([]string, 0, len(orderedInputs))
```

Inside the loop that appends pairs, add:

```go
pairsSummaries = append(pairsSummaries, orderedSummaries[i])
```

Inside the embedding batch loop, after:

```go
ids := pairsIDs[i:j]
```

add:

```go
summaries := pairsSummaries[i:j]
```

- [ ] **Step 9: Add Summary to embeddingUpdate and populate it**

Update the local `embeddingUpdate` struct:

```go
type embeddingUpdate struct {
	ID        uint64
	StartSec  int
	EndSec    int
	Summary   string
	Embedding pgvector.Vector
}
```

When appending `embeddingUpdate`, add:

```go
Summary:   summaries[k],
```

The append block should look like:

```go
allUpdates = append(allUpdates, embeddingUpdate{
	ID:        id,
	StartSec:  orderedStartSecs[i+k],
	EndSec:    orderedEndSecs[i+k],
	Summary:   summaries[k],
	Embedding: pgvector.NewVector(v),
})
```

- [ ] **Step 10: Use the helper in the transactional update**

Replace the update map inside the final transaction:

```go
Updates(map[string]any{
	"start_time": update.StartSec,
	"end_time":   update.EndSec,
	"embedding":  update.Embedding,
	"status":     int16(1),
})
```

with:

```go
Updates(buildSegmentEmbeddingUpdateValues(update.StartSec, update.EndSec, update.Summary, update.Embedding))
```

- [ ] **Step 11: Run package tests**

Run:

```bash
go test ./internal/worker/vectorworker/tasks
```

Expected: PASS.

- [ ] **Step 12: Run vector worker tests**

Run:

```bash
go test ./internal/worker/vectorworker/...
```

Expected: PASS.

- [ ] **Step 13: Run full service tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 14: Commit task 2 if you are committing locally**

Only do this if the user has asked for commits:

```bash
git add internal/worker/vectorworker/tasks/asr.go internal/worker/vectorworker/tasks/asr_refine_input_test.go
git commit -m "fix(vector-worker): persist refined segment summaries"
```

## Final Manual Review

- [ ] Confirm no prompt text changed.
- [ ] Confirm no config file changed.
- [ ] Confirm no database migration file was added.
- [ ] Confirm healthy high-confidence tests still assert zero refine ASR calls.
- [ ] Confirm low-confidence tests still assert exactly one refine ASR call.
- [ ] Confirm final update map includes `content_summary`.
- [ ] Confirm full test suite passes from `video-service/`.
