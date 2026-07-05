package videoapp

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pgvector/pgvector-go"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

func TestUploadVideoRejectsMissingFile(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, Paths{})

	_, err := svc.UploadVideo(context.Background(), UploadVideoInput{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Error() != "file is required" {
		t.Fatalf("unexpected validation message: %q", validationErr.Error())
	}
}

func TestUploadVideoRejectsArchiveMetadataFile(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, Paths{})

	_, err := svc.UploadVideo(context.Background(), UploadVideoInput{
		FileName: "._一口气学完中国近代史.mp4",
		Reader:   bytes.NewBufferString("apple-double"),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Error() != "unsupported metadata file" {
		t.Fatalf("unexpected validation message: %q", validationErr.Error())
	}
}

func TestUploadVideoSuccessRemovesTempFileAfterFinalize(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.UTC)
	repo := &uploadHTTPTestRepo{createdID: 88}
	statusStore := &uploadHTTPTestStatusStore{}
	store := &uploadHTTPTestObjectStore{}
	queue := &uploadHTTPTestQueue{}
	fs := &uploadHTTPTestFS{}
	svc := NewService(repo, queue, nil, statusStore, store, fs, nil, Paths{
		RawDir:       "/tmp/raw",
		HLSDir:       "/tmp/hls",
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})
	svc.Now = func() time.Time { return fixedNow }

	result, err := svc.UploadVideo(context.Background(), UploadVideoInput{
		FileName:    "lesson.mp4",
		Title:       "Physics",
		Description: "desc",
		Reader:      bytes.NewBufferString("video-bytes"),
	})
	if err != nil {
		t.Fatalf("UploadVideo returned error: %v", err)
	}

	if fs.createdWriter == nil {
		t.Fatal("expected upload writer to be created")
	}
	if fs.createdWriter.buf.String() != "video-bytes" {
		t.Fatalf("unexpected written bytes: %q", fs.createdWriter.buf.String())
	}
	if fs.removePath == "" {
		t.Fatal("expected temp file removal after finalize success")
	}
	if statusStore.lastTaskID != "88" || queue.lastTask.TaskID != "88" {
		t.Fatalf("unexpected created task ids: status=%q queue=%q", statusStore.lastTaskID, queue.lastTask.TaskID)
	}
	if result.VideoID != 88 {
		t.Fatalf("unexpected video id: %d", result.VideoID)
	}
	if repo.createdVideo == nil || repo.createdVideo.UserID != DefaultUploadUserID {
		t.Fatalf("created video userID = %d, want %d", repo.createdVideo.UserID, DefaultUploadUserID)
	}
}

func TestUploadVideoPersistsProvidedUserID(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.UTC)
	repo := &uploadHTTPTestRepo{createdID: 89}
	svc := NewService(repo, &uploadHTTPTestQueue{}, nil, &uploadHTTPTestStatusStore{}, &uploadHTTPTestObjectStore{}, &uploadHTTPTestFS{}, nil, Paths{
		RawDir:       "/tmp/raw",
		HLSDir:       "/tmp/hls",
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})
	svc.Now = func() time.Time { return fixedNow }

	if _, err := svc.UploadVideo(context.Background(), UploadVideoInput{
		FileName: "lesson.mp4",
		UserID:   42,
		Reader:   bytes.NewBufferString("video-bytes"),
	}); err != nil {
		t.Fatalf("UploadVideo returned error: %v", err)
	}

	if repo.createdVideo == nil || repo.createdVideo.UserID != 42 {
		t.Fatalf("created video userID = %d, want 42", repo.createdVideo.UserID)
	}
}

