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

	"nlp-video-analysis/internal/infrastructure/objectstorage"
	"nlp-video-analysis/internal/infrastructure/persistence"
	"nlp-video-analysis/internal/infrastructure/transcode"
	"nlp-video-analysis/internal/model"
)

const vectorStageCoarseSegment = "vector.coarse.segment"
const shortVideoSingleSegmentThresholdSec = 195

type prepareRepository interface {
	VideoExists(context.Context, uint64) (bool, error)
	HasExistingSegments(context.Context, uint64) (bool, error)
	UpsertPending(context.Context, persistence.VectorStageRecord) error
	MarkComplete(context.Context, persistence.VectorStageRecord) error
}

type rawVideoProber interface {
	Probe(context.Context, string) (int, error)
}

type taskScopedRawVideoProber interface {
	rawVideoProber
	ForTask(videoID uint64, taskID string) rawVideoProber
}

type prepareStageHandler struct {
	repo             prepareRepository
	prober           rawVideoProber
	coarseQueue      stageEnqueuer
	refineQueue      stageEnqueuer
	coarseSegmentSec int
}

func newPrepareStageHandler(repo prepareRepository, prober rawVideoProber, coarseQueue stageEnqueuer, coarseSegmentSec int) *prepareStageHandler {
	return newPrepareStageHandlerWithRefine(repo, prober, coarseQueue, nil, coarseSegmentSec)
}

func newPrepareStageHandlerWithRefine(repo prepareRepository, prober rawVideoProber, coarseQueue stageEnqueuer, refineQueue stageEnqueuer, coarseSegmentSec int) *prepareStageHandler {
	if coarseSegmentSec <= 0 {
		coarseSegmentSec = 60
	}
	return &prepareStageHandler{
		repo:             repo,
		prober:           prober,
		coarseQueue:      coarseQueue,
		refineQueue:      refineQueue,
		coarseSegmentSec: coarseSegmentSec,
	}
}

func (h *prepareStageHandler) Handle(ctx context.Context, task VectorStageTask) error {
	if task.TaskID == "" || task.VideoID == 0 || strings.TrimSpace(task.RawKey) == "" {
		return errors.New("invalid prepare task")
	}
	exists, err := h.repo.VideoExists(ctx, task.VideoID)
	if err != nil {
		return err
	}
	if !exists {
		zap.L().Debug("vector_stage_prepare_skip",
			zap.Uint64("video_id", task.VideoID),
			zap.String("task_id", task.TaskID),
			zap.String("reason", "video_not_found_or_deleted"))
		return nil
	}

	prober := h.prober
	if scoped, ok := h.prober.(taskScopedRawVideoProber); ok {
		prober = scoped.ForTask(task.VideoID, task.TaskID)
	}
	durationSec, err := prober.Probe(ctx, task.RawKey)
	if err != nil {
		return err
	}
	if durationSec <= 0 {
		return errors.New("invalid duration for hierarchical mode")
	}

	if h.refineQueue != nil {
		existing, err := h.repo.HasExistingSegments(ctx, task.VideoID)
		if err != nil {
			return err
		}
		if existing {
			if err := h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
				TaskID:  task.TaskID,
				VideoID: task.VideoID,
				Stage:   VectorStagePrepare,
				EndSec:  durationSec,
			}); err != nil {
				return err
			}
			return h.refineQueue.Enqueue(ctx, VectorStageTask{
				TaskID:  task.TaskID,
				VideoID: task.VideoID,
				RawKey:  task.RawKey,
				Stage:   VectorStageRefine,
				EndSec:  durationSec,
			})
		}
		if isShortSingleSegmentVideo(durationSec) {
			if err := h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
				TaskID:  task.TaskID,
				VideoID: task.VideoID,
				Stage:   VectorStagePrepare,
				EndSec:  durationSec,
			}); err != nil {
				return err
			}
			return h.refineQueue.Enqueue(ctx, VectorStageTask{
				TaskID:  task.TaskID,
				VideoID: task.VideoID,
				RawKey:  task.RawKey,
				Stage:   VectorStageRefine,
				EndSec:  durationSec,
			})
		}
	}

	prefix := fmt.Sprintf("segments/coarse/video_%d/%s", task.VideoID, strings.TrimSpace(task.TaskID))
	segIdx := 0
	for startSec := 0; startSec < durationSec; startSec += h.coarseSegmentSec {
		endSec := startSec + h.coarseSegmentSec
		if endSec > durationSec {
			endSec = durationSec
		}
		if endSec <= startSec {
			continue
		}
		key := filepath.ToSlash(filepath.Join(prefix, fmt.Sprintf("seg_%03d_%d_%d.mp4", segIdx, startSec, endSec)))
		if err := h.repo.UpsertPending(ctx, persistence.VectorStageRecord{
			TaskID:       task.TaskID,
			VideoID:      task.VideoID,
			Stage:        vectorStageCoarseSegment,
			SegmentIndex: segIdx,
			StartSec:     startSec,
			EndSec:       endSec,
			ObjectKey:    key,
		}); err != nil {
			return err
		}
		segIdx++
	}
	if segIdx == 0 {
		return errors.New("no coarse segments")
	}
	if err := h.repo.MarkComplete(ctx, persistence.VectorStageRecord{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		Stage:   VectorStagePrepare,
		EndSec:  durationSec,
	}); err != nil {
		return err
	}
	return h.coarseQueue.Enqueue(ctx, VectorStageTask{
		TaskID:  task.TaskID,
		VideoID: task.VideoID,
		RawKey:  task.RawKey,
		Stage:   VectorStageCoarse,
		EndSec:  durationSec,
	})
}

