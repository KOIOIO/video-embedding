package worker

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domainvideo "video-service/internal/domain/video"
)

func TestDefaultRetryPolicyRetriesDeadlineExceeded(t *testing.T) {
	decision := DefaultRetryPolicy(context.DeadlineExceeded, 1)

	if !decision.Retry {
		t.Fatal("Retry = false, want true")
	}
	if decision.Delay != 2*time.Minute {
		t.Fatalf("Delay = %s, want 2m", decision.Delay)
	}
	if decision.Reason != "timeout" {
		t.Fatalf("Reason = %q, want timeout", decision.Reason)
	}
}

func TestDefaultRetryPolicyRetriesTemporaryStorageError(t *testing.T) {
	decision := DefaultRetryPolicy(errors.New("storage temporarily unavailable"), 0)

	if !decision.Retry {
		t.Fatal("Retry = false, want true")
	}
	if decision.Delay != 30*time.Second {
		t.Fatalf("Delay = %s, want 30s", decision.Delay)
	}
	if decision.Reason != "temporary_storage_error" {
		t.Fatalf("Reason = %q, want temporary_storage_error", decision.Reason)
	}
}

func TestDefaultRetryPolicyStopsAfterMaxAttempts(t *testing.T) {
	decision := DefaultRetryPolicy(context.DeadlineExceeded, MaxRetryAttempts)

	if decision.Retry {
		t.Fatal("Retry = true, want false")
	}
	if decision.Reason != "retry_exhausted" {
		t.Fatalf("Reason = %q, want retry_exhausted", decision.Reason)
	}
}

func TestDefaultRetryPolicyTreatsUnknownErrorsAsTerminal(t *testing.T) {
	decision := DefaultRetryPolicy(errors.New("invalid media"), 0)

	if decision.Retry {
		t.Fatal("Retry = true, want false")
	}
	if decision.Reason != "terminal" {
		t.Fatalf("Reason = %q, want terminal", decision.Reason)
	}
}

func TestIsTemporaryStorageErrorMatchesCommonStorageFailures(t *testing.T) {
	for _, message := range []string{"request timeout", "connection reset by peer", "temporary failure", "service unavailable", "unexpected EOF"} {
		t.Run(message, func(t *testing.T) {
			if !IsTemporaryStorageError(errors.New(message)) {
				t.Fatalf("expected %q to be temporary", message)
			}
		})
	}

	if IsTemporaryStorageError(errors.New("bad media")) {
		t.Fatal("expected bad media to be terminal")
	}
}

func TestCoverObjectKeyNormalizesHLSPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		want   string
	}{
		{name: "hls prefix", prefix: "hls/2026/05/21/video", want: "cover/2026/05/21/video/cover.jpg"},
		{name: "slashes", prefix: "/hls/2026/05/21/video/", want: "cover/2026/05/21/video/cover.jpg"},
		{name: "custom prefix", prefix: "stream/2026/05/21/video", want: "cover/stream/2026/05/21/video/cover.jpg"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CoverObjectKey(tc.prefix); got != tc.want {
				t.Fatalf("CoverObjectKey(%q) = %q, want %q", tc.prefix, got, tc.want)
			}
		})
	}
}

func TestRunOnceUsesExecuteHookAndCounters(t *testing.T) {
	queue := &fakeQueue{
		msg: QueueMessage{
			MessageID: "1-0",
			Task: Task{
				VideoID:         7,
				RawKey:          "raw/7.mp4",
				HLSObjectPrefix: "hls/7",
				TaskID:          "7",
				HLSURL:          "/videos/hls/7/master.m3u8",
			},
		},
	}
	counters := &fakeCounters{}
	executeCalled := false
	svc := newTestService(queue)
	svc.Counters = counters
	svc.Execute = func(ctx context.Context, fn func(context.Context) error) error {
		executeCalled = true
		return fn(ctx)
	}

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if !executeCalled {
		t.Fatal("expected Execute hook to be called")
	}
	if len(queue.acked) != 1 || queue.acked[0] != "1-0" {
		t.Fatalf("acked = %#v, want [1-0]", queue.acked)
	}
	if counters.active != 0 {
		t.Fatalf("active counter = %d, want 0 after defer", counters.active)
	}
	if counters.maxActive != 1 {
		t.Fatalf("max active = %d, want 1", counters.maxActive)
	}
}

