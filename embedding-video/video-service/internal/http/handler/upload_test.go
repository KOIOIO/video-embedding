package handler_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/http/handler"
)

type stubUploadApp struct {
	uploadVideoFunc        func(context.Context, videoapp.UploadVideoInput) (videoapp.UploadResult, error)
	uploadVideoArchiveFunc func(context.Context, videoapp.UploadVideoArchiveInput) (videoapp.ArchiveUploadResult, error)
	setCoverFunc           func(context.Context, uint64, videoapp.UploadCoverInput) (string, bool, error)
	initiateChunkedFunc    func(context.Context, videoapp.InitiateChunkedUploadInput) (videoapp.ChunkedUploadStatus, error)
	uploadChunkFunc        func(context.Context, videoapp.UploadVideoChunkInput) (videoapp.ChunkedUploadStatus, error)
	chunkedStatusFunc      func(context.Context, string) (videoapp.ChunkedUploadStatus, error)
	completeChunkedFunc    func(context.Context, videoapp.CompleteChunkedUploadInput) (videoapp.UploadResult, error)
	initiateArchiveFunc    func(context.Context, videoapp.InitiateChunkedUploadInput) (videoapp.ChunkedUploadStatus, error)
	completeArchiveFunc    func(context.Context, videoapp.CompleteChunkedUploadInput) (videoapp.ArchiveUploadResult, error)
	archiveProgressFunc    func(context.Context, string) (videoapp.ArchiveProcessingProgress, error)

	uploadVideoInput videoapp.UploadVideoInput
	archiveInput     videoapp.UploadVideoArchiveInput
	setCoverVideoID  uint64
	setCoverInput    videoapp.UploadCoverInput
	initiateInput    videoapp.InitiateChunkedUploadInput
	initiateArchive  videoapp.InitiateChunkedUploadInput
	chunkInput       videoapp.UploadVideoChunkInput
	statusUploadID   string
	completeInput    videoapp.CompleteChunkedUploadInput
	completeArchive  videoapp.CompleteChunkedUploadInput
	setCoverCalls    int
	uploadCalls      int
	archiveCalls     int
	initiateCalls    int
	initArchiveCalls int
	chunkCalls       int
	statusCalls      int
	completeCalls    int
	archiveDoneCalls int
	progressCalls    int
	progressBatchID  string
}

func (s *stubUploadApp) UploadVideo(ctx context.Context, input videoapp.UploadVideoInput) (videoapp.UploadResult, error) {
	s.uploadCalls++
	s.uploadVideoInput = input
	if s.uploadVideoFunc != nil {
		return s.uploadVideoFunc(ctx, input)
	}
	return videoapp.UploadResult{}, nil
}

func (s *stubUploadApp) UploadVideoArchive(ctx context.Context, input videoapp.UploadVideoArchiveInput) (videoapp.ArchiveUploadResult, error) {
	s.archiveCalls++
	s.archiveInput = input
	if s.uploadVideoArchiveFunc != nil {
		return s.uploadVideoArchiveFunc(ctx, input)
	}
	return videoapp.ArchiveUploadResult{}, nil
}

func (s *stubUploadApp) UploadVideoCover(ctx context.Context, videoID uint64, input videoapp.UploadCoverInput) (string, bool, error) {
	s.setCoverCalls++
	s.setCoverVideoID = videoID
	s.setCoverInput = input
	if s.setCoverFunc != nil {
		return s.setCoverFunc(ctx, videoID, input)
	}
	return "", false, nil
}

func (s *stubUploadApp) InitiateChunkedUpload(ctx context.Context, input videoapp.InitiateChunkedUploadInput) (videoapp.ChunkedUploadStatus, error) {
	s.initiateCalls++
	s.initiateInput = input
	if s.initiateChunkedFunc != nil {
		return s.initiateChunkedFunc(ctx, input)
	}
	return videoapp.ChunkedUploadStatus{}, nil
}

func (s *stubUploadApp) UploadVideoChunk(ctx context.Context, input videoapp.UploadVideoChunkInput) (videoapp.ChunkedUploadStatus, error) {
	s.chunkCalls++
	s.chunkInput = input
	if s.uploadChunkFunc != nil {
		return s.uploadChunkFunc(ctx, input)
	}
	return videoapp.ChunkedUploadStatus{}, nil
}

func (s *stubUploadApp) GetChunkedUploadStatus(ctx context.Context, uploadID string) (videoapp.ChunkedUploadStatus, error) {
	s.statusCalls++
	s.statusUploadID = uploadID
	if s.chunkedStatusFunc != nil {
		return s.chunkedStatusFunc(ctx, uploadID)
	}
	return videoapp.ChunkedUploadStatus{}, nil
}

