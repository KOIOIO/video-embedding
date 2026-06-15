package videoapp

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestChunkedUploadCompletesAndEnqueuesTranscode(t *testing.T) {
	fixedNow := time.Date(2026, 6, 9, 10, 30, 0, 123456789, time.UTC)
	rawDir := t.TempDir()
	repo := &uploadHTTPTestRepo{createdID: 77}
	statusStore := &uploadHTTPTestStatusStore{}
	store := &uploadHTTPTestObjectStore{}
	queue := &uploadHTTPTestQueue{}
	svc := NewService(repo, queue, nil, statusStore, store, nil, nil, Paths{
		RawDir:       rawDir,
		HLSDir:       filepath.Join(t.TempDir(), "hls"),
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})
	svc.Now = func() time.Time { return fixedNow }

	session, err := svc.InitiateChunkedUpload(context.Background(), InitiateChunkedUploadInput{
		FileName:    "lesson.mp4",
		FileSize:    10,
		ChunkSize:   5,
		TotalChunks: 2,
		Title:       "Physics",
		Description: "desc",
	})
	if err != nil {
		t.Fatalf("InitiateChunkedUpload returned error: %v", err)
	}
	if session.UploadID == "" {
		t.Fatal("UploadID is empty")
	}
	if session.UploadedChunks == nil || len(session.UploadedChunks) != 0 {
		t.Fatalf("uploaded chunks = %#v, want empty list", session.UploadedChunks)
	}

	first, err := svc.UploadVideoChunk(context.Background(), UploadVideoChunkInput{
		UploadID:   session.UploadID,
		ChunkIndex: 0,
		Reader:     bytes.NewBufferString("hello"),
	})
	if err != nil {
		t.Fatalf("UploadVideoChunk first returned error: %v", err)
	}
	if !reflect.DeepEqual(first.UploadedChunks, []int{0}) {
		t.Fatalf("uploaded chunks after first = %#v, want [0]", first.UploadedChunks)
	}

	second, err := svc.UploadVideoChunk(context.Background(), UploadVideoChunkInput{
		UploadID:   session.UploadID,
		ChunkIndex: 1,
		Reader:     bytes.NewBufferString("world"),
	})
	if err != nil {
		t.Fatalf("UploadVideoChunk second returned error: %v", err)
	}
	if !second.Completed {
		t.Fatalf("completed = false after all chunks: %+v", second)
	}

	status, err := svc.GetChunkedUploadStatus(context.Background(), session.UploadID)
	if err != nil {
		t.Fatalf("GetChunkedUploadStatus returned error: %v", err)
	}
	if !reflect.DeepEqual(status.UploadedChunks, []int{0, 1}) {
		t.Fatalf("status uploaded chunks = %#v, want [0 1]", status.UploadedChunks)
	}

	result, err := svc.CompleteChunkedUpload(context.Background(), CompleteChunkedUploadInput{UploadID: session.UploadID})
	if err != nil {
		t.Fatalf("CompleteChunkedUpload returned error: %v", err)
	}
	if result.VideoID != 77 || result.TaskID != "77" {
		t.Fatalf("result = %+v, want video/task 77", result)
	}
	if repo.createdVideo == nil || repo.createdVideo.Title != "Physics" || repo.createdVideo.Description != "desc" {
		t.Fatalf("created video = %+v", repo.createdVideo)
	}
	if queue.lastTask.TaskID != "77" || queue.lastTask.RawKey == "" {
		t.Fatalf("queued task = %+v", queue.lastTask)
	}
	if statusStore.lastTaskID != "77" {
		t.Fatalf("status task id = %q, want 77", statusStore.lastTaskID)
	}
	if store.putFilePath == "" {
		t.Fatal("expected finalized raw file to be uploaded to object storage")
	}
	if _, err := os.Stat(filepath.Join(rawDir, ".uploads", session.UploadID)); !os.IsNotExist(err) {
		t.Fatalf("expected upload session directory to be removed, stat err=%v", err)
	}
}

func TestChunkedUploadRejectsIncompleteComplete(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, Paths{RawDir: t.TempDir()})

	session, err := svc.InitiateChunkedUpload(context.Background(), InitiateChunkedUploadInput{
		FileName:    "lesson.mp4",
		FileSize:    10,
		ChunkSize:   5,
		TotalChunks: 2,
	})
	if err != nil {
		t.Fatalf("InitiateChunkedUpload returned error: %v", err)
	}
	if _, err := svc.UploadVideoChunk(context.Background(), UploadVideoChunkInput{
		UploadID:   session.UploadID,
		ChunkIndex: 0,
		Reader:     bytes.NewBufferString("hello"),
	}); err != nil {
		t.Fatalf("UploadVideoChunk returned error: %v", err)
	}

	_, err = svc.CompleteChunkedUpload(context.Background(), CompleteChunkedUploadInput{UploadID: session.UploadID})
	if err == nil {
		t.Fatal("expected incomplete upload error")
	}
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if err.Error() != "upload is incomplete" {
		t.Fatalf("err = %q, want upload is incomplete", err.Error())
	}
}

