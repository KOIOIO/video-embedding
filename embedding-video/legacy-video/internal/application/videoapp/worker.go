package videoapp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	domainvideo "legacy-video/internal/domain/video"

	"go.uber.org/zap"
)

// Transcoder 抽象 HLS 转码与封面截图能力。
type Transcoder interface {
	ConvertToHLS(ctx context.Context, inputPath string, outputDir string) error
	GenerateCover(ctx context.Context, inputPath string, outputPath string) error
}

// ObjectDownloader 抽象从对象存储拉取原始视频到本地的能力。
type ObjectDownloader interface {
	DownloadToFile(ctx context.Context, objectKey string, filePath string) error
}

// DirUploader 抽象把整个 HLS 目录上传到对象存储的能力。
type DirUploader interface {
	UploadDir(ctx context.Context, localDir string, objectPrefix string) error
}

// Worker 负责执行单个转码任务的完整生命周期。
// 它从队列中取出任务，下载原视频、转码、上传产物、更新状态并清理临时文件。
type Worker struct {
	Queue       TranscodeTaskConsumer
	StatusStore TranscodeStatusStore
	Repo        VideoRepository
	Transcoder  Transcoder
	Store       ObjectStore
	Downloader  ObjectDownloader
	Uploader    DirUploader
	FS          FileStorage
	TempRawDir  string
	TempHlsDir  string
	Now         func() time.Time
	StatusTTL   time.Duration
	TaskTimeout time.Duration
	Leases      LeaseStore
	WorkerID    string
	LeaseTTL    time.Duration
	RetryPolicy RetryPolicy
	Execute     func(ctx context.Context, fn func(context.Context) error) error
}

// NewWorker 创建转码 worker。
func NewWorker(queue TranscodeTaskConsumer, statusStore TranscodeStatusStore, repo VideoRepository, transcoder Transcoder, store ObjectStore, downloader ObjectDownloader, uploader DirUploader, fs FileStorage, tempRawDir string, tempHlsDir string, taskTimeout time.Duration) *Worker {
	return &Worker{
		Queue:       queue,
		StatusStore: statusStore,
		Repo:        repo,
		Transcoder:  transcoder,
		Store:       store,
		Downloader:  downloader,
		Uploader:    uploader,
		FS:          fs,
		TempRawDir:  tempRawDir,
		TempHlsDir:  tempHlsDir,
		Now:         time.Now,
		StatusTTL:   24 * time.Hour,
		TaskTimeout: taskTimeout,
		LeaseTTL:    time.Minute,
		RetryPolicy: DefaultRetryPolicy,
		Execute: func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	}
}

// RunOnce 消费并执行一个转码任务。
// 任务阶段包括：校验视频存在、下载原片、转 HLS、上传 HLS、生成封面、更新数据库与缓存状态。
func (w *Worker) RunOnce(ctx context.Context) error {
	msg, err := w.Queue.Dequeue(ctx)
	if err != nil {
		return err
	}
	taskCtx := ctx
	var cancel context.CancelFunc
	if w.TaskTimeout > 0 {
		taskCtx, cancel = context.WithTimeout(ctx, w.TaskTimeout)
		defer cancel()
	}
	if w.Execute != nil {
		return w.Execute(taskCtx, func(runCtx context.Context) error {
			return w.runTask(runCtx, msg)
		})
	}
	return w.runTask(taskCtx, msg)
}