func TestUploadVideoRejectsUserWithoutUploadPermission(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.UTC)
	repo := &uploadHTTPTestRepo{uploadDenied: true}
	store := &uploadHTTPTestObjectStore{}
	svc := NewService(repo, &uploadHTTPTestQueue{}, nil, &uploadHTTPTestStatusStore{}, store, &uploadHTTPTestFS{}, nil, Paths{
		RawDir:       "/tmp/raw",
		HLSDir:       "/tmp/hls",
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})
	svc.Now = func() time.Time { return fixedNow }

	_, err := svc.UploadVideo(context.Background(), UploadVideoInput{
		FileName: "lesson.mp4",
		UserID:   42,
		Reader:   bytes.NewBufferString("video-bytes"),
	})
	if err == nil {
		t.Fatal("expected upload permission error")
	}
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Error() != "user is not allowed to upload videos" {
		t.Fatalf("unexpected validation message: %q", validationErr.Error())
	}
	if repo.createdVideo != nil {
		t.Fatalf("created video despite missing permission: %+v", repo.createdVideo)
	}
	if store.putFileObjectKey != "" {
		t.Fatalf("uploaded object despite missing permission: %q", store.putFileObjectKey)
	}
}

func TestUploadVideoRemovesTempFileWhenFinalizeFails(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.UTC)
	repo := &uploadHTTPTestRepo{createdID: 99}
	statusStore := &uploadHTTPTestStatusStore{setErr: errors.New("status failed")}
	store := &uploadHTTPTestObjectStore{}
	queue := &uploadHTTPTestQueue{}
	fs := &uploadHTTPTestFS{}
	svc := NewService(repo, queue, nil, statusStore, store, fs, nil, Paths{
		RawDir:       "/tmp/raw",
		HLSDir:       "/tmp/hls",
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})
	svc.Now = func() time.Time { return fixedNow }

	_, err := svc.UploadVideo(context.Background(), UploadVideoInput{
		FileName: "lesson.mp4",
		Reader:   bytes.NewBufferString("video-bytes"),
	})
	if err == nil {
		t.Fatal("expected finalize error")
	}
	if err.Error() != "status failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.removePath == "" {
		t.Fatal("expected temp file removal after finalize failure")
	}
}