func TestRunOnceCleansLocalInputOnDownloadFailure(t *testing.T) {
	queue := &fakeQueue{
		msg: QueueMessage{
			MessageID: "2-0",
			Task: Task{
				VideoID:         8,
				RawKey:          "raw/course/lesson.mp4",
				HLSObjectPrefix: "hls/8",
				TaskID:          "task-8",
				HLSURL:          "/videos/hls/8/master.m3u8",
			},
		},
	}
	fs := &recordingFS{}
	svc := newTestService(queue)
	svc.Downloader = failingDownloader{err: errors.New("download failed")}
	svc.FS = fs

	if err := svc.RunOnce(context.Background()); err == nil {
		t.Fatal("RunOnce returned nil, want download error")
	}

	wantInput := filepath.Join("tmp/raw", "task-8_lesson.mp4")
	if !containsString(fs.removed, wantInput) {
		t.Fatalf("removed files = %#v, want %q to be removed", fs.removed, wantInput)
	}
}

type fakeQueue struct {
	msg          QueueMessage
	acked        []string
	requeued     []QueueMessage
	deadLettered []QueueMessage
}

func (q *fakeQueue) Dequeue(context.Context) (QueueMessage, error) {
	return q.msg, nil
}

func (q *fakeQueue) Ack(_ context.Context, messageID string) error {
	q.acked = append(q.acked, messageID)
	return nil
}

func (q *fakeQueue) Requeue(_ context.Context, msg QueueMessage, _ time.Duration, _ string) error {
	q.requeued = append(q.requeued, msg)
	return nil
}

func (q *fakeQueue) MoveToDeadLetter(_ context.Context, msg QueueMessage, _ string) error {
	q.deadLettered = append(q.deadLettered, msg)
	return nil
}

type fakeCounters struct {
	active    int
	maxActive int
}

func (c *fakeCounters) Inc(name string) {
	if name != "transcode_tasks_active" {
		panic("unexpected counter: " + name)
	}
	c.active++
	if c.active > c.maxActive {
		c.maxActive = c.active
	}
}

func (c *fakeCounters) Dec(name string) {
	if name != "transcode_tasks_active" {
		panic("unexpected counter: " + name)
	}
	c.active--
}

type fakeRepo struct{}

func (fakeRepo) GetByID(context.Context, uint64) (bool, error) { return true, nil }
func (fakeRepo) UpdateStatusByID(context.Context, uint64, domainvideo.Status, string) error {
	return nil
}
func (fakeRepo) UpdateCoverByID(context.Context, uint64, string) (bool, error) { return true, nil }

type fakeStatusStore struct{}

func (fakeStatusStore) Set(context.Context, string, domainvideo.Status, string, time.Duration) error {
	return nil
}

type fakeTranscoder struct {
	durationSec int
	probeErr    error
}

func (f fakeTranscoder) ConvertToHLS(context.Context, string, string) error  { return nil }
func (f fakeTranscoder) GenerateCover(context.Context, string, string) error { return nil }
func (f fakeTranscoder) ProbeDurationSeconds(context.Context, string) (int, error) {
	return f.durationSec, f.probeErr
}

type fakeObjectStore struct{}

func (fakeObjectStore) PutFile(context.Context, string, string, string) error { return nil }

type fakeDownloader struct{}

func (fakeDownloader) DownloadToFile(context.Context, string, string) error { return nil }

type failingDownloader struct{ err error }

func (d failingDownloader) DownloadToFile(context.Context, string, string) error { return d.err }

type fakeUploader struct{}

func (fakeUploader) UploadDir(context.Context, string, string) error { return nil }

type fakeFS struct{}

func (fakeFS) MkdirAll(string) error  { return nil }
func (fakeFS) RemoveAll(string) error { return nil }
func (fakeFS) Remove(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("empty path")
	}
	return nil
}

type recordingFS struct {
	removed []string
}

