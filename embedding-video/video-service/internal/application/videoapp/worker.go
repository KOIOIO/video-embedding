package videoapp

import (
	"context"
	"time"

	workerapp "nlp-video-analysis/internal/application/videoapp/worker"
	domainvideo "nlp-video-analysis/internal/domain/video"
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
	Queue          TranscodeTaskConsumer
	StatusStore    TranscodeStatusStore
	Repo           VideoRepository
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
	return workerapp.Service{
		Queue:          transcodeTaskQueueAdapter{queue: w.Queue},
		StatusStore:    transcodeWorkerStatusStoreAdapter{store: w.StatusStore},
		Repo:           transcodeWorkerRepoAdapter{repo: w.Repo},
		Transcoder:     w.Transcoder,
		Store:          transcodeWorkerObjectStoreAdapter{store: w.Store},
		Downloader:     w.Downloader,
		Uploader:       w.Uploader,
		FS:             transcodeWorkerFileStorageAdapter{fs: w.FS},
		TempRawDir:     w.TempRawDir,
		TempHlsDir:     w.TempHlsDir,
		CoverURLPrefix: w.CoverURLPrefix,
		Now:            w.Now,
		StatusTTL:      w.StatusTTL,
		TaskTimeout:    w.TaskTimeout,
		Leases:         transcodeLeaseStoreAdapter{store: w.Leases},
		WorkerID:       w.WorkerID,
		LeaseTTL:       w.LeaseTTL,
		RetryPolicy:    adaptRetryPolicy(w.RetryPolicy),
		Execute:        w.Execute,
		Counters:       runtimeCounters,
	}.RunOnce(ctx)
}

// coverObjectKey 根据 HLS 目录前缀推导封面对象存储路径。
func coverObjectKey(hlsObjectPrefix string) string {
	return workerapp.CoverObjectKey(hlsObjectPrefix)
}

type transcodeTaskQueueAdapter struct {
	queue TranscodeTaskConsumer
}

func (a transcodeTaskQueueAdapter) Dequeue(ctx context.Context) (workerapp.QueueMessage, error) {
	msg, err := a.queue.Dequeue(ctx)
	if err != nil {
		return workerapp.QueueMessage{}, err
	}
	return workerapp.QueueMessage{
		MessageID: msg.MessageID,
		Task: workerapp.Task{
			VideoID:         msg.Task.VideoID,
			RawKey:          msg.Task.RawKey,
			HLSObjectPrefix: msg.Task.HLSObjectPrefix,
			TaskID:          msg.Task.TaskID,
			HLSURL:          msg.Task.HLSURL,
			RetryCount:      msg.Task.RetryCount,
		},
	}, nil
}

func (a transcodeTaskQueueAdapter) Ack(ctx context.Context, messageID string) error {
	return a.queue.Ack(ctx, messageID)
}

func (a transcodeTaskQueueAdapter) Requeue(ctx context.Context, msg workerapp.QueueMessage, delay time.Duration, reason string) error {
	return a.queue.Requeue(ctx, TranscodeQueueMessage{
		MessageID: msg.MessageID,
		Task: TranscodeTask{
			VideoID:         msg.Task.VideoID,
			RawKey:          msg.Task.RawKey,
			HLSObjectPrefix: msg.Task.HLSObjectPrefix,
			TaskID:          msg.Task.TaskID,
			HLSURL:          msg.Task.HLSURL,
			RetryCount:      msg.Task.RetryCount,
		},
	}, delay, reason)
}

func (a transcodeTaskQueueAdapter) MoveToDeadLetter(ctx context.Context, msg workerapp.QueueMessage, reason string) error {
	return a.queue.MoveToDeadLetter(ctx, TranscodeQueueMessage{
		MessageID: msg.MessageID,
		Task: TranscodeTask{
			VideoID:         msg.Task.VideoID,
			RawKey:          msg.Task.RawKey,
			HLSObjectPrefix: msg.Task.HLSObjectPrefix,
			TaskID:          msg.Task.TaskID,
			HLSURL:          msg.Task.HLSURL,
			RetryCount:      msg.Task.RetryCount,
		},
	}, reason)
}

type transcodeWorkerStatusStoreAdapter struct {
	store TranscodeStatusStore
}

func (a transcodeWorkerStatusStoreAdapter) Set(ctx context.Context, taskID string, status domainvideo.Status, hlsURL string, ttl time.Duration) error {
	return a.store.Set(ctx, taskID, status, hlsURL, ttl)
}

type transcodeWorkerRepoAdapter struct {
	repo VideoRepository
}

func (a transcodeWorkerRepoAdapter) GetByID(ctx context.Context, id uint64) (bool, error) {
	_, ok, err := a.repo.GetByID(ctx, id)
	return ok, err
}

func (a transcodeWorkerRepoAdapter) UpdateStatusByID(ctx context.Context, id uint64, status domainvideo.Status, errMsg string) error {
	return a.repo.UpdateStatusByID(ctx, id, status, errMsg)
}

func (a transcodeWorkerRepoAdapter) UpdateCoverByID(ctx context.Context, id uint64, coverURL string) (bool, error) {
	return a.repo.UpdateCoverByID(ctx, id, coverURL)
}

type transcodeWorkerObjectStoreAdapter struct {
	store ObjectStore
}

func (a transcodeWorkerObjectStoreAdapter) PutFile(ctx context.Context, objectKey string, filePath string, contentType string) error {
	return a.store.PutFile(ctx, objectKey, filePath, contentType)
}

type transcodeWorkerFileStorageAdapter struct {
	fs FileStorage
}

func (a transcodeWorkerFileStorageAdapter) MkdirAll(path string) error {
	return a.fs.MkdirAll(path)
}

func (a transcodeWorkerFileStorageAdapter) RemoveAll(path string) error {
	return a.fs.RemoveAll(path)
}

func (a transcodeWorkerFileStorageAdapter) Remove(path string) error {
	return a.fs.Remove(path)
}

type transcodeLeaseStoreAdapter struct {
	store LeaseStore
}

func (a transcodeLeaseStoreAdapter) Acquire(ctx context.Context, lease workerapp.Lease, ttl time.Duration) error {
	if a.store == nil {
		return nil
	}
	return a.store.Acquire(ctx, Lease{
		TaskID:    lease.TaskID,
		MessageID: lease.MessageID,
		WorkerID:  lease.WorkerID,
		Stage:     lease.Stage,
		ExpiresAt: lease.ExpiresAt,
	}, ttl)
}

func (a transcodeLeaseStoreAdapter) Renew(ctx context.Context, lease workerapp.Lease, ttl time.Duration) error {
	if a.store == nil {
		return nil
	}
	return a.store.Renew(ctx, Lease{
		TaskID:    lease.TaskID,
		MessageID: lease.MessageID,
		WorkerID:  lease.WorkerID,
		Stage:     lease.Stage,
		ExpiresAt: lease.ExpiresAt,
	}, ttl)
}

func (a transcodeLeaseStoreAdapter) Release(ctx context.Context, taskID string) error {
	if a.store == nil {
		return nil
	}
	return a.store.Release(ctx, taskID)
}

func adaptRetryPolicy(fn RetryPolicy) workerapp.RetryPolicy {
	if fn == nil {
		return nil
	}
	return func(err error, retries int) workerapp.RetryDecision {
		decision := fn(err, retries)
		return workerapp.RetryDecision{
			Retry:  decision.Retry,
			Delay:  decision.Delay,
			Reason: decision.Reason,
		}
	}
}
