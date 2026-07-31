package knowledgevideo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkerReadyRedeliveryOnlyAcks(t *testing.T) {
	deps := newWorkerTestDependencies(t, Video{ID: 8, BatchID: 3, Status: VideoReady})
	if err := deps.worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if deps.queue.ackCount != 1 || deps.transcoder.convertCount != 0 || deps.repository.refreshCount != 1 {
		t.Fatalf("acks=%d transcodes=%d refresh=%d", deps.queue.ackCount, deps.transcoder.convertCount, deps.repository.refreshCount)
	}
}

func TestWorkerMissingVideoOnlyAcks(t *testing.T) {
	deps := newWorkerTestDependencies(t, Video{})
	deps.repository.found = false
	if err := deps.worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if deps.queue.ackCount != 1 || deps.downloader.calls != 0 {
		t.Fatalf("acks=%d downloads=%d", deps.queue.ackCount, deps.downloader.calls)
	}
}

func TestWorkerTranscodesPendingVideoToReadyAndCleansLocalFiles(t *testing.T) {
	deps := newWorkerTestDependencies(t, Video{ID: 8, BatchID: 3, Status: VideoPending, SourceFileName: "lesson.mp4"})
	if err := deps.worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if deps.repository.markTranscoding != 1 || deps.repository.readyKey != "hls/8/master.m3u8" || deps.repository.readyDuration != 42 {
		t.Fatalf("repository = %+v", deps.repository)
	}
	if deps.uploader.prefix != "hls/8" || deps.queue.ackCount != 1 || deps.repository.refreshCount != 1 {
		t.Fatalf("prefix=%q ack=%d refresh=%d", deps.uploader.prefix, deps.queue.ackCount, deps.repository.refreshCount)
	}
	entries, err := os.ReadDir(deps.tempRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temp entries=%v err=%v", entries, err)
	}
}

func TestWorkerRetryableFailureRequeuesWithIncrementedRetry(t *testing.T) {
	deps := newWorkerTestDependencies(t, Video{ID: 8, BatchID: 3, Status: VideoTranscoding, SourceFileName: "lesson.mp4"})
	deps.downloader.err = errors.New("storage unavailable")
	if err := deps.worker.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() error = nil")
	}
	if deps.queue.requeued.Task.RetryCount != 1 || deps.queue.dlqCount != 0 || deps.repository.failedMessage != "" {
		t.Fatalf("queue=%+v repository=%+v", deps.queue, deps.repository)
	}
}

func TestWorkerExhaustedFailureMarksFailedRefreshesAndDeadLetters(t *testing.T) {
	deps := newWorkerTestDependencies(t, Video{ID: 8, BatchID: 3, Status: VideoTranscoding, SourceFileName: "lesson.mp4"})
	deps.queue.message.Task.RetryCount = 4
	deps.transcoder.err = errors.New("ffmpeg failed")
	if err := deps.worker.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce() error = nil")
	}
	if deps.queue.dlqCount != 1 || deps.repository.failedMessage != "ffmpeg failed" || deps.repository.refreshCount != 1 {
		t.Fatalf("queue=%+v repository=%+v", deps.queue, deps.repository)
	}
}

type workerTestDependencies struct {
	worker     *Worker
	repository *workerTestRepository
	queue      *workerTestQueue
	downloader *workerTestDownloader
	transcoder *workerTestTranscoder
	uploader   *workerTestUploader
	tempRoot   string
}

func newWorkerTestDependencies(t *testing.T, video Video) *workerTestDependencies {
	t.Helper()
	tempRoot := t.TempDir()
	repository := &workerTestRepository{video: video, found: true}
	queue := &workerTestQueue{message: QueueMessage{MessageID: "1-0", Task: TranscodeTask{KnowledgeVideoID: 8, SourceObjectKey: "raw/8/source.mp4", HLSObjectPrefix: "hls/8", TaskID: "knowledge-video-8"}}}
	downloader := &workerTestDownloader{}
	transcoder := &workerTestTranscoder{}
	uploader := &workerTestUploader{}
	worker := &Worker{Repository: repository, Queue: queue, Downloader: downloader, Transcoder: transcoder, Uploader: uploader, TempRoot: tempRoot, MasterPlaylist: "master.m3u8"}
	return &workerTestDependencies{worker: worker, repository: repository, queue: queue, downloader: downloader, transcoder: transcoder, uploader: uploader, tempRoot: tempRoot}
}

type workerTestRepository struct {
	video           Video
	found           bool
	markTranscoding int
	readyKey        string
	readyDuration   int
	failedMessage   string
	refreshCount    int
}

func (r *workerTestRepository) GetVideo(context.Context, uint64) (Video, bool, error) {
	return r.video, r.found, nil
}

func (r *workerTestRepository) MarkTranscoding(context.Context, uint64) (bool, error) {
	r.markTranscoding++
	r.video.Status = VideoTranscoding
	return true, nil
}

func (r *workerTestRepository) MarkReady(_ context.Context, _ uint64, key string, duration int) (bool, error) {
	r.readyKey, r.readyDuration = key, duration
	return true, nil
}

func (r *workerTestRepository) MarkFailed(_ context.Context, _ uint64, message string) (bool, error) {
	r.failedMessage = message
	return true, nil
}

func (r *workerTestRepository) RefreshBatchStatus(context.Context, uint64) (Batch, bool, error) {
	r.refreshCount++
	return Batch{}, true, nil
}

type workerTestQueue struct {
	message  QueueMessage
	ackCount int
	requeued QueueMessage
	dlqCount int
}

func (q *workerTestQueue) Dequeue(context.Context) (QueueMessage, error) { return q.message, nil }
func (q *workerTestQueue) Ack(context.Context, string) error             { q.ackCount++; return nil }
func (q *workerTestQueue) Requeue(_ context.Context, message QueueMessage, _ time.Duration, _ string) error {
	q.requeued = message
	return nil
}
func (q *workerTestQueue) MoveToDeadLetter(context.Context, QueueMessage, string) error {
	q.dlqCount++
	return nil
}

type workerTestDownloader struct {
	calls int
	err   error
}

func (d *workerTestDownloader) DownloadToFile(_ context.Context, _ string, filePath string) error {
	d.calls++
	if d.err != nil {
		return d.err
	}
	return os.WriteFile(filePath, []byte("video"), 0o600)
}

type workerTestTranscoder struct {
	convertCount int
	err          error
}

func (t *workerTestTranscoder) ConvertToHLS(_ context.Context, _ string, outputDir string) error {
	t.convertCount++
	if t.err != nil {
		return t.err
	}
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "master.m3u8"), []byte("#EXTM3U"), 0o600)
}

func (*workerTestTranscoder) ProbeDurationSeconds(context.Context, string) (int, error) {
	return 42, nil
}

type workerTestUploader struct{ prefix string }

func (u *workerTestUploader) UploadDir(_ context.Context, _ string, prefix string) error {
	u.prefix = prefix
	return nil
}
