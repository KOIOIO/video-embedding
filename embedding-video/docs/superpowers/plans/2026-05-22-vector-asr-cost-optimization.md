# Vector ASR Cost Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce hierarchical vector-worker ASR consumption by defaulting to coarse transcript reuse and limiting refine ASR to one expanded-window call for `boundary_confidence == "low"` segments.

**Architecture:** Keep the existing coarse ASR stage as-is, pass `coarseItems` into the refine stage, and split refine transcript generation into two paths: coarse-text reuse for normal segments and single-shot expanded-window ASR for low-confidence segments. Preserve current embedding composition and task persistence behavior while removing repeated boundary-probe ASR from the common path.

**Tech Stack:** Go, GORM, existing vector worker task pipeline, FFmpeg transcoder, OpenAI-compatible ASR/embedding client, Go testing package.

---

## File Map

- Modify: `video-service/internal/worker/vectorworker/task.go`
  - Pass `coarseItems` into the refine phase call sites.
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
  - Add coarse-text reuse helpers.
  - Add low-confidence single-shot fallback helpers.
  - Refactor `RefineSegmentsASRAndEmbed` to use the new default path.
- Create: `video-service/internal/worker/vectorworker/tasks/asr_coarse_text_test.go`
  - Unit tests for coarse transcript derivation, low-confidence decision, and expanded-window calculation.
- Create: `video-service/internal/worker/vectorworker/tasks/asr_refine_input_test.go`
  - Unit tests for per-segment transcript selection and fallback behavior.

### Task 1: Add failing tests for helper behavior

**Files:**
- Create: `video-service/internal/worker/vectorworker/tasks/asr_coarse_text_test.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
- Test: `video-service/internal/worker/vectorworker/tasks/asr_coarse_text_test.go`

- [ ] **Step 1: Write the failing test for coarse transcript reuse**

```go
package tasks

import "testing"

