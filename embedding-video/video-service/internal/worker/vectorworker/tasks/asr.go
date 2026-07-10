package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/infrastructure/transcode"
	"nlp-video-analysis/internal/model"
	"nlp-video-analysis/internal/worker/antspool"
)

const maxRefineASRWorkers = 20

const defaultLowConfidenceExpandSec = 3

func normalizeRefineASRWorkers(asrWorkers int) int {
	if asrWorkers <= 0 {
		asrWorkers = 4
	}
	if asrWorkers > maxRefineASRWorkers {
		asrWorkers = maxRefineASRWorkers
	}
	return asrWorkers
}

func buildTranscriptFromCoarseItems(items []CoarseItem, startSec int, endSec int) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if item.EndSec <= startSec || item.StartSec >= endSec {
			continue
		}
		text := normalizeText(item.Text)
		if text == "" {
			continue
		}
		if len(parts) > 0 && slices.Contains(parts, text) {
			continue
		}
		parts = append(parts, text)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func shouldUseRefineASRFallback(boundaryConfidence string) bool {
	return NormalizeBoundaryConfidence(boundaryConfidence) == "low"
}

func buildExpandedFallbackWindow(startSec int, endSec int, nextStartSec int, videoDurationSec int, expandSec int) (int, int) {
	if expandSec <= 0 {
		expandSec = defaultLowConfidenceExpandSec
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
	if end <= start {
		return startSec, endSec
	}
	return start, end
}

func alignSegmentForRefine(
	ctx context.Context,
	seg LLMSegment,
	next *LLMSegment,
	videoDurationSec int,
	probeRange func(startSec int, endSec int) (string, error),
	fallbackTail func(context.Context, TailAlignmentConfig, int, int, int, int, func(int) (string, error)) (int, string, error),
) (int, int, string, bool, bool, error) {
	startMin, startMax, endMin, endMax := buildBoundaryWindows(seg.StartTimeSec, seg.EndTimeSec, videoDurationSec)
	startCandidates := make([]boundaryCandidate, 0, startMax-startMin+1)
	for sec := startMin; sec <= startMax; sec++ {
		endSec := sec + 1
		if endSec > startMax {
			endSec = startMax
		}
		if endSec <= sec {
			endSec = sec + 1
		}
		text, err := probeRange(sec, endSec)
		if err == nil {
			startCandidates = append(startCandidates, boundaryCandidate{Sec: sec, Text: text})
		}
	}
	endCandidates := make([]boundaryCandidate, 0, endMax-endMin+1)
	for sec := endMin; sec <= endMax; sec++ {
		startSec := sec - 1
		if startSec < endMin {
			startSec = endMin
		}
		if startSec >= sec {
			startSec = sec - 1
		}
		if startSec < 0 {
			startSec = 0
		}
		text, err := probeRange(startSec, sec)
		if err == nil {
			endCandidates = append(endCandidates, boundaryCandidate{Sec: sec, Text: text})
		}
	}
	if len(startCandidates) > 0 && len(endCandidates) > 0 {
		aligned := alignSegmentBoundaries(seg, next, boundaryAlignmentSnapshot{StartCandidates: startCandidates, EndCandidates: endCandidates})
		text, err := probeRange(aligned.StartTimeSec, aligned.EndTimeSec)
		if err == nil && strings.TrimSpace(text) != "" {
			return aligned.StartTimeSec, aligned.EndTimeSec, normalizeText(text), true, false, nil
		}
	}
	endSec, text, err := fallbackTail(ctx, NormalizeTailAlignmentConfig(TailAlignmentConfig{Enabled: true}), seg.StartTimeSec, seg.EndTimeSec, 0, videoDurationSec, func(endSec int) (string, error) {
		return probeRange(seg.StartTimeSec, endSec)
	})
	if err != nil {
		return seg.StartTimeSec, seg.EndTimeSec, "", false, true, err
	}
	return seg.StartTimeSec, endSec, normalizeText(text), false, true, nil
}

// openAICompatClient 定义 hierarchical 细分段补处理所需的最小 AI 能力接口。
type openAICompatClient interface {
	Transcribe(ctx context.Context, audioPath string) (string, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	ChatCompletionsWithTimeout(ctx context.Context, model string, prompt string, timeoutMinutes int) (string, error)
}

type summaryRewriter func(context.Context, string) (string, error)

type refineInputJob struct {
	StartSec           int
	EndSec             int
	NextStartSec       int
	Summary            string
	BoundaryConfidence string
}

type refineSegmentInputResult struct {
	Input       string
	Summary     string
	StartSec    int
	EndSec      int
	SummaryMode string
}

type RefineASRHints struct {
	CoarseItems       []CoarseItem
	Segments          []LLMSegment
	LLMModel          string
	LLMTimeoutMinutes int
}

type refineJob struct {
	JobIndex           int
	ID                 uint64
	SegmentIndex       int
	StartSec           int
	EndSec             int
	NextStartSec       int
	Summary            string
	StartAnchorText    string
	EndAnchorText      string
	BoundaryConfidence string
}

func buildRefineJobs(pending []model.EduVideoSegment, all []model.EduVideoSegment, hintsByIndex map[int]LLMSegment) []refineJob {
	nextStartByIndex := make(map[int]int, len(all))
	for i, s := range all {
		if i+1 < len(all) {
			nextStartByIndex[s.SegmentIndex] = all[i+1].StartTimeSec
		}
	}
	jobs := make([]refineJob, 0, len(pending))
	for _, s := range pending {
		boundaryConfidence := ""
		if hinted, ok := hintsByIndex[s.SegmentIndex]; ok {
			boundaryConfidence = hinted.BoundaryConfidence
		}
		jobs = append(jobs, refineJob{
			JobIndex:           len(jobs),
			ID:                 s.ID,
			SegmentIndex:       s.SegmentIndex,
			StartSec:           s.StartTimeSec,
			EndSec:             s.EndTimeSec,
			NextStartSec:       nextStartByIndex[s.SegmentIndex],
			Summary:            strings.TrimSpace(s.ContentSummary),
			StartAnchorText:    "",
			EndAnchorText:      "",
			BoundaryConfidence: boundaryConfidence,
		})
	}
	return jobs
}

func buildRefineSegmentInputWithSummaryRewrite(ctx context.Context, job refineInputJob, coarseItems []CoarseItem, videoDurationSec int, transcribeRange func(context.Context, int, int) (string, error), rewriteSummary summaryRewriter) (refineSegmentInputResult, error) {
	coarseText := buildTranscriptFromCoarseItems(coarseItems, job.StartSec, job.EndSec)
	seg := LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   job.StartSec,
		EndTimeSec:     job.EndSec,
		ContentSummary: job.Summary,
	}
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
	alignSummary := func(seg LLMSegment, text string) (LLMSegment, string) {
		alignedSeg, mode := alignSummaryToStableTextWithMode(seg, text)
		if mode == "original" || rewriteSummary == nil || !shouldUseLLMSummaryRewrite(text) {
			return alignedSeg, mode
		}
		rewritten, err := rewriteSummary(ctx, text)
		if err != nil {
			return alignedSeg, mode + "_llm_failed"
		}
		rewritten = normalizeSegmentTitle(rewritten)
		if strings.TrimSpace(rewritten) == "" {
			return alignedSeg, mode + "_llm_empty"
		}
		alignedSeg.ContentSummary = rewritten
		return alignedSeg, "llm_rewrite"
	}
	shouldRefine := shouldUseRefineASRFallback(job.BoundaryConfidence) || detectSummaryContentMismatch(LLMSegment{
		SegmentIndex:   0,
		StartTimeSec:   job.StartSec,
		EndTimeSec:     job.EndSec,
		ContentSummary: job.Summary,
	}, coarseText, "")
	if !shouldRefine {
		alignedSeg, summaryMode := alignSummary(seg, coarseText)
		zap.L().Debug("vectorize_hierarchical_summary_aligned",
			zap.String("mode", summaryMode),
			zap.Int("start_sec", job.StartSec),
			zap.Int("end_sec", job.EndSec),
			zap.String("old_summary", seg.ContentSummary),
			zap.String("new_summary", alignedSeg.ContentSummary),
			zap.Int("text_chars", len(coarseText)))
		return buildResult(alignedSeg.ContentSummary, coarseText, job.StartSec, job.EndSec, summaryMode), nil
	}
	start, end := buildExpandedFallbackWindow(job.StartSec, job.EndSec, job.NextStartSec, videoDurationSec, defaultLowConfidenceExpandSec)
	text, err := transcribeRange(ctx, start, end)
	if err != nil {
		alignedSeg, summaryMode := alignSummary(seg, coarseText)
		zap.L().Debug("vectorize_hierarchical_summary_aligned",
			zap.String("mode", summaryMode),
			zap.Int("start_sec", job.StartSec),
			zap.Int("end_sec", job.EndSec),
			zap.String("old_summary", seg.ContentSummary),
			zap.String("new_summary", alignedSeg.ContentSummary),
			zap.Int("text_chars", len(coarseText)))
		return buildResult(alignedSeg.ContentSummary, coarseText, job.StartSec, job.EndSec, summaryMode), nil
	}
	finalText := normalizeText(text)
	if finalText == "" {
		alignedSeg, summaryMode := alignSummary(seg, coarseText)
		zap.L().Debug("vectorize_hierarchical_summary_aligned",
			zap.String("mode", summaryMode),
			zap.Int("start_sec", job.StartSec),
			zap.Int("end_sec", job.EndSec),
			zap.String("old_summary", seg.ContentSummary),
			zap.String("new_summary", alignedSeg.ContentSummary),
			zap.Int("text_chars", len(coarseText)))
		return buildResult(alignedSeg.ContentSummary, coarseText, job.StartSec, job.EndSec, summaryMode), nil
	}
	alignedSeg, summaryMode := alignSummary(seg, finalText)
	zap.L().Debug("vectorize_hierarchical_summary_aligned",
		zap.String("mode", summaryMode),
		zap.Int("start_sec", start),
		zap.Int("end_sec", end),
		zap.String("old_summary", seg.ContentSummary),
		zap.String("new_summary", alignedSeg.ContentSummary),
		zap.Int("text_chars", len(finalText)))
	return buildResult(alignedSeg.ContentSummary, finalText, start, end, summaryMode), nil
}

func buildSegmentEmbeddingUpdateValues(startSec int, endSec int, summary string, embedding pgvector.Vector) map[string]any {
	return map[string]any{
		"start_time":      startSec,
		"end_time":        endSec,
		"content_summary": normalizeSegmentTitle(summary),
		"embedding":       embedding,
		"status":          int16(1),
	}
}

func alignSegmentTail(
	ctx context.Context,
	cfg TailAlignmentConfig,
	startSec int,
	originalEndSec int,
	nextSegmentStartSec int,
	videoDurationSec int,
	probe func(endSec int) (string, error),
) (int, string, error) {
	_ = startSec
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

// RefineSegmentsASRAndEmbed 对 hierarchical 模式下的细分段做二次 ASR 和 embedding。
func RefineSegmentsASRAndEmbed(ctx context.Context, db *gorm.DB, ff *transcode.FFmpegTranscoder, client openAICompatClient, tmpRoot string, localVideo string, videoID uint64, taskID string, videoDurationSec int, asrWorkers int, embedBatch int, embeddingDim int, tailCfg TailAlignmentConfig, hints *RefineASRHints, stageRecorder StageRecorder) error {
	if strings.TrimSpace(localVideo) == "" {
		return errors.New("localVideo is required")
	}
	if videoID == 0 {
		return errors.New("videoID is required")
	}
	if strings.TrimSpace(taskID) == "" {
		return errors.New("taskID is required")
	}
	tailCfg = NormalizeTailAlignmentConfig(tailCfg)
	useSelectiveRefine := hints != nil && len(hints.CoarseItems) > 0 && len(hints.Segments) > 0

	var allSegs []model.EduVideoSegment
	if err := db.WithContext(ctx).
		Model(&model.EduVideoSegment{}).
		Where("video_id = ? AND deleted = 0", videoID).
		Order("segment_index ASC").
		Find(&allSegs).Error; err != nil {
		return err
	}

	var segs []model.EduVideoSegment
	for _, seg := range allSegs {
		if len(seg.Embedding.Slice()) == 0 || seg.Status == 0 {
			segs = append(segs, seg)
		}
	}
	if len(allSegs) == 0 {
		return nil
	}
	if len(segs) == 0 {
		return nil
	}

	asrWorkers = normalizeRefineASRWorkers(asrWorkers)
	if embedBatch <= 0 {
		embedBatch = 64
	}
	zap.L().Debug("vectorize_hierarchical_refine_start",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Int("segments", len(segs)),
		zap.Int("asr_workers", asrWorkers),
		zap.Int("embed_batch", embedBatch))

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

	asrCtx, cancelASRAll := context.WithCancel(ctx)
	defer cancelASRAll()

	hintsByIndex := make(map[int]LLMSegment, len(segs))
	if useSelectiveRefine {
		for _, seg := range hints.Segments {
			hintsByIndex[seg.SegmentIndex] = seg
		}
	}
	jobList := buildRefineJobs(segs, allSegs, hintsByIndex)
	for _, j := range jobList {
		recordStagePending(ctx, stageRecorder, StageRecord{
			TaskID:       taskID,
			VideoID:      videoID,
			Stage:        "vector.refine.asr",
			SegmentIndex: j.SegmentIndex,
			SegmentID:    j.ID,
			StartSec:     j.StartSec,
			EndSec:       j.EndSec,
		})
	}

	jobs := make(chan refineJob, asrWorkers*2)
	results := make(chan result, asrWorkers*2)
	enqueueErrCh := make(chan error, 1)
	pool, err := antspool.New(asrCtx, antspool.Options{Name: "vector.refine_asr", Size: asrWorkers, Logger: zap.L()})
	if err != nil {
		return err
	}
	defer pool.Release()
	sendResult := func(res result) bool {
		select {
		case results <- res:
			return true
		case <-asrCtx.Done():
			return false
		}
	}
	for w := 0; w < asrWorkers; w++ {
		workerID := w
		if err := pool.Submit(func() error {
			for j := range jobs {
				if asrCtx.Err() != nil {
					return nil
				}
				dur := j.EndSec - j.StartSec
				if dur <= 0 {
					_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, StartSec: j.StartSec, EndSec: j.EndSec, Input: "", Err: nil})
					continue
				}
				videoapp.RuntimeCounters().Inc("vector_refine_asr_active")

				zap.L().Debug("vectorize_hierarchical_refine_asr_start",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Int("worker", workerID),
					zap.Uint64("seg_id", j.ID),
					zap.Int("start_sec", j.StartSec),
					zap.Int("end_sec", j.EndSec))
				oneStart := time.Now()
				zap.L().Debug("tail_alignment_start",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Uint64("seg_id", j.ID),
					zap.Int("start_sec", j.StartSec),
					zap.Int("end_sec", j.EndSec))

				probeCount := 0
				transcribeRange := func(startSec int, endSec int) (string, error) {
					probeCount++
					if startSec != j.StartSec || endSec != j.EndSec {
						zap.L().Debug("boundary_alignment_probe",
							zap.Uint64("video_id", videoID),
							zap.String("task_id", taskID),
							zap.Uint64("seg_id", j.ID),
							zap.Int("from_start_sec", j.StartSec),
							zap.Int("to_start_sec", startSec),
							zap.Int("from_end_sec", j.EndSec),
							zap.Int("to_end_sec", endSec),
							zap.Int("attempt", probeCount))
					}
					dur := endSec - startSec
					if dur <= 0 {
						return "", nil
					}
					audioPath := filepath.Join(tmpRoot, fmt.Sprintf("%s_refine_%d_%d_%d.wav", taskID, j.ID, startSec, endSec))
					_ = os.Remove(audioPath)

					extractCtx, cancelExtract := context.WithTimeout(asrCtx, 8*time.Minute)
					err := ff.ExtractAudioSegment(extractCtx, localVideo, audioPath, startSec, dur)
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

				if useSelectiveRefine {
					var rewriteSummary summaryRewriter
					summaryMode := ""
					if strings.TrimSpace(hints.LLMModel) != "" {
						rewriteSummary = func(runCtx context.Context, text string) (string, error) {
							prompt := BuildSummaryRewritePrompt(text)
							return client.ChatCompletionsWithTimeout(runCtx, hints.LLMModel, prompt, hints.LLMTimeoutMinutes)
						}
					}
					refineResult, err := buildRefineSegmentInputWithSummaryRewrite(asrCtx, refineInputJob{
						StartSec:           j.StartSec,
						EndSec:             j.EndSec,
						NextStartSec:       j.NextStartSec,
						Summary:            j.Summary,
						BoundaryConfidence: j.BoundaryConfidence,
					}, hints.CoarseItems, videoDurationSec, func(_ context.Context, startSec int, endSec int) (string, error) {
						return transcribeRange(startSec, endSec)
					}, rewriteSummary)
					input := refineResult.Input
					newStartSec := refineResult.StartSec
					newEndSec := refineResult.EndSec
					summaryMode = refineResult.SummaryMode
					if err != nil {
						videoapp.RuntimeCounters().Dec("vector_refine_asr_active")
						recordStageFailed(asrCtx, stageRecorder, StageRecord{
							TaskID:       taskID,
							VideoID:      videoID,
							Stage:        "vector.refine.asr",
							SegmentIndex: j.SegmentIndex,
							SegmentID:    j.ID,
							StartSec:     j.StartSec,
							EndSec:       j.EndSec,
						}, err)
						zap.L().Error("vectorize_hierarchical_refine_input_failed",
							zap.Uint64("video_id", videoID),
							zap.String("task_id", taskID),
							zap.Int("worker", workerID),
							zap.Uint64("seg_id", j.ID),
							zap.Int("start_sec", j.StartSec),
							zap.Int("end_sec", j.EndSec),
							zap.Duration("cost", time.Since(oneStart)),
							zap.Error(err))
						_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, StartSec: j.StartSec, EndSec: j.EndSec, Err: err})
						cancelASRAll()
						continue
					}
					zap.L().Debug("vectorize_hierarchical_refine_input_ready",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Int("worker", workerID),
						zap.Uint64("seg_id", j.ID),
						zap.String("boundary_confidence", j.BoundaryConfidence),
						zap.Int("start_sec", newStartSec),
						zap.Int("end_sec", newEndSec),
						zap.Bool("used_refine_asr", shouldUseRefineASRFallback(j.BoundaryConfidence)),
						zap.String("summary", refineResult.Summary),
						zap.String("summary_mode", summaryMode),
						zap.Int("input_chars", len(input)),
						zap.Int("probe_count", probeCount),
						zap.Duration("cost", time.Since(oneStart)))
					videoapp.RuntimeCounters().Dec("vector_refine_asr_active")
					recordStageComplete(asrCtx, stageRecorder, StageRecord{
						TaskID:       taskID,
						VideoID:      videoID,
						Stage:        "vector.refine.asr",
						SegmentIndex: j.SegmentIndex,
						SegmentID:    j.ID,
						Text:         input,
						StartSec:     newStartSec,
						EndSec:       newEndSec,
					})
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
					continue
				}
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

				alignedStartSec, alignedEndSec, text, usedBoundaryAlignment, usedTailFallback, err := alignSegmentForRefine(asrCtx, seg, next, videoDurationSec, transcribeRange, alignSegmentTail)
				if err != nil {
					videoapp.RuntimeCounters().Dec("vector_refine_asr_active")
					recordStageFailed(asrCtx, stageRecorder, StageRecord{
						TaskID:       taskID,
						VideoID:      videoID,
						Stage:        "vector.refine.asr",
						SegmentIndex: j.SegmentIndex,
						SegmentID:    j.ID,
						StartSec:     j.StartSec,
						EndSec:       j.EndSec,
					}, err)
					stage := "tail_alignment"
					if !usedTailFallback {
						stage = "boundary_alignment"
					}
					zap.L().Error("vectorize_hierarchical_refine_asr_failed",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Int("worker", workerID),
						zap.Uint64("seg_id", j.ID),
						zap.Int("start_sec", j.StartSec),
						zap.Int("end_sec", j.EndSec),
						zap.String("stage", stage),
						zap.Duration("cost", time.Since(oneStart)),
						zap.Error(err))
					_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, StartSec: j.StartSec, EndSec: j.EndSec, Err: err})
					cancelASRAll()
					continue
				}
				if usedTailFallback {
					zap.L().Warn("boundary_alignment_fallback_to_tail",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Uint64("seg_id", j.ID),
						zap.Int("old_start_sec", j.StartSec),
						zap.Int("new_start_sec", alignedStartSec),
						zap.Int("old_end_sec", j.EndSec),
						zap.Int("new_end_sec", alignedEndSec),
						zap.Bool("used_boundary_alignment", usedBoundaryAlignment),
						zap.Bool("used_tail_fallback", usedTailFallback),
						zap.String("start_anchor_text", seg.StartAnchorText),
						zap.String("end_anchor_text", seg.EndAnchorText),
						zap.String("boundary_confidence", seg.BoundaryConfidence))
				}
				if alignedEndSec > j.EndSec || alignedStartSec != j.StartSec {
					zap.L().Debug("tail_alignment_extended",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Uint64("seg_id", j.ID),
						zap.Int("old_start_sec", j.StartSec),
						zap.Int("new_start_sec", alignedStartSec),
						zap.Int("old_end_sec", j.EndSec),
						zap.Int("new_end_sec", alignedEndSec),
						zap.Bool("used_boundary_alignment", usedBoundaryAlignment),
						zap.Bool("used_tail_fallback", usedTailFallback),
						zap.String("start_anchor_text", seg.StartAnchorText),
						zap.String("end_anchor_text", seg.EndAnchorText),
						zap.String("boundary_confidence", seg.BoundaryConfidence),
						zap.Int("probe_count", probeCount))
				} else {
					zap.L().Debug("tail_alignment_skipped",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Uint64("seg_id", j.ID),
						zap.Int("old_start_sec", j.StartSec),
						zap.Int("new_start_sec", alignedStartSec),
						zap.Int("old_end_sec", j.EndSec),
						zap.Int("new_end_sec", alignedEndSec),
						zap.Bool("used_boundary_alignment", usedBoundaryAlignment),
						zap.Bool("used_tail_fallback", usedTailFallback),
						zap.String("start_anchor_text", seg.StartAnchorText),
						zap.String("end_anchor_text", seg.EndAnchorText),
						zap.String("boundary_confidence", seg.BoundaryConfidence),
						zap.String("reason", "already_sentence_end_or_no_extension_needed"))
				}

				base := strings.TrimSpace(j.Summary)
				// 把 LLM 摘要和更精确的 ASR 文本拼接起来，兼顾主题摘要与原句信息。
				combined := strings.TrimSpace(base + "\n" + text)
				if combined == "" {
					combined = base
				}
				zap.L().Debug("vectorize_hierarchical_refine_asr_one_done",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Int("worker", workerID),
					zap.Uint64("seg_id", j.ID),
					zap.Int("start_sec", alignedStartSec),
					zap.Int("end_sec", alignedEndSec),
					zap.Int("old_start_sec", j.StartSec),
					zap.Int("new_start_sec", alignedStartSec),
					zap.Int("old_end_sec", j.EndSec),
					zap.Int("new_end_sec", alignedEndSec),
					zap.Bool("used_boundary_alignment", usedBoundaryAlignment),
					zap.Bool("used_tail_fallback", usedTailFallback),
					zap.Int("text_chars", len(text)),
					zap.Int("input_chars", len(combined)),
					zap.Duration("cost", time.Since(oneStart)))
				videoapp.RuntimeCounters().Dec("vector_refine_asr_active")
				recordStageComplete(asrCtx, stageRecorder, StageRecord{
					TaskID:       taskID,
					VideoID:      videoID,
					Stage:        "vector.refine.asr",
					SegmentIndex: j.SegmentIndex,
					SegmentID:    j.ID,
					Text:         combined,
					StartSec:     alignedStartSec,
					EndSec:       alignedEndSec,
				})
				_ = sendResult(result{
					JobIndex: j.JobIndex,
					ID:       j.ID,
					StartSec: alignedStartSec,
					EndSec:   alignedEndSec,
					Input:    combined,
					Summary:  base,
					Err:      nil,
				})
			}
			return nil
		}); err != nil {
			return err
		}
	}
	go func() {
		defer close(jobs)
		for _, j := range jobList {
			select {
			case jobs <- j:
			case <-asrCtx.Done():
				enqueueErrCh <- asrCtx.Err()
				return
			}
		}
		enqueueErrCh <- nil
	}()
	go func() {
		_ = pool.Wait()
		close(results)
	}()
	orderedInputs := make([]string, len(jobList))
	orderedSummaries := make([]string, len(jobList))
	orderedIDs := make([]uint64, len(jobList))
	orderedSegmentIndexes := make([]int, len(jobList))
	orderedStartSecs := make([]int, len(jobList))
	orderedEndSecs := make([]int, len(jobList))
	summaryModeCounts := make(map[string]int)
	for i := range orderedIDs {
		orderedIDs[i] = jobList[i].ID
		orderedSegmentIndexes[i] = jobList[i].SegmentIndex
		orderedSummaries[i] = strings.TrimSpace(jobList[i].Summary)
		orderedStartSecs[i] = jobList[i].StartSec
		orderedEndSecs[i] = jobList[i].EndSec
	}

	asrStart := time.Now()
	var firstErr error
	got := 0
	for r := range results {
		got++
		if r.Err != nil && firstErr == nil {
			firstErr = r.Err
		}
		if r.JobIndex >= 0 && r.JobIndex < len(orderedInputs) {
			orderedInputs[r.JobIndex] = r.Input
			orderedSummaries[r.JobIndex] = r.Summary
			orderedStartSecs[r.JobIndex] = r.StartSec
			orderedEndSecs[r.JobIndex] = r.EndSec
		}
		if strings.TrimSpace(r.SummaryMode) != "" {
			summaryModeCounts[r.SummaryMode]++
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if enqueueErr := <-enqueueErrCh; enqueueErr != nil && !errors.Is(enqueueErr, context.Canceled) {
		return enqueueErr
	}
	zap.L().Debug("vectorize_hierarchical_refine_asr_done",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Int("results", got),
		zap.Duration("cost", time.Since(asrStart)))
	if len(summaryModeCounts) > 0 {
		zap.L().Debug("vectorize_hierarchical_summary_alignment_stats",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("original_count", summaryModeCounts["original"]),
			zap.Int("rule_rewrite_count", summaryModeCounts["rule_rewrite"]),
			zap.Int("llm_rewrite_count", summaryModeCounts["llm_rewrite"]),
			zap.Int("llm_failed_fallback_count", summaryModeCounts["rule_rewrite_llm_failed"]),
			zap.Int("llm_empty_fallback_count", summaryModeCounts["rule_rewrite_llm_empty"]))
	}

	if embedBatch <= 0 {
		embedBatch = 64
	}
	if embeddingDim <= 0 {
		embeddingDim = 1536
	}

	pairsIDs := make([]uint64, 0, len(orderedInputs))
	pairsSegmentIndexes := make([]int, 0, len(orderedInputs))
	pairsInputs := make([]string, 0, len(orderedInputs))
	pairsSummaries := make([]string, 0, len(orderedInputs))
	pairsStartSecs := make([]int, 0, len(orderedInputs))
	pairsEndSecs := make([]int, 0, len(orderedInputs))
	for i := range orderedInputs {
		if strings.TrimSpace(orderedInputs[i]) == "" {
			continue
		}
		pairsIDs = append(pairsIDs, orderedIDs[i])
		pairsSegmentIndexes = append(pairsSegmentIndexes, orderedSegmentIndexes[i])
		pairsInputs = append(pairsInputs, orderedInputs[i])
		pairsSummaries = append(pairsSummaries, orderedSummaries[i])
		pairsStartSecs = append(pairsStartSecs, orderedStartSecs[i])
		pairsEndSecs = append(pairsEndSecs, orderedEndSecs[i])
	}
	if len(pairsInputs) == 0 {
		zap.L().Debug("vectorize_hierarchical_refine_skipped_embed",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.String("reason", "no_inputs"))
		return nil
	}

	type embeddingUpdate struct {
		ID        uint64
		StartSec  int
		EndSec    int
		Summary   string
		Embedding pgvector.Vector
	}
	var allUpdates []embeddingUpdate

	for i := 0; i < len(pairsInputs); i += embedBatch {
		j := i + embedBatch
		if j > len(pairsInputs) {
			j = len(pairsInputs)
		}
		batchStart := time.Now()
		inputs := pairsInputs[i:j]
		ids := pairsIDs[i:j]
		segmentIndexes := pairsSegmentIndexes[i:j]
		summaries := pairsSummaries[i:j]
		startSecs := pairsStartSecs[i:j]
		endSecs := pairsEndSecs[i:j]

		embedCtx, cancelEmbed := context.WithTimeout(ctx, 12*time.Minute)
		vecs, err := client.Embed(embedCtx, inputs)
		cancelEmbed()
		if err != nil {
			for k, id := range ids {
				recordStageFailed(ctx, stageRecorder, StageRecord{
					TaskID:       taskID,
					VideoID:      videoID,
					Stage:        "vector.embedding",
					SegmentIndex: segmentIndexes[k],
					SegmentID:    id,
					StartSec:     startSecs[k],
					EndSec:       endSecs[k],
				}, err)
			}
			return err
		}
		if len(vecs) != len(ids) {
			return errors.New("embedding result mismatch")
		}

		for k, id := range ids {
			recordStagePending(ctx, stageRecorder, StageRecord{
				TaskID:       taskID,
				VideoID:      videoID,
				Stage:        "vector.embedding",
				SegmentIndex: segmentIndexes[k],
				SegmentID:    id,
				Text:         inputs[k],
				StartSec:     startSecs[k],
				EndSec:       endSecs[k],
			})
			v := normalizeEmbeddingDim(vecs[k], embeddingDim)
			if len(v) == 0 {
				recordStageFailed(ctx, stageRecorder, StageRecord{
					TaskID:       taskID,
					VideoID:      videoID,
					Stage:        "vector.embedding",
					SegmentIndex: segmentIndexes[k],
					SegmentID:    id,
					Text:         inputs[k],
					StartSec:     startSecs[k],
					EndSec:       endSecs[k],
				}, errors.New("empty embedding"))
				continue
			}
			allUpdates = append(allUpdates, embeddingUpdate{
				ID:        id,
				StartSec:  startSecs[k],
				EndSec:    endSecs[k],
				Summary:   summaries[k],
				Embedding: pgvector.NewVector(v),
			})
			recordStageComplete(ctx, stageRecorder, StageRecord{
				TaskID:       taskID,
				VideoID:      videoID,
				Stage:        "vector.embedding",
				SegmentIndex: segmentIndexes[k],
				SegmentID:    id,
				Text:         inputs[k],
				StartSec:     startSecs[k],
				EndSec:       endSecs[k],
			})
		}
		zap.L().Debug("vectorize_hierarchical_refine_embed_batch_done",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("from", i),
			zap.Int("to", j),
			zap.Int("collected", len(ids)),
			zap.Duration("cost", time.Since(batchStart)))
	}

	if len(allUpdates) > 0 {
		// embedding 全部准备好后再统一写库，避免中途失败时只更新部分分段。
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, update := range allUpdates {
				if err := tx.Model(&model.EduVideoSegment{}).
					Where("id = ? AND deleted = 0 AND status = 0", update.ID).
					Updates(buildSegmentEmbeddingUpdateValues(update.StartSec, update.EndSec, update.Summary, update.Embedding)).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		zap.L().Debug("vectorize_hierarchical_refine_db_updated",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("total_updated", len(allUpdates)))
	}
	zap.L().Debug("vectorize_hierarchical_refine_done",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID))
	return nil
}

// normalizeText 清理 ASR 输出中的空行与首尾空白，同时保留多行结构。
func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