func TestUploadVideoArchiveUploadsVideosAndSkipsUnsafeOrNonVideoEntries(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.UTC)
	repo := &uploadHTTPTestRepo{nextIDs: []uint64{101, 102}}
	statusStore := &uploadHTTPTestStatusStore{}
	store := &uploadHTTPTestObjectStore{}
	queue := &uploadHTTPTestQueue{}
	fs := &uploadHTTPTestFS{}
	svc := NewService(repo, queue, nil, statusStore, store, fs, nil, Paths{
		RawDir:       "/tmp/raw",
		HLSDir:       "/tmp/hls",
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})
	svc.Now = func() time.Time { return fixedNow }

	archive := &bytes.Buffer{}
	zw := zip.NewWriter(archive)
	entries := []struct {
		name    string
		content string
	}{
		{name: "lesson-a.mp4", content: "video-a"},
		{name: "nested/b.mov", content: "video-b"},
		{name: "notes.txt", content: "notes"},
		{name: "../escape.mp4", content: "bad"},
		{name: "__MACOSX/._lesson-a.mp4", content: "apple-double"},
		{name: "nested/.DS_Store", content: "finder"},
		{name: "nested/._b.mov", content: "apple-double"},
	}
	for _, entry := range entries {
		w, err := zw.Create(entry.name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(entry.content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	result, err := svc.UploadVideoArchive(context.Background(), UploadVideoArchiveInput{
		FileName:    "lessons.zip",
		Description: "batch-desc",
		Reader:      bytes.NewReader(archive.Bytes()),
	})
	if err != nil {
		t.Fatalf("UploadVideoArchive returned error: %v", err)
	}

	if result.Total != 7 {
		t.Fatalf("total = %d, want 7", result.Total)
	}
	if len(result.Uploaded) != 2 {
		t.Fatalf("uploaded = %d, want 2: %+v", len(result.Uploaded), result.Uploaded)
	}
	if result.Uploaded[0].VideoID != 101 || result.Uploaded[1].VideoID != 102 {
		t.Fatalf("unexpected uploaded ids: %+v", result.Uploaded)
	}
	if len(result.Skipped) != 5 {
		t.Fatalf("skipped = %d, want 5: %+v", len(result.Skipped), result.Skipped)
	}
	if result.Uploaded[0].Name != "lesson-a.mp4" || result.Uploaded[1].Name != "b.mov" {
		t.Fatalf("uploaded names = %+v", result.Uploaded)
	}
	if repo.createdTitles[0] != "lesson-a" || repo.createdTitles[1] != "b" {
		t.Fatalf("created titles = %+v", repo.createdTitles)
	}
	if repo.createdDescriptions[0] != "batch-desc" || repo.createdDescriptions[1] != "batch-desc" {
		t.Fatalf("created descriptions = %+v", repo.createdDescriptions)
	}
	if repo.createdUserIDs[0] != DefaultUploadUserID || repo.createdUserIDs[1] != DefaultUploadUserID {
		t.Fatalf("created userIDs = %+v, want all %d", repo.createdUserIDs, DefaultUploadUserID)
	}
	if len(queue.tasks) != 2 {
		t.Fatalf("queued tasks = %d, want 2", len(queue.tasks))
	}
}

func TestUploadVideoArchiveRejectsUserWithoutUploadPermission(t *testing.T) {
	repo := &uploadHTTPTestRepo{uploadDenied: true}
	store := &uploadHTTPTestObjectStore{}
	svc := NewService(repo, &uploadHTTPTestQueue{}, nil, &uploadHTTPTestStatusStore{}, store, &uploadHTTPTestFS{}, nil, Paths{
		RawDir:       "/tmp/raw",
		HLSDir:       "/tmp/hls",
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})

	archive := &bytes.Buffer{}
	zw := zip.NewWriter(archive)
	w, err := zw.Create("lesson.mp4")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := w.Write([]byte("video-bytes")); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	_, err = svc.UploadVideoArchive(context.Background(), UploadVideoArchiveInput{
		FileName: "lessons.zip",
		UserID:   42,
		Reader:   bytes.NewReader(archive.Bytes()),
	})
	if err == nil {
		t.Fatal("expected upload permission error")
	}
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Error() != "user is not allowed to upload videos" {
		t.Fatalf("unexpected validation message: %q", validationErr.Error())
	}
	if repo.createdVideo != nil {
		t.Fatalf("created video despite missing permission: %+v", repo.createdVideo)
	}
	if store.putFileObjectKey != "" {
		t.Fatalf("uploaded object despite missing permission: %q", store.putFileObjectKey)
	}
}

func TestUploadVideoCoverRejectsInvalidInput(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, Paths{})
	tests := []struct {
		name  string
		video uint64
		input UploadCoverInput
		want  string
	}{
		{name: "missing video id", video: 0, input: UploadCoverInput{FileName: "cover.jpg", Reader: bytes.NewBufferString("x")}, want: "video_id is required"},
		{name: "missing file name", video: 1, input: UploadCoverInput{Reader: bytes.NewBufferString("x")}, want: "file is required"},
		{name: "missing reader", video: 1, input: UploadCoverInput{FileName: "cover.jpg"}, want: "file is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := svc.UploadVideoCover(context.Background(), tc.video, tc.input)
			if err == nil {
				t.Fatal("expected validation error")
			}
			var validationErr ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected ValidationError, got %T", err)
			}
			if validationErr.Error() != tc.want {
				t.Fatalf("unexpected validation message: %q", validationErr.Error())
			}
		})
	}
}

