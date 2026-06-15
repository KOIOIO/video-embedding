package vectorworker

import (
	"context"
	"errors"
	"os"
	"strings"

	"gorm.io/gorm"

	"nlp-video-analysis/internal/infrastructure/objectstorage"
	"nlp-video-analysis/internal/infrastructure/persistence"
	"nlp-video-analysis/internal/infrastructure/transcode"
	"nlp-video-analysis/internal/model"
	"nlp-video-analysis/internal/worker/vectorworker/tasks"
)

type refineStageRepository interface {
	ListStage(context.Context, string, string) ([]persistence.VectorStageRecord, error)
	HasExistingSegments(context.Context, uint64) (bool, error)
	MarkComplete(context.Context, persistence.VectorStageRecord) error
}

type refineStageProcessor interface {
	ProcessRefine(context.Context, VectorStageTask, []persistence.VectorStageRecord) error
}

type refineStageHandler struct {
	repo          refineStageRepository
	processor     refineStageProcessor
	finalizeQueue stageEnqueuer
}

type gormRefineStageRepository struct {
	db        *gorm.DB
	stageRepo *persistence.VectorStageRepository
}

func newGormRefineStageRepository(db *gorm.DB, stageRepo *persistence.VectorStageRepository) *gormRefineStageRepository {
	return &gormRefineStageRepository{db: db, stageRepo: stageRepo}
}

func (r *gormRefineStageRepository) ListStage(ctx context.Context, taskID string, stage string) ([]persistence.VectorStageRecord, error) {
	return r.stageRepo.ListStage(ctx, taskID, stage)
}

func (r *gormRefineStageRepository) HasExistingSegments(ctx context.Context, videoID uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.EduVideoSegment{}).
		Where("video_id = ? AND deleted = 0", videoID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *gormRefineStageRepository) MarkComplete(ctx context.Context, rec persistence.VectorStageRecord) error {
	return r.stageRepo.MarkComplete(ctx, rec)
}

func newRefineStageHandler(repo refineStageRepository, processor refineStageProcessor, finalizeQueue stageEnqueuer) *refineStageHandler {
	return &refineStageHandler{repo: repo, processor: processor, finalizeQueue: finalizeQueue}
}

func (h *refineStageHandler) Handle(ctx context.Context, task VectorStageTask) error {
	if task.TaskID == "" || task.VideoID == 0 || strings.TrimSpace(task.RawKey) == "" {
		return errors.New("invalid refine task")
	}
	coarse, err := h.repo.ListStage(ctx, task.TaskID, vectorStageCoarseSegment)
	if err != nil {
		return err
	}
	if len(coarse) == 0 {
		existing, err := h.repo.HasExistingSegments(ctx, task.VideoID)
		if err != nil {
			return err
		}
		if !existing {
			return errors.New("coarse transcript list is empty")
		}
	}
	if err := h.processor.ProcessRefine(ctx, task, coarse); err != nil {
		return err
	}
	if err := h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		Stage:   VectorStageRefine,
		EndSec:  task.EndSec,
	}); err != nil {
		return err
	}
	return h.finalizeQueue.Enqueue(ctx, VectorStageTask{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		RawKey:  task.RawKey,
		Stage:   VectorStageFinalize,
		EndSec:  task.EndSec,
	})
}

type productionRefineStageProcessor struct {
	db                  *gorm.DB
	store               *objectstorage.RustFS
	ff                  *transcode.FFmpegTranscoder
	client              *openAICompatClient
	tmpRoot             string
	coarseSegmentSec    int
	refineMinSegmentSec int
	refineMaxSegmentSec int
	llmModel            string
	llmTimeoutMinutes   int
	asrWorkers          int
	embedBatch          int
	embeddingDim        int
	tailCfg             tasks.TailAlignmentConfig
	stageRecorder       *vectorStageRecorder
}

func newProductionRefineStageProcessor(db *gorm.DB, store *objectstorage.RustFS, ff *transcode.FFmpegTranscoder, client *openAICompatClient, tmpRoot string, coarseSegmentSec int, refineMinSegmentSec int, refineMaxSegmentSec int, llmModel string, llmTimeoutMinutes int, asrWorkers int, embedBatch int, embeddingDim int, tailCfg tasks.TailAlignmentConfig, stageRecorder *vectorStageRecorder) *productionRefineStageProcessor {
	return &productionRefineStageProcessor{
		db:                  db,
		store:               store,
		ff:                  ff,
		client:              client,
		tmpRoot:             tmpRoot,
		coarseSegmentSec:    coarseSegmentSec,
		refineMinSegmentSec: refineMinSegmentSec,
		refineMaxSegmentSec: refineMaxSegmentSec,
		llmModel:            llmModel,
		llmTimeoutMinutes:   llmTimeoutMinutes,
		asrWorkers:          asrWorkers,
		embedBatch:          embedBatch,
		embeddingDim:        embeddingDim,
		tailCfg:             tailCfg,
		stageRecorder:       stageRecorder,
	}
}

func (p *productionRefineStageProcessor) ProcessRefine(ctx context.Context, task VectorStageTask, coarseRows []persistence.VectorStageRecord) error {
	localVideo, err := downloadStageRawVideo(ctx, p.store, p.tmpRoot, task)
	if err != nil {
		return err
	}
	defer os.Remove(localVideo)

	coarseItems := make([]tasks.CoarseItem, 0, len(coarseRows))
	for _, rec := range coarseRows {
		if strings.TrimSpace(rec.Text) == "" && strings.TrimSpace(rec.ObjectKey) == "" {
			continue
		}
		if rec.EndSec <= rec.StartSec {
			continue
		}
		coarseItems = append(coarseItems, tasks.CoarseItem{
			Index:     rec.SegmentIndex,
			StartSec:  rec.StartSec,
			EndSec:    rec.EndSec,
			Text:      strings.TrimSpace(rec.Text),
			ObjectKey: strings.TrimSpace(rec.ObjectKey),
		})
	}
	return processHierarchicalRefine(ctx, hierarchicalRefineInput{
		DB:                  p.db,
		FFmpeg:              p.ff,
		Client:              p.client,
		TmpRoot:             p.tmpRoot,
		LocalVideo:          localVideo,
		VideoID:             task.VideoID,
		TaskID:              task.TaskID,
		DurationSec:         task.EndSec,
		CoarseSegmentSec:    p.coarseSegmentSec,
		RefineMinSegmentSec: p.refineMinSegmentSec,
		RefineMaxSegmentSec: p.refineMaxSegmentSec,
		LLMModel:            p.llmModel,
		LLMTimeoutMinutes:   p.llmTimeoutMinutes,
		ASRWorkers:          p.asrWorkers,
		EmbedBatch:          p.embedBatch,
		EmbeddingDim:        p.embeddingDim,
		TailCfg:             p.tailCfg,
		CoarseItems:         coarseItems,
		StageRecorder:       p.stageRecorder,
	})
}
