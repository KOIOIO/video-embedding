package videoapp

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

func TestBuildUploadPlanBuildsStablePaths(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.FixedZone("CST", 8*3600))
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, Paths{
		RawDir:       "/tmp/raw",
		HLSDir:       "/tmp/hls",
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})
	svc.Now = func() time.Time { return fixedNow }

	plan, err := svc.BuildUploadPlan("lesson.mp4")
	if err != nil {
		t.Fatalf("BuildUploadPlan returned error: %v", err)
	}

	if plan.StoredFileName != "1779337800123456789.mp4" {
		t.Fatalf("unexpected stored file name: %q", plan.StoredFileName)
	}
	if plan.DatePath != "2026/05/21" {
		t.Fatalf("unexpected date path: %q", plan.DatePath)
	}
	if plan.RawAbsPath != filepath.Join("/tmp/raw", "2026/05/21", "1779337800123456789.mp4") {
		t.Fatalf("unexpected raw abs path: %q", plan.RawAbsPath)
	}
	if plan.RawObjectKey != "raw/2026/05/21/1779337800123456789.mp4" {
		t.Fatalf("unexpected raw object key: %q", plan.RawObjectKey)
	}
	if plan.RawURL != "/videos/raw/2026/05/21/1779337800123456789.mp4" {
		t.Fatalf("unexpected raw url: %q", plan.RawURL)
	}
	if plan.HLSAbsDir != filepath.Join("/tmp/hls", "2026/05/21", "1779337800123456789") {
		t.Fatalf("unexpected hls abs dir: %q", plan.HLSAbsDir)
	}
	if plan.HLSObjectPrefix != "hls/2026/05/21/1779337800123456789" {
		t.Fatalf("unexpected hls object prefix: %q", plan.HLSObjectPrefix)
	}
	if plan.HLSURL != "/videos/hls/2026/05/21/1779337800123456789/master.m3u8" {
		t.Fatalf("unexpected hls url: %q", plan.HLSURL)
	}
	if plan.RawUploaded {
		t.Fatal("expected RawUploaded to default to false")
	}
}

func TestBuildUploadPlanUsesConfiguredObjectPrefixesAndMasterName(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.UTC)
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, Paths{
		RawDir:          "/tmp/raw",
		HLSDir:          "/tmp/hls",
		RawURLPrefix:    "/media/source",
		HLSURLPrefix:    "/media/stream",
		RawObjectPrefix: "source",
		HLSObjectPrefix: "stream",
		HLSMasterName:   "playlist.m3u8",
	})
	svc.Now = func() time.Time { return fixedNow }

	plan, err := svc.BuildUploadPlan("lesson.mp4")
	if err != nil {
		t.Fatalf("BuildUploadPlan returned error: %v", err)
	}

	if plan.RawObjectKey != "source/2026/05/21/1779366600123456789.mp4" {
		t.Fatalf("unexpected raw object key: %q", plan.RawObjectKey)
	}
	if plan.RawURL != "/media/source/2026/05/21/1779366600123456789.mp4" {
		t.Fatalf("unexpected raw url: %q", plan.RawURL)
	}
	if plan.HLSObjectPrefix != "stream/2026/05/21/1779366600123456789" {
		t.Fatalf("unexpected hls object prefix: %q", plan.HLSObjectPrefix)
	}
	if plan.HLSURL != "/media/stream/2026/05/21/1779366600123456789/playlist.m3u8" {
		t.Fatalf("unexpected hls url: %q", plan.HLSURL)
	}
}

func TestBuildUploadPlanRejectsEmptyFileName(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, Paths{})
	svc.Now = time.Now

	_, err := svc.BuildUploadPlan("   ")
	if err == nil {
		t.Fatal("expected error for empty file name")
	}
	if !strings.Contains(err.Error(), "originalFileName is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenUploadWriterCreatesParentDirAndFile(t *testing.T) {
	fs := &uploadTestFS{}
	svc := NewService(nil, nil, nil, nil, nil, fs, nil, Paths{})
	plan := UploadPlan{RawAbsPath: filepath.Join("/tmp/raw", "2026/05/21", "lesson.mp4")}

	writer, err := svc.OpenUploadWriter(plan)
	if err != nil {
		t.Fatalf("OpenUploadWriter returned error: %v", err)
	}
	_ = writer.Close()

	if fs.mkdirAllPath != filepath.Join("/tmp/raw", "2026/05/21") {
		t.Fatalf("unexpected MkdirAll path: %q", fs.mkdirAllPath)
	}
	if fs.createPath != plan.RawAbsPath {
		t.Fatalf("unexpected Create path: %q", fs.createPath)
	}
}

