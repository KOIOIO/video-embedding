package worker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	domainvideo "nlp-video-analysis/internal/domain/video"

	"go.uber.org/zap"
)

const MaxRetryAttempts = 5

type Task struct {
	VideoID         uint64
	RawKey          string
	HLSObjectPrefix string
	TaskID          string
	HLSURL          string
	RetryCount      int
}

type QueueMessage struct {
	MessageID string
	Task      Task
}

type Queue interface {
	Dequeue(ctx context.Context) (QueueMessage, error)
	Ack(ctx context.Context, messageID string) error
	Requeue(ctx context.Context, msg QueueMessage, delay time.Duration, reason string) error
	MoveToDeadLetter(ctx context.Context, msg QueueMessage, reason string) error
}

type RetryDecision struct {
	Retry  bool
	Delay  time.Duration
	Reason string
}

type RetryPolicy func(err error, retries int) RetryDecision

type Lease struct {
	TaskID    string
	MessageID string
	WorkerID  string
	Stage     string
	ExpiresAt time.Time
}

type LeaseStore interface {
	Acquire(ctx context.Context, lease Lease, ttl time.Duration) error
	Renew(ctx context.Context, lease Lease, ttl time.Duration) error
	Release(ctx context.Context, taskID string) error
}

type CounterStore interface {
	Inc(name string)
	Dec(name string)
}

type StatusStore interface {
	Set(ctx context.Context, taskID string, status domainvideo.Status, hlsURL string, ttl time.Duration) error
}

type Repository interface {
	GetByID(ctx context.Context, id uint64) (bool, error)
	UpdateStatusByID(ctx context.Context, id uint64, status domainvideo.Status, errMsg string) error
	UpdateCoverByID(ctx context.Context, id uint64, coverURL string) (bool, error)
}

type Transcoder interface {
	ConvertToHLS(ctx context.Context, inputPath string, outputDir string) error
	GenerateCover(ctx context.Context, inputPath string, outputPath string) error
}

type ObjectStore interface {
	PutFile(ctx context.Context, objectKey string, filePath string, contentType string) error
}

type ObjectDownloader interface {
	DownloadToFile(ctx context.Context, objectKey string, filePath string) error
}

type DirUploader interface {
	UploadDir(ctx context.Context, localDir string, objectPrefix string) error
}

type FileStorage interface {
	MkdirAll(path string) error
	RemoveAll(path string) error
	Remove(path string) error
}

type Service struct {
	Queue          Queue
	StatusStore    StatusStore
	Repo           Repository
	Transcoder     Transcoder
	Store          ObjectStore
	Downloader     ObjectDownloader
	Uploader       DirUploader
	FS             FileStorage
	TempRawDir     string
	TempHlsDir     string
	CoverURLPrefix string
	Now            func() time.Time
	StatusTTL      time.Duration
	TaskTimeout    time.Duration
	Leases         LeaseStore
	WorkerID       string
	LeaseTTL       time.Duration
	RetryPolicy    RetryPolicy
	Execute        func(ctx context.Context, fn func(context.Context) error) error
	Counters       CounterStore
}

func (s Service) RunOnce(ctx context.Context) error {
	msg, err := s.Queue.Dequeue(ctx)
	if err != nil {
		return err
	}
	taskCtx := ctx
	var cancel context.CancelFunc
	if s.TaskTimeout > 0 {
		taskCtx, cancel = context.WithTimeout(ctx, s.TaskTimeout)
		defer cancel()
	}
	if s.Execute != nil {
		return s.Execute(taskCtx, func(runCtx context.Context) error {
			return s.runTask(runCtx, msg)
		})
	}
	return s.runTask(taskCtx, msg)
}

