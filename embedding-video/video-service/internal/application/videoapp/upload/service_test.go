package upload

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestUploadVideoRejectsMissingFile(t *testing.T) {
	errInvalid := errors.New("invalid")
	svc := Service{
		InvalidArgument: func(message string) error {
			if message != "file is required" {
				t.Fatalf("message = %q, want file is required", message)
			}
			return errInvalid
		},
	}

	if _, err := svc.UploadVideo(context.Background(), UploadVideoInput{FileName: "lesson.mp4"}); !errors.Is(err, errInvalid) {
		t.Fatalf("err = %v, want %v", err, errInvalid)
	}
	if _, err := svc.UploadVideo(context.Background(), UploadVideoInput{Reader: strings.NewReader("body")}); !errors.Is(err, errInvalid) {
		t.Fatalf("err = %v, want %v", err, errInvalid)
	}
}

func TestUploadVideoRemovesRawFileWhenCopyFails(t *testing.T) {
	fs := &fakeFileRemover{}
	svc := Service{
		Planner: &fakePlanner{
			plan:   Plan{RawAbsPath: "/tmp/raw/lesson.mp4"},
			writer: &recordingWriter{writeErr: errors.New("copy failed")},
		},
		FS:              fs,
		InvalidArgument: invalidArgumentError,
	}

	_, err := svc.UploadVideo(context.Background(), UploadVideoInput{
		FileName: "lesson.mp4",
		Reader:   strings.NewReader("body"),
	})
	if err == nil {
		t.Fatal("expected copy error")
	}
	if fs.removed != "/tmp/raw/lesson.mp4" {
		t.Fatalf("removed = %q, want raw file cleanup", fs.removed)
	}
}

func TestUploadVideoArchiveUploadsSupportedFilesAndSkipsMetadata(t *testing.T) {
	planner := &fakePlanner{
		writer: &recordingWriter{},
		results: []Result{
			{VideoID: 1, TaskID: "1", RawURL: "/videos/raw/a.mp4", HLSURL: "/videos/hls/a/master.m3u8"},
			{VideoID: 2, TaskID: "2", RawURL: "/videos/raw/b.webm", HLSURL: "/videos/hls/b/master.m3u8"},
		},
	}
	svc := Service{
		Planner:         planner,
		FS:              &fakeFileRemover{},
		InvalidArgument: invalidArgumentError,
	}

	result, err := svc.UploadVideoArchive(context.Background(), UploadVideoArchiveInput{
		FileName:    "lessons.zip",
		Description: "desc",
		Reader: archiveReader(t, map[string]string{
			"chapter/lesson.mp4": "mp4 body",
			"trailer.webm":       "webm body",
			"notes.txt":          "skip",
			"__MACOSX/._junk":    "skip",
			"../escape.mp4":      "skip",
		}),
	})
	if err != nil {
		t.Fatalf("UploadVideoArchive returned error: %v", err)
	}

	if result.Total != 5 {
		t.Fatalf("total = %d, want 5", result.Total)
	}
	if len(result.Uploaded) != 2 {
		t.Fatalf("uploaded = %d, want 2", len(result.Uploaded))
	}
	if result.Uploaded[0].Name != "lesson.mp4" {
		t.Fatalf("first uploaded name = %q", result.Uploaded[0].Name)
	}
	if result.Uploaded[1].Name != "trailer.webm" {
		t.Fatalf("second uploaded name = %q", result.Uploaded[1].Name)
	}
	if len(result.Skipped) != 3 {
		t.Fatalf("skipped = %#v, want 3 entries", result.Skipped)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("failed = %#v, want none", result.Failed)
	}
	if planner.fileNames[0] != "lesson.mp4" || planner.fileNames[1] != "trailer.webm" {
		t.Fatalf("planner file names = %#v", planner.fileNames)
	}
}

func TestUploadVideoCoverUsesDefaultsAndUpdatesRepository(t *testing.T) {
	now := time.Date(2026, 5, 21, 12, 30, 0, 42, time.UTC)
	store := &fakeObjectStore{}
	repo := &fakeCoverRepository{found: true, updated: true}
	svc := Service{
		Repo:            repo,
		Store:           store,
		Now:             func() time.Time { return now },
		InvalidArgument: invalidArgumentError,
	}

	coverURL, updated, err := svc.UploadVideoCover(context.Background(), 9, UploadCoverInput{
		FileName: "cover",
		Reader:   strings.NewReader("image"),
		Size:     -5,
	})
	if err != nil {
		t.Fatalf("UploadVideoCover returned error: %v", err)
	}
	if !updated {
		t.Fatal("updated = false, want true")
	}
	wantKey := "cover/2026/05/21/vid_9_1779366600000000042.jpg"
	if store.putKey != wantKey {
		t.Fatalf("put key = %q, want %q", store.putKey, wantKey)
	}
	if store.putSize != -1 {
		t.Fatalf("put size = %d, want -1", store.putSize)
	}
	if store.putContentType != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", store.putContentType)
	}
	if coverURL != "/videos/"+wantKey {
		t.Fatalf("coverURL = %q", coverURL)
	}
	if repo.coverURL != coverURL {
		t.Fatalf("repo coverURL = %q, want %q", repo.coverURL, coverURL)
	}
}