func TestOpenUploadWriterReturnsMkdirAllError(t *testing.T) {
	fs := &uploadTestFS{mkdirAllErr: errors.New("mkdir failed")}
	svc := NewService(nil, nil, nil, nil, nil, fs, nil, Paths{})

	_, err := svc.OpenUploadWriter(UploadPlan{RawAbsPath: filepath.Join("/tmp/raw", "lesson.mp4")})
	if err == nil {
		t.Fatal("expected MkdirAll error")
	}
	if err.Error() != "mkdir failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.createPath != "" {
		t.Fatal("expected Create not to be called when MkdirAll fails")
	}
}

func TestOpenUploadWriterReturnsCreateError(t *testing.T) {
	fs := &uploadTestFS{createErr: errors.New("create failed")}
	svc := NewService(nil, nil, nil, nil, nil, fs, nil, Paths{})
	plan := UploadPlan{RawAbsPath: filepath.Join("/tmp/raw", "lesson.mp4")}

	_, err := svc.OpenUploadWriter(plan)
	if err == nil {
		t.Fatal("expected Create error")
	}
	if err.Error() != "create failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFinalizeUploadEnqueuesTranscodeAndIgnoresVectorFailure(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 0, time.UTC)
	repo := &uploadTestRepo{createdID: 42}
	statusStore := &uploadTestStatusStore{}
	objectStore := &uploadTestObjectStore{}
	transcodeQueue := &uploadTestQueue{}
	vectorQueue := &uploadTestVectorQueue{err: errors.New("vector queue unavailable")}
	fs := &uploadTestFS{}
	svc := NewService(repo, transcodeQueue, vectorQueue, statusStore, objectStore, fs, nil, Paths{})
	svc.Now = func() time.Time { return fixedNow }
	svc.StatusTTL = 2 * time.Hour
	svc.DeleteLocal = true

	plan := UploadPlan{
		OriginalFileName: "lesson.mp4",
		StoredFileName:   "stored.mp4",
		RawAbsPath:       "/tmp/raw/stored.mp4",
		RawObjectKey:     "raw/2026/05/21/stored.mp4",
		RawURL:           "/videos/raw/2026/05/21/stored.mp4",
		HLSObjectPrefix:  "hls/2026/05/21/stored",
		HLSURL:           "/videos/hls/2026/05/21/stored/master.m3u8",
	}

	result, err := svc.FinalizeUpload(context.Background(), plan, UploadMeta{Title: "  高中物理  ", Description: "desc"})
	if err != nil {
		t.Fatalf("FinalizeUpload returned error: %v", err)
	}

	if objectStore.putFileObjectKey != plan.RawObjectKey {
		t.Fatalf("unexpected uploaded object key: %q", objectStore.putFileObjectKey)
	}
	if objectStore.putFilePath != plan.RawAbsPath {
		t.Fatalf("unexpected uploaded file path: %q", objectStore.putFilePath)
	}
	if repo.createdVideo == nil {
		t.Fatal("expected video to be created")
	}
	if repo.createdVideo.Title != "高中物理" {
		t.Fatalf("unexpected created title: %q", repo.createdVideo.Title)
	}
	if repo.createdVideo.VideoURL != plan.RawURL {
		t.Fatalf("unexpected created raw url: %q", repo.createdVideo.VideoURL)
	}
	if statusStore.lastTaskID != "42" || statusStore.lastStatus != domainvideo.StatusUploaded {
		t.Fatalf("unexpected status store update: taskID=%q status=%v", statusStore.lastTaskID, statusStore.lastStatus)
	}
	if statusStore.lastHLSURL != plan.HLSURL {
		t.Fatalf("unexpected status hls url: %q", statusStore.lastHLSURL)
	}
	if statusStore.lastTTL != 2*time.Hour {
		t.Fatalf("unexpected status ttl: %s", statusStore.lastTTL)
	}
	if transcodeQueue.lastTask.VideoID != 42 || transcodeQueue.lastTask.TaskID != "42" {
		t.Fatalf("unexpected transcode task: %+v", transcodeQueue.lastTask)
	}
	if vectorQueue.lastTask.VideoID != 42 || vectorQueue.lastTask.TaskID != "42" {
		t.Fatalf("unexpected vector task: %+v", vectorQueue.lastTask)
	}
	if fs.removePath != plan.RawAbsPath {
		t.Fatalf("expected local file removal, got %q", fs.removePath)
	}
	if result.VideoID != 42 || result.TaskID != "42" || result.RawURL != plan.RawURL || result.HLSURL != plan.HLSURL || result.Name != plan.StoredFileName {
		t.Fatalf("unexpected upload result: %+v", result)
	}
}