func TestUploadVideoCoverReturnsFalseWhenVideoMissing(t *testing.T) {
	repo := &uploadHTTPTestRepo{getByIDOK: false}
	svc := NewService(repo, nil, nil, nil, &uploadHTTPTestObjectStore{}, nil, nil, Paths{})
	svc.Now = time.Now

	coverURL, updated, err := svc.UploadVideoCover(context.Background(), 1, UploadCoverInput{
		FileName: "cover.jpg",
		Reader:   bytes.NewBufferString("cover"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated {
		t.Fatal("expected updated=false when video missing")
	}
	if coverURL != "" {
		t.Fatalf("expected empty cover url, got %q", coverURL)
	}
}

func TestUploadVideoCoverSuccessUsesDerivedContentType(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.UTC)
	repo := &uploadHTTPTestRepo{getByIDOK: true, updateCoverOK: true}
	store := &uploadHTTPTestObjectStore{}
	svc := NewService(repo, nil, nil, nil, store, nil, nil, Paths{})
	svc.Now = func() time.Time { return fixedNow }

	coverURL, updated, err := svc.UploadVideoCover(context.Background(), 12, UploadCoverInput{
		FileName: "cover",
		Reader:   bytes.NewBufferString("cover-bytes"),
		Size:     11,
	})
	if err != nil {
		t.Fatalf("UploadVideoCover returned error: %v", err)
	}
	if !updated {
		t.Fatal("expected updated=true")
	}
	if coverURL != "/videos/cover/2026/05/21/vid_12_1779366600123456789.jpg" {
		t.Fatalf("unexpected cover url: %q", coverURL)
	}
	if store.putObjectKey != "cover/2026/05/21/vid_12_1779366600123456789.jpg" {
		t.Fatalf("unexpected object key: %q", store.putObjectKey)
	}
	if store.putContentType != "image/jpeg" {
		t.Fatalf("unexpected content type: %q", store.putContentType)
	}
	if repo.lastCoverURL != coverURL {
		t.Fatalf("expected repo cover update with %q, got %q", coverURL, repo.lastCoverURL)
	}
}

func TestUploadVideoCoverRollsBackObjectOnUpdateError(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.UTC)
	repo := &uploadHTTPTestRepo{getByIDOK: true, updateCoverErr: errors.New("db failed")}
	store := &uploadHTTPTestObjectStore{}
	svc := NewService(repo, nil, nil, nil, store, nil, nil, Paths{})
	svc.Now = func() time.Time { return fixedNow }

	_, updated, err := svc.UploadVideoCover(context.Background(), 12, UploadCoverInput{
		FileName: "cover.png",
		Reader:   bytes.NewBufferString("cover-bytes"),
	})
	if err == nil {
		t.Fatal("expected update error")
	}
	if updated {
		t.Fatal("expected updated=false")
	}
	if err.Error() != "db failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.deletedObjectKey != "cover/2026/05/21/vid_12_1779366600123456789.png" {
		t.Fatalf("expected rollback delete, got %q", store.deletedObjectKey)
	}
}