func (w *Worker) runTask(taskCtx context.Context, msg TranscodeQueueMessage) error {
	task := msg.Task
	if w.Leases != nil {
		leaseTTL := w.LeaseTTL
		if leaseTTL <= 0 {
			leaseTTL = time.Minute
		}
		if err := w.Leases.Acquire(taskCtx, Lease{
			TaskID:    task.TaskID,
			MessageID: msg.MessageID,
			WorkerID:  w.WorkerID,
			Stage:     "start",
			ExpiresAt: time.Now().Add(leaseTTL),
		}, leaseTTL); err != nil {
			return err
		}
		defer func() {
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(taskCtx), 5*time.Second)
			defer cancel()
			_ = w.Leases.Release(releaseCtx, task.TaskID)
		}()
	}

	if _, ok, err := w.Repo.GetByID(taskCtx, task.VideoID); err != nil {
		return err
	} else if !ok {
		_ = w.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusFailed, task.HLSURL, w.StatusTTL)
		zap.L().Info("skip",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("reason", "video_not_found_or_deleted"),
		)
		_ = w.Queue.Ack(taskCtx, msg.MessageID)
		return nil
	}

	start := time.Now()
	localInput := filepath.Join(w.TempRawDir, task.TaskID+"_"+filepath.Base(task.RawKey))
	localOutputDir := filepath.Join(w.TempHlsDir, task.TaskID)
	zap.L().Info("start",
		zap.String("task_id", task.TaskID),
		zap.Uint64("video_id", task.VideoID),
		zap.String("raw_key", task.RawKey),
		zap.String("hls_prefix", task.HLSObjectPrefix),
		zap.String("cover", "on"),
	)

	_ = w.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusProcessing, task.HLSURL, w.StatusTTL)
	_ = w.Repo.UpdateStatusByID(taskCtx, task.VideoID, domainvideo.StatusProcessing, "")

	_ = w.FS.MkdirAll(w.TempRawDir)
	_ = w.FS.MkdirAll(w.TempHlsDir)
	_ = w.FS.RemoveAll(localOutputDir)

	// 先把原始视频拉到本地临时目录，后续 FFmpeg 统一在本地文件上工作。
	if err := w.Downloader.DownloadToFile(taskCtx, task.RawKey, localInput); err != nil {
		return w.handleTaskFailure(taskCtx, msg, start, "download", err)
	}

	// 转码成功后，目录内应产出 master.m3u8 与切片文件。
	if err := w.Transcoder.ConvertToHLS(taskCtx, localInput, localOutputDir); err != nil {
		_ = w.FS.RemoveAll(localOutputDir)
		_ = w.FS.Remove(localInput)
		return w.handleTaskFailure(taskCtx, msg, start, "transcode", err)
	}

	// HLS 目录整体上传，保证切片与索引文件的一致性。
	if err := w.Uploader.UploadDir(taskCtx, localOutputDir, task.HLSObjectPrefix); err != nil {
		_ = w.FS.RemoveAll(localOutputDir)
		_ = w.FS.Remove(localInput)
		return w.handleTaskFailure(taskCtx, msg, start, "upload", err)
	}

	// 封面生成失败不阻塞主流程，因此这里按旁路方式处理。
	coverLocalPath := filepath.Join(localOutputDir, "cover.jpg")
	coverKey := coverObjectKey(task.HLSObjectPrefix)
	coverURL := "/videos/" + coverKey
	if err := w.Transcoder.GenerateCover(taskCtx, localInput, coverLocalPath); err != nil {
		zap.L().Info("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "skip"),
			zap.String("err", err.Error()),
		)
	} else if err := w.Store.PutFile(taskCtx, coverKey, coverLocalPath, "image/jpeg"); err != nil {
		zap.L().Error("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "upload_failed"),
			zap.String("key", coverKey),
			zap.String("err", err.Error()),
		)
	} else if ok, err := w.Repo.UpdateCoverByID(taskCtx, task.VideoID, coverURL); err != nil {
		zap.L().Error("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "db_failed"),
			zap.String("url", coverURL),
			zap.String("err", err.Error()),
		)
	} else if !ok {
		zap.L().Info("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "db_not_found"),
			zap.String("url", coverURL),
		)
	} else {
		zap.L().Info("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "ok"),
			zap.String("url", coverURL),
		)
	}

	_ = w.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusDone, task.HLSURL, w.StatusTTL)
	_ = w.Repo.UpdateStatusByID(taskCtx, task.VideoID, domainvideo.StatusDone, "")
	_ = w.FS.RemoveAll(localOutputDir)
	_ = w.FS.Remove(localInput)
	_ = w.Queue.Ack(taskCtx, msg.MessageID)
	zap.L().Info("done",
		zap.String("task_id", task.TaskID),
		zap.Uint64("video_id", task.VideoID),
		zap.String("status", "done"),
		zap.Int64("latency_ms", time.Since(start).Milliseconds()),
	)
	return nil
}

func (w *Worker) handleTaskFailure(taskCtx context.Context, msg TranscodeQueueMessage, start time.Time, stage string, err error) error {
	task := msg.Task
	wrapped := fmt.Errorf("%s failed task_id=%s: %w", stage, task.TaskID, err)
	decision := DefaultRetryPolicy(wrapped, task.RetryCount)
	if w.RetryPolicy != nil {
		decision = w.RetryPolicy(wrapped, task.RetryCount)
	}
	if decision.Retry {
		retryMsg := msg
		retryMsg.Task.RetryCount++
		_ = w.Queue.Requeue(taskCtx, retryMsg, decision.Delay, decision.Reason)
		_ = w.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusUploaded, task.HLSURL, w.StatusTTL)
		_ = w.Repo.UpdateStatusByID(taskCtx, task.VideoID, domainvideo.StatusUploaded, decision.Reason)
	} else {
		_ = w.Queue.MoveToDeadLetter(taskCtx, msg, decision.Reason)
		_ = w.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusFailed, task.HLSURL, w.StatusTTL)
		_ = w.Repo.UpdateStatusByID(taskCtx, task.VideoID, domainvideo.StatusFailed, err.Error())
	}
	zap.L().Error("failed",
		zap.String("task_id", task.TaskID),
		zap.Uint64("video_id", task.VideoID),
		zap.String("stage", stage),
		zap.Int64("latency_ms", time.Since(start).Milliseconds()),
		zap.Bool("retry", decision.Retry),
		zap.String("reason", decision.Reason),
		zap.String("err", err.Error()),
	)
	return wrapped
}

// coverObjectKey 根据 HLS 目录前缀推导封面对象存储路径。
func coverObjectKey(hlsObjectPrefix string) string {
	prefix := strings.TrimSpace(hlsObjectPrefix)
	prefix = strings.Trim(prefix, "/")
	prefix = strings.TrimPrefix(prefix, "hls/")
	return filepath.ToSlash(filepath.Join("cover", prefix, "cover.jpg"))
}
