package knowledgevideos

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/adminauth"
	"video-service/internal/application/knowledgevideo"
	"video-service/middleware"
)

func TestKnowledgeVideoImportUsesAuthenticatedAdminID(t *testing.T) {
	stub := &handlerServiceStub{}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	archive, _ := writer.CreateFormFile("archive", "videos.zip")
	_, _ = archive.Write([]byte("zip"))
	mapping, _ := writer.CreateFormFile("mapping", "mapping.xlsx")
	_, _ = mapping.Write([]byte("xlsx"))
	_ = writer.WriteField("upload_user_id", "999")
	_ = writer.Close()

	r := gin.New()
	h := New(stub, 1024)
	r.POST("/api/admin/knowledge-videos/batches", func(c *gin.Context) {
		c.Set(middleware.AdminContextKey, adminauth.Admin{ID: 7, Username: "admin"})
		c.Next()
	}, h.Import)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/knowledge-videos/batches", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted || stub.importInput.UploadUserID != 7 {
		t.Fatalf("status=%d body=%s upload_user_id=%d", w.Code, w.Body.String(), stub.importInput.UploadUserID)
	}
}

func TestKnowledgeVideoPlaybackListsVideosOnSingularAndPluralRoutes(t *testing.T) {
	stub := &handlerServiceStub{resolution: knowledgevideo.PlaybackResolution{
		KnowledgePointID: 9, KnowledgePointName: "一次函数",
		Videos: []knowledgevideo.PlaybackVideo{
			{KnowledgeVideoID: 41, SourceFileName: "first.mp4", DisplayName: "first", Duration: 42, PlaybackURL: "/knowledge-video-media/hls/41/master.m3u8"},
			{KnowledgeVideoID: 43, SourceFileName: "second.mp4.mp4", DisplayName: "second.mp4", Duration: 84, PlaybackURL: "/knowledge-video-media/hls/43/master.m3u8"},
		},
	}}
	for _, path := range []string{"/api/knowledge-points/9/video?user_id=7", "/api/knowledge-points/9/videos"} {
		w := httptest.NewRecorder()
		testRouter(stub).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		body := w.Body.String()
		if w.Code != http.StatusOK || !contains(body, `"videos":[{"knowledge_video_id":41`) || !contains(body, `"display_name":"second.mp4"`) {
			t.Fatalf("path=%s status=%d body=%s", path, w.Code, body)
		}
		if !contains(body, `"video_id":41`) || !contains(body, `"playback_url":"/knowledge-video-media/hls/41/master.m3u8"`) {
			t.Fatalf("compatibility fields missing: %s", body)
		}
	}
	if stub.listKnowledgePointID != 9 {
		t.Fatalf("listed knowledge point ID = %d", stub.listKnowledgePointID)
	}
}

func TestKnowledgeVideoPlaybackReturnsEmptyVideosWithoutLegacyFields(t *testing.T) {
	stub := &handlerServiceStub{resolution: knowledgevideo.PlaybackResolution{KnowledgePointID: 9, KnowledgePointName: "一次函数", Videos: []knowledgevideo.PlaybackVideo{}}}
	w := httptest.NewRecorder()
	testRouter(stub).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/knowledge-points/9/videos", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK || !contains(body, `"videos":[]`) || contains(body, `"video_id"`) || contains(body, `"playback_url"`) {
		t.Fatalf("status=%d body=%s", w.Code, body)
	}
}

func TestKnowledgeVideoRecordPlaybackUsesVideoIDAndUserBody(t *testing.T) {
	stub := &handlerServiceStub{}
	w := httptest.NewRecorder()
	testRouter(stub).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/knowledge-videos/88/playbacks", strings.NewReader(`{"user_id":7}`)))
	if w.Code != http.StatusOK || stub.recordUserID != 7 || stub.recordVideoID != 88 {
		t.Fatalf("status=%d body=%s recorded=%d/%d", w.Code, w.Body.String(), stub.recordUserID, stub.recordVideoID)
	}
}

