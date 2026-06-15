package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"legacy-video/internal/infrastructure/transcode"
	"legacy-video/internal/model"
	"legacy-video/internal/worker/antspool"
)

// openAICompatClient 定义 hierarchical 细分段补处理所需的最小 AI 能力接口。
type openAICompatClient interface {
	Transcribe(ctx context.Context, audioPath string) (string, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
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
func RefineSegmentsASRAndEmbed(ctx context.Context, db *gorm.DB, ff *transcode.FFmpegTranscoder, client openAICompatClient, tmpRoot string, localVideo string, videoID uint64, taskID string, videoDurationSec int, asrWorkers int, embedBatch int, tailCfg TailAlignmentConfig) error {
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

	var segs []model.EduVideoSegment
	if err := db.WithContext(ctx).
		Model(&model.EduVideoSegment{}).
		Where("video_id = ? AND deleted = 0 AND (embedding IS NULL OR status = 0)", videoID).
		Order("segment_index ASC").
		Find(&segs).Error; err != nil {
		return err
	}
	if len(segs) == 0 {
		return nil
	}

	if asrWorkers <= 0 {
		asrWorkers = 4
	}
	if asrWorkers > 4 {
		asrWorkers = 4
	}
	if embedBatch <= 0 {
		embedBatch = 64
	}
	zap.L().Info("vectorize_hierarchical_refine_start",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Int("segments", len(segs)),
		zap.Int("asr_workers", asrWorkers),
		zap.Int("embed_batch", embedBatch))

	type job struct {
		JobIndex     int
		ID           uint64
		StartSec     int
		EndSec       int
		NextStartSec int
		Summary      string
	}
	type result struct {
		JobIndex int
		ID       uint64
		EndSec   int
		Input    string
		Err      error
	}

	asrCtx, cancelASRAll := context.WithCancel(ctx)
	defer cancelASRAll()

	jobList := make([]job, 0, len(segs))
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

	jobs := make(chan job, asrWorkers*2)
	results := make(chan result, asrWorkers*2)
	enqueueErrCh := make(chan error, 1)
	pool, err := antspool.New(asrCtx, antspool.Options{
		Name:   "vector.refine_asr",
		Size:   asrWorkers,
		Logger: zap.L(),
	})
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
					_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, EndSec: j.EndSec, Input: "", Err: nil})
					continue
				}

				zap.L().Info("vectorize_hierarchical_refine_asr_start",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Int("worker", workerID),
					zap.Uint64("seg_id", j.ID),
					zap.Int("start_sec", j.StartSec),
					zap.Int("end_sec", j.EndSec))
				oneStart := time.Now()
				zap.L().Info("tail_alignment_start",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Uint64("seg_id", j.ID),
					zap.Int("start_sec", j.StartSec),
					zap.Int("end_sec", j.EndSec))

				probeCount := 0
				probe := func(endSec int) (string, error) {
					probeCount++
					if endSec != j.EndSec {
						zap.L().Info("tail_alignment_probe",
							zap.Uint64("video_id", videoID),
							zap.String("task_id", taskID),
							zap.Uint64("seg_id", j.ID),
							zap.Int("from_end_sec", j.EndSec),
							zap.Int("to_end_sec", endSec),
							zap.Int("attempt", probeCount))
					}
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
					zap.L().Error("vectorize_hierarchical_refine_asr_failed",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Int("worker", workerID),
						zap.Uint64("seg_id", j.ID),
						zap.Int("start_sec", j.StartSec),
						zap.Int("end_sec", j.EndSec),
						zap.String("stage", "tail_alignment"),
						zap.Duration("cost", time.Since(oneStart)),
						zap.Error(err))
					_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, EndSec: j.EndSec, Err: err})
					cancelASRAll()
					continue
				}
				if alignedEndSec > j.EndSec {
					zap.L().Info("tail_alignment_extended",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Uint64("seg_id", j.ID),
						zap.Int("old_end_sec", j.EndSec),
						zap.Int("new_end_sec", alignedEndSec),
						zap.Int("probe_count", probeCount))
				} else {
					zap.L().Info("tail_alignment_skipped",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Uint64("seg_id", j.ID),
						zap.String("reason", "already_sentence_end_or_no_extension_needed"))
				}

				base := strings.TrimSpace(j.Summary)
				// 把 LLM 摘要和更精确的 ASR 文本拼接起来，兼顾主题摘要与原句信息。
				combined := strings.TrimSpace(base + "\n" + text)
				if combined == "" {
					combined = base
				}
				zap.L().Info("vectorize_hierarchical_refine_asr_one_done",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Int("worker", workerID),
					zap.Uint64("seg_id", j.ID),
					zap.Int("start_sec", j.StartSec),
					zap.Int("end_sec", alignedEndSec),
					zap.Int("text_chars", len(text)),
					zap.Int("input_chars", len(combined)),
					zap.Duration("cost", time.Since(oneStart)))
				_ = sendResult(result{JobIndex: j.JobIndex, ID: j.ID, EndSec: alignedEndSec, Input: combined, Err: nil})
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
	orderedIDs := make([]uint64, len(jobList))
	orderedEndSecs := make([]int, len(jobList))
	for i := range orderedIDs {
		orderedIDs[i] = jobList[i].ID
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
			orderedEndSecs[r.JobIndex] = r.EndSec
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if enqueueErr := <-enqueueErrCh; enqueueErr != nil && !errors.Is(enqueueErr, context.Canceled) {
		return enqueueErr
	}
	zap.L().Info("vectorize_hierarchical_refine_asr_done",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Int("results", got),
		zap.Duration("cost", time.Since(asrStart)))

	if embedBatch <= 0 {
		embedBatch = 64
	}
	const embeddingDim = 1536

	pairsIDs := make([]uint64, 0, len(orderedInputs))
	pairsInputs := make([]string, 0, len(orderedInputs))
	for i := range orderedInputs {
		if strings.TrimSpace(orderedInputs[i]) == "" {
			continue
		}
		pairsIDs = append(pairsIDs, orderedIDs[i])
		pairsInputs = append(pairsInputs, orderedInputs[i])
	}
	if len(pairsInputs) == 0 {
		zap.L().Info("vectorize_hierarchical_refine_skipped_embed",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.String("reason", "no_inputs"))
		return nil
	}

	type embeddingUpdate struct {
		ID        uint64
		EndSec    int
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

		embedCtx, cancelEmbed := context.WithTimeout(ctx, 12*time.Minute)
		vecs, err := client.Embed(embedCtx, inputs)
		cancelEmbed()
		if err != nil {
			return err
		}
		if len(vecs) != len(ids) {
			return errors.New("embedding result mismatch")
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
		zap.L().Info("vectorize_hierarchical_refine_embed_batch_done",
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
					Updates(map[string]any{
						"end_time":  update.EndSec,
						"embedding": update.Embedding,
						"status":    int16(1),
					}).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		zap.L().Info("vectorize_hierarchical_refine_db_updated",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("total_updated", len(allUpdates)))
	}
	zap.L().Info("vectorize_hierarchical_refine_done",
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