func (*recordingFS) MkdirAll(string) error { return nil }
func (*recordingFS) RemoveAll(string) error {
	return nil
}
func (fs *recordingFS) Remove(path string) error {
	fs.removed = append(fs.removed, path)
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func newTestService(queue *fakeQueue) Service {
	return Service{
		Queue:       queue,
		StatusStore: fakeStatusStore{},
		Repo:        fakeRepo{},
		Transcoder:  fakeTranscoder{},
		Store:       fakeObjectStore{},
		Downloader:  fakeDownloader{},
		Uploader:    fakeUploader{},
		FS:          fakeFS{},
		TempRawDir:  "tmp/raw",
		TempHlsDir:  "tmp/hls",
		StatusTTL:   time.Hour,
	}
}

type recordingStatusStore struct {
	statuses []domainvideo.Status
}

func (s *recordingStatusStore) Set(_ context.Context, _ string, status domainvideo.Status, _ string, _ time.Duration) error {
	s.statuses = append(s.statuses, status)
	return nil
}

type recordingRepo struct {
	statuses []domainvideo.Status
	errMsgs  []string
}

func (r *recordingRepo) GetByID(context.Context, uint64) (bool, error) { return true, nil }
func (r *recordingRepo) UpdateStatusByID(_ context.Context, _ uint64, status domainvideo.Status, errMsg string) error {
	r.statuses = append(r.statuses, status)
	r.errMsgs = append(r.errMsgs, errMsg)
	return nil
}
func (r *recordingRepo) UpdateCoverByID(context.Context, uint64, string) (bool, error) { return true, nil }

func TestRunOnceMarksFailedWhenDurationExceedsLimit(t *testing.T) {
	queue := &fakeQueue{
		msg: QueueMessage{
			MessageID: "dur-0",
			Task: Task{
				VideoID:         99,
				RawKey:          "raw/long.mp4",
				HLSObjectPrefix: "hls/99",
				TaskID:          "task-99",
				HLSURL:          "/videos/hls/99/master.m3u8",
			},
		},
	}
	statusStore := &recordingStatusStore{}
	repo := &recordingRepo{}
	svc := newTestService(queue)
	svc.StatusStore = statusStore
	svc.Repo = repo
	svc.Transcoder = fakeTranscoder{durationSec: 200} // > 180

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	// Should be acked (not retried / dead-lettered)
	if len(queue.acked) != 1 || queue.acked[0] != "dur-0" {
		t.Fatalf("acked = %#v, want [dur-0]", queue.acked)
	}
	if len(queue.requeued) > 0 {
		t.Fatalf("requeued = %#v, want empty", queue.requeued)
	}
	// Last status should be Failed
	foundFailed := false
	for _, s := range repo.statuses {
		if s == domainvideo.StatusFailed {
			foundFailed = true
		}
	}
	if !foundFailed {
		t.Fatalf("repo statuses = %#v, want StatusFailed", repo.statuses)
	}
	if len(repo.errMsgs) == 0 || !strings.Contains(repo.errMsgs[len(repo.errMsgs)-1], "exceeds") {
		t.Fatalf("last errMsg = %q, want contains 'exceeds'", repo.errMsgs)
	}
}

func TestRunOnceContinuesWhenDurationWithinLimit(t *testing.T) {
	queue := &fakeQueue{
		msg: QueueMessage{
			MessageID: "dur-1",
			Task: Task{
				VideoID:         100,
				RawKey:          "raw/short.mp4",
				HLSObjectPrefix: "hls/100",
				TaskID:          "task-100",
				HLSURL:          "/videos/hls/100/master.m3u8",
			},
		},
	}
	svc := newTestService(queue)
	svc.Transcoder = fakeTranscoder{durationSec: 60} // <= 180

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(queue.acked) != 1 {
		t.Fatalf("acked = %#v, want 1 ack", queue.acked)
	}
}

func TestRunOnceContinuesWhenProbeFails(t *testing.T) {
	queue := &fakeQueue{
		msg: QueueMessage{
			MessageID: "dur-2",
			Task: Task{
				VideoID:         101,
				RawKey:          "raw/probe_err.mp4",
				HLSObjectPrefix: "hls/101",
				TaskID:          "task-101",
				HLSURL:          "/videos/hls/101/master.m3u8",
			},
		},
	}
	svc := newTestService(queue)
	svc.Transcoder = fakeTranscoder{probeErr: errors.New("ffprobe not found")}

	if err := svc.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(queue.acked) != 1 {
		t.Fatalf("acked = %#v, want 1 ack (probe failure should not block)", queue.acked)
	}
}