func (s Service) runTask(taskCtx context.Context, msg QueueMessage) error {
	task := msg.Task
	if s.Counters != nil {
		s.Counters.Inc("transcode_tasks_active")
		defer s.Counters.Dec("transcode_tasks_active")
	}
	if s.Leases != nil {
		leaseTTL := s.LeaseTTL
		if leaseTTL <= 0 {
			leaseTTL = time.Minute
		}
		if err := s.Leases.Acquire(taskCtx, Lease{
			TaskID:    task.TaskID,
			MessageID: msg.MessageID,
			WorkerID:  s.WorkerID,
			Stage:     "start",
			ExpiresAt: time.Now().Add(leaseTTL),
		}, leaseTTL); err != nil {
			return err
		}
		defer func() {
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(taskCtx), 5*time.Second)
			defer cancel()
			_ = s.Leases.Release(releaseCtx, task.TaskID)
		}()
	}

	if ok, err := s.Repo.GetByID(taskCtx, task.VideoID); err != nil {
		return err
	} else if !ok {
		_ = s.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusFailed, task.HLSURL, s.StatusTTL)
		zap.L().Debug("skip",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("reason", "video_not_found_or_deleted"),
		)
		_ = s.Queue.Ack(taskCtx, msg.MessageID)
		return nil
	}

	start := time.Now()
	localInput := filepath.Join(s.TempRawDir, task.TaskID+"_"+filepath.Base(task.RawKey))
	localOutputDir := filepath.Join(s.TempHlsDir, task.TaskID)
	zap.L().Info("transcode_task_start",
		zap.String("task_id", task.TaskID),
		zap.Uint64("video_id", task.VideoID),
		zap.String("raw_key", task.RawKey),
		zap.String("hls_prefix", task.HLSObjectPrefix),
		zap.Bool("cover_enabled", true),
	)

	_ = s.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusProcessing, task.HLSURL, s.StatusTTL)
	_ = s.Repo.UpdateStatusByID(taskCtx, task.VideoID, domainvideo.StatusProcessing, "")

	_ = s.FS.MkdirAll(s.TempRawDir)
	_ = s.FS.MkdirAll(s.TempHlsDir)
	_ = s.FS.RemoveAll(localOutputDir)

	if err := s.Downloader.DownloadToFile(taskCtx, task.RawKey, localInput); err != nil {
		_ = s.FS.RemoveAll(localOutputDir)
		_ = s.FS.Remove(localInput)
		return s.handleTaskFailure(taskCtx, msg, start, "download", err)
	}

	if err := s.Transcoder.ConvertToHLS(taskCtx, localInput, localOutputDir); err != nil {
		_ = s.FS.RemoveAll(localOutputDir)
		_ = s.FS.Remove(localInput)
		return s.handleTaskFailure(taskCtx, msg, start, "transcode", err)
	}

	if err := s.Uploader.UploadDir(taskCtx, localOutputDir, task.HLSObjectPrefix); err != nil {
		_ = s.FS.RemoveAll(localOutputDir)
		_ = s.FS.Remove(localInput)
		return s.handleTaskFailure(taskCtx, msg, start, "upload", err)
	}

	coverLocalPath := filepath.Join(localOutputDir, "cover.jpg")
	coverKey := CoverObjectKey(task.HLSObjectPrefix)
	coverURL := strings.TrimRight(defaultString(s.CoverURLPrefix, "/videos"), "/") + "/" + coverKey
	if err := s.Transcoder.GenerateCover(taskCtx, localInput, coverLocalPath); err != nil {
		zap.L().Debug("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "skip"),
			zap.String("err", err.Error()),
		)
	} else if err := s.Store.PutFile(taskCtx, coverKey, coverLocalPath, "image/jpeg"); err != nil {
		zap.L().Error("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "upload_failed"),
			zap.String("key", coverKey),
			zap.String("err", err.Error()),
		)
	} else if ok, err := s.Repo.UpdateCoverByID(taskCtx, task.VideoID, coverURL); err != nil {
		zap.L().Error("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "db_failed"),
			zap.String("url", coverURL),
			zap.String("err", err.Error()),
		)
	} else if !ok {
		zap.L().Debug("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "db_not_found"),
			zap.String("url", coverURL),
		)
	} else {
		zap.L().Debug("cover",
			zap.String("task_id", task.TaskID),
			zap.Uint64("video_id", task.VideoID),
			zap.String("status", "ok"),
			zap.String("url", coverURL),
		)
	}

	_ = s.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusDone, task.HLSURL, s.StatusTTL)
	_ = s.Repo.UpdateStatusByID(taskCtx, task.VideoID, domainvideo.StatusDone, "")
	_ = s.FS.RemoveAll(localOutputDir)
	_ = s.FS.Remove(localInput)
	_ = s.Queue.Ack(taskCtx, msg.MessageID)
	zap.L().Info("transcode_task_done",
		zap.String("task_id", task.TaskID),
		zap.Uint64("video_id", task.VideoID),
		zap.Int64("latency_ms", time.Since(start).Milliseconds()),
	)
	return nil
}

func (s Service) handleTaskFailure(taskCtx context.Context, msg QueueMessage, start time.Time, stage string, err error) error {
	task := msg.Task
	wrapped := fmt.Errorf("%s failed task_id=%s: %w", stage, task.TaskID, err)
	decision := DefaultRetryPolicy(wrapped, task.RetryCount)
	if s.RetryPolicy != nil {
		decision = s.RetryPolicy(wrapped, task.RetryCount)
	}
	if decision.Retry {
		retryMsg := msg
		retryMsg.Task.RetryCount++
		_ = s.Queue.Requeue(taskCtx, retryMsg, decision.Delay, decision.Reason)
		_ = s.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusUploaded, task.HLSURL, s.StatusTTL)
		_ = s.Repo.UpdateStatusByID(taskCtx, task.VideoID, domainvideo.StatusUploaded, decision.Reason)
	} else {
		_ = s.Queue.MoveToDeadLetter(taskCtx, msg, decision.Reason)
		_ = s.StatusStore.Set(taskCtx, task.TaskID, domainvideo.StatusFailed, task.HLSURL, s.StatusTTL)
		_ = s.Repo.UpdateStatusByID(taskCtx, task.VideoID, domainvideo.StatusFailed, err.Error())
	}
	zap.L().Error("transcode_task_failed",
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

func DefaultRetryPolicy(err error, retries int) RetryDecision {
	if err == nil {
		return RetryDecision{}
	}
	if retries >= MaxRetryAttempts {
		return RetryDecision{Reason: "retry_exhausted"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * time.Minute, Reason: "timeout"}
	}
	if IsTemporaryStorageError(err) {
		return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * 30 * time.Second, Reason: "temporary_storage_error"}
	}
	return RetryDecision{Reason: "terminal"}
}

func IsTemporaryStorageError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, token := range []string{"timeout", "connection reset", "tempor", "unavailable", "eof"} {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func CoverObjectKey(hlsObjectPrefix string) string {
	prefix := strings.TrimSpace(hlsObjectPrefix)
	prefix = strings.Trim(prefix, "/")
	prefix = strings.TrimPrefix(prefix, "hls/")
	return filepath.ToSlash(filepath.Join("cover", prefix, "cover.jpg"))
}