func (s *stubUploadApp) CompleteChunkedUpload(ctx context.Context, input videoapp.CompleteChunkedUploadInput) (videoapp.UploadResult, error) {
	s.completeCalls++
	s.completeInput = input
	if s.completeChunkedFunc != nil {
		return s.completeChunkedFunc(ctx, input)
	}
	return videoapp.UploadResult{}, nil
}

func (s *stubUploadApp) InitiateChunkedArchiveUpload(ctx context.Context, input videoapp.InitiateChunkedUploadInput) (videoapp.ChunkedUploadStatus, error) {
	s.initArchiveCalls++
	s.initiateArchive = input
	if s.initiateArchiveFunc != nil {
		return s.initiateArchiveFunc(ctx, input)
	}
	return videoapp.ChunkedUploadStatus{}, nil
}

func (s *stubUploadApp) CompleteChunkedArchiveUpload(ctx context.Context, input videoapp.CompleteChunkedUploadInput) (videoapp.ArchiveUploadResult, error) {
	s.archiveDoneCalls++
	s.completeArchive = input
	if s.completeArchiveFunc != nil {
		return s.completeArchiveFunc(ctx, input)
	}
	return videoapp.ArchiveUploadResult{}, nil
}

func (s *stubUploadApp) GetArchiveProcessingProgress(ctx context.Context, batchID string) (videoapp.ArchiveProcessingProgress, error) {
	s.progressCalls++
	s.progressBatchID = batchID
	if s.archiveProgressFunc != nil {
		return s.archiveProgressFunc(ctx, batchID)
	}
	return videoapp.ArchiveProcessingProgress{}, nil
}