func TestUploadVideoCoverRollsBackObjectWhenRepositoryUpdateFails(t *testing.T) {
	store := &fakeObjectStore{}
	repo := &fakeCoverRepository{found: true, updateErr: errors.New("db failed")}
	svc := Service{
		Repo:            repo,
		Store:           store,
		Now:             func() time.Time { return time.Unix(1, 0).UTC() },
		InvalidArgument: invalidArgumentError,
	}

	_, _, err := svc.UploadVideoCover(context.Background(), 5, UploadCoverInput{
		FileName: "cover.png",
		Reader:   strings.NewReader("image"),
	})
	if err == nil {
		t.Fatal("expected update error")
	}
	if store.deletedKey != store.putKey {
		t.Fatalf("deleted key = %q, want put key %q", store.deletedKey, store.putKey)
	}
}

func TestArchiveEntryAndContentTypeHelpers(t *testing.T) {
	if !IsZipFileName(" lessons.ZIP ") {
		t.Fatal("expected ZIP extension to be accepted")
	}
	if IsSafeArchiveEntryName("../escape.mp4") {
		t.Fatal("expected parent traversal to be unsafe")
	}
	if IsSafeArchiveEntryName("/absolute.mp4") {
		t.Fatal("expected absolute entry to be unsafe")
	}
	if !IsSafeArchiveEntryName("chapter/lesson.mp4") {
		t.Fatal("expected nested video path to be safe")
	}
	if !IsArchiveMetadataEntryName("__MACOSX/._lesson.mp4") {
		t.Fatal("expected macOS metadata to be detected")
	}
	if IsArchiveMetadataEntryName("chapter/lesson.mp4") {
		t.Fatal("expected normal video entry not to be metadata")
	}
	if !IsSupportedVideoFileName("clip.MKV") {
		t.Fatal("expected MKV extension to be supported")
	}
	if ContentTypeFromVideoExtension(".webm") != "video/webm" {
		t.Fatalf("unexpected webm content type")
	}
	if ContentTypeFromExtension(".webp") != "image/webp" {
		t.Fatalf("unexpected webp content type")
	}
}

type fakePlanner struct {
	plan      Plan
	writer    *recordingWriter
	results   []Result
	fileNames []string
	calls     int
}

func (p *fakePlanner) BuildUploadPlan(originalFileName string) (Plan, error) {
	p.fileNames = append(p.fileNames, originalFileName)
	plan := p.plan
	if strings.TrimSpace(plan.RawAbsPath) == "" {
		plan.RawAbsPath = "/tmp/raw/" + originalFileName
	}
	return plan, nil
}

func (p *fakePlanner) OpenUploadWriter(Plan) (io.WriteCloser, error) {
	if p.writer == nil {
		p.writer = &recordingWriter{}
	}
	return p.writer, nil
}

func (p *fakePlanner) FinalizeUpload(context.Context, Plan, Meta) (Result, error) {
	if p.calls >= len(p.results) {
		return Result{}, errors.New("unexpected finalize call")
	}
	result := p.results[p.calls]
	p.calls++
	return result, nil
}

type recordingWriter struct {
	writeErr error
	closed   int
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return len(p), nil
}

func (w *recordingWriter) Close() error {
	w.closed++
	return nil
}

type fakeFileRemover struct {
	removed string
}

func (f *fakeFileRemover) Remove(path string) error {
	f.removed = path
	return nil
}

type fakeCoverRepository struct {
	found     bool
	updated   bool
	updateErr error
	coverURL  string
}

func (r *fakeCoverRepository) GetByID(context.Context, uint64) (any, bool, error) {
	return struct{}{}, r.found, nil
}

func (r *fakeCoverRepository) SetVideoCover(_ context.Context, _ uint64, coverURL string) (bool, error) {
	r.coverURL = coverURL
	if r.updateErr != nil {
		return false, r.updateErr
	}
	return r.updated, nil
}

type fakeObjectStore struct {
	putKey         string
	putSize        int64
	putContentType string
	deletedKey     string
}

func (s *fakeObjectStore) Put(_ context.Context, objectKey string, _ io.Reader, size int64, contentType string) error {
	s.putKey = objectKey
	s.putSize = size
	s.putContentType = contentType
	return nil
}

func (s *fakeObjectStore) Delete(_ context.Context, objectKey string) error {
	s.deletedKey = objectKey
	return nil
}

func invalidArgumentError(message string) error {
	return errors.New(message)
}

func archiveReader(t *testing.T, entries map[string]string) io.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return bytes.NewReader(buf.Bytes())
}