func TestKnowledgeVideoRecordPlaybackMapsInvalidAndNotReady(t *testing.T) {
	tests := []struct {
		name string
		body string
		err  error
		want int
	}{
		{name: "invalid body", body: `{}`, want: http.StatusBadRequest},
		{name: "not ready", body: `{"user_id":7}`, err: &knowledgevideo.NotReady{Status: knowledgevideo.VideoPending}, want: http.StatusConflict},
		{name: "missing", body: `{"user_id":7}`, err: &knowledgevideo.NotFound{Resource: "knowledge video"}, want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &handlerServiceStub{recordErr: tt.err}
			w := httptest.NewRecorder()
			testRouter(stub).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/knowledge-videos/88/playbacks", strings.NewReader(tt.body)))
			if w.Code != tt.want {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestKnowledgeVideoWatchSessionReturnsServerAggregate(t *testing.T) {
	stub := &handlerServiceStub{watchResult: knowledgevideo.WatchSessionResult{
		SessionID: "session-00000001", SessionWatchedSeconds: 40, TotalWatchedSeconds: 60,
		DurationSeconds: 100, ProgressRatio: 0.6, EffectiveWatch: true,
	}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/knowledge-videos/88/watch-sessions/session-00000001", strings.NewReader(`{"user_id":7,"watched_seconds":40}`))
	req.Header.Set("Content-Type", "application/json")
	testRouter(stub).ServeHTTP(w, req)
	body := w.Body.String()
	if w.Code != http.StatusOK || !contains(body, `"session_watched_seconds":40`) || !contains(body, `"total_watched_seconds":60`) || !contains(body, `"progress_ratio":0.6`) || !contains(body, `"effective_watch":true`) {
		t.Fatalf("status=%d body=%s", w.Code, body)
	}
	if stub.watchInput.UserID != 7 || stub.watchInput.KnowledgeVideoID != 88 || stub.watchInput.SessionID != "session-00000001" || stub.watchInput.WatchedSeconds != 40 {
		t.Fatalf("input=%+v", stub.watchInput)
	}
}

func TestKnowledgeVideoTreeReturnsAllVideosAndCompatibilityVideo(t *testing.T) {
	videos := []knowledgevideo.KnowledgeTreeVideo{{ID: 88, Status: knowledgevideo.VideoReady}, {ID: 89, Status: knowledgevideo.VideoPending}}
	stub := &handlerServiceStub{tree: []knowledgevideo.KnowledgeTreeNode{{ID: 1, Name: "函数", Children: []knowledgevideo.KnowledgeTreeNode{{ID: 9, ParentID: 1, Name: "一次函数", Videos: videos, Video: &videos[0]}}}}}
	w := httptest.NewRecorder()
	testRouter(stub).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/knowledge-videos/tree", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK || !contains(body, `"videos":[{"id":88`) || !contains(body, `{"id":89`) || !contains(body, `"video":{"id":88`) {
		t.Fatalf("status=%d body=%s", w.Code, body)
	}
}

func testRouter(service Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := New(service, 1024)
	r.GET("/api/knowledge-videos/tree", h.Tree)
	r.GET("/api/knowledge-points/:knowledgePointId/video", h.Playback)
	r.GET("/api/knowledge-points/:knowledgePointId/videos", h.Playback)
	r.POST("/api/knowledge-videos/:knowledgeVideoId/playbacks", h.RecordPlayback)
	r.PUT("/api/knowledge-videos/:knowledgeVideoId/watch-sessions/:sessionId", h.ReportWatchSession)
	return r
}

type handlerServiceStub struct {
	importInput          knowledgevideo.ImportInput
	resolution           knowledgevideo.PlaybackResolution
	resolveErr           error
	listKnowledgePointID uint64
	recordUserID         uint64
	recordVideoID        uint64
	recordErr            error
	tree                 []knowledgevideo.KnowledgeTreeNode
	watchInput           knowledgevideo.WatchSessionInput
	watchResult          knowledgevideo.WatchSessionResult
	watchErr             error
}

func (s *handlerServiceStub) Import(_ context.Context, input knowledgevideo.ImportInput) (knowledgevideo.ImportResult, error) {
	s.importInput = input
	return knowledgevideo.ImportResult{BatchID: 1, TotalCount: 1, Status: knowledgevideo.BatchProcessing}, nil
}
func (*handlerServiceStub) GetBatch(context.Context, uint64) (knowledgevideo.Batch, []knowledgevideo.Video, bool, error) {
	return knowledgevideo.Batch{}, nil, false, errors.New("unused")
}
func (s *handlerServiceStub) ListPlayback(_ context.Context, knowledgePointID uint64) (knowledgevideo.PlaybackResolution, error) {
	s.listKnowledgePointID = knowledgePointID
	return s.resolution, s.resolveErr
}
func (s *handlerServiceStub) RecordPlayback(_ context.Context, userID, knowledgeVideoID uint64) error {
	s.recordUserID, s.recordVideoID = userID, knowledgeVideoID
	return s.recordErr
}
func (s *handlerServiceStub) ListTree(context.Context) ([]knowledgevideo.KnowledgeTreeNode, error) {
	return s.tree, nil
}

func (s *handlerServiceStub) ReportWatchSession(_ context.Context, input knowledgevideo.WatchSessionInput) (knowledgevideo.WatchSessionResult, error) {
	s.watchInput = input
	return s.watchResult, s.watchErr
}

func contains(value, part string) bool { return strings.Contains(value, part) }