func TestFinalizeUploadSkipsRawUploadWhenAlreadyUploaded(t *testing.T) {
	repo := &uploadTestRepo{createdID: 7}
	statusStore := &uploadTestStatusStore{}
	transcodeQueue := &uploadTestQueue{}
	objectStore := &uploadTestObjectStore{}
	svc := NewService(repo, transcodeQueue, nil, statusStore, objectStore, &uploadTestFS{}, nil, Paths{})
	svc.Now = func() time.Time { return time.Unix(1, 0) }
	svc.DeleteLocal = false

	plan := UploadPlan{
		OriginalFileName: "lesson.mp4",
		StoredFileName:   "stored.mp4",
		RawAbsPath:       "/tmp/raw/stored.mp4",
		RawObjectKey:     "raw/2026/05/21/stored.mp4",
		RawURL:           "/videos/raw/2026/05/21/stored.mp4",
		HLSObjectPrefix:  "hls/2026/05/21/stored",
		HLSURL:           "/videos/hls/2026/05/21/stored/master.m3u8",
		RawUploaded:      true,
	}

	if _, err := svc.FinalizeUpload(context.Background(), plan, UploadMeta{}); err != nil {
		t.Fatalf("FinalizeUpload returned error: %v", err)
	}
	if objectStore.putFileCalled {
		t.Fatal("expected PutFile to be skipped when RawUploaded is true")
	}
}

func TestFinalizeUploadUsesOriginalFileNameWithoutExtensionAsDefaultTitle(t *testing.T) {
	repo := &uploadTestRepo{createdID: 7}
	svc := NewService(repo, &uploadTestQueue{}, nil, &uploadTestStatusStore{}, &uploadTestObjectStore{}, &uploadTestFS{}, nil, Paths{})
	svc.Now = func() time.Time { return time.Unix(1, 0) }

	if _, err := svc.FinalizeUpload(context.Background(), validUploadPlan(), UploadMeta{}); err != nil {
		t.Fatalf("FinalizeUpload returned error: %v", err)
	}
	if repo.createdVideo == nil {
		t.Fatal("expected video to be created")
	}
	if repo.createdVideo.Title != "lesson" {
		t.Fatalf("created title = %q, want %q", repo.createdVideo.Title, "lesson")
	}
}

func TestFinalizeUploadReturnsPutFileError(t *testing.T) {
	objectStore := &uploadTestObjectStore{putFileErr: errors.New("put failed")}
	repo := &uploadTestRepo{createdID: 7}
	svc := NewService(repo, &uploadTestQueue{}, nil, &uploadTestStatusStore{}, objectStore, &uploadTestFS{}, nil, Paths{})
	svc.Now = func() time.Time { return time.Unix(1, 0) }

	_, err := svc.FinalizeUpload(context.Background(), validUploadPlan(), UploadMeta{})
	if err == nil {
		t.Fatal("expected PutFile error")
	}
	if err.Error() != "put failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createdVideo != nil {
		t.Fatal("expected repo Create not to run after PutFile failure")
	}
}

func TestFinalizeUploadReturnsCreateError(t *testing.T) {
	repo := &uploadTestRepo{createdID: 7, createErr: errors.New("create failed")}
	statusStore := &uploadTestStatusStore{}
	objectStore := &uploadTestObjectStore{}
	svc := NewService(repo, &uploadTestQueue{}, nil, statusStore, objectStore, &uploadTestFS{}, nil, Paths{})
	svc.Now = func() time.Time { return time.Unix(1, 0) }

	_, err := svc.FinalizeUpload(context.Background(), validUploadPlan(), UploadMeta{})
	if err == nil {
		t.Fatal("expected Create error")
	}
	if err.Error() != "create failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusStore.lastTaskID != "" {
		t.Fatal("expected status store not to be updated after Create failure")
	}
	if objectStore.deletedKey != validUploadPlan().RawObjectKey {
		t.Fatalf("expected raw object cleanup after create failure, got %q", objectStore.deletedKey)
	}
}