func TestUploadVideoCoverRollsBackObjectWhenUpdateReturnsFalse(t *testing.T) {
	fixedNow := time.Date(2026, 5, 21, 12, 30, 0, 123456789, time.UTC)
	repo := &uploadHTTPTestRepo{getByIDOK: true, updateCoverOK: false}
	store := &uploadHTTPTestObjectStore{}
	svc := NewService(repo, nil, nil, nil, store, nil, nil, Paths{})
	svc.Now = func() time.Time { return fixedNow }

	coverURL, updated, err := svc.UploadVideoCover(context.Background(), 12, UploadCoverInput{
		FileName: "cover.webp",
		Reader:   bytes.NewBufferString("cover-bytes"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated {
		t.Fatal("expected updated=false")
	}
	if coverURL != "" {
		t.Fatalf("expected empty cover url, got %q", coverURL)
	}
	if store.deletedObjectKey != "cover/2026/05/21/vid_12_1779366600123456789.webp" {
		t.Fatalf("expected rollback delete, got %q", store.deletedObjectKey)
	}
}

func TestContentTypeFromExtension(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{ext: ".jpg", want: "image/jpeg"},
		{ext: ".jpeg", want: "image/jpeg"},
		{ext: ".png", want: "image/png"},
		{ext: ".webp", want: "image/webp"},
		{ext: ".gif", want: "image/gif"},
		{ext: ".bin", want: "application/octet-stream"},
	}

	for _, tc := range tests {
		if got := contentTypeFromExtension(tc.ext); got != tc.want {
			t.Fatalf("contentTypeFromExtension(%q) = %q, want %q", tc.ext, got, tc.want)
		}
	}
}

type uploadHTTPTestRepo struct {
	createdID           uint64
	nextIDs             []uint64
	createdVideo        *domainvideo.Video
	createdTitles       []string
	createdDescriptions []string
	createdUserIDs      []uint64
	getByIDOK           bool
	getByIDErr          error
	updateCoverOK       bool
	updateCoverErr      error
	lastCoverURL        string
	lastStatusID        uint64
	lastStatus          domainvideo.Status
	archiveProgress     ArchiveProcessingProgress
	progressVideoIDs    []uint64
	uploadPermissionErr error
	uploadDenied        bool
}

func (r *uploadHTTPTestRepo) Create(_ context.Context, v *domainvideo.Video) error {
	r.createdVideo = v
	r.createdTitles = append(r.createdTitles, v.Title)
	r.createdDescriptions = append(r.createdDescriptions, v.Description)
	r.createdUserIDs = append(r.createdUserIDs, v.UserID)
	if len(r.nextIDs) > 0 {
		v.ID = r.nextIDs[0]
		r.nextIDs = r.nextIDs[1:]
		return nil
	}
	v.ID = r.createdID
	return nil
}
func (*uploadHTTPTestRepo) List(context.Context, ListFilter) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) ListRecommendPool(context.Context) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (r *uploadHTTPTestRepo) CanUploadVideo(context.Context, uint64) (bool, error) {
	if r.uploadPermissionErr != nil {
		return false, r.uploadPermissionErr
	}
	return !r.uploadDenied, nil
}
func (r *uploadHTTPTestRepo) GetByID(context.Context, uint64) (domainvideo.Video, bool, error) {
	if r.getByIDErr != nil {
		return domainvideo.Video{}, false, r.getByIDErr
	}
	if !r.getByIDOK {
		return domainvideo.Video{}, false, nil
	}
	return domainvideo.Video{ID: 12}, true, nil
}
func (*uploadHTTPTestRepo) DeleteByID(context.Context, uint64) (bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) UpdateMetadata(context.Context, uint64, string, string) (bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) UpdatePublished(context.Context, uint64, bool) (bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) UpdateRecommend(context.Context, uint64, bool, uint64, int16, float64) (bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) IncrementViewCount(context.Context, uint64) (int, bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) GetViewCount(context.Context, uint64) (int, bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) SubmitVideoReaction(context.Context, uint64, uint64, VideoReactionType) (bool, bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) ApplyVideoReactionState(context.Context, uint64, uint64, VideoReactionType, bool) (bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) GetVideoUserReaction(context.Context, uint64, uint64) (VideoReactionType, bool, bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) GetVideoReactionCounts(context.Context, uint64) (VideoReactionCounts, bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) FindSimilar(context.Context, uint64, int) ([]domainvideo.Video, error) {
	panic("unexpected call")
}
func (r *uploadHTTPTestRepo) UpdateCoverByID(_ context.Context, _ uint64, coverURL string) (bool, error) {
	r.lastCoverURL = coverURL
	if r.updateCoverErr != nil {
		return false, r.updateCoverErr
	}
	return r.updateCoverOK, nil
}
func (r *uploadHTTPTestRepo) UpdateStatusByID(_ context.Context, id uint64, status domainvideo.Status, _ string) error {
	r.lastStatusID = id
	r.lastStatus = status
	return nil
}
func (*uploadHTTPTestRepo) GetSegmentEmbeddingDim(context.Context) (int, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) GetQuestionEmbeddingTextByID(context.Context, uint64) (string, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) ListQuestions(context.Context, int, int) (QuestionPage, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) GetQuestionByID(context.Context, uint64) (QuestionItem, bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) FindRecommendedSegments(context.Context, pgvector.Vector, int) ([]RecommendCandidate, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) FindRecommendedSegmentsByWeakKnowledge(context.Context, uint64, int, int) ([]RecommendCandidate, error) {
	return nil, nil
}
func (*uploadHTTPTestRepo) SaveUserVideoRecommendation(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) ListRecommendations(context.Context, uint64, uint64, int) ([]RecommendationRecord, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) GetVideoIDBySegmentID(context.Context, uint64) (uint64, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) HasWatchedVideoForQuestion(context.Context, uint64, uint64, uint64) (bool, error) {
	panic("unexpected call")
}
func (*uploadHTTPTestRepo) SaveWatchRecord(context.Context, uint64, uint64, uint64, uint64, bool, int, time.Time) (bool, error) {
	panic("unexpected call")
}
func (r *uploadHTTPTestRepo) GetArchiveProcessingProgress(_ context.Context, videoIDs []uint64) (ArchiveProcessingProgress, error) {
	r.progressVideoIDs = append([]uint64(nil), videoIDs...)
	return r.archiveProgress, nil
}

type uploadHTTPTestStatusStore struct {
	lastTaskID string
	setErr     error
}

func (s *uploadHTTPTestStatusStore) Set(_ context.Context, taskID string, _ domainvideo.Status, _ string, _ time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.lastTaskID = taskID
	return nil
}

func (*uploadHTTPTestStatusStore) Get(context.Context, string) (TranscodeStatus, bool, error) {
	panic("unexpected call")
}

type uploadHTTPTestObjectStore struct {
	putFileObjectKey string
	putFilePath      string
	putObjectKey     string
	putContentType   string
	deletedObjectKey string
}

func (s *uploadHTTPTestObjectStore) PutFile(_ context.Context, objectKey string, filePath string, _ string) error {
	s.putFileObjectKey = objectKey
	s.putFilePath = filePath
	return nil
}

func (s *uploadHTTPTestObjectStore) Put(_ context.Context, objectKey string, r io.Reader, _ int64, contentType string) error {
	_, _ = io.ReadAll(r)
	s.putObjectKey = objectKey
	s.putContentType = contentType
	return nil
}

func (s *uploadHTTPTestObjectStore) Delete(_ context.Context, objectKey string) error {
	s.deletedObjectKey = objectKey
	return nil
}

type uploadHTTPTestQueue struct {
	lastTask TranscodeTask
	tasks    []TranscodeTask
}

func (q *uploadHTTPTestQueue) Enqueue(_ context.Context, task TranscodeTask) error {
	q.lastTask = task
	q.tasks = append(q.tasks, task)
	return nil
}

type uploadHTTPTestFS struct {
	mkdirAllPath  string
	createPath    string
	removePath    string
	createdWriter *uploadHTTPTestWriteCloser
}

func (fs *uploadHTTPTestFS) MkdirAll(path string) error {
	fs.mkdirAllPath = path
	return nil
}

func (fs *uploadHTTPTestFS) Create(path string) (io.WriteCloser, error) {
	fs.createPath = path
	fs.createdWriter = &uploadHTTPTestWriteCloser{}
	return fs.createdWriter, nil
}

func (*uploadHTTPTestFS) RemoveAll(string) error { panic("unexpected call") }

func (fs *uploadHTTPTestFS) Remove(path string) error {
	fs.removePath = path
	return nil
}

type uploadHTTPTestWriteCloser struct {
	buf    bytes.Buffer
	closed bool
}

func (w *uploadHTTPTestWriteCloser) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *uploadHTTPTestWriteCloser) Close() error {
	w.closed = true
	return nil
}

func TestRollbackObjectSkipsNilStoreAndEmptyKey(t *testing.T) {
	rollbackObject(context.Background(), nil, "cover/x.jpg")
	store := &uploadHTTPTestObjectStore{}
	rollbackObject(context.Background(), store, "   ")
	if store.deletedObjectKey != "" {
		t.Fatalf("expected no delete for empty object key, got %q", store.deletedObjectKey)
	}
}

func TestUploadVideoCoverPropagatesRepoError(t *testing.T) {
	repo := &uploadHTTPTestRepo{getByIDErr: errors.New("db read failed")}
	svc := NewService(repo, nil, nil, nil, &uploadHTTPTestObjectStore{}, nil, nil, Paths{})

	_, _, err := svc.UploadVideoCover(context.Background(), 1, UploadCoverInput{
		FileName: "cover.jpg",
		Reader:   strings.NewReader("cover"),
	})
	if err == nil {
		t.Fatal("expected repo error")
	}
	if err.Error() != "db read failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}
