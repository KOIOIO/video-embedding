# Vector Worker Tail Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `hierarchical` 向量化链路增加“尾部保守对齐”，让片段在一句话讲完后再结束，默认最多多保留 3 秒。

**Architecture:** 保留现有“粗分段 ASR -> LLM 细分段 -> 细分段二次 ASR -> embedding”链路，只在细分段补处理阶段增加一个 worker 内部的尾部校准层。校准层先按当前 `end_time` 转写文本，再用纯逻辑规则判断是否像半句；若像半句，则按 1 秒步长向后试探，命中更自然句尾或达到上限后，将校准后的 `end_time` 连同 embedding/status 一起写回数据库。

**Tech Stack:** Go + GORM + FFmpeg + OpenAI-compatible ASR/Embedding + zap + Go test

---

## Context

- 实际代码入口在 `nlp-video-project/internal/worker/vectorworker/app.go`。
- `hierarchical` 模式主流程在 `nlp-video-project/internal/worker/vectorworker/task.go`。
- 细分段二次 ASR 和 embedding 在 `nlp-video-project/internal/worker/vectorworker/tasks/asr.go`。
- 当前 `EduVideoSegment` 已经有 `start_time` / `end_time` 字段，不需要改表。
- 这次只改 worker 内部算法和配置，不改前端、接口协议、数据库表结构。

## File Structure

**Create:**
- `nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment.go`
  - 纯逻辑模块：配置标准化、句尾判断、下一次试探结束时间计算。
- `nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment_test.go`
  - 纯逻辑单元测试。
- `nlp-video-project/internal/worker/vectorworker/tasks/asr_tail_alignment_test.go`
  - 细分段尾部试探 helper 测试。

**Modify:**
- `nlp-video-project/internal/config/types.go`
  - 增加 `VectorWorker` 尾部对齐配置项。
- `nlp-video-project/internal/worker/vectorworker/app.go`
  - 读取新配置、填默认值、记录日志、向任务链路透传。
- `nlp-video-project/internal/worker/vectorworker/task.go`
  - 把尾部对齐配置和视频总时长传入 `RefineSegmentsASRAndEmbed`。
- `nlp-video-project/internal/worker/vectorworker/tasks/asr.go`
  - 在二次 ASR 前增加尾部试探逻辑，并把校准后的 `end_time` 一起写库。
- `nlp-video-project/configs/video.yml`
  - 增加显式配置键，便于本地调参。
- `nlp-video-project/configs/video_prod.yml`
  - 增加显式配置键，便于生产调参。

**Docs to consult while implementing:**
- `nlp-video-project/docs/superpowers/specs/2026-04-28-vector-worker-tail-alignment-design.md`

---

### Task 1: Add Tail Alignment Pure Logic

**Files:**
- Create: `nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment.go`
- Test: `nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment_test.go`

- [ ] **Step 1: Write the failing test for config normalization and sentence-end detection**

```go
package tasks

import "testing"

func TestNormalizeTailAlignmentConfigDefaults(t *testing.T) {
	got := NormalizeTailAlignmentConfig(TailAlignmentConfig{})
	if got.Enabled {
		t.Fatalf("Enabled = true, want false before startup defaulting")
	}
	if got.MaxExtendSec != 3 {
		t.Fatalf("MaxExtendSec = %d, want 3", got.MaxExtendSec)
	}
	if got.ProbeStepSec != 1 {
		t.Fatalf("ProbeStepSec = %d, want 1", got.ProbeStepSec)
	}
	if got.MaxOverlapSec != 6 {
		t.Fatalf("MaxOverlapSec = %d, want 6", got.MaxOverlapSec)
	}
}

func TestLooksLikeSentenceEnd(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "punctuation", text: "所以这一步就完成了。", want: true},
		{name: "closing phrase", text: "这一题我们就讲到这里", want: true},
		{name: "connector tail", text: "接下来我们来看", want: false},
		{name: "half sentence", text: "所以这里我们可以得到", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LooksLikeSentenceEnd(tt.text)
			if got != tt.want {
				t.Fatalf("LooksLikeSentenceEnd(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestNeedsTailExtension(t *testing.T) {
	if !NeedsTailExtension("所以这里我们可以得到") {
		t.Fatal("NeedsTailExtension returned false for half sentence")
	}
	if NeedsTailExtension("所以这里我们可以得到结论。") {
		t.Fatal("NeedsTailExtension returned true for complete sentence")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run "TestNormalizeTailAlignmentConfigDefaults|TestLooksLikeSentenceEnd|TestNeedsTailExtension" -v`