func TestChunkedUploadRejectsWrongSizedChunk(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, Paths{RawDir: t.TempDir()})

	session, err := svc.InitiateChunkedUpload(context.Background(), InitiateChunkedUploadInput{
		FileName:    "lesson.mp4",
		FileSize:    10,
		ChunkSize:   5,
		TotalChunks: 2,
	})
	if err != nil {
		t.Fatalf("InitiateChunkedUpload returned error: %v", err)
	}

	_, err = svc.UploadVideoChunk(context.Background(), UploadVideoChunkInput{
		UploadID:   session.UploadID,
		ChunkIndex: 0,
		Reader:     bytes.NewBufferString("hel"),
	})
	if err == nil {
		t.Fatal("expected wrong chunk size error")
	}
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if err.Error() != "chunk size is invalid" {
		t.Fatalf("err = %q, want chunk size is invalid", err.Error())
	}

	status, err := svc.GetChunkedUploadStatus(context.Background(), session.UploadID)
	if err != nil {
		t.Fatalf("GetChunkedUploadStatus returned error: %v", err)
	}
	if len(status.UploadedChunks) != 0 {
		t.Fatalf("uploaded chunks = %#v, want empty", status.UploadedChunks)
	}
}

func TestChunkedArchiveUploadCompletesAndImportsVideos(t *testing.T) {
	fixedNow := time.Date(2026, 6, 9, 11, 30, 0, 123456789, time.UTC)
	rawDir := t.TempDir()
	repo := &uploadHTTPTestRepo{nextIDs: []uint64{201, 202}}
	statusStore := &uploadHTTPTestStatusStore{}
	store := &uploadHTTPTestObjectStore{}
	queue := &uploadHTTPTestQueue{}
	fs := &uploadHTTPTestFS{}
	svc := NewService(repo, queue, nil, statusStore, store, nil, nil, Paths{
		RawDir:       rawDir,
		HLSDir:       filepath.Join(t.TempDir(), "hls"),
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})
	svc.FS = fs
	svc.Now = func() time.Time { return fixedNow }

	payload := buildChunkedUploadZip(t, map[string]string{
		"lesson-a.mp4": "video-a",
		"nested/b.mov": "video-b",
		"notes.txt":    "notes",
	})
	session, err := svc.InitiateChunkedArchiveUpload(context.Background(), InitiateChunkedUploadInput{
		FileName:    "lessons.zip",
		FileSize:    int64(len(payload)),
		ChunkSize:   int64(len(payload)),
		TotalChunks: 1,
		Description: "batch-desc",
	})
	if err != nil {
		t.Fatalf("InitiateChunkedArchiveUpload returned error: %v", err)
	}
	if _, err := svc.UploadVideoChunk(context.Background(), UploadVideoChunkInput{
		UploadID:   session.UploadID,
		ChunkIndex: 0,
		Reader:     bytes.NewReader(payload),
	}); err != nil {
		t.Fatalf("UploadVideoChunk returned error: %v", err)
	}

	result, err := svc.CompleteChunkedArchiveUpload(context.Background(), CompleteChunkedUploadInput{UploadID: session.UploadID})
	if err != nil {
		t.Fatalf("CompleteChunkedArchiveUpload returned error: %v", err)
	}
	if result.Total != 3 || len(result.Uploaded) != 2 || len(result.Skipped) != 1 {
		t.Fatalf("result = %+v, want total 3 uploaded 2 skipped 1", result)
	}
	if result.Uploaded[0].VideoID != 201 || result.Uploaded[1].VideoID != 202 {
		t.Fatalf("uploaded ids = %+v", result.Uploaded)
	}
	if repo.createdTitles[0] != "lesson-a" || repo.createdTitles[1] != "b" {
		t.Fatalf("created titles = %+v", repo.createdTitles)
	}
	if repo.createdDescriptions[0] != "batch-desc" || repo.createdDescriptions[1] != "batch-desc" {
		t.Fatalf("created descriptions = %+v", repo.createdDescriptions)
	}
	if len(queue.tasks) != 2 {
		t.Fatalf("queued tasks = %d, want 2", len(queue.tasks))
	}
	if result.Skipped[0] != "notes.txt" {
		t.Fatalf("skipped = %+v, want notes.txt", result.Skipped)
	}
	if _, err := os.Stat(filepath.Join(rawDir, ".uploads", session.UploadID)); !os.IsNotExist(err) {
		t.Fatalf("expected archive upload session directory to be removed, stat err=%v", err)
	}
}

func TestChunkedArchiveUploadRejectsNonZipFile(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil, Paths{RawDir: t.TempDir()})

	_, err := svc.InitiateChunkedArchiveUpload(context.Background(), InitiateChunkedUploadInput{
		FileName:    "lesson.mp4",
		FileSize:    10,
		ChunkSize:   5,
		TotalChunks: 2,
	})
	if err == nil {
		t.Fatal("expected zip validation error")
	}
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if err.Error() != "zip archive is required" {
		t.Fatalf("err = %q, want zip archive is required", err.Error())
	}
}

func buildChunkedUploadZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}
