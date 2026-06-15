package vectorworker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"legacy-video/internal/infrastructure/objectstorage"
	"legacy-video/internal/infrastructure/transcode"
	"legacy-video/internal/model"
	"legacy-video/internal/worker/antspool"
	"legacy-video/internal/worker/vectorworker/tasks"
)

type segMeta struct {
	Index int
	Start int
	End   int
	Text  string
}

type llmSegment = tasks.LLMSegment

type coarseItem = tasks.CoarseItem

// upsertHierarchicalSegments 保留旧调用点，内部转发到 tasks 子包实现。
func upsertHierarchicalSegments(ctx context.Context, db *gorm.DB, videoID uint64, segs []llmSegment) error {
	return tasks.UpsertHierarchicalSegments(ctx, db, videoID, []tasks.LLMSegment(segs))
}

// refineSegmentsASRAndEmbed 保留旧调用点，内部转发到 tasks 子包实现。
func refineSegmentsASRAndEmbed(ctx context.Context, db *gorm.DB, ff *transcode.FFmpegTranscoder, client *openAICompatClient, tmpRoot string, localVideo string, videoID uint64, taskID string, videoDurationSec int, asrWorkers int, embedBatch int, tailCfg tasks.TailAlignmentConfig) error {
	return tasks.RefineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, videoDurationSec, asrWorkers, embedBatch, tailCfg)
}

// extractFirstJSONObject 保留旧调用点，内部转发到 tasks 子包实现。
func extractFirstJSONObject(s string) (string, bool) {
	return tasks.ExtractFirstJSONObject(s)
}

// normalizeTags 保留旧调用点，内部转发到 tasks 子包实现。
func normalizeTags(tags []string) []string {
	return tasks.NormalizeTags(tags)
}

// mergeTags 保留旧调用点，内部转发到 tasks 子包实现。
func mergeTags(a []string, b []string) []string {
	return tasks.MergeTags(a, b)
}

// isUniformSegments 保留旧调用点，内部转发到 tasks 子包实现。
func isUniformSegments(segs []llmSegment) (bool, tasks.UniformStats) {
	return tasks.IsUniformSegments([]tasks.LLMSegment(segs))
}

// normalizeLLMSegments 保留旧调用点，内部转发到 tasks 子包实现。
func normalizeLLMSegments(llmOut string, durationSec int, minSec int, maxSec int) ([]llmSegment, error) {
	segs, err := tasks.NormalizeLLMSegments(llmOut, durationSec, minSec, maxSec)
	if err != nil {
		return nil, err
	}
	return []llmSegment(segs), nil
}

// buildHierarchicalSegmentationRetryPrompt 保留旧调用点，内部转发到 tasks 子包实现。
func buildHierarchicalSegmentationRetryPrompt(durationSec int, coarseSegmentSec int, refineMinSec int, refineMaxSec int, coarseItems []coarseItem) (string, error) {
	return tasks.BuildHierarchicalSegmentationRetryPrompt(durationSec, coarseSegmentSec, refineMinSec, refineMaxSec, []tasks.CoarseItem(coarseItems))
}

// buildHierarchicalSegmentationPrompt 保留旧调用点，内部转发到 tasks 子包实现。
func buildHierarchicalSegmentationPrompt(durationSec int, coarseSegmentSec int, refineMinSec int, refineMaxSec int, coarseItems []coarseItem) (string, error) {
	return tasks.BuildHierarchicalSegmentationPrompt(durationSec, coarseSegmentSec, refineMinSec, refineMaxSec, []tasks.CoarseItem(coarseItems))
}

// buildSampleOffsets 保留旧调用点，内部转发到 tasks 子包实现。
func buildSampleOffsets(durationSec int, sampleCount int) []int {
	return tasks.BuildSampleOffsets(durationSec, sampleCount)
}

// normalizeEmbeddingDim 保留旧调用点，内部转发到 tasks 子包实现。
func normalizeEmbeddingDim(vec []float32, dim int) []float32 {
	return tasks.NormalizeEmbeddingDim(vec, dim)
}