func TestFinalizeUploadReturnsStatusStoreError(t *testing.T) {
	repo := &uploadTestRepo{createdID: 7}
	statusStore := &uploadTestStatusStore{setErr: errors.New("status failed")}
	queue := &uploadTestQueue{}
	objectStore := &uploadTestObjectStore{}
	svc := NewService(repo, queue, nil, statusStore, objectStore, &uploadTestFS{}, nil, Paths{})
	svc.Now = func() time.Time { return time.Unix(1, 0) }

	_, err := svc.FinalizeUpload(context.Background(), validUploadPlan(), UploadMeta{})
	if err == nil {
		t.Fatal("expected status store error")
	}
	if err.Error() != "status failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if queue.lastTask.TaskID != "" {
		t.Fatal("expected queue not to enqueue after status failure")
	}
	if repo.lastStatusID != 7 || repo.lastStatus != domainvideo.StatusFailed {
		t.Fatalf("expected video status to be marked failed, id=%d status=%v", repo.lastStatusID, repo.lastStatus)
	}
	if objectStore.deletedKey != validUploadPlan().RawObjectKey {
		t.Fatalf("expected raw object cleanup after status failure, got %q", objectStore.deletedKey)
	}
}

func TestFinalizeUploadReturnsQueueError(t *testing.T) {
	repo := &uploadTestRepo{createdID: 7}
	statusStore := &uploadTestStatusStore{}
	queue := &uploadTestQueue{err: errors.New("enqueue failed")}
	fs := &uploadTestFS{}
	objectStore := &uploadTestObjectStore{}
	svc := NewService(repo, queue, nil, statusStore, objectStore, fs, nil, Paths{})
	svc.Now = func() time.Time { return time.Unix(1, 0) }
	svc.DeleteLocal = true

	_, err := svc.FinalizeUpload(context.Background(), validUploadPlan(), UploadMeta{})
	if err == nil {
		t.Fatal("expected enqueue error")
	}
	if err.Error() != "enqueue failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.removePath != "" {
		t.Fatal("expected local file not to be removed when enqueue fails")
	}
	if repo.lastStatusID != 7 || repo.lastStatus != domainvideo.StatusFailed {
		t.Fatalf("expected video status to be marked failed, id=%d status=%v", repo.lastStatusID, repo.lastStatus)
	}
	if objectStore.deletedKey != validUploadPlan().RawObjectKey {
		t.Fatalf("expected raw object cleanup after queue failure, got %q", objectStore.deletedKey)
	}
}

func validUploadPlan() UploadPlan {
	return UploadPlan{
		OriginalFileName: "lesson.mp4",
		StoredFileName:   "stored.mp4",
		RawAbsPath:       "/tmp/raw/stored.mp4",
		RawObjectKey:     "raw/2026/05/21/stored.mp4",
		RawURL:           "/videos/raw/2026/05/21/stored.mp4",
		HLSObjectPrefix:  "hls/2026/05/21/stored",
		HLSURL:           "/videos/hls/2026/05/21/stored/master.m3u8",
	}
}

type uploadTestRepo struct {
	createdID     uint64
	createdVideo  *domainvideo.Video
	createErr     error
	lastStatusID  uint64
	lastStatus    domainvideo.Status
	lastStatusErr string
}

func (r *uploadTestRepo) Create(_ context.Context, v *domainvideo.Video) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.createdVideo = v
	v.ID = r.createdID
	return nil
}

