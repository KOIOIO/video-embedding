package vectorworker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/infrastructure/objectstorage"
	"nlp-video-analysis/internal/infrastructure/transcode"
	"nlp-video-analysis/internal/worker/antspool"
	"nlp-video-analysis/internal/worker/vectorworker/tasks"
)

type coarseClipJob struct {
	Index    int
	StartSec int
	EndSec   int
	DurSec   int
	Key      string
}

type hierarchicalCoarseInput struct {
	Store         *objectstorage.RustFS
	FFmpeg        *transcode.FFmpegTranscoder
	Client        *openAICompatClient
	TmpRoot       string
	LocalVideo    string
	VideoID       uint64
	TaskID        string
	ASRWorkers    int
	CoarseWorkers int
	Jobs          []coarseClipJob
	StageRecorder *vectorStageRecorder
}

func processHierarchicalCoarseSegments(ctx context.Context, input hierarchicalCoarseInput) ([]tasks.CoarseItem, error) {
	if len(input.Jobs) == 0 {
		return nil, errors.New("no coarse segments")
	}
	type clipResult struct {
		Index int
		Item  tasks.CoarseItem
		Err   error
	}
	type clippedSegment struct {
		Job       coarseClipJob
		ClipPath  string
		AudioPath string
	}
	type uploadedSegment struct {
		Job       coarseClipJob
		AudioPath string
	}

	for _, j := range input.Jobs {
		input.StageRecorder.Pending(ctx, VectorStageTask{
			TaskID:       input.TaskID,
			VideoID:      input.VideoID,
			Stage:        VectorStageCoarseClip,
			SegmentIndex: j.Index,
			StartSec:     j.StartSec,
			EndSec:       j.EndSec,
			ObjectKey:    j.Key,
		})
	}

	requestedCoarseWorkers := input.CoarseWorkers
	clipWorkers, uploadWorkers, coarseASRWorkers := calcCoarsePipelineStageWorkers(requestedCoarseWorkers, input.ASRWorkers, len(input.Jobs))
	zap.L().Debug("vectorize_hierarchical_coarse_start",
		zap.Uint64("video_id", input.VideoID),
		zap.String("task_id", input.TaskID),
		zap.Int("segments", len(input.Jobs)),
		zap.Int("requested_workers", requestedCoarseWorkers),
		zap.Int("workers", clipWorkers),
		zap.Int("clip_workers", clipWorkers),
		zap.Int("upload_workers", uploadWorkers),
		zap.Int("coarse_asr_workers", coarseASRWorkers),
		zap.Int("max_workers", maxCoarseWorkers))

	coarseCtx, cancelCoarse := context.WithCancel(ctx)
	defer cancelCoarse()

	jobs := make(chan coarseClipJob, clipWorkers*2)
	clippedCh := make(chan clippedSegment, clipWorkers*2)
	uploadedCh := make(chan uploadedSegment, uploadWorkers*2)
	results := make(chan clipResult, clipWorkers*2)
	enqueueErrCh := make(chan error, 1)

	clipPool, err := antspool.New(coarseCtx, antspool.Options{Name: poolVectorCoarse, Size: clipWorkers, Logger: zap.L()})
	if err != nil {
		return nil, err
	}
	defer clipPool.Release()
	uploadPool, err := antspool.New(coarseCtx, antspool.Options{Name: poolVectorCoarse, Size: uploadWorkers, Logger: zap.L()})
	if err != nil {
		return nil, err
	}
	defer uploadPool.Release()
	asrPool, err := antspool.New(coarseCtx, antspool.Options{Name: poolVectorCoarse, Size: coarseASRWorkers, Logger: zap.L()})
	if err != nil {
		return nil, err
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

				clipPath := filepath.Join(input.TmpRoot, fmt.Sprintf("%s_coarse_%03d_%d_%d.mp4", input.TaskID, j.Index, j.StartSec, j.EndSec))
				audioPath := filepath.Join(input.TmpRoot, fmt.Sprintf("%s_coarse_%03d_%d_%d.wav", input.TaskID, j.Index, j.StartSec, j.EndSec))
				_ = os.Remove(clipPath)
				_ = os.Remove(audioPath)

				clipCtx, cancelClip := context.WithTimeout(coarseCtx, 12*time.Minute)
				err := input.FFmpeg.ClipVideoSegmentWithAudio(clipCtx, input.LocalVideo, clipPath, audioPath, j.StartSec, j.DurSec)
				cancelClip()
				if err != nil {
					_ = os.Remove(clipPath)
					_ = os.Remove(audioPath)
					videoapp.RuntimeCounters().Dec("vector_coarse_clip_active")
					err := fmt.Errorf("clip+audio failed seg=%d: %w", j.Index, err)
					input.StageRecorder.Fail(coarseCtx, VectorStageTask{
						TaskID:       input.TaskID,
						VideoID:      input.VideoID,
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
			return nil, err
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
				err := input.Store.PutFile(putCtx, item.Job.Key, item.ClipPath, "video/mp4")
				cancelPut()
				_ = os.Remove(item.ClipPath)
				if err != nil {
					_ = os.Remove(item.AudioPath)
					videoapp.RuntimeCounters().Dec("vector_coarse_upload_active")
					err := fmt.Errorf("upload failed seg=%d key=%s: %w", item.Job.Index, item.Job.Key, err)
					input.StageRecorder.Fail(coarseCtx, VectorStageTask{
						TaskID:       input.TaskID,
						VideoID:      input.VideoID,
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
				input.StageRecorder.Complete(coarseCtx, VectorStageTask{
					TaskID:       input.TaskID,
					VideoID:      input.VideoID,
					Stage:        VectorStageCoarseClip,
					SegmentIndex: item.Job.Index,
					StartSec:     item.Job.StartSec,
					EndSec:       item.Job.EndSec,
					ObjectKey:    item.Job.Key,
				}, "")
				input.StageRecorder.Pending(coarseCtx, VectorStageTask{
					TaskID:       input.TaskID,
					VideoID:      input.VideoID,
					Stage:        VectorStageCoarseASR,
					SegmentIndex: item.Job.Index,
					StartSec:     item.Job.StartSec,
					EndSec:       item.Job.EndSec,
					ObjectKey:    item.Job.Key,
				})
				zap.L().Debug("vectorize_hierarchical_coarse_uploaded",
					zap.Uint64("video_id", input.VideoID),
					zap.String("task_id", input.TaskID),
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
			return nil, err
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
				text, err := input.Client.Transcribe(asrCtx, item.AudioPath)
				cancelASR()
				_ = os.Remove(item.AudioPath)
				if err != nil {
					videoapp.RuntimeCounters().Dec("vector_coarse_asr_active")
					err := fmt.Errorf("asr failed seg=%d: %w", item.Job.Index, err)
					input.StageRecorder.Fail(coarseCtx, VectorStageTask{
						TaskID:       input.TaskID,
						VideoID:      input.VideoID,
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
				input.StageRecorder.Complete(coarseCtx, VectorStageTask{
					TaskID:       input.TaskID,
					VideoID:      input.VideoID,
					Stage:        VectorStageCoarseASR,
					SegmentIndex: item.Job.Index,
					StartSec:     item.Job.StartSec,
					EndSec:       item.Job.EndSec,
					ObjectKey:    item.Job.Key,
				}, text)
				input.StageRecorder.Complete(coarseCtx, VectorStageTask{
					TaskID:       input.TaskID,
					VideoID:      input.VideoID,
					Stage:        vectorStageCoarseSegment,
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
			return nil, err
		}
	}

	go func() {
		defer close(jobs)
		for _, j := range input.Jobs {
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

	byIndex := make([]tasks.CoarseItem, len(input.Jobs))
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
		return nil, firstErr
	}
	if enqueueErr := <-enqueueErrCh; enqueueErr != nil && !errors.Is(enqueueErr, context.Canceled) {
		return nil, enqueueErr
	}
	if got < len(input.Jobs) {
		return nil, errors.New("coarse segmentation incomplete")
	}

	coarseItems := make([]tasks.CoarseItem, 0, len(byIndex))
	for i := 0; i < len(byIndex); i++ {
		if byIndex[i].EndSec <= byIndex[i].StartSec {
			return nil, fmt.Errorf("coarse segment missing index=%d", i)
		}
		coarseItems = append(coarseItems, byIndex[i])
	}
	zap.L().Debug("vectorize_hierarchical_coarse_done",
		zap.Uint64("video_id", input.VideoID),
		zap.String("task_id", input.TaskID),
		zap.Int("total", len(coarseItems)))
	return coarseItems, nil
}

type hierarchicalRefineInput struct {
	DB                  *gorm.DB
	FFmpeg              *transcode.FFmpegTranscoder
	Client              *openAICompatClient
	TmpRoot             string
	LocalVideo          string
	VideoID             uint64
	TaskID              string
	DurationSec         int
	CoarseSegmentSec    int
	RefineMinSegmentSec int
	RefineMaxSegmentSec int
	LLMModel            string
	LLMTimeoutMinutes   int
	ASRWorkers          int
	EmbedBatch          int
	EmbeddingDim        int
	TailCfg             tasks.TailAlignmentConfig
	CoarseItems         []tasks.CoarseItem
	StageRecorder       *vectorStageRecorder
}

func processHierarchicalRefine(ctx context.Context, input hierarchicalRefineInput) error {
	if input.DurationSec <= 0 {
		return errors.New("invalid duration for hierarchical mode")
	}
	if len(input.CoarseItems) == 0 {
		zap.L().Debug("vectorize_hierarchical_resume",
			zap.Uint64("video_id", input.VideoID),
			zap.String("task_id", input.TaskID),
			zap.String("action", "skip_segmentation_resume_embedding"))
		if err := tasks.RefineSegmentsASRAndEmbed(ctx, input.DB, input.FFmpeg, input.Client, input.TmpRoot, input.LocalVideo, input.VideoID, input.TaskID, input.DurationSec, input.ASRWorkers, input.EmbedBatch, input.EmbeddingDim, input.TailCfg, nil, input.StageRecorder); err != nil {
			return err
		}
		zap.L().Debug("vectorize_hierarchical_resume_done",
			zap.Uint64("video_id", input.VideoID),
			zap.String("task_id", input.TaskID))
		return nil
	}
	if strings.TrimSpace(input.LLMModel) == "" {
		return errors.New("llm_model is required for hierarchical mode")
	}

	llmTask := VectorStageTask{
		TaskID:  input.TaskID,
		VideoID: input.VideoID,
		Stage:   VectorStageSegmentLLM,
		EndSec:  input.DurationSec,
	}
	input.StageRecorder.Pending(ctx, llmTask)
	prompt, err := tasks.BuildHierarchicalSegmentationPrompt(input.DurationSec, input.CoarseSegmentSec, input.RefineMinSegmentSec, input.RefineMaxSegmentSec, input.CoarseItems)
	if err != nil {
		input.StageRecorder.Fail(ctx, llmTask, err)
		return err
	}
	zap.L().Debug("vectorize_hierarchical_llm_prompt_ready",
		zap.Uint64("video_id", input.VideoID),
		zap.String("task_id", input.TaskID),
		zap.Int("chars", len(prompt)))
	llmOut, err := input.Client.ChatCompletionsWithTimeout(ctx, input.LLMModel, prompt, input.LLMTimeoutMinutes)
	if err != nil {
		input.StageRecorder.Fail(ctx, llmTask, err)
		return err
	}
	segs, err := tasks.NormalizeLLMSegments(llmOut, input.DurationSec, input.RefineMinSegmentSec, input.RefineMaxSegmentSec)
	if err != nil {
		preview := llmOut
		if len(preview) > 800 {
			preview = preview[:800]
		}
		err := fmt.Errorf("llm segmentation invalid: %w preview=%s", err, preview)
		input.StageRecorder.Fail(ctx, llmTask, err)
		return err
	}
	segs = tasks.RepairMismatchedSegments(segs, input.CoarseItems)
	uniform, st := tasks.IsUniformSegments(segs)
	zap.L().Debug("vectorize_hierarchical_llm_segments_stats",
		zap.Uint64("video_id", input.VideoID),
		zap.String("task_id", input.TaskID),
		zap.Int("count", st.Count),
		zap.Int("mode_bin", st.ModeBin),
		zap.Float64("mode_ratio", st.ModeRatio),
		zap.Int("min_sec", st.MinLen),
		zap.Int("max_sec", st.MaxLen),
		zap.Bool("uniform", uniform))
	if uniform {
		retryPrompt, err := tasks.BuildHierarchicalSegmentationRetryPrompt(input.DurationSec, input.CoarseSegmentSec, input.RefineMinSegmentSec, input.RefineMaxSegmentSec, input.CoarseItems)
		if err != nil {
			return err
		}
		zap.L().Debug("vectorize_hierarchical_llm_retry_prompt_ready",
			zap.Uint64("video_id", input.VideoID),
			zap.String("task_id", input.TaskID),
			zap.Int("chars", len(retryPrompt)))
		llmOut2, err := input.Client.ChatCompletionsWithTimeout(ctx, input.LLMModel, retryPrompt, input.LLMTimeoutMinutes)
		if err != nil {
			zap.L().Error("vectorize_hierarchical_llm_retry_failed",
				zap.Uint64("video_id", input.VideoID),
				zap.String("task_id", input.TaskID),
				zap.Error(err))
		} else {
			segs2, err := tasks.NormalizeLLMSegments(llmOut2, input.DurationSec, input.RefineMinSegmentSec, input.RefineMaxSegmentSec)
			if err != nil {
				preview := llmOut2
				if len(preview) > 800 {
					preview = preview[:800]
				}
				zap.L().Error("vectorize_hierarchical_llm_retry_invalid",
					zap.Uint64("video_id", input.VideoID),
					zap.String("task_id", input.TaskID),
					zap.Error(err),
					zap.String("preview", preview))
			} else {
				uniform2, st2 := tasks.IsUniformSegments(segs2)
				zap.L().Debug("vectorize_hierarchical_llm_retry_segments_stats",
					zap.Uint64("video_id", input.VideoID),
					zap.String("task_id", input.TaskID),
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
		zap.Uint64("video_id", input.VideoID),
		zap.String("task_id", input.TaskID),
		zap.Int("total", len(segs)))
	if err := tasks.UpsertHierarchicalSegments(ctx, input.DB, input.VideoID, segs); err != nil {
		input.StageRecorder.Fail(ctx, llmTask, err)
		return err
	}
	input.StageRecorder.Complete(ctx, llmTask, fmt.Sprintf("segments=%d", len(segs)))
	zap.L().Debug("vectorize_hierarchical_segments_saved",
		zap.Uint64("video_id", input.VideoID),
		zap.String("task_id", input.TaskID),
		zap.Int("total", len(segs)))

	if err := tasks.RefineSegmentsASRAndEmbed(ctx, input.DB, input.FFmpeg, input.Client, input.TmpRoot, input.LocalVideo, input.VideoID, input.TaskID, input.DurationSec, input.ASRWorkers, input.EmbedBatch, input.EmbeddingDim, input.TailCfg, &tasks.RefineASRHints{
		CoarseItems:       input.CoarseItems,
		Segments:          segs,
		LLMModel:          input.LLMModel,
		LLMTimeoutMinutes: input.LLMTimeoutMinutes,
	}, input.StageRecorder); err != nil {
		return err
	}
	zap.L().Debug("vectorize_hierarchical_segments_refined",
		zap.Uint64("video_id", input.VideoID),
		zap.String("task_id", input.TaskID))
	return nil
}