func isShortSingleSegmentVideo(durationSec int) bool {
	return durationSec > 0 && durationSec < shortVideoSingleSegmentThresholdSec
}

type gormPrepareRepository struct {
	db        *gorm.DB
	stageRepo *persistence.VectorStageRepository
}

func newGormPrepareRepository(db *gorm.DB, stageRepo *persistence.VectorStageRepository) *gormPrepareRepository {
	return &gormPrepareRepository{db: db, stageRepo: stageRepo}
}

func (r *gormPrepareRepository) VideoExists(ctx context.Context, videoID uint64) (bool, error) {
	var exists int64
	if err := r.db.WithContext(ctx).Model(&model.EduVideoResource{}).
		Where("id = ? AND deleted = 0", videoID).
		Count(&exists).Error; err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (r *gormPrepareRepository) HasExistingSegments(ctx context.Context, videoID uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.EduVideoSegment{}).
		Where("video_id = ? AND deleted = 0", videoID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *gormPrepareRepository) UpsertPending(ctx context.Context, rec persistence.VectorStageRecord) error {
	return r.stageRepo.UpsertPending(ctx, rec)
}

func (r *gormPrepareRepository) MarkComplete(ctx context.Context, rec persistence.VectorStageRecord) error {
	return r.stageRepo.MarkComplete(ctx, rec)
}

type objectStorageRawVideoProber struct {
	store   *objectstorage.RustFS
	ff      *transcode.FFmpegTranscoder
	tmpRoot string
	db      *gorm.DB
	videoID uint64
	taskID  string
}

func newObjectStorageRawVideoProber(store *objectstorage.RustFS, ff *transcode.FFmpegTranscoder, tmpRoot string, db *gorm.DB) *objectStorageRawVideoProber {
	return &objectStorageRawVideoProber{store: store, ff: ff, tmpRoot: tmpRoot, db: db}
}

func (p *objectStorageRawVideoProber) Probe(ctx context.Context, rawKey string) (int, error) {
	localVideo := filepath.Join(p.tmpRoot, p.taskID+"_"+filepath.Base(rawKey))
	if strings.TrimSpace(p.taskID) == "" {
		localVideo = filepath.Join(p.tmpRoot, "probe_"+filepath.Base(rawKey))
	}
	_ = os.Remove(localVideo)
	downloadCtx, cancelDownload := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelDownload()
	if err := p.store.DownloadToFile(downloadCtx, rawKey, localVideo); err != nil {
		return 0, fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(localVideo)
	if st, err := os.Stat(localVideo); err == nil {
		zap.L().Debug("vector_stage_prepare_downloaded",
			zap.Uint64("video_id", p.videoID),
			zap.String("task_id", p.taskID),
			zap.Int64("size", st.Size()),
			zap.String("path", localVideo))
	}

	probeCtx, cancelProbe := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelProbe()
	durationSec, err := p.ff.ProbeDurationSeconds(probeCtx, localVideo)
	if err != nil {
		zap.L().Error("vector_stage_prepare_probe_failed",
			zap.Uint64("video_id", p.videoID),
			zap.String("task_id", p.taskID),
			zap.Error(err))
		durationSec = 0
	}
	if durationSec > 0 && p.db != nil && p.videoID != 0 {
		_ = p.db.WithContext(ctx).Model(&model.EduVideoResource{}).
			Where("id = ? AND deleted = 0", p.videoID).
			Update("duration", durationSec).Error
	}
	return durationSec, nil
}

func (p *objectStorageRawVideoProber) ForTask(videoID uint64, taskID string) rawVideoProber {
	cp := *p
	cp.videoID = videoID
	cp.taskID = taskID
	return &cp
}