func (*uploadTestRepo) List(context.Context, ListFilter) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) ListRecommendPool(context.Context) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) GetByID(context.Context, uint64) (domainvideo.Video, bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) DeleteByID(context.Context, uint64) (bool, error) { panic("unexpected call") }
func (*uploadTestRepo) UpdateMetadata(context.Context, uint64, string, string) (bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) UpdatePublished(context.Context, uint64, bool) (bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) UpdateRecommend(context.Context, uint64, bool, uint64, int16, float64) (bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) IncrementViewCount(context.Context, uint64) (int, bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) GetViewCount(context.Context, uint64) (int, bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) SubmitVideoReaction(context.Context, uint64, uint64, VideoReactionType) (bool, bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) ApplyVideoReactionState(context.Context, uint64, uint64, VideoReactionType, bool) (bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) GetVideoUserReaction(context.Context, uint64, uint64) (VideoReactionType, bool, bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) GetVideoReactionCounts(context.Context, uint64) (VideoReactionCounts, bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) FindSimilar(context.Context, uint64, int) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) UpdateCoverByID(context.Context, uint64, string) (bool, error) {
	panic("unexpected call")
}
func (r *uploadTestRepo) UpdateStatusByID(_ context.Context, id uint64, status domainvideo.Status, errMsg string) error {
	r.lastStatusID = id
	r.lastStatus = status
	r.lastStatusErr = errMsg
	return nil
}
func (*uploadTestRepo) GetSegmentEmbeddingDim(context.Context) (int, error) { panic("unexpected call") }
func (*uploadTestRepo) GetQuestionEmbeddingTextByID(context.Context, uint64) (string, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) ListQuestions(context.Context, int, int) (QuestionPage, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) GetQuestionByID(context.Context, uint64) (QuestionItem, bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) FindRecommendedSegments(context.Context, pgvector.Vector, int) ([]RecommendCandidate, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) FindRecommendedSegmentsByWeakKnowledge(context.Context, uint64, int, int) ([]RecommendCandidate, error) {
	return nil, nil
}
func (*uploadTestRepo) SaveUserVideoRecommendation(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
	panic("unexpected call")
}
func (*uploadTestRepo) ListRecommendations(context.Context, uint64, uint64, int) ([]RecommendationRecord, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) GetVideoIDBySegmentID(context.Context, uint64) (uint64, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) HasWatchedVideoForQuestion(context.Context, uint64, uint64, uint64) (bool, error) {
	panic("unexpected call")
}
func (*uploadTestRepo) SaveWatchRecord(context.Context, uint64, uint64, uint64, uint64, bool, int, time.Time) (bool, error) {
	panic("unexpected call")
}

type uploadTestStatusStore struct {
	lastTaskID string
	lastStatus domainvideo.Status
	lastHLSURL string
	lastTTL    time.Duration
	setErr     error
}

func (s *uploadTestStatusStore) Set(_ context.Context, taskID string, status domainvideo.Status, hlsURL string, ttl time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.lastTaskID = taskID
	s.lastStatus = status
	s.lastHLSURL = hlsURL
	s.lastTTL = ttl
	return nil
}

func (*uploadTestStatusStore) Get(context.Context, string) (TranscodeStatus, bool, error) {
	panic("unexpected call")
}

type uploadTestObjectStore struct {
	putFileCalled    bool
	putFileObjectKey string
	putFilePath      string
	putFileErr       error
	deletedKey       string
}

func (s *uploadTestObjectStore) PutFile(_ context.Context, objectKey string, filePath string, _ string) error {
	if s.putFileErr != nil {
		return s.putFileErr
	}
	s.putFileCalled = true
	s.putFileObjectKey = objectKey
	s.putFilePath = filePath
	return nil
}

func (*uploadTestObjectStore) Put(context.Context, string, io.Reader, int64, string) error {
	panic("unexpected call")
}

func (s *uploadTestObjectStore) Delete(_ context.Context, objectKey string) error {
	s.deletedKey = objectKey
	return nil
}

type uploadTestQueue struct {
	lastTask TranscodeTask
	err      error
}

func (q *uploadTestQueue) Enqueue(_ context.Context, task TranscodeTask) error {
	q.lastTask = task
	return q.err
}

type uploadTestVectorQueue struct {
	lastTask VectorizeTask
	err      error
}

func (q *uploadTestVectorQueue) Enqueue(_ context.Context, task VectorizeTask) error {
	q.lastTask = task
	return q.err
}

type uploadTestFS struct {
	mkdirAllPath string
	createPath   string
	removePath   string
	mkdirAllErr  error
	createErr    error
}

func (fs *uploadTestFS) MkdirAll(path string) error {
	if fs.mkdirAllErr != nil {
		return fs.mkdirAllErr
	}
	fs.mkdirAllPath = path
	return nil
}

func (fs *uploadTestFS) Create(path string) (io.WriteCloser, error) {
	if fs.createErr != nil {
		return nil, fs.createErr
	}
	fs.createPath = path
	return uploadTestWriteCloser{}, nil
}

func (*uploadTestFS) RemoveAll(string) error { panic("unexpected call") }

func (fs *uploadTestFS) Remove(path string) error {
	fs.removePath = path
	return nil
}

type uploadTestWriteCloser struct{}

func (uploadTestWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (uploadTestWriteCloser) Close() error                { return nil }