func TestUploadVideo_RequiresFile(t *testing.T) {
	h := handler.NewUploadHandler(&stubUploadApp{})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("title", "Sample"); err != nil {
		t.Fatalf("write title: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router := gin.New()
	router.POST("/api/videos", h.UploadVideo)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"file is required"`)
}

func TestUploadVideo_Success(t *testing.T) {
	stub := &stubUploadApp{
		uploadVideoFunc: func(_ context.Context, input videoapp.UploadVideoInput) (videoapp.UploadResult, error) {
			if input.FileName != "demo.mp4" {
				t.Fatalf("unexpected file name: %q", input.FileName)
			}
			if input.ContentType != "video/mp4" {
				t.Fatalf("unexpected content type: %q", input.ContentType)
			}
			if input.Title != "Sample" || input.Description != "Desc" {
				t.Fatalf("unexpected metadata: title=%q description=%q", input.Title, input.Description)
			}
			if input.UserID != 42 {
				t.Fatalf("unexpected user id: %d", input.UserID)
			}
			payload, err := io.ReadAll(input.Reader)
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			if string(payload) != "video-bytes" {
				t.Fatalf("unexpected payload: %q", string(payload))
			}
			return videoapp.UploadResult{VideoID: 9, TaskID: "task-9", RawURL: "/videos/raw/demo.mp4", HLSURL: "/videos/hls/demo/master.m3u8", Name: "stored.mp4"}, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("title", "Sample"); err != nil {
		t.Fatalf("write title: %v", err)
	}
	if err := writer.WriteField("description", "Desc"); err != nil {
		t.Fatalf("write description: %v", err)
	}
	if err := writer.WriteField("user_id", "42"); err != nil {
		t.Fatalf("write user_id: %v", err)
	}
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="demo.mp4"`)
	partHeader.Set("Content-Type", "video/mp4")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("video-bytes")); err != nil {
		t.Fatalf("write file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router := gin.New()
	router.POST("/api/videos", h.UploadVideo)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.uploadCalls != 1 {
		t.Fatalf("expected one upload call, got %d", stub.uploadCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"success":true`)
	assertBodyContains(t, w.Body.Bytes(), `"video_id":9`)
	assertBodyContains(t, w.Body.Bytes(), `"task_id":"task-9"`)
	assertBodyContains(t, w.Body.Bytes(), `"raw_url":"/videos/raw/demo.mp4"`)
	assertBodyContains(t, w.Body.Bytes(), `"hls_url":"/videos/hls/demo/master.m3u8"`)
}

func TestUploadVideoArchive_Success(t *testing.T) {
	stub := &stubUploadApp{
		uploadVideoArchiveFunc: func(_ context.Context, input videoapp.UploadVideoArchiveInput) (videoapp.ArchiveUploadResult, error) {
			if input.FileName != "lessons.zip" {
				t.Fatalf("unexpected file name: %q", input.FileName)
			}
			if input.ContentType != "application/zip" {
				t.Fatalf("unexpected content type: %q", input.ContentType)
			}
			if input.Description != "Batch" {
				t.Fatalf("unexpected description: %q", input.Description)
			}
			if input.UserID != 42 {
				t.Fatalf("unexpected user id: %d", input.UserID)
			}
			payload, err := io.ReadAll(input.Reader)
			if err != nil {
				t.Fatalf("read archive input: %v", err)
			}
			zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
			if err != nil {
				t.Fatalf("archive payload was not forwarded as zip: %v", err)
			}
			if len(zr.File) != 3 {
				t.Fatalf("zip entries = %d, want 3", len(zr.File))
			}
			return videoapp.ArchiveUploadResult{
				Total: 3,
				Uploaded: []videoapp.UploadResult{
					{VideoID: 10, TaskID: "10", RawURL: "/videos/raw/a.mp4", HLSURL: "/videos/hls/a/master.m3u8", Name: "a.mp4"},
					{VideoID: 11, TaskID: "11", RawURL: "/videos/raw/b.mov", HLSURL: "/videos/hls/b/master.m3u8", Name: "b.mov"},
				},
				Skipped: []string{"notes.txt"},
			}, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	archive := &bytes.Buffer{}
	zw := zip.NewWriter(archive)
	for name, content := range map[string]string{
		"a.mp4":     "lesson-a",
		"b.mov":     "lesson-b",
		"notes.txt": "notes",
	} {
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

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("description", "Batch"); err != nil {
		t.Fatalf("write description: %v", err)
	}
	if err := writer.WriteField("user_id", "42"); err != nil {
		t.Fatalf("write user_id: %v", err)
	}
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="lessons.zip"`)
	partHeader.Set("Content-Type", "application/zip")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(archive.Bytes()); err != nil {
		t.Fatalf("write archive content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/archive", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router := gin.New()
	router.POST("/api/videos/archive", h.UploadVideoArchive)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.archiveCalls != 1 {
		t.Fatalf("expected one archive upload call, got %d", stub.archiveCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"success":true`)
	assertBodyContains(t, w.Body.Bytes(), `"total":3`)
	assertBodyContains(t, w.Body.Bytes(), `"uploaded":2`)
	assertBodyContains(t, w.Body.Bytes(), `"skipped":1`)
	assertBodyContains(t, w.Body.Bytes(), `"video_id":10`)
	assertBodyContains(t, w.Body.Bytes(), `"file_name":"a.mp4"`)
	assertBodyContains(t, w.Body.Bytes(), `"skipped_files":["notes.txt"]`)
}

func TestUploadVideoCover_Success(t *testing.T) {
	stub := &stubUploadApp{
		setCoverFunc: func(_ context.Context, videoID uint64, input videoapp.UploadCoverInput) (string, bool, error) {
			if videoID != 22 {
				t.Fatalf("unexpected videoID: %d", videoID)
			}
			if input.FileName != "cover.jpg" {
				t.Fatalf("unexpected file name: %q", input.FileName)
			}
			if input.ContentType != "image/jpeg" {
				t.Fatalf("unexpected content type: %q", input.ContentType)
			}
			payload, err := io.ReadAll(input.Reader)
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			if string(payload) != "cover-bytes" {
				t.Fatalf("unexpected payload: %q", string(payload))
			}
			return "/videos/cover/2026/04/28/cover.jpg", true, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="cover.jpg"`)
	partHeader.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("cover-bytes")); err != nil {
		t.Fatalf("write cover content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/22/cover", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router := gin.New()
	router.POST("/api/videos/:id/cover", h.UploadVideoCover)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.setCoverCalls != 1 {
		t.Fatalf("expected one cover call, got %d", stub.setCoverCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"success":true`)
	assertBodyContains(t, w.Body.Bytes(), `"video_id":22`)
	assertBodyContains(t, w.Body.Bytes(), `"cover_url":"/videos/cover/2026/04/28/cover.jpg"`)
}

func TestUploadVideoCover_NotFound(t *testing.T) {
	stub := &stubUploadApp{
		setCoverFunc: func(_ context.Context, videoID uint64, input videoapp.UploadCoverInput) (string, bool, error) {
			return "", false, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="cover.jpg"`)
	partHeader.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("cover-bytes")); err != nil {
		t.Fatalf("write cover content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/404/cover", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router := gin.New()
	router.POST("/api/videos/:id/cover", h.UploadVideoCover)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"video_not_found"`)
}

func TestInitiateChunkedUpload_Success(t *testing.T) {
	stub := &stubUploadApp{
		initiateChunkedFunc: func(_ context.Context, input videoapp.InitiateChunkedUploadInput) (videoapp.ChunkedUploadStatus, error) {
			if input.FileName != "big.mp4" || input.ContentType != "video/mp4" {
				t.Fatalf("unexpected file metadata: %+v", input)
			}
			if input.Title != "Big Lesson" || input.Description != "Desc" {
				t.Fatalf("unexpected metadata: %+v", input)
			}
			if input.UserID != 42 {
				t.Fatalf("unexpected user id: %d", input.UserID)
			}
			if input.FileSize != 10 || input.ChunkSize != 5 || input.TotalChunks != 2 {
				t.Fatalf("unexpected chunk config: %+v", input)
			}
			return videoapp.ChunkedUploadStatus{
				UploadID:       "upload-1",
				FileName:       "big.mp4",
				FileSize:       10,
				ChunkSize:      5,
				TotalChunks:    2,
				UploadedChunks: []int{},
				Completed:      false,
			}, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	body := bytes.NewBufferString(`{"file_name":"big.mp4","content_type":"video/mp4","title":"Big Lesson","description":"Desc","user_id":42,"file_size":10,"chunk_size":5,"total_chunks":2}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/uploads", body)
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/uploads", h.InitiateChunkedUpload)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.initiateCalls != 1 {
		t.Fatalf("expected one initiate call, got %d", stub.initiateCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"upload_id":"upload-1"`)
	assertBodyContains(t, w.Body.Bytes(), `"file_name":"big.mp4"`)
	assertBodyContains(t, w.Body.Bytes(), `"uploaded_chunks":[]`)
}

func TestUploadVideoChunk_Success(t *testing.T) {
	stub := &stubUploadApp{
		uploadChunkFunc: func(_ context.Context, input videoapp.UploadVideoChunkInput) (videoapp.ChunkedUploadStatus, error) {
			if input.UploadID != "upload-1" || input.ChunkIndex != 3 {
				t.Fatalf("unexpected chunk input: %+v", input)
			}
			payload, err := io.ReadAll(input.Reader)
			if err != nil {
				t.Fatalf("read chunk: %v", err)
			}
			if string(payload) != "chunk-bytes" {
				t.Fatalf("unexpected payload: %q", string(payload))
			}
			return videoapp.ChunkedUploadStatus{
				UploadID:       "upload-1",
				FileName:       "big.mp4",
				FileSize:       20,
				ChunkSize:      5,
				TotalChunks:    4,
				UploadedChunks: []int{0, 3},
				Completed:      false,
			}, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/videos/uploads/upload-1/chunks/3", bytes.NewBufferString("chunk-bytes"))
	router := gin.New()
	router.PUT("/api/videos/uploads/:uploadId/chunks/:chunkIndex", h.UploadVideoChunk)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.chunkCalls != 1 {
		t.Fatalf("expected one chunk call, got %d", stub.chunkCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"uploaded_chunks":[0,3]`)
	assertBodyContains(t, w.Body.Bytes(), `"completed":false`)
}

func TestGetChunkedUploadStatus_Success(t *testing.T) {
	stub := &stubUploadApp{
		chunkedStatusFunc: func(_ context.Context, uploadID string) (videoapp.ChunkedUploadStatus, error) {
			if uploadID != "upload-1" {
				t.Fatalf("unexpected upload id: %q", uploadID)
			}
			return videoapp.ChunkedUploadStatus{
				UploadID:       "upload-1",
				FileName:       "big.mp4",
				FileSize:       10,
				ChunkSize:      5,
				TotalChunks:    2,
				UploadedChunks: []int{0},
				Completed:      false,
			}, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos/uploads/upload-1", nil)
	router := gin.New()
	router.GET("/api/videos/uploads/:uploadId", h.GetChunkedUploadStatus)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.statusCalls != 1 {
		t.Fatalf("expected one status call, got %d", stub.statusCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"uploaded_chunks":[0]`)
}

func TestCompleteChunkedUpload_Success(t *testing.T) {
	stub := &stubUploadApp{
		completeChunkedFunc: func(_ context.Context, input videoapp.CompleteChunkedUploadInput) (videoapp.UploadResult, error) {
			if input.UploadID != "upload-1" {
				t.Fatalf("unexpected complete input: %+v", input)
			}
			return videoapp.UploadResult{
				VideoID: 12,
				TaskID:  "12",
				RawURL:  "/videos/raw/big.mp4",
				HLSURL:  "/videos/hls/big/master.m3u8",
				Name:    "big.mp4",
			}, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/uploads/upload-1/complete", nil)
	router := gin.New()
	router.POST("/api/videos/uploads/:uploadId/complete", h.CompleteChunkedUpload)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.completeCalls != 1 {
		t.Fatalf("expected one complete call, got %d", stub.completeCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":12`)
	assertBodyContains(t, w.Body.Bytes(), `"task_id":"12"`)
}

func TestInitiateChunkedArchiveUpload_Success(t *testing.T) {
	stub := &stubUploadApp{
		initiateArchiveFunc: func(_ context.Context, input videoapp.InitiateChunkedUploadInput) (videoapp.ChunkedUploadStatus, error) {
			if input.FileName != "lessons.zip" || input.ContentType != "application/zip" {
				t.Fatalf("unexpected archive metadata: %+v", input)
			}
			if input.Description != "Batch" {
				t.Fatalf("unexpected description: %q", input.Description)
			}
			if input.UserID != 42 {
				t.Fatalf("unexpected user id: %d", input.UserID)
			}
			if input.FileSize != 20 || input.ChunkSize != 5 || input.TotalChunks != 4 {
				t.Fatalf("unexpected chunk config: %+v", input)
			}
			return videoapp.ChunkedUploadStatus{
				UploadID:       "archive-1",
				FileName:       "lessons.zip",
				FileSize:       20,
				ChunkSize:      5,
				TotalChunks:    4,
				UploadedChunks: []int{},
				Completed:      false,
			}, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	body := bytes.NewBufferString(`{"file_name":"lessons.zip","content_type":"application/zip","description":"Batch","user_id":42,"file_size":20,"chunk_size":5,"total_chunks":4}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/archive/uploads", body)
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/archive/uploads", h.InitiateChunkedArchiveUpload)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.initArchiveCalls != 1 {
		t.Fatalf("expected one archive initiate call, got %d", stub.initArchiveCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"upload_id":"archive-1"`)
	assertBodyContains(t, w.Body.Bytes(), `"file_name":"lessons.zip"`)
}

func TestCompleteChunkedArchiveUpload_Success(t *testing.T) {
	stub := &stubUploadApp{
		completeArchiveFunc: func(_ context.Context, input videoapp.CompleteChunkedUploadInput) (videoapp.ArchiveUploadResult, error) {
			if input.UploadID != "archive-1" {
				t.Fatalf("unexpected complete archive input: %+v", input)
			}
			return videoapp.ArchiveUploadResult{
				BatchID: "batch-1",
				Total:   2,
				Uploaded: []videoapp.UploadResult{
					{VideoID: 31, TaskID: "31", RawURL: "/videos/raw/a.mp4", HLSURL: "/videos/hls/a/master.m3u8", Name: "a.mp4"},
				},
				Skipped: []string{"notes.txt"},
			}, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/archive/uploads/archive-1/complete", nil)
	router := gin.New()
	router.POST("/api/videos/archive/uploads/:uploadId/complete", h.CompleteChunkedArchiveUpload)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.archiveDoneCalls != 1 {
		t.Fatalf("expected one archive complete call, got %d", stub.archiveDoneCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"total":2`)
	assertBodyContains(t, w.Body.Bytes(), `"batch_id":"batch-1"`)
	assertBodyContains(t, w.Body.Bytes(), `"uploaded":1`)
	assertBodyContains(t, w.Body.Bytes(), `"skipped_files":["notes.txt"]`)
	assertBodyContains(t, w.Body.Bytes(), `"video_id":31`)
}

func TestGetArchiveProcessingProgress_Success(t *testing.T) {
	stub := &stubUploadApp{
		archiveProgressFunc: func(_ context.Context, batchID string) (videoapp.ArchiveProcessingProgress, error) {
			if batchID != "batch-1" {
				t.Fatalf("unexpected batch id: %s", batchID)
			}
			return videoapp.ArchiveProcessingProgress{Total: 237, Transcoded: 12, Vectorized: 8}, nil
		},
	}
	h := handler.NewUploadHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos/archive/batches/batch-1/progress", nil)
	router := gin.New()
	router.GET("/api/videos/archive/batches/:batchId/progress", h.GetArchiveProcessingProgress)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.progressCalls != 1 || stub.progressBatchID != "batch-1" {
		t.Fatalf("unexpected progress call: calls=%d batch=%q", stub.progressCalls, stub.progressBatchID)
	}
	assertBodyContains(t, w.Body.Bytes(), `"total":237`)
	assertBodyContains(t, w.Body.Bytes(), `"transcoded":12`)
	assertBodyContains(t, w.Body.Bytes(), `"vectorized":8`)
}