// handleVectorizeTask 执行单个向量化任务。
// 根据 mode 不同，它要么直接做窗口式 ASR + Embedding，要么走粗分段 -> LLM 细切分 -> 细分段 ASR + Embedding 的 hierarchical 链路。
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
	startAt := time.Now()
	zap.L().Info("vectorize_start",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.String("raw_key", rawKey),
		zap.String("tmp", tmpRoot),
		zap.String("mode", strings.TrimSpace(mode)),
		zap.Int("window_sec", windowSec),
		zap.Int("step_sec", stepSec),
		zap.Int("coarse_workers", coarseWorkers),
		zap.Int("coarse_segment_sec", coarseSegmentSec),
		zap.Int("refine_min_sec", refineMinSegmentSec),
		zap.Int("refine_max_sec", refineMaxSegmentSec),
		zap.String("llm_model", strings.TrimSpace(llmModel)),
		zap.Int("llm_timeout_min", llmTimeoutMinutes))
	if videoID == 0 || strings.TrimSpace(rawKey) == "" {
		return errors.New("invalid task")
	}

	var exists int64
	if err := db.WithContext(ctx).Model(&model.EduVideoResource{}).Where("id = ? AND deleted = 0", videoID).Count(&exists).Error; err != nil {
		return err
	}
	if exists == 0 {
		zap.L().Info("vectorize_skip",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.String("reason", "video_not_found_or_deleted"))
		return nil
	}

	localVideo := filepath.Join(tmpRoot, taskID+"_"+filepath.Base(rawKey))
	_ = os.Remove(localVideo)

	downloadCtx, cancelDownload := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelDownload()
	if err := store.DownloadToFile(downloadCtx, rawKey, localVideo); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(localVideo)
	if st, err := os.Stat(localVideo); err == nil {
		zap.L().Info("vectorize_downloaded",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int64("size", st.Size()),
			zap.String("path", localVideo))
	}

	probeCtx, cancelProbe := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelProbe()
	durationSec, err := ff.ProbeDurationSeconds(probeCtx, localVideo)
	if err != nil {
		zap.L().Error("vectorize_probe_failed",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Error(err))
		durationSec = 0
	}
	zap.L().Info("vectorize_probe_ok",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Int("duration_sec", durationSec))
	if durationSec > 0 {
		_ = db.WithContext(ctx).Model(&model.EduVideoResource{}).Where("id = ? AND deleted = 0", videoID).Update("duration", durationSec).Error
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "full"
	}

	if mode == "hierarchical" {
		// hierarchical 模式优先复用已有细分段，避免任务重试时重复调用 LLM。
		if durationSec <= 0 {
			return errors.New("invalid duration for hierarchical mode")
		}
		if coarseSegmentSec <= 0 {
			coarseSegmentSec = 60
		}

		var existingUnfinishedSegments int64
		if err := db.WithContext(ctx).Model(&model.EduVideoSegment{}).
			Where("video_id = ? AND deleted = 0 AND (embedding IS NULL OR status = 0)", videoID).
			Count(&existingUnfinishedSegments).Error; err != nil {
			return err
		}

		var existingFinishedSegments int64
		if err := db.WithContext(ctx).Model(&model.EduVideoSegment{}).
			Where("video_id = ? AND deleted = 0 AND embedding IS NOT NULL AND status = 1", videoID).
			Count(&existingFinishedSegments).Error; err != nil {
			return err
		}

		if existingUnfinishedSegments > 0 || existingFinishedSegments > 0 {
			zap.L().Info("vectorize_hierarchical_resume",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID),
				zap.Int64("unfinished_segments", existingUnfinishedSegments),
				zap.Int64("finished_segments", existingFinishedSegments),
				zap.String("action", "skip_segmentation_resume_embedding"))

			if err := refineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, durationSec, asrWorkers, embedBatch, tailCfg); err != nil {
				return err
			}
			zap.L().Info("vectorize_hierarchical_resume_done",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID))
			return nil
		}

		prefix := fmt.Sprintf("segments/coarse/video_%d/%s", videoID, strings.TrimSpace(taskID))
		type clipJob struct {
			Index    int
			StartSec int
			EndSec   int
			DurSec   int
			Key      string
		}
		type clipResult struct {
			Index int
			Item  coarseItem
			Err   error
		}

		// 先把整段视频切成粗分段，再对每个粗分段独立做裁剪、上传和 ASR。
		jobsList := make([]clipJob, 0, int(math.Ceil(float64(durationSec)/float64(coarseSegmentSec))))
		segIdx := 0
		for startSec := 0; startSec < durationSec; startSec += coarseSegmentSec {
			endSec := startSec + coarseSegmentSec
			if endSec > durationSec {
				endSec = durationSec
			}
			if endSec <= startSec {
				continue
			}
			dur := endSec - startSec
			key := filepath.ToSlash(filepath.Join(prefix, fmt.Sprintf("seg_%03d_%d_%d.mp4", segIdx, startSec, endSec)))
			jobsList = append(jobsList, clipJob{
				Index:    segIdx,
				StartSec: startSec,
				EndSec:   endSec,
				DurSec:   dur,
				Key:      key,
			})
			segIdx++
		}
		if len(jobsList) == 0 {
			return errors.New("no coarse segments")
		}

		requestedCoarseWorkers := coarseWorkers
		if requestedCoarseWorkers <= 0 {
			requestedCoarseWorkers = asrWorkers
		}
		if requestedCoarseWorkers <= 0 {
			requestedCoarseWorkers = 2
		}
		coarseWorkers = requestedCoarseWorkers
		if asrWorkers > 0 && coarseWorkers > asrWorkers {
			coarseWorkers = asrWorkers
		}
		if coarseWorkers > len(jobsList) {
			coarseWorkers = len(jobsList)
		}
		if coarseWorkers <= 0 {
			coarseWorkers = 1
		}
		zap.L().Info("vectorize_hierarchical_coarse_start",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("segments", len(jobsList)),
			zap.Int("requested_workers", requestedCoarseWorkers),
			zap.Int("workers", coarseWorkers),
			zap.Int("max_workers", requestedCoarseWorkers))

		coarseCtx, cancelCoarse := context.WithCancel(ctx)
		defer cancelCoarse()

		jobs := make(chan clipJob, coarseWorkers*2)
		results := make(chan clipResult, coarseWorkers*2)
		enqueueErrCh := make(chan error, 1)
		pool, err := antspool.New(coarseCtx, antspool.Options{
			Name:   "vector.coarse",
			Size:   coarseWorkers,
			Logger: zap.L(),
		})
		if err != nil {
			return err
		}
		defer pool.Release()
		sendResult := func(res clipResult) bool {
			select {
			case results <- res:
				return true
			case <-coarseCtx.Done():
				return false
			}
		}
		for w := 0; w < coarseWorkers; w++ {
			workerID := w
			if err := pool.Submit(func() error {
				for j := range jobs {
					if coarseCtx.Err() != nil {
						return nil
					}

					clipPath := filepath.Join(tmpRoot, fmt.Sprintf("%s_coarse_%03d_%d_%d.mp4", taskID, j.Index, j.StartSec, j.EndSec))
					audioPath := filepath.Join(tmpRoot, fmt.Sprintf("%s_coarse_%03d_%d_%d.wav", taskID, j.Index, j.StartSec, j.EndSec))
					_ = os.Remove(clipPath)
					_ = os.Remove(audioPath)

					// 每个粗分段都会产出一个 mp4 片段和一个 wav 音频片段。
					clipCtx, cancelClip := context.WithTimeout(coarseCtx, 12*time.Minute)
					err := ff.ClipVideoSegmentWithAudio(clipCtx, localVideo, clipPath, audioPath, j.StartSec, j.DurSec)
					cancelClip()
					if err != nil {
						_ = os.Remove(clipPath)
						_ = os.Remove(audioPath)
						_ = sendResult(clipResult{Index: j.Index, Err: fmt.Errorf("clip+audio failed seg=%d: %w", j.Index, err)})
						cancelCoarse()
						continue
					}

					putCtx, cancelPut := context.WithTimeout(coarseCtx, 10*time.Minute)
					err = store.PutFile(putCtx, j.Key, clipPath, "video/mp4")
					cancelPut()
					_ = os.Remove(clipPath)
					if err != nil {
						_ = os.Remove(audioPath)
						_ = sendResult(clipResult{Index: j.Index, Err: fmt.Errorf("upload failed seg=%d key=%s: %w", j.Index, j.Key, err)})
						cancelCoarse()
						continue
					}
					zap.L().Info("vectorize_hierarchical_coarse_uploaded",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Int("worker", workerID),
						zap.Int("seg", j.Index),
						zap.Int("start_sec", j.StartSec),
						zap.Int("end_sec", j.EndSec),
						zap.String("key", j.Key))

					asrCtx, cancelASR := context.WithTimeout(coarseCtx, 12*time.Minute)
					text, err := client.Transcribe(asrCtx, audioPath)
					cancelASR()
					_ = os.Remove(audioPath)
					if err != nil {
						_ = sendResult(clipResult{Index: j.Index, Err: fmt.Errorf("asr failed seg=%d: %w", j.Index, err)})
						cancelCoarse()
						continue
					}
					text = normalizeText(text)

					_ = sendResult(clipResult{
						Index: j.Index,
						Item: coarseItem{
							Index:     j.Index,
							StartSec:  j.StartSec,
							EndSec:    j.EndSec,
							Text:      text,
							ObjectKey: j.Key,
						},
						Err: nil,
					})
				}
				return nil
			}); err != nil {
				return err
			}
		}
		go func() {
			defer close(jobs)
			for _, j := range jobsList {
				select {
				case jobs <- j:
				case <-coarseCtx.Done():
					enqueueErrCh <- coarseCtx.Err()
					return
				}
			}
			enqueueErrCh <- nil
		}()
		go func() {
			_ = pool.Wait()
			close(results)
		}()

		byIndex := make([]coarseItem, len(jobsList))
		var firstErr error
		got := 0
		for r := range results {
			got++
			if r.Err != nil && firstErr == nil {
				firstErr = r.Err
			}
			if r.Err == nil && r.Index >= 0 && r.Index < len(byIndex) {
				byIndex[r.Index] = r.Item
			}
		}
		if firstErr != nil {
			return firstErr
		}
		if enqueueErr := <-enqueueErrCh; enqueueErr != nil && !errors.Is(enqueueErr, context.Canceled) {
			return enqueueErr
		}
		if got < len(jobsList) {
			return errors.New("coarse segmentation incomplete")
		}

		coarseItems := make([]coarseItem, 0, len(byIndex))
		for i := 0; i < len(byIndex); i++ {
			if byIndex[i].EndSec <= byIndex[i].StartSec {
				return fmt.Errorf("coarse segment missing index=%d", i)
			}
			coarseItems = append(coarseItems, byIndex[i])
		}

		zap.L().Info("vectorize_hierarchical_coarse_done",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("total", len(coarseItems)),
			zap.Duration("cost", time.Since(startAt)))

		if strings.TrimSpace(llmModel) == "" {
			return errors.New("llm_model is required for hierarchical mode")
		}
		// LLM 只负责决定内容边界，真正的时间清洗由 normalizeLLMSegments 统一收口。
		prompt, err := buildHierarchicalSegmentationPrompt(durationSec, coarseSegmentSec, refineMinSegmentSec, refineMaxSegmentSec, coarseItems)
		if err != nil {
			return err
		}
		zap.L().Info("vectorize_hierarchical_llm_prompt_ready",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("chars", len(prompt)))
		llmOut, err := client.ChatCompletionsWithTimeout(ctx, llmModel, prompt, llmTimeoutMinutes)
		if err != nil {
			return err
		}
		segs, err := normalizeLLMSegments(llmOut, durationSec, refineMinSegmentSec, refineMaxSegmentSec)
		if err != nil {
			preview := llmOut
			if len(preview) > 800 {
				preview = preview[:800]
			}
			return fmt.Errorf("llm segmentation invalid: %w preview=%s", err, preview)
		}
		uniform, st := isUniformSegments(segs)
		zap.L().Info("vectorize_hierarchical_llm_segments_stats",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("count", st.Count),
			zap.Int("mode_bin", st.ModeBin),
			zap.Float64("mode_ratio", st.ModeRatio),
			zap.Int("min_sec", st.MinLen),
			zap.Int("max_sec", st.MaxLen),
			zap.Bool("uniform", uniform))
		if uniform {
			retryPrompt, err := buildHierarchicalSegmentationRetryPrompt(durationSec, coarseSegmentSec, refineMinSegmentSec, refineMaxSegmentSec, coarseItems)
			if err != nil {
				return err
			}
			zap.L().Info("vectorize_hierarchical_llm_retry_prompt_ready",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID),
				zap.Int("chars", len(retryPrompt)))
			llmOut2, err := client.ChatCompletionsWithTimeout(ctx, llmModel, retryPrompt, llmTimeoutMinutes)
			if err != nil {
				zap.L().Error("vectorize_hierarchical_llm_retry_failed",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Error(err))
			} else {
				segs2, err := normalizeLLMSegments(llmOut2, durationSec, refineMinSegmentSec, refineMaxSegmentSec)
				if err != nil {
					preview := llmOut2
					if len(preview) > 800 {
						preview = preview[:800]
					}
					zap.L().Error("vectorize_hierarchical_llm_retry_invalid",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Error(err),
						zap.String("preview", preview))
				} else {
					uniform2, st2 := isUniformSegments(segs2)
					zap.L().Info("vectorize_hierarchical_llm_retry_segments_stats",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Int("count", st2.Count),
						zap.Int("mode_bin", st2.ModeBin),
						zap.Float64("mode_ratio", st2.ModeRatio),
						zap.Int("min_sec", st2.MinLen),
						zap.Int("max_sec", st2.MaxLen),
						zap.Bool("uniform", uniform2))
					if !uniform2 || st2.ModeRatio < st.ModeRatio-0.05 || len(segs2) != len(segs) {
						segs = segs2
					}
				}
			}
		}
		zap.L().Info("vectorize_hierarchical_llm_segments_normalized",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("total", len(segs)))
		if len(segs) > 0 {
			head := segs[0]
			zap.L().Info("vectorize_hierarchical_llm_segments_head",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID),
				zap.Int("idx", head.SegmentIndex),
				zap.Int("start_sec", head.StartTimeSec),
				zap.Int("end_sec", head.EndTimeSec),
				zap.Int("tags", len(head.KnowledgeTags)))
		}
		if err := upsertHierarchicalSegments(ctx, db, videoID, segs); err != nil {
			return err
		}
		zap.L().Info("vectorize_hierarchical_segments_saved",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("total", len(segs)))

		if err := refineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, durationSec, asrWorkers, embedBatch, tailCfg); err != nil {
			return err
		}
		zap.L().Info("vectorize_hierarchical_segments_refined",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID))
		return nil
	}

	// 非 hierarchical 模式下，直接按窗口/采样切音频并做 ASR，再对文本生成 embedding。
	metas := make([]segMeta, 0, 16)
	segIdx := 0
	if durationSec <= 0 {
		durationSec = windowSec
	}

	attempted := 0
	extractOK := 0
	extractErrs := 0
	asrOK := 0
	asrErrs := 0
	nonEmpty := 0
	var firstExtractErr error
	var firstASRErr error
	extractLogLeft := 5
	asrLogLeft := 5

	if asrWorkers <= 0 {
		asrWorkers = 2
	}
	type asrJob struct {
		Index    int
		StartSec int
		EndSec   int
		ClipDur  int
		Path     string
	}
	asrCh := make(chan asrJob, asrWorkers*2)
	var mu sync.Mutex
	pool, err := antspool.New(ctx, antspool.Options{
		Name:   "vector.sample_asr",
		Size:   asrWorkers,
		Logger: zap.L(),
	})
	if err != nil {
		return err
	}
	defer pool.Release()
	sendASRJob := func(job asrJob) bool {
		select {
		case asrCh <- job:
			return true
		case <-ctx.Done():
			return false
		}
	}
	for i := 0; i < asrWorkers; i++ {
		if err := pool.Submit(func() error {
			for job := range asrCh {
				asrCtx, cancelASR := context.WithTimeout(ctx, 8*time.Minute)
				zap.L().Info("vectorize_asr_start",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Int("seg", job.Index))
				text, err := client.Transcribe(asrCtx, job.Path)
				cancelASR()
				_ = os.Remove(job.Path)
				if err != nil {
					mu.Lock()
					asrErrs++
					if firstASRErr == nil {
						firstASRErr = err
					}
					if asrLogLeft > 0 {
						asrLogLeft--
						zap.L().Error("vectorize_asr_failed",
							zap.Uint64("video_id", videoID),
							zap.String("task_id", taskID),
							zap.Int("seg", job.Index),
							zap.Int("start_sec", job.StartSec),
							zap.Int("dur_sec", job.ClipDur),
							zap.Error(err))
					}
					mu.Unlock()
					continue
				}
				text = normalizeText(text)
				if text == "" {
					mu.Lock()
					asrOK++
					mu.Unlock()
					continue
				}
				mu.Lock()
				asrOK++
				nonEmpty++
				metas = append(metas, segMeta{
					Index: job.Index,
					Start: job.StartSec,
					End:   job.EndSec,
					Text:  text,
				})
				mu.Unlock()
			}
			return nil
		}); err != nil {
			close(asrCh)
			return err
		}
	}

	if mode == "sample" {
		if sampleCount <= 0 {
			sampleCount = 3
		}
		if sampleDurSec <= 0 {
			sampleDurSec = 10
		}
		offsets := buildSampleOffsets(durationSec, sampleCount)
		for _, startSec := range offsets {
			endSec := startSec + sampleDurSec
			if endSec > durationSec {
				endSec = durationSec
			}
			if endSec <= startSec {
				continue
			}
			clipDur := endSec - startSec
			attempted++
			audioPath := filepath.Join(tmpRoot, fmt.Sprintf("%s_seg_%03d.wav", taskID, segIdx))
			_ = os.Remove(audioPath)
			zap.L().Info("vectorize_extract_start",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID),
				zap.Int("seg", segIdx),
				zap.Int("start_sec", startSec),
				zap.Int("dur_sec", clipDur),
				zap.String("out", audioPath))
			extractCtx, cancelExtract := context.WithTimeout(ctx, 2*time.Minute)
			err := ff.ExtractAudioSegment(extractCtx, localVideo, audioPath, startSec, clipDur)
			cancelExtract()
			if err != nil {
				extractErrs++
				if firstExtractErr == nil {
					firstExtractErr = err
				}
				if extractLogLeft > 0 {
					extractLogLeft--
					zap.L().Error("vectorize_extract_failed",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Int("seg", segIdx),
						zap.Int("start_sec", startSec),
						zap.Int("dur_sec", clipDur),
						zap.Error(err))
				}
				_ = os.Remove(audioPath)
				segIdx++
				continue
			}
			extractOK++
			if st, err := os.Stat(audioPath); err == nil {
				zap.L().Info("vectorize_extract_ok",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Int("seg", segIdx),
					zap.Int64("size", st.Size()))
			}
			if !sendASRJob(asrJob{
				Index:    segIdx,
				StartSec: startSec,
				EndSec:   endSec,
				ClipDur:  clipDur,
				Path:     audioPath,
			}) {
				_ = os.Remove(audioPath)
				close(asrCh)
				_ = pool.Wait()
				return ctx.Err()
			}
			segIdx++
		}
	} else {
		for startSec := 0; startSec < durationSec; startSec += stepSec {
			endSec := startSec + windowSec
			if endSec > durationSec {
				endSec = durationSec
			}
			if endSec <= startSec {
				break
			}
			clipDur := endSec - startSec
			if clipDur <= 0 {
				break
			}
			attempted++
			if attempted == 1 || attempted%5 == 0 {
				zap.L().Info("vectorize_progress",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Int("attempted", attempted),
					zap.Int("extract_ok", extractOK),
					zap.Int("extract_err", extractErrs),
					zap.Int("asr_ok", asrOK),
					zap.Int("asr_err", asrErrs),
					zap.Int("non_empty", nonEmpty))
			}

			audioPath := filepath.Join(tmpRoot, fmt.Sprintf("%s_seg_%03d.wav", taskID, segIdx))
			_ = os.Remove(audioPath)
			zap.L().Info("vectorize_extract_start",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID),
				zap.Int("seg", segIdx),
				zap.Int("start_sec", startSec),
				zap.Int("dur_sec", clipDur),
				zap.String("out", audioPath))
			extractCtx, cancelExtract := context.WithTimeout(ctx, 3*time.Minute)
			err := ff.ExtractAudioSegment(extractCtx, localVideo, audioPath, startSec, clipDur)
			cancelExtract()
			if err != nil {
				extractErrs++
				if firstExtractErr == nil {
					firstExtractErr = err
				}
				if extractLogLeft > 0 {
					extractLogLeft--
					zap.L().Error("vectorize_extract_failed",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Int("seg", segIdx),
						zap.Int("start_sec", startSec),
						zap.Int("dur_sec", clipDur),
						zap.Error(err))
				}
				_ = os.Remove(audioPath)
				segIdx++
				continue
			}
			extractOK++
			if st, err := os.Stat(audioPath); err == nil {
				zap.L().Info("vectorize_extract_ok",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Int("seg", segIdx),
					zap.Int64("size", st.Size()))
			}
			if !sendASRJob(asrJob{
				Index:    segIdx,
				StartSec: startSec,
				EndSec:   endSec,
				ClipDur:  clipDur,
				Path:     audioPath,
			}) {
				_ = os.Remove(audioPath)
				close(asrCh)
				_ = pool.Wait()
				return ctx.Err()
			}
			segIdx++
		}
	}

	close(asrCh)
	_ = pool.Wait()

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Index < metas[j].Index
	})

	if len(metas) > 0 {
		zap.L().Info("vectorize_asr_ok",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("segments_non_empty", len(metas)))
	}

	if len(metas) == 0 {
		// 这里显式区分“抽音频失败”和“ASR 失败”，便于排查是 FFmpeg 还是模型链路的问题。
		if extractOK == 0 && extractErrs > 0 && firstExtractErr != nil {
			msg := strings.ToLower(firstExtractErr.Error())
			if strings.Contains(msg, "ffmpeg not found") || strings.Contains(msg, "executable file not found") || strings.Contains(msg, "docker") {
				return fmt.Errorf("vectorize failed: audio extraction unavailable: %w", firstExtractErr)
			}
		}
		if extractOK > 0 && asrOK == 0 && asrErrs > 0 && firstASRErr != nil {
			return fmt.Errorf("vectorize failed: asr error: %w", firstASRErr)
		}
		zap.L().Info("vectorize_done_empty",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.String("status", "EMPTY"),
			zap.Int("duration_sec", durationSec),
			zap.Int("attempted", attempted),
			zap.Int("extract_ok", extractOK),
			zap.Int("extract_err", extractErrs),
			zap.Int("asr_ok", asrOK),
			zap.Int("asr_err", asrErrs),
			zap.Int("non_empty", nonEmpty),
			zap.Duration("latency", time.Since(startAt)))
		return nil
	}

	texts := make([]string, 0, len(metas))
	for _, m := range metas {
		texts = append(texts, m.Text)
	}

	if embedBatch <= 0 {
		embedBatch = 64
	}
	zap.L().Info("vectorize_embedding_start",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Int("batches", (len(texts)+(embedBatch-1))/embedBatch))
	vecs := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += embedBatch {
		j := i + embedBatch
		if j > len(texts) {
			j = len(texts)
		}
		embedCtx, cancelEmbed := context.WithTimeout(ctx, 8*time.Minute)
		part, err := client.Embed(embedCtx, texts[i:j])
		cancelEmbed()
		if err != nil {
			return err
		}
		vecs = append(vecs, part...)
	}
	if len(vecs) != len(metas) {
		return errors.New("embedding result mismatch")
	}

	segs := make([]model.EduVideoSegment, 0, len(metas))
	const embeddingDim = 1536
	for i, m := range metas {
		v := normalizeEmbeddingDim(vecs[i], embeddingDim)
		if len(v) == 0 {
			continue
		}
		summary := m.Text
		if len([]rune(summary)) > 500 {
			summary = string([]rune(summary)[:500])
		}
		segs = append(segs, model.EduVideoSegment{
			VideoID:        videoID,
			SegmentIndex:   m.Index,
			StartTimeSec:   m.Start,
			EndTimeSec:     m.End,
			ContentSummary: summary,
			Embedding:      pgvector.NewVector(v),
			KnowledgeTags:  model.TextArray{},
			Status:         1,
			Deleted:        0,
		})
	}

	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 写入前先把旧分段软删除，避免同一视频保留多版向量数据。
		res := tx.Model(&model.EduVideoSegment{}).Where("video_id = ? AND deleted = 0", videoID).Update("deleted", 1)
		if res.Error != nil {
			return res.Error
		}
		zap.L().Info("vectorize_db_mark_deleted",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int64("rows", res.RowsAffected))
		if len(segs) == 0 {
			return nil
		}
		if err := tx.Create(&segs).Error; err != nil {
			return err
		}
		zap.L().Info("vectorize_db_inserted",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("rows", len(segs)))
		return nil
	}); err != nil {
		return err
	}

	zap.L().Info("vectorize_done",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Int("segments", len(segs)),
		zap.Int("duration_sec", durationSec),
		zap.Duration("latency", time.Since(startAt)))
	return nil
}

// meanInt 返回两个整数的平均值，零值会被当作“未提供”处理。
func meanInt(a, b int) int {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	return int(math.Round(float64(a+b) / 2))
}
