package videoapp

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

func TestWorkerRunOnceRequeuesRetryableFailure(t *testing.T) {
	queue := &fakeTaskQueue{
		msg: TranscodeQueueMessage{
			MessageID: "1-0",
			Task: TranscodeTask{
				VideoID:         7,
				RawKey:          "raw/7.mp4",
				HLSObjectPrefix: "hls/7",
				TaskID:          "7",
				HLSURL:          "/videos/hls/7/master.m3u8",
			},
		},
	}
	worker := newTestWorker(queue)
	worker.Downloader = fakeDownloader{err: context.DeadlineExceeded}

	err := worker.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if len(queue.requeued) != 1 {
		t.Fatalf("expected 1 requeue, got %d", len(queue.requeued))
	}
	if len(queue.acked) != 0 {
		t.Fatalf("expected 0 ack, got %d", len(queue.acked))
	}
	if len(queue.deadLettered) != 0 {
		t.Fatalf("expected 0 dead-letter entries, got %d", len(queue.deadLettered))
	}
	leaseStore := worker.Leases.(*fakeLeaseStore)
	if leaseStore.released != "7" {
		t.Fatalf("expected lease release for task 7, got %q", leaseStore.released)
	}
	statusStore := worker.StatusStore.(*fakeStatusStore)
	if statusStore.lastStatus != domainvideo.StatusUploaded {
		t.Fatalf("expected uploaded status after retry scheduling, got %v", statusStore.lastStatus)
	}
}

