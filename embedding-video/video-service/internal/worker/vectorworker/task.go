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

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/infrastructure/objectstorage"
	"nlp-video-analysis/internal/infrastructure/transcode"
	"nlp-video-analysis/internal/model"
	"nlp-video-analysis/internal/worker/antspool"
	"nlp-video-analysis/internal/worker/vectorworker/tasks"
)

type segMeta struct {
	Index int
	Start int
	End   int
	Text  string
}

const maxCoarseWorkers = 20

func calcCoarseWorkers(requested int, fallback int, segments int) int {
	workers := requested
	if workers <= 0 {
		workers = fallback
	}
	if workers <= 0 {
		workers = 2
	}
	if workers > maxCoarseWorkers {
		workers = maxCoarseWorkers
	}
	if segments > 0 && workers > segments {
		workers = segments
	}
	if workers <= 0 {
		workers = 1
	}
	return workers
}

func calcCoarsePipelineStageWorkers(requested int, fallback int, segments int) (int, int, int) {
	workers := calcCoarseWorkers(requested, fallback, segments)
	return workers, workers, workers
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
	embeddingDim int,
	tailCfg tasks.TailAlignmentConfig,
	stageRecorder *vectorStageRecorder,
	videoID uint64,
	taskID string,
	rawKey string,
) error {
	startAt := time.Now()
	zap.L().Debug("vectorize_start",
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
	prepareTask := VectorStageTask{
		TaskID:    taskID,
		VideoID:   videoID,
		RawKey:    rawKey,
		Stage:     VectorStagePrepare,
		ObjectKey: rawKey,
	}
	stageRecorder.Pending(ctx, prepareTask)

	var exists int64
	if err := db.WithContext(ctx).Model(&model.EduVideoResource{}).Where("id = ? AND deleted = 0", videoID).Count(&exists).Error; err != nil {
		stageRecorder.Fail(ctx, prepareTask, err)
		return err
	}
	if exists == 0 {
		zap.L().Debug("vectorize_skip",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.String("reason", "video_not_found_or_deleted"))
		stageRecorder.Fail(ctx, prepareTask, errors.New("video not found or deleted"))
		return nil
	}

	localVideo := filepath.Join(tmpRoot, taskID+"_"+filepath.Base(rawKey))
	_ = os.Remove(localVideo)

	downloadCtx, cancelDownload := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelDownload()
	if err := store.DownloadToFile(downloadCtx, rawKey, localVideo); err != nil {
		err := fmt.Errorf("download failed: %w", err)
		stageRecorder.Fail(ctx, prepareTask, err)
		return err
	}
	defer os.Remove(localVideo)
	if st, err := os.Stat(localVideo); err == nil {
		zap.L().Debug("vectorize_downloaded",
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
	zap.L().Debug("vectorize_probe_ok",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Int("duration_sec", durationSec))
	if durationSec > 0 {
		_ = db.WithContext(ctx).Model(&model.EduVideoResource{}).Where("id = ? AND deleted = 0", videoID).Update("duration", durationSec).Error
	}
	prepareTask.EndSec = durationSec
	stageRecorder.Complete(ctx, prepareTask, "")

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
			zap.L().Debug("vectorize_hierarchical_resume",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID),
				zap.Int64("unfinished_segments", existingUnfinishedSegments),
				zap.Int64("finished_segments", existingFinishedSegments),
				zap.String("action", "skip_segmentation_resume_embedding"))

			if err := tasks.RefineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, durationSec, asrWorkers, embedBatch, embeddingDim, tailCfg, nil, stageRecorder); err != nil {
				return err
			}
			zap.L().Debug("vectorize_hierarchical_resume_done",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID))
			stageRecorder.Complete(ctx, VectorStageTask{
				TaskID:  taskID,
				VideoID: videoID,
				Stage:   VectorStageFinalize,
				EndSec:  durationSec,
			}, "resume")
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
			Item  tasks.CoarseItem
			Err   error
		}
		type clippedSegment struct {
			Job       clipJob
			ClipPath  string
			AudioPath string
		}
		type uploadedSegment struct {
			Job       clipJob
			AudioPath string
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
			stageRecorder.Pending(ctx, VectorStageTask{
				TaskID:       taskID,
				VideoID:      videoID,
				Stage:        VectorStageCoarseClip,
				SegmentIndex: segIdx,
				StartSec:     startSec,
				EndSec:       endSec,
				ObjectKey:    key,
			})
			segIdx++
		}
		if len(jobsList) == 0 {
			return errors.New("no coarse segments")
		}

		requestedCoarseWorkers := coarseWorkers
		clipWorkers, uploadWorkers, coarseASRWorkers := calcCoarsePipelineStageWorkers(requestedCoarseWorkers, asrWorkers, len(jobsList))
		coarseWorkers = clipWorkers
		zap.L().Debug("vectorize_hierarchical_coarse_start",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("segments", len(jobsList)),
			zap.Int("requested_workers", requestedCoarseWorkers),
			zap.Int("workers", coarseWorkers),
			zap.Int("clip_workers", clipWorkers),
			zap.Int("upload_workers", uploadWorkers),
			zap.Int("coarse_asr_workers", coarseASRWorkers),
			zap.Int("max_workers", maxCoarseWorkers))

		coarseCtx, cancelCoarse := context.WithCancel(ctx)
		defer cancelCoarse()

		jobs := make(chan clipJob, clipWorkers*2)
		clippedCh := make(chan clippedSegment, clipWorkers*2)
		uploadedCh := make(chan uploadedSegment, uploadWorkers*2)
		results := make(chan clipResult, coarseWorkers*2)
		enqueueErrCh := make(chan error, 1)
		clipPool, err := antspool.New(coarseCtx, antspool.Options{Name: "vector.coarse", Size: clipWorkers, Logger: zap.L()})
		if err != nil {
			return err
		}
		defer clipPool.Release()
		uploadPool, err := antspool.New(coarseCtx, antspool.Options{Name: "vector.coarse", Size: uploadWorkers, Logger: zap.L()})
		if err != nil {
			return err
		}
		defer uploadPool.Release()
		asrPool, err := antspool.New(coarseCtx, antspool.Options{Name: "vector.coarse", Size: coarseASRWorkers, Logger: zap.L()})
		if err != nil {
			return err
		}
		defer asrPool.Release()
		sendResult := func(res clipResult) bool {
			select {
			case results <- res:
				return true
			case <-coarseCtx.Done():
				return false
			}
		}
		for w := 0; w < clipWorkers; w++ {
			if err := clipPool.Submit(func() error {
				for j := range jobs {
					if coarseCtx.Err() != nil {
						return nil
					}
					videoapp.RuntimeCounters().Inc("vector_coarse_clip_active")

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
						videoapp.RuntimeCounters().Dec("vector_coarse_clip_active")
						err := fmt.Errorf("clip+audio failed seg=%d: %w", j.Index, err)
						stageRecorder.Fail(coarseCtx, VectorStageTask{
							TaskID:       taskID,
							VideoID:      videoID,
							Stage:        VectorStageCoarseClip,
							SegmentIndex: j.Index,
							StartSec:     j.StartSec,
							EndSec:       j.EndSec,
							ObjectKey:    j.Key,
						}, err)
						_ = sendResult(clipResult{Index: j.Index, Err: err})
						cancelCoarse()
						continue
					}
					videoapp.RuntimeCounters().Dec("vector_coarse_clip_active")
					select {
					case clippedCh <- clippedSegment{Job: j, ClipPath: clipPath, AudioPath: audioPath}:
					case <-coarseCtx.Done():
						_ = os.Remove(clipPath)
						_ = os.Remove(audioPath)
						return nil
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
		for w := 0; w < uploadWorkers; w++ {
			workerID := w
			if err := uploadPool.Submit(func() error {
				for item := range clippedCh {
					if coarseCtx.Err() != nil {
						return nil
					}
					videoapp.RuntimeCounters().Inc("vector_coarse_upload_active")
					putCtx, cancelPut := context.WithTimeout(coarseCtx, 10*time.Minute)
					err := store.PutFile(putCtx, item.Job.Key, item.ClipPath, "video/mp4")
					cancelPut()
					_ = os.Remove(item.ClipPath)
					if err != nil {
						_ = os.Remove(item.AudioPath)
						videoapp.RuntimeCounters().Dec("vector_coarse_upload_active")
						err := fmt.Errorf("upload failed seg=%d key=%s: %w", item.Job.Index, item.Job.Key, err)
						stageRecorder.Fail(coarseCtx, VectorStageTask{
							TaskID:       taskID,
							VideoID:      videoID,
							Stage:        VectorStageCoarseClip,
							SegmentIndex: item.Job.Index,
							StartSec:     item.Job.StartSec,
							EndSec:       item.Job.EndSec,
							ObjectKey:    item.Job.Key,
						}, err)
						_ = sendResult(clipResult{Index: item.Job.Index, Err: err})
						cancelCoarse()
						continue
					}
					videoapp.RuntimeCounters().Dec("vector_coarse_upload_active")
					stageRecorder.Complete(coarseCtx, VectorStageTask{
						TaskID:       taskID,
						VideoID:      videoID,
						Stage:        VectorStageCoarseClip,
						SegmentIndex: item.Job.Index,
						StartSec:     item.Job.StartSec,
						EndSec:       item.Job.EndSec,
						ObjectKey:    item.Job.Key,
					}, "")
					stageRecorder.Pending(coarseCtx, VectorStageTask{
						TaskID:       taskID,
						VideoID:      videoID,
						Stage:        VectorStageCoarseASR,
						SegmentIndex: item.Job.Index,
						StartSec:     item.Job.StartSec,
						EndSec:       item.Job.EndSec,
						ObjectKey:    item.Job.Key,
					})
					zap.L().Debug("vectorize_hierarchical_coarse_uploaded",
						zap.Uint64("video_id", videoID),
						zap.String("task_id", taskID),
						zap.Int("worker", workerID),
						zap.Int("seg", item.Job.Index),
						zap.Int("start_sec", item.Job.StartSec),
						zap.Int("end_sec", item.Job.EndSec),
						zap.String("key", item.Job.Key))
					select {
					case uploadedCh <- uploadedSegment{Job: item.Job, AudioPath: item.AudioPath}:
					case <-coarseCtx.Done():
						_ = os.Remove(item.AudioPath)
						return nil
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
		for w := 0; w < coarseASRWorkers; w++ {
			if err := asrPool.Submit(func() error {
				for item := range uploadedCh {
					if coarseCtx.Err() != nil {
						return nil
					}
					videoapp.RuntimeCounters().Inc("vector_coarse_asr_active")
					asrCtx, cancelASR := context.WithTimeout(coarseCtx, 12*time.Minute)
					text, err := client.Transcribe(asrCtx, item.AudioPath)
					cancelASR()
					_ = os.Remove(item.AudioPath)
					if err != nil {
						videoapp.RuntimeCounters().Dec("vector_coarse_asr_active")
						err := fmt.Errorf("asr failed seg=%d: %w", item.Job.Index, err)
						stageRecorder.Fail(coarseCtx, VectorStageTask{
							TaskID:       taskID,
							VideoID:      videoID,
							Stage:        VectorStageCoarseASR,
							SegmentIndex: item.Job.Index,
							StartSec:     item.Job.StartSec,
							EndSec:       item.Job.EndSec,
							ObjectKey:    item.Job.Key,
						}, err)
						_ = sendResult(clipResult{Index: item.Job.Index, Err: err})
						cancelCoarse()
						continue
					}
					videoapp.RuntimeCounters().Dec("vector_coarse_asr_active")
					text = normalizeText(text)
					stageRecorder.Complete(coarseCtx, VectorStageTask{
						TaskID:       taskID,
						VideoID:      videoID,
						Stage:        VectorStageCoarseASR,
						SegmentIndex: item.Job.Index,
						StartSec:     item.Job.StartSec,
						EndSec:       item.Job.EndSec,
						ObjectKey:    item.Job.Key,
					}, text)
					_ = sendResult(clipResult{
						Index: item.Job.Index,
						Item: tasks.CoarseItem{
							Index:     item.Job.Index,
							StartSec:  item.Job.StartSec,
							EndSec:    item.Job.EndSec,
							Text:      text,
							ObjectKey: item.Job.Key,
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
			_ = clipPool.Wait()
			close(clippedCh)
		}()
		go func() {
			_ = uploadPool.Wait()
			close(uploadedCh)
		}()
		go func() {
			_ = asrPool.Wait()
			close(results)
		}()

		byIndex := make([]tasks.CoarseItem, len(jobsList))
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

		coarseItems := make([]tasks.CoarseItem, 0, len(byIndex))
		for i := 0; i < len(byIndex); i++ {
			if byIndex[i].EndSec <= byIndex[i].StartSec {
				return fmt.Errorf("coarse segment missing index=%d", i)
			}
			coarseItems = append(coarseItems, byIndex[i])
		}

		zap.L().Debug("vectorize_hierarchical_coarse_done",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("total", len(coarseItems)),
			zap.Duration("cost", time.Since(startAt)))

		if strings.TrimSpace(llmModel) == "" {
			return errors.New("llm_model is required for hierarchical mode")
		}
		llmTask := VectorStageTask{
			TaskID:  taskID,
			VideoID: videoID,
			Stage:   VectorStageSegmentLLM,
			EndSec:  durationSec,
		}
		stageRecorder.Pending(ctx, llmTask)
		// LLM 只负责决定内容边界，真正的时间清洗由 normalizeLLMSegments 统一收口。
		prompt, err := tasks.BuildHierarchicalSegmentationPrompt(durationSec, coarseSegmentSec, refineMinSegmentSec, refineMaxSegmentSec, coarseItems)
		if err != nil {
			stageRecorder.Fail(ctx, llmTask, err)
			return err
		}
		zap.L().Debug("vectorize_hierarchical_llm_prompt_ready",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("chars", len(prompt)))
		llmOut, err := client.ChatCompletionsWithTimeout(ctx, llmModel, prompt, llmTimeoutMinutes)
		if err != nil {
			stageRecorder.Fail(ctx, llmTask, err)
			return err
		}
		segs, err := tasks.NormalizeLLMSegments(llmOut, durationSec, refineMinSegmentSec, refineMaxSegmentSec)
		if err != nil {
			preview := llmOut
			if len(preview) > 800 {
				preview = preview[:800]
			}
			err := fmt.Errorf("llm segmentation invalid: %w preview=%s", err, preview)
			stageRecorder.Fail(ctx, llmTask, err)
			return err
		}
		segs = tasks.RepairMismatchedSegments(segs, coarseItems)
		uniform, st := tasks.IsUniformSegments(segs)
		zap.L().Debug("vectorize_hierarchical_llm_segments_stats",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("count", st.Count),
			zap.Int("mode_bin", st.ModeBin),
			zap.Float64("mode_ratio", st.ModeRatio),
			zap.Int("min_sec", st.MinLen),
			zap.Int("max_sec", st.MaxLen),
			zap.Bool("uniform", uniform))
		if uniform {
			retryPrompt, err := tasks.BuildHierarchicalSegmentationRetryPrompt(durationSec, coarseSegmentSec, refineMinSegmentSec, refineMaxSegmentSec, coarseItems)
			if err != nil {
				return err
			}
			zap.L().Debug("vectorize_hierarchical_llm_retry_prompt_ready",
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
				segs2, err := tasks.NormalizeLLMSegments(llmOut2, durationSec, refineMinSegmentSec, refineMaxSegmentSec)
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
					uniform2, st2 := tasks.IsUniformSegments(segs2)
					zap.L().Debug("vectorize_hierarchical_llm_retry_segments_stats",
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
		zap.L().Debug("vectorize_hierarchical_llm_segments_normalized",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("total", len(segs)))
		if len(segs) > 0 {
			head := segs[0]
			tail := segs[len(segs)-1]
			zap.L().Debug("vectorize_hierarchical_llm_segments_head",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID),
				zap.Int("idx", head.SegmentIndex),
				zap.Int("start_sec", head.StartTimeSec),
				zap.Int("end_sec", head.EndTimeSec),
				zap.String("summary", head.ContentSummary),
				zap.Int("tags", len(head.KnowledgeTags)))
			zap.L().Debug("vectorize_hierarchical_llm_segments_tail",
				zap.Uint64("video_id", videoID),
				zap.String("task_id", taskID),
				zap.Int("idx", tail.SegmentIndex),
				zap.Int("start_sec", tail.StartTimeSec),
				zap.Int("end_sec", tail.EndTimeSec),
				zap.String("summary", tail.ContentSummary),
				zap.Int("tags", len(tail.KnowledgeTags)))
		}
		if err := tasks.UpsertHierarchicalSegments(ctx, db, videoID, segs); err != nil {
			stageRecorder.Fail(ctx, llmTask, err)
			return err
		}
		stageRecorder.Complete(ctx, llmTask, fmt.Sprintf("segments=%d", len(segs)))
		zap.L().Debug("vectorize_hierarchical_segments_saved",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("total", len(segs)))

		if err := tasks.RefineSegmentsASRAndEmbed(ctx, db, ff, client, tmpRoot, localVideo, videoID, taskID, durationSec, asrWorkers, embedBatch, embeddingDim, tailCfg, &tasks.RefineASRHints{
			CoarseItems:       coarseItems,
			Segments:          segs,
			LLMModel:          llmModel,
			LLMTimeoutMinutes: llmTimeoutMinutes,
		}, stageRecorder); err != nil {
			return err
		}
		zap.L().Debug("vectorize_hierarchical_segments_refined",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID))
		stageRecorder.Complete(ctx, VectorStageTask{
			TaskID:  taskID,
			VideoID: videoID,
			Stage:   VectorStageFinalize,
			EndSec:  durationSec,
		}, fmt.Sprintf("segments=%d", len(segs)))
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
	pool, err := antspool.New(ctx, antspool.Options{Name: "vector.sample_asr", Size: asrWorkers, Logger: zap.L()})
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
				asrTask := VectorStageTask{
					TaskID:       taskID,
					VideoID:      videoID,
					Stage:        VectorStageRefineASR,
					SegmentIndex: job.Index,
					StartSec:     job.StartSec,
					EndSec:       job.EndSec,
				}
				stageRecorder.Pending(ctx, asrTask)
				asrCtx, cancelASR := context.WithTimeout(ctx, 8*time.Minute)
				zap.L().Debug("vectorize_asr_start",
					zap.Uint64("video_id", videoID),
					zap.String("task_id", taskID),
					zap.Int("seg", job.Index))
				text, err := client.Transcribe(asrCtx, job.Path)
				cancelASR()
				_ = os.Remove(job.Path)
				if err != nil {
					stageRecorder.Fail(ctx, asrTask, err)
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
				stageRecorder.Complete(ctx, asrTask, text)
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
		offsets := tasks.BuildSampleOffsets(durationSec, sampleCount)
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
			zap.L().Debug("vectorize_extract_start",
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
				zap.L().Debug("vectorize_extract_ok",
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
				zap.L().Debug("vectorize_progress",
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
			zap.L().Debug("vectorize_extract_start",
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
				zap.L().Debug("vectorize_extract_ok",
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
		zap.L().Debug("vectorize_asr_ok",
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
		zap.L().Info("vectorize_task_done_empty",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("duration_sec", durationSec),
			zap.Int("attempted", attempted),
			zap.Int("extract_ok", extractOK),
			zap.Int("extract_err", extractErrs),
			zap.Int("asr_ok", asrOK),
			zap.Int("asr_err", asrErrs),
			zap.Int("non_empty", nonEmpty),
			zap.Int64("latency_ms", time.Since(startAt).Milliseconds()))
		return nil
	}

	texts := make([]string, 0, len(metas))
	for _, m := range metas {
		texts = append(texts, m.Text)
	}

	if embedBatch <= 0 {
		embedBatch = 64
	}
	zap.L().Debug("vectorize_embedding_start",
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
	if embeddingDim <= 0 {
		embeddingDim = 1536
	}
	for i, m := range metas {
		embeddingTask := VectorStageTask{
			TaskID:       taskID,
			VideoID:      videoID,
			Stage:        VectorStageEmbedding,
			SegmentIndex: m.Index,
			StartSec:     m.Start,
			EndSec:       m.End,
		}
		stageRecorder.Pending(ctx, embeddingTask)
		v := tasks.NormalizeEmbeddingDim(vecs[i], embeddingDim)
		if len(v) == 0 {
			stageRecorder.Fail(ctx, embeddingTask, errors.New("empty embedding"))
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
		stageRecorder.Complete(ctx, embeddingTask, summary)
	}

	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 写入前先把旧分段软删除，避免同一视频保留多版向量数据。
		res := tx.Model(&model.EduVideoSegment{}).Where("video_id = ? AND deleted = 0", videoID).Update("deleted", 1)
		if res.Error != nil {
			return res.Error
		}
		zap.L().Debug("vectorize_db_mark_deleted",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int64("rows", res.RowsAffected))
		if len(segs) == 0 {
			return nil
		}
		if err := tx.Create(&segs).Error; err != nil {
			return err
		}
		zap.L().Debug("vectorize_db_inserted",
			zap.Uint64("video_id", videoID),
			zap.String("task_id", taskID),
			zap.Int("rows", len(segs)))
		return nil
	}); err != nil {
		return err
	}

	zap.L().Info("vectorize_task_done",
		zap.Uint64("video_id", videoID),
		zap.String("task_id", taskID),
		zap.Int("segments", len(segs)),
		zap.Int("duration_sec", durationSec),
		zap.Int64("latency_ms", time.Since(startAt).Milliseconds()))
	stageRecorder.Complete(ctx, VectorStageTask{
		TaskID:  taskID,
		VideoID: videoID,
		Stage:   VectorStageFinalize,
		EndSec:  durationSec,
	}, fmt.Sprintf("segments=%d", len(segs)))
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