Expected: FAIL with undefined `TailAlignmentConfig`, `NormalizeTailAlignmentConfig`, `LooksLikeSentenceEnd`, or `NeedsTailExtension`

- [ ] **Step 3: Write the minimal tail-alignment logic**

```go
package tasks

import "strings"

type TailAlignmentConfig struct {
	Enabled       bool
	MaxExtendSec  int
	ProbeStepSec  int
	MaxOverlapSec int
}

var sentenceEndTokens = []string{"。", "！", "？", ".", "!", "?"}

var sentenceEndPhrases = []string{
	"讲到这里",
	"到这里",
	"总结一下",
	"就是这样",
	"讲完了",
}

var trailingConnectors = []string{
	"所以",
	"然后",
	"但是",
	"因为",
	"如果",
	"接下来",
	"我们来看",
	"也就是说",
}

func NormalizeTailAlignmentConfig(cfg TailAlignmentConfig) TailAlignmentConfig {
	if cfg.MaxExtendSec <= 0 {
		cfg.MaxExtendSec = 3
	}
	if cfg.ProbeStepSec <= 0 {
		cfg.ProbeStepSec = 1
	}
	if cfg.MaxOverlapSec <= 0 {
		cfg.MaxOverlapSec = 6
	}
	return cfg
}

func LooksLikeSentenceEnd(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, token := range sentenceEndTokens {
		if strings.HasSuffix(text, token) {
			return true
		}
	}
	for _, phrase := range sentenceEndPhrases {
		if strings.HasSuffix(text, phrase) {
			return true
		}
	}
	for _, connector := range trailingConnectors {
		if strings.HasSuffix(text, connector) {
			return false
		}
	}
	return false
}

func NeedsTailExtension(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return !LooksLikeSentenceEnd(text)
}
```

- [ ] **Step 4: Add the failing test for next probe end-time calculation**

```go
func TestNextAlignedEndSec(t *testing.T) {
	cfg := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       true,
		MaxExtendSec:  3,
		ProbeStepSec:  1,
		MaxOverlapSec: 2,
	})

	tests := []struct {
		name             string
		currentEndSec    int
		originalEndSec   int
		nextSegmentStart int
		videoDurationSec int
		want             int
	}{
		{name: "normal step", currentEndSec: 10, originalEndSec: 10, nextSegmentStart: 20, videoDurationSec: 60, want: 11},
		{name: "extend limit", currentEndSec: 13, originalEndSec: 10, nextSegmentStart: 20, videoDurationSec: 60, want: 13},
		{name: "overlap limit", currentEndSec: 10, originalEndSec: 10, nextSegmentStart: 11, videoDurationSec: 60, want: 12},
		{name: "duration limit", currentEndSec: 10, originalEndSec: 10, nextSegmentStart: 40, videoDurationSec: 11, want: 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NextAlignedEndSec(tt.currentEndSec, tt.originalEndSec, tt.nextSegmentStart, tt.videoDurationSec, cfg)
			if got != tt.want {
				t.Fatalf("NextAlignedEndSec() = %d, want %d", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 5: Run test to verify it fails**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run TestNextAlignedEndSec -v`

Expected: FAIL with undefined `NextAlignedEndSec`

- [ ] **Step 6: Implement bounded probe-end calculation**

