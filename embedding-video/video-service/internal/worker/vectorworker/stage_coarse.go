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

	"nlp-video-analysis/internal/infrastructure/objectstorage"
	"nlp-video-analysis/internal/infrastructure/persistence"
	"nlp-video-analysis/internal/infrastructure/transcode"
)

type coarseStageRepository interface {
	ListStage(context.Context, string, string) ([]persistence.VectorStageRecord, error)
	MarkComplete(context.Context, persistence.VectorStageRecord) error
}

type coarseStageProcessor interface {
	ProcessCoarse(context.Context, VectorStageTask, []persistence.VectorStageRecord) error
}

type coarseStageHandler struct {
	repo        coarseStageRepository
	processor   coarseStageProcessor
	refineQueue stageEnqueuer
}

func newCoarseStageHandler(repo coarseStageRepository, processor coarseStageProcessor, refineQueue stageEnqueuer) *coarseStageHandler {
	return &coarseStageHandler{repo: repo, processor: processor, refineQueue: refineQueue}
}

func (h *coarseStageHandler) Handle(ctx context.Context, task VectorStageTask) error {
	if task.TaskID == "" || task.VideoID == 0 || strings.TrimSpace(task.RawKey) == "" {
		return errors.New("invalid coarse task")
	}
	plan, err := h.repo.ListStage(ctx, task.TaskID, vectorStageCoarseSegment)
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		return errors.New("coarse segment plan is empty")
	}
	if err := h.processor.ProcessCoarse(ctx, task, plan); err != nil {
		return err
	}
	if err := h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		Stage:   VectorStageCoarse,
		EndSec:  task.EndSec,
	}); err != nil {
		return err
	}
	return h.refineQueue.Enqueue(ctx, VectorStageTask{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		RawKey:  task.RawKey,
		Stage:   VectorStageRefine,
		EndSec:  task.EndSec,
	})
}

type productionCoarseStageProcessor struct {
	store         *objectstorage.RustFS
	ff            *transcode.FFmpegTranscoder
	client        vectorAIClient
	tmpRoot       string
	asrWorkers    int
	coarseWorkers int
	stageRecorder *vectorStageRecorder
}

func newProductionCoarseStageProcessor(store *objectstorage.RustFS, ff *transcode.FFmpegTranscoder, client vectorAIClient, tmpRoot string, asrWorkers int, coarseWorkers int, stageRecorder *vectorStageRecorder) *productionCoarseStageProcessor {
	return &productionCoarseStageProcessor{
		store:         store,
		ff:            ff,
		client:        client,
		tmpRoot:       tmpRoot,
		asrWorkers:    asrWorkers,
		coarseWorkers: coarseWorkers,
		stageRecorder: stageRecorder,
	}
}

func (p *productionCoarseStageProcessor) ProcessCoarse(ctx context.Context, task VectorStageTask, plan []persistence.VectorStageRecord) error {
	localVideo, err := downloadStageRawVideo(ctx, p.store, p.tmpRoot, task)
	if err != nil {
		return err
	}
	defer os.Remove(localVideo)

	jobs := make([]coarseClipJob, 0, len(plan))
	for _, rec := range plan {
		if rec.EndSec <= rec.StartSec {
			return fmt.Errorf("invalid coarse segment plan index=%d", rec.SegmentIndex)
		}
		key := strings.TrimSpace(rec.ObjectKey)
		if key == "" {
			return fmt.Errorf("coarse segment object key missing index=%d", rec.SegmentIndex)
		}
		jobs = append(jobs, coarseClipJob{
			Index:    rec.SegmentIndex,
			StartSec: rec.StartSec,
			EndSec:   rec.EndSec,
			DurSec:   rec.EndSec - rec.StartSec,
			Key:      key,
		})
	}
	_, err = processHierarchicalCoarseSegments(ctx, hierarchicalCoarseInput{
		Store:         p.store,
		FFmpeg:        p.ff,
		Client:        p.client,
		TmpRoot:       p.tmpRoot,
		LocalVideo:    localVideo,
		VideoID:       task.VideoID,
		TaskID:        task.TaskID,
		ASRWorkers:    p.asrWorkers,
		CoarseWorkers: p.coarseWorkers,
		Jobs:          jobs,
		StageRecorder: p.stageRecorder,
	})
	return err
}

func downloadStageRawVideo(ctx context.Context, store *objectstorage.RustFS, tmpRoot string, task VectorStageTask) (string, error) {
	if store == nil {
		return "", errors.New("object storage is required")
	}
	localVideo := filepath.Join(tmpRoot, task.TaskID+"_"+filepath.Base(task.RawKey))
	_ = os.Remove(localVideo)
	downloadCtx, cancelDownload := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelDownload()
	if err := store.DownloadToFile(downloadCtx, task.RawKey, localVideo); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	if st, err := os.Stat(localVideo); err == nil {
		zap.L().Debug("vector_stage_downloaded",
			zap.Uint64("video_id", task.VideoID),
			zap.String("task_id", task.TaskID),
			zap.String("stage", task.Stage),
			zap.Int64("size", st.Size()),
			zap.String("path", localVideo))
	}
	return localVideo, nil
}