func TestBuildTranscriptFromCoarseItemsConcatenatesOverlapsInOrder(t *testing.T) {
	items := []CoarseItem{
		{Index: 0, StartSec: 0, EndSec: 60, Text: "第一段。"},
		{Index: 1, StartSec: 60, EndSec: 120, Text: "第二段。"},
		{Index: 2, StartSec: 120, EndSec: 180, Text: "第三段。"},
	}

	got := buildTranscriptFromCoarseItems(items, 30, 130)
	want := "第一段。\n第二段。\n第三段。"
	if got != want {
		t.Fatalf("buildTranscriptFromCoarseItems() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildTranscriptFromCoarseItemsConcatenatesOverlapsInOrder`

Expected: FAIL with `undefined: buildTranscriptFromCoarseItems`

- [ ] **Step 3: Write the failing test for low-confidence fallback decision**

```go
func TestShouldUseRefineASRFallbackOnlyForLowConfidence(t *testing.T) {
	if !shouldUseRefineASRFallback("low") {
		t.Fatal("expected low confidence to trigger fallback")
	}
	for _, confidence := range []string{"", "medium", "high"} {
		if shouldUseRefineASRFallback(confidence) {
			t.Fatalf("confidence %q should not trigger fallback", confidence)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestShouldUseRefineASRFallbackOnlyForLowConfidence`

Expected: FAIL with `undefined: shouldUseRefineASRFallback`

- [ ] **Step 5: Write the failing test for expanded fallback window clamping**

```go
func TestBuildExpandedFallbackWindowClampsToBoundsAndNextSegment(t *testing.T) {
	start, end := buildExpandedFallbackWindow(2, 20, 22, 100, 4)
	if start != 0 || end != 22 {
		t.Fatalf("buildExpandedFallbackWindow() = (%d, %d), want (0, 22)", start, end)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildExpandedFallbackWindowClampsToBoundsAndNextSegment`

Expected: FAIL with `undefined: buildExpandedFallbackWindow`

- [ ] **Step 7: Write minimal helper implementations**

```go
func buildTranscriptFromCoarseItems(items []CoarseItem, startSec int, endSec int) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.EndSec <= startSec || item.StartSec >= endSec {
			continue
		}
		text := normalizeText(item.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func shouldUseRefineASRFallback(boundaryConfidence string) bool {
	return strings.TrimSpace(boundaryConfidence) == "low"
}

func buildExpandedFallbackWindow(startSec int, endSec int, nextStartSec int, videoDurationSec int, expandSec int) (int, int) {
	if expandSec <= 0 {
		expandSec = 3
	}
	start := startSec - expandSec
	if start < 0 {
		start = 0
	}
	end := endSec + expandSec
	if nextStartSec > 0 && end > nextStartSec {
		end = nextStartSec
	}
	if videoDurationSec > 0 && end > videoDurationSec {
		end = videoDurationSec
	}
	if end <= start {
		end = start + 1
		if nextStartSec > 0 && end > nextStartSec {
			end = nextStartSec
		}
		if videoDurationSec > 0 && end > videoDurationSec {
			end = videoDurationSec
		}
	}
	return start, end
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestBuildTranscriptFromCoarseItemsConcatenatesOverlapsInOrder|TestShouldUseRefineASRFallbackOnlyForLowConfidence|TestBuildExpandedFallbackWindowClampsToBoundsAndNextSegment'`

Expected: PASS

### Task 2: Add failing tests for per-segment transcript selection

**Files:**
- Create: `video-service/internal/worker/vectorworker/tasks/asr_refine_input_test.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
- Test: `video-service/internal/worker/vectorworker/tasks/asr_refine_input_test.go`

- [ ] **Step 1: Write the failing test for coarse-text-only path**

```go
package tasks

import (
	"context"
	"testing"
)

func TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow(t *testing.T) {
	called := false
	input, start, end, err := buildRefineSegmentInput(context.Background(), refineInputJob{
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
	if start != 60 || end != 120 {
		t.Fatalf("window = (%d, %d), want (60, 120)", start, end)
	}
	if input != "定义\n这一段在讲定义。" {
		t.Fatalf("input = %q", input)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow`

Expected: FAIL with `undefined: buildRefineSegmentInput` and `undefined: refineInputJob`

- [ ] **Step 3: Write the failing test for low-confidence single-shot fallback**

```go
func TestBuildRefineSegmentInputUsesSingleShotASRForLowConfidence(t *testing.T) {
	var calls int
	var gotStart int
	var gotEnd int
	input, start, end, err := buildRefineSegmentInput(context.Background(), refineInputJob{
		StartSec:           60,
		EndSec:             120,
		NextStartSec:       123,
		Summary:            "例题",
		BoundaryConfidence: "low",
	}, []CoarseItem{{StartSec: 60, EndSec: 120, Text: "coarse text"}}, 200, func(_ context.Context, startSec int, endSec int) (string, error) {
		calls++
		gotStart = startSec
		gotEnd = endSec
		return "低置信度段补识别文本。", nil
	})
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("ASR calls = %d, want 1", calls)
	}
	if gotStart != 57 || gotEnd != 123 {
		t.Fatalf("ASR window = (%d, %d), want (57, 123)", gotStart, gotEnd)
	}
	if start != 57 || end != 123 {
		t.Fatalf("returned window = (%d, %d), want (57, 123)", start, end)
	}
	if input != "例题\n低置信度段补识别文本。" {
		t.Fatalf("input = %q", input)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildRefineSegmentInputUsesSingleShotASRForLowConfidence`

Expected: FAIL with `undefined: buildRefineSegmentInput`

- [ ] **Step 5: Write the failing test for fallback-to-coarse-on-ASR-error**

```go
func TestBuildRefineSegmentInputFallsBackToCoarseTextWhenLowConfidenceASRFails(t *testing.T) {
	var calls int
	input, start, end, err := buildRefineSegmentInput(context.Background(), refineInputJob{
		StartSec:           30,
		EndSec:             70,
		NextStartSec:       0,
		Summary:            "总结",
		BoundaryConfidence: "low",
	}, []CoarseItem{{StartSec: 0, EndSec: 90, Text: "这部分是总结内容。"}}, 100, func(context.Context, int, int) (string, error) {
		calls++
		return "", context.DeadlineExceeded
	})
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("ASR calls = %d, want 1", calls)
	}
	if start != 30 || end != 70 {
		t.Fatalf("returned window = (%d, %d), want original (30, 70)", start, end)
	}
	if input != "总结\n这部分是总结内容。" {
		t.Fatalf("input = %q", input)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildRefineSegmentInputFallsBackToCoarseTextWhenLowConfidenceASRFails`

Expected: FAIL with `undefined: buildRefineSegmentInput`

- [ ] **Step 7: Write minimal implementation for segment input selection**

```go
type refineInputJob struct {
	StartSec           int
	EndSec             int
	NextStartSec       int
	Summary            string
	BoundaryConfidence string
}

func buildRefineSegmentInput(ctx context.Context, job refineInputJob, coarseItems []CoarseItem, videoDurationSec int, transcribeRange func(context.Context, int, int) (string, error)) (string, int, int, error) {
	coarseText := buildTranscriptFromCoarseItems(coarseItems, job.StartSec, job.EndSec)
	combine := func(text string) string {
		base := strings.TrimSpace(job.Summary)
		text = strings.TrimSpace(text)
		if base == "" {
			return text
		}
		if text == "" {
			return base
		}
		return base + "\n" + text
	}
	if !shouldUseRefineASRFallback(job.BoundaryConfidence) {
		return combine(coarseText), job.StartSec, job.EndSec, nil
	}
	start, end := buildExpandedFallbackWindow(job.StartSec, job.EndSec, job.NextStartSec, videoDurationSec, 3)
	text, err := transcribeRange(ctx, start, end)
	if err != nil {
		return combine(coarseText), job.StartSec, job.EndSec, nil
	}
	return combine(normalizeText(text)), start, end, nil
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow|TestBuildRefineSegmentInputUsesSingleShotASRForLowConfidence|TestBuildRefineSegmentInputFallsBackToCoarseTextWhenLowConfidenceASRFails'`

Expected: PASS

### Task 3: Refactor `RefineSegmentsASRAndEmbed` to use the new path

**Files:**
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
- Test: `video-service/internal/worker/vectorworker/tasks/asr_coarse_text_test.go`
- Test: `video-service/internal/worker/vectorworker/tasks/asr_refine_input_test.go`

- [ ] **Step 1: Write the failing test for default coarse-text transcript path integration**

Add this test to `asr_refine_input_test.go`:

```go
func TestBuildRefineSegmentInputUsesContentSummaryOnlyWhenCoarseTextIsEmpty(t *testing.T) {
	input, start, end, err := buildRefineSegmentInput(context.Background(), refineInputJob{
		StartSec:           10,
		EndSec:             20,
		Summary:            "标题",
		BoundaryConfidence: "high",
	}, nil, 100, func(context.Context, int, int) (string, error) {
		return "", nil
	})
	if err != nil {
		t.Fatalf("buildRefineSegmentInput error = %v", err)
	}
	if start != 10 || end != 20 {
		t.Fatalf("window = (%d, %d), want (10, 20)", start, end)
	}
	if input != "标题" {
		t.Fatalf("input = %q, want %q", input, "标题")
	}
}
```

- [ ] **Step 2: Run test to verify current helper behavior fails if not yet wired**

Run: `go test ./internal/worker/vectorworker/tasks -run TestBuildRefineSegmentInputUsesContentSummaryOnlyWhenCoarseTextIsEmpty`

Expected: PASS if helper already satisfies this; otherwise FAIL and then fix before continuing.

- [ ] **Step 3: Refactor `RefineSegmentsASRAndEmbed` job preparation to carry real confidence and coarse items**

Replace the job-building portion with code shaped like this:

```go
func RefineSegmentsASRAndEmbed(ctx context.Context, db *gorm.DB, ff *transcode.FFmpegTranscoder, client openAICompatClient, tmpRoot string, localVideo string, videoID uint64, taskID string, videoDurationSec int, asrWorkers int, embedBatch int, coarseItems []CoarseItem) error {
	// existing validation and segment query stay in place
	jobList := make([]job, 0, len(segs))
	for i, s := range segs {
		nextStartSec := 0
		if i+1 < len(segs) {
			nextStartSec = segs[i+1].StartTimeSec
		}
		jobList = append(jobList, job{
			JobIndex:           len(jobList),
			ID:                 s.ID,
			StartSec:           s.StartTimeSec,
			EndSec:             s.EndTimeSec,
			NextStartSec:       nextStartSec,
			Summary:            strings.TrimSpace(s.ContentSummary),
			BoundaryConfidence: normalizePersistedBoundaryConfidence(s.BoundaryReason),
		})
	}
```

Also add this small parser in `asr.go` so the first iteration can recover confidence without schema changes:

```go
func normalizePersistedBoundaryConfidence(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	for _, confidence := range []string{"low", "medium", "high"} {
		if strings.Contains(raw, confidence) {
			return confidence
		}
	}
	return ""
}
```

- [ ] **Step 4: Replace multi-probe runtime path with single input-builder flow**

Inside the worker loop, replace `alignSegmentForRefine(...)` usage with code shaped like this:

```go
	transcribeRange := func(runCtx context.Context, startSec int, endSec int) (string, error) {
		dur := endSec - startSec
		if dur <= 0 {
			return "", nil
		}
		audioPath := filepath.Join(tmpRoot, fmt.Sprintf("%s_refine_%d_%d_%d.wav", taskID, j.ID, startSec, endSec))
		_ = os.Remove(audioPath)
		extractCtx, cancelExtract := context.WithTimeout(runCtx, 8*time.Minute)
		err := ff.ExtractAudioSegment(extractCtx, localVideo, audioPath, startSec, dur)
		cancelExtract()
		if err != nil {
			_ = os.Remove(audioPath)
			return "", err
		}
		oneASRCtx, cancelOneASR := context.WithTimeout(runCtx, 12*time.Minute)
		text, err := client.Transcribe(oneASRCtx, audioPath)
		cancelOneASR()
		_ = os.Remove(audioPath)
		if err != nil {
			return "", err
		}
		return normalizeText(text), nil
	}

	input, startSec, endSec, err := buildRefineSegmentInput(asrCtx, refineInputJob{
		StartSec:           j.StartSec,
		EndSec:             j.EndSec,
		NextStartSec:       j.NextStartSec,
		Summary:            j.Summary,
		BoundaryConfidence: j.BoundaryConfidence,
	}, coarseItems, videoDurationSec, transcribeRange)
```

- [ ] **Step 5: Update result handling and logs to match the new behavior**

Keep the existing result channel flow, but update the log block to use:

```go
zap.L().Debug("vectorize_hierarchical_refine_input_ready",
	zap.Uint64("video_id", videoID),
	zap.String("task_id", taskID),
	zap.Uint64("seg_id", j.ID),
	zap.String("boundary_confidence", j.BoundaryConfidence),
	zap.Int("start_sec", startSec),
	zap.Int("end_sec", endSec),
	zap.Bool("used_refine_asr", shouldUseRefineASRFallback(j.BoundaryConfidence)),
	zap.Int("input_chars", len(input)))
```

Return results with `Input: input`, `StartSec: startSec`, and `EndSec: endSec`.

- [ ] **Step 6: Run targeted task tests**

Run: `go test ./internal/worker/vectorworker/tasks -run 'TestBuildTranscriptFromCoarseItemsConcatenatesOverlapsInOrder|TestShouldUseRefineASRFallbackOnlyForLowConfidence|TestBuildExpandedFallbackWindowClampsToBoundsAndNextSegment|TestBuildRefineSegmentInputUsesCoarseTextWhenConfidenceIsNotLow|TestBuildRefineSegmentInputUsesSingleShotASRForLowConfidence|TestBuildRefineSegmentInputFallsBackToCoarseTextWhenLowConfidenceASRFails|TestBuildRefineSegmentInputUsesContentSummaryOnlyWhenCoarseTextIsEmpty'`

Expected: PASS

### Task 4: Wire coarse items through `task.go` and run package verification

**Files:**
- Modify: `video-service/internal/worker/vectorworker/task.go`
- Modify: `video-service/internal/worker/vectorworker/tasks/asr.go`
- Test: `video-service/internal/worker/vectorworker/tasks/...`
- Test: `video-service/internal/worker/vectorworker/...`

- [ ] **Step 1: Write the failing compile-time integration change**

Update the wrapper and both call sites to pass `coarseItems`:

```go
func refineSegmentsASRAndEmbed(ctx context.Context, db *gorm.DB, ff *transcode.FFmpegTranscoder, client *openAICompatClient, tmpRoot string, localVideo string, videoID uint64, taskID string, videoDurationSec int, asrWorkers int, embedBatch int, coarseItems []tasks.CoarseItem) error {
	return tasks.RefineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, videoDurationSec, asrWorkers, embedBatch, coarseItems)
}
```

And replace the two current invocations with:

```go
if err := refineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, durationSec, asrWorkers, embedBatch, nil); err != nil {
	return err
}
```

for resume mode, and:

```go
if err := refineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, durationSec, asrWorkers, embedBatch, []tasks.CoarseItem(coarseItems)); err != nil {
	return err
}
```

for the fresh hierarchical path.

- [ ] **Step 2: Run package tests to verify the compile break is resolved**

Run: `go test ./internal/worker/vectorworker/...`

Expected: PASS

- [ ] **Step 3: Run focused module verification**

Run: `go test ./internal/worker/vectorworker/tasks`

Expected: PASS

- [ ] **Step 4: Run broader worker verification**

Run: `go test ./internal/worker/vectorworker/...`

Expected: PASS

- [ ] **Step 5: Run service-level verification**

Run: `go test ./...`

Expected: PASS, or if unrelated external-environment-sensitive tests fail, record the exact failing package and stop there.

## Self-Review Notes

- Spec coverage:
  - coarse-text default path: covered by Task 1 and Task 3
  - low-confidence-only refine fallback: covered by Task 1 and Task 2
  - single-shot expanded-window ASR: covered by Task 1 and Task 2
  - no schema change: preserved by Task 3 parser approach and Task 4 wiring
  - fallback-to-coarse on ASR error: covered by Task 2
- Placeholder scan: no `TODO`, `TBD`, or unspecified test commands remain.
- Type consistency:
  - `CoarseItem`, `RefineSegmentsASRAndEmbed`, and wrapper signatures are named consistently across tasks.
  - Helper names are consistent across tests and implementation steps.
