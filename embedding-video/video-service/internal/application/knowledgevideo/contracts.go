package knowledgevideo

import (
	"context"
	"io"
	"time"
)

type DictionaryLookup interface {
	LookupKnowledgePoints(ctx context.Context, ids []uint64) ([]DictionaryKnowledgePoint, error)
}

type KnowledgeTreeReader interface {
	ListKnowledgeTree(ctx context.Context) ([]KnowledgeTreeNode, error)
}

type IDReservation interface {
	ReserveBatchID(ctx context.Context) (uint64, error)
	ReserveVideoIDs(ctx context.Context, count int) ([]uint64, error)
}

type BatchCreator interface {
	CreateBatch(ctx context.Context, batch Batch, videos []Video) error
}

type ReadyVideosResolver interface {
	ListReadyByKnowledgePoint(ctx context.Context, knowledgePointID uint64) ([]Video, error)
}

type PlaybackRecorder interface {
	RecordPlayback(ctx context.Context, record PlayRecord) error
}

type VideoStateStore interface {
	GetVideo(ctx context.Context, id uint64) (Video, bool, error)
	MarkTranscoding(ctx context.Context, id uint64) (bool, error)
	MarkReady(ctx context.Context, id uint64, masterObjectKey string, duration int) (bool, error)
	MarkFailed(ctx context.Context, id uint64, errorMessage string) (bool, error)
	SetEnqueueTime(ctx context.Context, id uint64, enqueueTime time.Time) (bool, error)
}

type BatchStatusRefresher interface {
	RefreshBatchStatus(ctx context.Context, batchID uint64) (Batch, bool, error)
}

type BatchReader interface {
	GetBatch(ctx context.Context, batchID uint64) (Batch, []Video, bool, error)
}

type StalePendingLister interface {
	ListStalePending(ctx context.Context, cutoff time.Time, limit int) ([]Video, error)
}

type KnowledgeVideoRepository interface {
	DictionaryLookup
	KnowledgeTreeReader
	IDReservation
	BatchCreator
	ReadyVideosResolver
	PlaybackRecorder
	VideoStateStore
	BatchStatusRefresher
	BatchReader
	StalePendingLister
}

type TranscodeQueue interface {
	Enqueue(ctx context.Context, task TranscodeTask) error
	Dequeue(ctx context.Context) (QueueMessage, error)
	Ack(ctx context.Context, messageID string) error
	Requeue(ctx context.Context, message QueueMessage, delay time.Duration, reason string) error
	MoveToDeadLetter(ctx context.Context, message QueueMessage, reason string) error
}

type ObjectUploader interface {
	Put(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	PutFile(ctx context.Context, objectKey string, filePath string, contentType string) error
}

type ObjectDeleter interface {
	Delete(ctx context.Context, objectKey string) error
}

type ObjectDownloader interface {
	DownloadToFile(ctx context.Context, objectKey string, filePath string) error
}

type DirectoryUploader interface {
	UploadDir(ctx context.Context, localDir string, objectPrefix string) error
}

type FFmpegConverter interface {
	ConvertToHLS(ctx context.Context, inputPath string, outputDir string) error
	ProbeDurationSeconds(ctx context.Context, inputPath string) (int, error)
}

type FilesystemCleanup interface {
	MkdirAll(path string) error
	RemoveAll(path string) error
	Remove(path string) error
}