```go
func NextAlignedEndSec(currentEndSec int, originalEndSec int, nextSegmentStartSec int, videoDurationSec int, cfg TailAlignmentConfig) int {
	cfg = NormalizeTailAlignmentConfig(cfg)
	limit := originalEndSec + cfg.MaxExtendSec
	if videoDurationSec > 0 && videoDurationSec < limit {
		limit = videoDurationSec
	}
	if nextSegmentStartSec > 0 {
		overlapLimit := nextSegmentStartSec + cfg.MaxOverlapSec
		if overlapLimit < limit {
			limit = overlapLimit
		}
	}
	next := currentEndSec + cfg.ProbeStepSec
	if next > limit {
		next = limit
	}
	if next < currentEndSec {
		return currentEndSec
	}
	return next
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run "TestNormalizeTailAlignmentConfigDefaults|TestLooksLikeSentenceEnd|TestNeedsTailExtension|TestNextAlignedEndSec" -v`

Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment.go nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment_test.go
git commit -m "test: add tail alignment heuristics"
```

---

### Task 2: Plumb Tail Alignment Config Through Worker Startup

**Files:**
- Modify: `nlp-video-project/internal/config/types.go`
- Modify: `nlp-video-project/internal/worker/vectorworker/app.go`
- Modify: `nlp-video-project/internal/worker/vectorworker/task.go`
- Modify: `nlp-video-project/configs/video.yml`
- Modify: `nlp-video-project/configs/video_prod.yml`
- Test: `nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment_test.go`

- [ ] **Step 1: Extend the test to cover explicit config values and disabled mode**

```go
func TestNormalizeTailAlignmentConfigKeepsExplicitValues(t *testing.T) {
	got := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       false,
		MaxExtendSec:  5,
		ProbeStepSec:  2,
		MaxOverlapSec: 4,
	})
	if got.Enabled {
		t.Fatal("Enabled = true, want false")
	}
	if got.MaxExtendSec != 5 || got.ProbeStepSec != 2 || got.MaxOverlapSec != 4 {
		t.Fatalf("unexpected config: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run TestNormalizeTailAlignmentConfigKeepsExplicitValues -v`

Expected: PASS if Task 1 left `Enabled` untouched; FAIL if Task 1 incorrectly forced `Enabled=true`

- [ ] **Step 3: Fix config normalization so disabled mode is preserved, then add startup config fields**

```go
// nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment.go
func NormalizeTailAlignmentConfig(cfg TailAlignmentConfig) TailAlignmentConfig {
	if cfg.MaxExtendSec <= 0 {
		cfg.MaxExtendSec = 3
	}
	if cfg.ProbeStepSec <= 0 {
		cfg.ProbeStepSec = 1
	}
	if cfg.MaxOverlapSec <= 0 {
		cfg.MaxOverlapSec = 6
	}
	return cfg
}
```

```go
// nlp-video-project/internal/config/types.go
type VectorWorkerConfig struct {
	Mode                     string `yaml:"Mode"`
	CoarseSegmentSec         int    `yaml:"CoarseSegmentSec"`
	RefineMinSegmentSec      int    `yaml:"RefineMinSegmentSec"`
	RefineMaxSegmentSec      int    `yaml:"RefineMaxSegmentSec"`
	LLMModel                 string `yaml:"LLMModel"`
	LLMTimeoutMinutes        int    `yaml:"LLMTimeoutMinutes"`
	TailAlignmentEnabled     bool   `yaml:"TailAlignmentEnabled"`
	TailAlignmentMaxExtendSec int   `yaml:"TailAlignmentMaxExtendSec"`
	TailAlignmentProbeStepSec int   `yaml:"TailAlignmentProbeStepSec"`
	TailAlignmentMaxOverlapSec int  `yaml:"TailAlignmentMaxOverlapSec"`

	SegmentWindowSec   int `yaml:"SegmentWindowSec"`
	SegmentStepSec     int `yaml:"SegmentStepSec"`
	ASRWorkers         int `yaml:"ASRWorkers"`
	CoarseWorkers      int `yaml:"CoarseWorkers"`
	EmbedBatch         int `yaml:"EmbedBatch"`
	SampleCount        int `yaml:"SampleCount"`
	SampleDurSec       int `yaml:"SampleDurSec"`
	TaskTimeoutMinutes int `yaml:"TaskTimeoutMinutes"`
	ShutdownTimeoutSec int `yaml:"ShutdownTimeoutSec"`
}
```

```go
// nlp-video-project/internal/worker/vectorworker/app.go
tailCfg := tasks.NormalizeTailAlignmentConfig(tasks.TailAlignmentConfig{
	Enabled:       cfg.VectorWorker.TailAlignmentEnabled,
	MaxExtendSec:  cfg.VectorWorker.TailAlignmentMaxExtendSec,
	ProbeStepSec:  cfg.VectorWorker.TailAlignmentProbeStepSec,
	MaxOverlapSec: cfg.VectorWorker.TailAlignmentMaxOverlapSec,
})

tailAlignmentExplicitlyConfigured := cfg.VectorWorker.TailAlignmentEnabled ||
	cfg.VectorWorker.TailAlignmentMaxExtendSec > 0 ||
	cfg.VectorWorker.TailAlignmentProbeStepSec > 0 ||
	cfg.VectorWorker.TailAlignmentMaxOverlapSec > 0
if !tailAlignmentExplicitlyConfigured {
	tailCfg.Enabled = true
}

zap.L().Info("vector_worker_start",
	// existing fields...
	zap.Bool("tail_alignment_enabled", tailCfg.Enabled),
	zap.Int("tail_alignment_max_extend_sec", tailCfg.MaxExtendSec),
	zap.Int("tail_alignment_probe_step_sec", tailCfg.ProbeStepSec),
	zap.Int("tail_alignment_max_overlap_sec", tailCfg.MaxOverlapSec),
)
```

- [ ] **Step 4: Thread the config and video duration into the task/refine call chain**

```go
// nlp-video-project/internal/worker/vectorworker/task.go
func refineSegmentsASRAndEmbed(
	ctx context.Context,
	db *gorm.DB,
	ff *transcode.FFmpegTranscoder,
	client *openAICompatClient,
	tmpRoot string,
	localVideo string,
	videoID uint64,
	taskID string,
	videoDurationSec int,
	asrWorkers int,
	embedBatch int,
	tailCfg tasks.TailAlignmentConfig,
) error {
	return tasks.RefineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, videoDurationSec, asrWorkers, embedBatch, tailCfg)
}
```

```go
// update the two call sites in task.go
if err := refineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, durationSec, asrWorkers, embedBatch, tailCfg); err != nil {
	return err
}
```

```go
// update the handleVectorizeTask signature in task.go
func handleVectorizeTask(
	ctx context.Context,
	db *gorm.DB,
	store *objectstorage.RustFS,
	ff *transcode.FFmpegTranscoder,
	client *openAICompatClient,
	tmpRoot string,
	mode string,
	windowSec int,
	stepSec int,
	asrWorkers int,
	coarseWorkers int,
	embedBatch int,
	sampleCount int,
	sampleDurSec int,
	coarseSegmentSec int,
	refineMinSegmentSec int,
	refineMaxSegmentSec int,
	llmModel string,
	llmTimeoutMinutes int,
	tailCfg tasks.TailAlignmentConfig,
	videoID uint64,
	taskID string,
	rawKey string,
) error {
```

```go
// update the call in app.go
lastErr = handleVectorizeTask(taskCtx, db, store, ff, client, tmpRoot, mode, windowSec, stepSec, asrWorkers, coarseWorkers, embedBatch, sampleCount, sampleDurSec, coarseSegmentSec, refineMinSegmentSec, refineMaxSegmentSec, llmModel, llmTimeoutMinutes, tailCfg, task.VideoID, task.TaskID, task.RawKey)
```

- [ ] **Step 5: Add explicit config keys to both YAML files**

```yaml
# nlp-video-project/configs/video.yml
VectorWorker:
  TailAlignmentEnabled: true
  TailAlignmentMaxExtendSec: 3
  TailAlignmentProbeStepSec: 1
  TailAlignmentMaxOverlapSec: 6
```
```

```yaml
# nlp-video-project/configs/video_prod.yml
VectorWorker:
  TailAlignmentEnabled: true
  TailAlignmentMaxExtendSec: 3
  TailAlignmentProbeStepSec: 1
  TailAlignmentMaxOverlapSec: 6
```
```

- [ ] **Step 6: Run tests to verify config logic and package compile still pass**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run "TestNormalizeTailAlignmentConfigDefaults|TestNormalizeTailAlignmentConfigKeepsExplicitValues|TestLooksLikeSentenceEnd|TestNeedsTailExtension|TestNextAlignedEndSec" -v`

Expected: PASS

Run: `go test ./nlp-video-project/internal/worker/vectorworker/... -run TestDoesNotExist`

Expected: package compile succeeds with `ok` / `[no test files]`

- [ ] **Step 7: Commit**

```bash
git add nlp-video-project/internal/config/types.go nlp-video-project/internal/worker/vectorworker/app.go nlp-video-project/internal/worker/vectorworker/task.go nlp-video-project/configs/video.yml nlp-video-project/configs/video_prod.yml nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment.go nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment_test.go
git commit -m "feat: add vector worker tail alignment config"
```

---

### Task 3: Add a Testable Tail-Probing Helper for Refine ASR

**Files:**
- Modify: `nlp-video-project/internal/worker/vectorworker/tasks/asr.go`
- Create: `nlp-video-project/internal/worker/vectorworker/tasks/asr_tail_alignment_test.go`

- [ ] **Step 1: Write the failing test for probe-until-sentence-end behavior**

```go
package tasks

import (
	"context"
	"fmt"
	"testing"
)

func TestAlignSegmentTailStopsAtNaturalSentenceEnd(t *testing.T) {
	cfg := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       true,
		MaxExtendSec:  3,
		ProbeStepSec:  1,
		MaxOverlapSec: 6,
	})

	responses := map[int]string{
		10: "所以这里我们可以得到",
		11: "所以这里我们可以得到这个结论",
		12: "所以这里我们可以得到这个结论。",
	}

	called := make([]int, 0, 3)
	probe := func(endSec int) (string, error) {
		called = append(called, endSec)
		text, ok := responses[endSec]
		if !ok {
			return "", fmt.Errorf("unexpected endSec=%d", endSec)
		}
		return text, nil
	}

	gotEnd, gotText, err := alignSegmentTail(context.Background(), cfg, 0, 10, 20, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 12 {
		t.Fatalf("gotEnd = %d, want 12", gotEnd)
	}
	if gotText != responses[12] {
		t.Fatalf("gotText = %q, want %q", gotText, responses[12])
	}
	if len(called) != 3 {
		t.Fatalf("probe calls = %v, want [10 11 12]", called)
	}
}
```

- [ ] **Step 2: Add the failing tests for disabled mode and overlap ceiling**

```go
func TestAlignSegmentTailSkipsWhenDisabled(t *testing.T) {
	probeCount := 0
	probe := func(endSec int) (string, error) {
		probeCount++
		return "接下来我们来看", nil
	}

	gotEnd, _, err := alignSegmentTail(context.Background(), TailAlignmentConfig{Enabled: false, MaxExtendSec: 3, ProbeStepSec: 1, MaxOverlapSec: 6}, 0, 10, 20, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 10 {
		t.Fatalf("gotEnd = %d, want 10", gotEnd)
	}
	if probeCount != 1 {
		t.Fatalf("probeCount = %d, want 1", probeCount)
	}
}

func TestAlignSegmentTailHonorsOverlapLimit(t *testing.T) {
	cfg := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       true,
		MaxExtendSec:  3,
		ProbeStepSec:  1,
		MaxOverlapSec: 1,
	})

	probe := func(endSec int) (string, error) {
		return "接下来我们来看", nil
	}

	gotEnd, _, err := alignSegmentTail(context.Background(), cfg, 0, 10, 10, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 11 {
		t.Fatalf("gotEnd = %d, want 11", gotEnd)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run "TestAlignSegmentTailStopsAtNaturalSentenceEnd|TestAlignSegmentTailSkipsWhenDisabled|TestAlignSegmentTailHonorsOverlapLimit" -v`

Expected: FAIL with undefined `alignSegmentTail`

- [ ] **Step 4: Implement the helper in `asr.go` before touching worker orchestration**

```go
func alignSegmentTail(
	ctx context.Context,
	cfg TailAlignmentConfig,
	startSec int,
	originalEndSec int,
	nextSegmentStartSec int,
	videoDurationSec int,
	probe func(endSec int) (string, error),
) (int, string, error) {
	cfg = NormalizeTailAlignmentConfig(cfg)
	text, err := probe(originalEndSec)
	if err != nil {
		return originalEndSec, "", err
	}
	if !cfg.Enabled {
		return originalEndSec, text, nil
	}
	if !NeedsTailExtension(text) {
		return originalEndSec, text, nil
	}

	currentEndSec := originalEndSec
	currentText := text
	for {
		nextEndSec := NextAlignedEndSec(currentEndSec, originalEndSec, nextSegmentStartSec, videoDurationSec, cfg)
		if nextEndSec <= currentEndSec {
			return currentEndSec, currentText, nil
		}
		nextText, err := probe(nextEndSec)
		if err != nil {
			return currentEndSec, currentText, err
		}
		currentEndSec = nextEndSec
		currentText = nextText
		if LooksLikeSentenceEnd(nextText) {
			return currentEndSec, currentText, nil
		}
		select {
		case <-ctx.Done():
			return currentEndSec, currentText, ctx.Err()
		default:
		}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run "TestAlignSegmentTailStopsAtNaturalSentenceEnd|TestAlignSegmentTailSkipsWhenDisabled|TestAlignSegmentTailHonorsOverlapLimit" -v`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add nlp-video-project/internal/worker/vectorworker/tasks/asr.go nlp-video-project/internal/worker/vectorworker/tasks/asr_tail_alignment_test.go
git commit -m "test: cover refine tail probing helper"
```

---

### Task 4: Apply Tail Alignment Inside RefineSegmentsASRAndEmbed

**Files:**
- Modify: `nlp-video-project/internal/worker/vectorworker/tasks/asr.go`
- Test: `nlp-video-project/internal/worker/vectorworker/tasks/asr_tail_alignment_test.go`

- [ ] **Step 1: Add the failing test for updating end_time with the aligned result**

```go
func TestAlignSegmentTailStopsAtMaxExtendWhenNoSentenceEnd(t *testing.T) {
	cfg := NormalizeTailAlignmentConfig(TailAlignmentConfig{
		Enabled:       true,
		MaxExtendSec:  2,
		ProbeStepSec:  1,
		MaxOverlapSec: 6,
	})

	called := make([]int, 0, 3)
	probe := func(endSec int) (string, error) {
		called = append(called, endSec)
		return "接下来我们来看", nil
	}

	gotEnd, _, err := alignSegmentTail(context.Background(), cfg, 0, 10, 30, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 12 {
		t.Fatalf("gotEnd = %d, want 12", gotEnd)
	}
	if len(called) != 3 {
		t.Fatalf("probe calls = %v, want [10 11 12]", called)
	}
}
```

- [ ] **Step 2: Run test to verify it fails if your helper returns too early or overshoots**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run TestAlignSegmentTailStopsAtMaxExtendWhenNoSentenceEnd -v`

Expected: FAIL if the helper does not stop exactly at the bounded max extension

- [ ] **Step 3: Update `RefineSegmentsASRAndEmbed` signature and job/result structs**

```go
func RefineSegmentsASRAndEmbed(
	ctx context.Context,
	db *gorm.DB,
	ff *transcode.FFmpegTranscoder,
	client openAICompatClient,
	tmpRoot string,
	localVideo string,
	videoID uint64,
	taskID string,
	videoDurationSec int,
	asrWorkers int,
	embedBatch int,
	tailCfg TailAlignmentConfig,
) error {
```

```go
type job struct {
	JobIndex      int
	ID            uint64
	StartSec      int
	EndSec        int
	NextStartSec  int
	Summary       string
}

type result struct {
	JobIndex int
	ID       uint64
	EndSec   int
	Input    string
	Err      error
}
```

- [ ] **Step 4: Build `jobList` with next-segment boundaries**

```go
for i, s := range segs {
	nextStartSec := 0
	if i+1 < len(segs) {
		nextStartSec = segs[i+1].StartTimeSec
	}
	jobList = append(jobList, job{
		JobIndex:     len(jobList),
		ID:           s.ID,
		StartSec:     s.StartTimeSec,
		EndSec:       s.EndTimeSec,
		NextStartSec: nextStartSec,
		Summary:      strings.TrimSpace(s.ContentSummary),
	})
}
```

- [ ] **Step 5: Replace the old single-pass ASR block with a probe closure + tail alignment call**

```go
probe := func(endSec int) (string, error) {
	dur := endSec - j.StartSec
	if dur <= 0 {
		return "", nil
	}
	audioPath := filepath.Join(tmpRoot, fmt.Sprintf("%s_refine_%d_%d_%d.wav", taskID, j.ID, j.StartSec, endSec))
	_ = os.Remove(audioPath)

	extractCtx, cancelExtract := context.WithTimeout(asrCtx, 8*time.Minute)
	err := ff.ExtractAudioSegment(extractCtx, localVideo, audioPath, j.StartSec, dur)
	cancelExtract()
	if err != nil {
		_ = os.Remove(audioPath)
		return "", err
	}
	oneASRCtx, cancelOneASR := context.WithTimeout(asrCtx, 12*time.Minute)
	text, err := client.Transcribe(oneASRCtx, audioPath)
	cancelOneASR()
	_ = os.Remove(audioPath)
	if err != nil {
		return "", err
	}
	return normalizeText(text), nil
}

alignedEndSec, text, err := alignSegmentTail(asrCtx, tailCfg, j.StartSec, j.EndSec, j.NextStartSec, videoDurationSec, probe)
	if err != nil {
		_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, Err: err})
		cancelASRAll()
		continue
	}
	base := strings.TrimSpace(j.Summary)
	combined := strings.TrimSpace(base + "\n" + text)
	if combined == "" {
		combined = base
	}
	_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, EndSec: alignedEndSec, Input: combined, Err: nil})
```

- [ ] **Step 6: Carry aligned end-times through result ordering and DB update**

```go
orderedInputs := make([]string, len(jobList))
orderedIDs := make([]uint64, len(jobList))
orderedEndSecs := make([]int, len(jobList))
for i := range orderedIDs {
	orderedIDs[i] = jobList[i].ID
	orderedEndSecs[i] = jobList[i].EndSec
}

for r := range results {
	got++
	if r.Err != nil && firstErr == nil {
		firstErr = r.Err
	}
	if r.JobIndex >= 0 && r.JobIndex < len(orderedInputs) {
		orderedInputs[r.JobIndex] = r.Input
		orderedEndSecs[r.JobIndex] = r.EndSec
	}
}
```

```go
type embeddingUpdate struct {
	ID        uint64
	EndSec    int
	Embedding pgvector.Vector
}

for k, id := range ids {
	v := normalizeEmbeddingDim(vecs[k], embeddingDim)
	if len(v) == 0 {
		continue
	}
	allUpdates = append(allUpdates, embeddingUpdate{
		ID:        id,
		EndSec:    orderedEndSecs[i+k],
		Embedding: pgvector.NewVector(v),
	})
}
```

```go
if err := tx.Model(&model.EduVideoSegment{}).
	Where("id = ? AND deleted = 0 AND status = 0", update.ID).
	Updates(map[string]any{
		"end_time":  update.EndSec,
		"embedding": update.Embedding,
		"status":    int16(1),
	}).Error; err != nil {
	return err
}
```

- [ ] **Step 7: Add structured logs for skipped/probed/extended cases**

```go
zap.L().Info("tail_alignment_start",
	zap.Uint64("video_id", videoID),
	zap.String("task_id", taskID),
	zap.Uint64("seg_id", j.ID),
	zap.Int("start_sec", j.StartSec),
	zap.Int("end_sec", j.EndSec),
)

zap.L().Info("tail_alignment_extended",
	zap.Uint64("video_id", videoID),
	zap.String("task_id", taskID),
	zap.Uint64("seg_id", j.ID),
	zap.Int("old_end_sec", j.EndSec),
	zap.Int("new_end_sec", alignedEndSec),
)
```

- [ ] **Step 8: Run targeted tests to verify tail-probing behavior passes**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run "TestAlignSegmentTailStopsAtNaturalSentenceEnd|TestAlignSegmentTailSkipsWhenDisabled|TestAlignSegmentTailHonorsOverlapLimit|TestAlignSegmentTailStopsAtMaxExtendWhenNoSentenceEnd" -v`

Expected: PASS

- [ ] **Step 9: Run package-level tests for the vector worker**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/...`

Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add nlp-video-project/internal/worker/vectorworker/tasks/asr.go nlp-video-project/internal/worker/vectorworker/tasks/asr_tail_alignment_test.go nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment.go nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment_test.go nlp-video-project/internal/worker/vectorworker/task.go nlp-video-project/internal/worker/vectorworker/app.go
git commit -m "feat: align hierarchical segment tails conservatively"
```

---

### Task 5: Full Verification and Cleanup

**Files:**
- Modify: `nlp-video-project/internal/worker/vectorworker/tasks/asr.go`
- Modify: `nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment.go`
- Test: `nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment_test.go`
- Test: `nlp-video-project/internal/worker/vectorworker/tasks/asr_tail_alignment_test.go`

- [ ] **Step 1: Add one regression test for complete sentences staying unchanged**

```go
func TestAlignSegmentTailKeepsCompleteSentenceUnchanged(t *testing.T) {
	probeCount := 0
	probe := func(endSec int) (string, error) {
		probeCount++
		return "这一题我们就讲到这里。", nil
	}

	gotEnd, gotText, err := alignSegmentTail(context.Background(), NormalizeTailAlignmentConfig(TailAlignmentConfig{Enabled: true}), 0, 10, 30, 60, probe)
	if err != nil {
		t.Fatalf("alignSegmentTail error = %v", err)
	}
	if gotEnd != 10 {
		t.Fatalf("gotEnd = %d, want 10", gotEnd)
	}
	if gotText != "这一题我们就讲到这里。" {
		t.Fatalf("gotText = %q", gotText)
	}
	if probeCount != 1 {
		t.Fatalf("probeCount = %d, want 1", probeCount)
	}
}
```

- [ ] **Step 2: Run tests to verify the regression case passes**

Run: `go test ./nlp-video-project/internal/worker/vectorworker/tasks -run TestAlignSegmentTailKeepsCompleteSentenceUnchanged -v`

Expected: PASS

- [ ] **Step 3: Run the full backend test sweep**

Run: `go test ./nlp-video-project/...`

Expected: PASS

- [ ] **Step 4: Manually inspect logs during one hierarchical task run**

Run: `go run ./nlp-video-project/cmd/worker`

Expected:
- worker starts successfully
- startup log contains `tail_alignment_enabled`, `tail_alignment_max_extend_sec`, `tail_alignment_probe_step_sec`, `tail_alignment_max_overlap_sec`
- when a segment is extended, logs include `tail_alignment_start` and `tail_alignment_extended`

- [ ] **Step 5: Commit**

```bash
git add nlp-video-project/internal/worker/vectorworker/tasks/asr.go nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment.go nlp-video-project/internal/worker/vectorworker/tasks/tail_alignment_test.go nlp-video-project/internal/worker/vectorworker/tasks/asr_tail_alignment_test.go
git commit -m "test: verify vector worker tail alignment"
```

---

## Self-Review

### Spec coverage

- Worker-only scope: covered by Tasks 2-4.
- Default behavior “宁可多 1-3 秒，也别截半句”: covered by Tasks 1, 3, 4.
- No frontend/API/schema changes: preserved throughout.
- Configurable behavior: covered by Task 2.
- Logging and verification: covered by Tasks 4-5.

### Placeholder scan

- No `TBD` / `TODO` / “later” placeholders remain.
- Every code-changing step includes an explicit code block.
- Every verification step includes an exact command and expected result.

### Type consistency

- `TailAlignmentConfig`, `NormalizeTailAlignmentConfig`, `LooksLikeSentenceEnd`, `NeedsTailExtension`, `NextAlignedEndSec`, and `alignSegmentTail` are introduced once and reused consistently.
- `RefineSegmentsASRAndEmbed` and `handleVectorizeTask` signatures are updated consistently to carry `videoDurationSec` and `tailCfg`.

---

Plan complete and saved to `nlp-video-project/docs/superpowers/plans/2026-04-28-vector-worker-tail-alignment.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