func TestWorkerRunOnceDeadLettersTerminalFailure(t *testing.T) {
	queue := &fakeTaskQueue{
		msg: TranscodeQueueMessage{
			MessageID: "2-0",
			Task: TranscodeTask{
				VideoID:         8,
				RawKey:          "raw/8.mp4",
				HLSObjectPrefix: "hls/8",
				TaskID:          "8",
				HLSURL:          "/videos/hls/8/master.m3u8",
			},
		},
	}
	worker := newTestWorker(queue)
	worker.Downloader = fakeDownloader{err: errors.New("bad media")}

	err := worker.RunOnce(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if len(queue.deadLettered) != 1 {
		t.Fatalf("expected 1 dead-letter entry, got %d", len(queue.deadLettered))
	}
	if len(queue.requeued) != 0 {
		t.Fatalf("expected 0 requeue entries, got %d", len(queue.requeued))
	}
	statusStore := worker.StatusStore.(*fakeStatusStore)
	if statusStore.lastStatus != domainvideo.StatusFailed {
		t.Fatalf("expected failed status, got %v", statusStore.lastStatus)
	}
}

func TestWorkerRunOnceAcksSuccessfulTask(t *testing.T) {
	queue := &fakeTaskQueue{
		msg: TranscodeQueueMessage{
			MessageID: "3-0",
			Task: TranscodeTask{
				VideoID:         9,
				RawKey:          "raw/9.mp4",
				HLSObjectPrefix: "hls/9",
				TaskID:          "9",
				HLSURL:          "/videos/hls/9/master.m3u8",
			},
		},
	}
	worker := newTestWorker(queue)

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(queue.acked) != 1 || queue.acked[0] != "3-0" {
		t.Fatalf("expected ack for message 3-0, got %#v", queue.acked)
	}
	statusStore := worker.StatusStore.(*fakeStatusStore)
	if statusStore.lastStatus != domainvideo.StatusDone {
		t.Fatalf("expected done status, got %v", statusStore.lastStatus)
	}
}

func TestWorkerRunOnceSkipsMissingVideo(t *testing.T) {
	queue := &fakeTaskQueue{
		msg: TranscodeQueueMessage{
			MessageID: "4-0",
			Task: TranscodeTask{
				VideoID:         10,
				RawKey:          "raw/10.mp4",
				HLSObjectPrefix: "hls/10",
				TaskID:          "10",
				HLSURL:          "/videos/hls/10/master.m3u8",
			},
		},
	}
	worker := newTestWorker(queue)
	worker.Repo = &workerTestRepo{exists: false}

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(queue.acked) != 1 || queue.acked[0] != "4-0" {
		t.Fatalf("expected ack for message 4-0, got %#v", queue.acked)
	}
	statusStore := worker.StatusStore.(*fakeStatusStore)
	if statusStore.lastStatus != domainvideo.StatusFailed {
		t.Fatalf("expected failed status for skipped task, got %v", statusStore.lastStatus)
	}
	leaseStore := worker.Leases.(*fakeLeaseStore)
	if leaseStore.released != "10" {
		t.Fatalf("expected lease release for task 10, got %q", leaseStore.released)
	}
}

type fakeTaskQueue struct {
	msg          TranscodeQueueMessage
	requeued     []TranscodeQueueMessage
	deadLettered []TranscodeQueueMessage
	acked        []string
}

func (f *fakeTaskQueue) Dequeue(context.Context) (TranscodeQueueMessage, error) {
	return f.msg, nil
}

func (f *fakeTaskQueue) Ack(_ context.Context, messageID string) error {
	f.acked = append(f.acked, messageID)
	return nil
}

func (f *fakeTaskQueue) Requeue(_ context.Context, msg TranscodeQueueMessage, _ time.Duration, _ string) error {
	f.requeued = append(f.requeued, msg)
	return nil
}

func (f *fakeTaskQueue) MoveToDeadLetter(_ context.Context, msg TranscodeQueueMessage, _ string) error {
	f.deadLettered = append(f.deadLettered, msg)
	return nil
}

type fakeLeaseStore struct {
	acquired Lease
	released string
}

func (f *fakeLeaseStore) Acquire(_ context.Context, lease Lease, _ time.Duration) error {
	f.acquired = lease
	return nil
}

func (f *fakeLeaseStore) Renew(context.Context, Lease, time.Duration) error {
	return nil
}

func (f *fakeLeaseStore) Release(_ context.Context, taskID string) error {
	f.released = taskID
	return nil
}

type fakeStatusStore struct {
	lastTaskID string
	lastStatus domainvideo.Status
}

func (f *fakeStatusStore) Set(_ context.Context, taskID string, status domainvideo.Status, _ string, _ time.Duration) error {
	f.lastTaskID = taskID
	f.lastStatus = status
	return nil
}

func (f *fakeStatusStore) Get(context.Context, string) (TranscodeStatus, bool, error) {
	return TranscodeStatus{}, false, nil
}

type fakeDownloader struct{ err error }

func (f fakeDownloader) DownloadToFile(context.Context, string, string) error { return f.err }

type fakeTranscoder struct{}

func (fakeTranscoder) ConvertToHLS(context.Context, string, string) error  { return nil }
func (fakeTranscoder) GenerateCover(context.Context, string, string) error { return nil }

type fakeUploader struct{}

func (fakeUploader) UploadDir(context.Context, string, string) error { return nil }

type fakeFileStorage struct{}

func (fakeFileStorage) MkdirAll(string) error                 { return nil }
func (fakeFileStorage) Create(string) (io.WriteCloser, error) { return nopWriteCloser{}, nil }
func (fakeFileStorage) RemoveAll(string) error                { return nil }
func (fakeFileStorage) Remove(string) error                   { return nil }

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                { return nil }

func newTestWorker(queue *fakeTaskQueue) *Worker {
	statusStore := &fakeStatusStore{}
	leaseStore := &fakeLeaseStore{}
	return &Worker{
		Queue:       queue,
		StatusStore: statusStore,
		Repo:        &workerTestRepo{exists: true},
		Transcoder:  fakeTranscoder{},
		Store:       fakeObjectStore{},
		Downloader:  fakeDownloader{},
		Uploader:    fakeUploader{},
		FS:          fakeFileStorage{},
		TempRawDir:  "tmp/raw",
		TempHlsDir:  "tmp/hls",
		Now:         time.Now,
		StatusTTL:   24 * time.Hour,
		TaskTimeout: time.Minute,
		Leases:      leaseStore,
		WorkerID:    "worker-1",
		LeaseTTL:    time.Minute,
		RetryPolicy: func(err error, retries int) RetryDecision {
			if errors.Is(err, context.DeadlineExceeded) {
				return RetryDecision{Retry: true, Delay: time.Second, Reason: "timeout"}
			}
			return RetryDecision{Retry: false, Reason: "terminal"}
		},
		Execute: func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	}
}

type workerTestRepo struct{ exists bool }

func (*workerTestRepo) Create(context.Context, *domainvideo.Video) error { panic("unexpected call") }
func (*workerTestRepo) List(context.Context, ListFilter) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (*workerTestRepo) ListRecommendPool(context.Context) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (r *workerTestRepo) GetByID(context.Context, uint64) (domainvideo.Video, bool, error) {
	if !r.exists {
		return domainvideo.Video{}, false, nil
	}
	return domainvideo.Video{ID: 1}, true, nil
}
func (*workerTestRepo) DeleteByID(context.Context, uint64) (bool, error) { panic("unexpected call") }
func (*workerTestRepo) UpdateMetadata(context.Context, uint64, string, string) (bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) UpdatePublished(context.Context, uint64, bool) (bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) UpdateRecommend(context.Context, uint64, bool, uint64, int16, float64) (bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) IncrementViewCount(context.Context, uint64) (int, bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) GetViewCount(context.Context, uint64) (int, bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) SubmitVideoReaction(context.Context, uint64, uint64, VideoReactionType) (bool, bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) ApplyVideoReactionState(context.Context, uint64, uint64, VideoReactionType, bool) (bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) GetVideoUserReaction(context.Context, uint64, uint64) (VideoReactionType, bool, bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) GetVideoReactionCounts(context.Context, uint64) (VideoReactionCounts, bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) FindSimilar(context.Context, uint64, int) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (*workerTestRepo) UpdateCoverByID(context.Context, uint64, string) (bool, error) {
	return true, nil
}
func (*workerTestRepo) UpdateStatusByID(context.Context, uint64, domainvideo.Status, string) error {
	return nil
}
func (*workerTestRepo) GetSegmentEmbeddingDim(context.Context) (int, error) {
	panic("unexpected call")
}
func (*workerTestRepo) GetQuestionEmbeddingTextByID(context.Context, uint64) (string, error) {
	panic("unexpected call")
}
func (*workerTestRepo) ListQuestions(context.Context, int, int) (QuestionPage, error) {
	panic("unexpected call")
}
func (*workerTestRepo) GetQuestionByID(context.Context, uint64) (QuestionItem, bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) FindRecommendedSegments(context.Context, pgvector.Vector, int) ([]RecommendCandidate, error) {
	panic("unexpected call")
}
func (*workerTestRepo) SaveUserVideoRecommendation(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
	panic("unexpected call")
}
func (*workerTestRepo) ListRecommendations(context.Context, uint64, uint64, int) ([]RecommendationRecord, error) {
	panic("unexpected call")
}
func (*workerTestRepo) GetVideoIDBySegmentID(context.Context, uint64) (uint64, error) {
	panic("unexpected call")
}
func (*workerTestRepo) HasWatchedVideoForQuestion(context.Context, uint64, uint64, uint64) (bool, error) {
	panic("unexpected call")
}
func (*workerTestRepo) SaveWatchRecord(context.Context, uint64, uint64, uint64, uint64, bool, int, time.Time) (bool, error) {
	panic("unexpected call")
}

type fakeObjectStore struct{}

func (fakeObjectStore) PutFile(context.Context, string, string, string) error { return nil }
func (fakeObjectStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}
func (fakeObjectStore) Delete(context.Context, string) error { return nil }

func TestDecideRetry(t *testing.T) {
	decision := DefaultRetryPolicy(context.DeadlineExceeded, 0)
	if !decision.Retry {
		t.Fatal("expected timeout to be retryable")
	}
	if decision.Delay <= 0 {
		t.Fatal("expected positive retry delay")
	}

	decision = DefaultRetryPolicy(errors.New("invalid media data"), 0)
	if decision.Retry {
		t.Fatal("expected invalid media to be terminal")
	}

	decision = DefaultRetryPolicy(context.DeadlineExceeded, maxRetryAttempts)
	if decision.Retry {
		t.Fatal("expected exhausted retry budget to stop retrying")
	}
}

func TestIsTemporaryStorageError(t *testing.T) {
	if !isTemporaryStorageError(errors.New("s3 timeout while downloading")) {
		t.Fatal("expected timeout error to be temporary")
	}
	if isTemporaryStorageError(errors.New("permission denied")) {
		t.Fatal("expected permission denied to be terminal")
	}
	if !isTemporaryStorageError(errors.New(strings.ToUpper("connection reset by peer"))) {
		t.Fatal("expected connection reset to be temporary")
	}
}
