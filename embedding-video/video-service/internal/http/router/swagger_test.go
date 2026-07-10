package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "nlp-video-analysis/docs/swagger"

	"nlp-video-analysis/internal/application/videoapp"
	domainvideo "nlp-video-analysis/internal/domain/video"
	appbuilder "nlp-video-analysis/internal/http/app"
	"nlp-video-analysis/internal/http/router"
)

type stubStatusStore struct{}

func (stubStatusStore) Set(context.Context, string, domainvideo.Status, string, time.Duration) error {
	return nil
}

func (stubStatusStore) Get(context.Context, string) (videoapp.TranscodeStatus, bool, error) {
	return videoapp.TranscodeStatus{}, false, nil
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSwaggerRouteRegistered(t *testing.T) {
	r := router.New(&appbuilder.App{})
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected swagger route available, got %d", w.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected swagger json payload: %v\nbody=%s", err, w.Body.String())
	}
	if payload["swagger"] != "2.0" {
		t.Fatalf("expected swagger version 2.0, got %#v", payload["swagger"])
	}
}

func TestSwaggerDocOmitsLegacyAliasPaths(t *testing.T) {
	r := router.New(&appbuilder.App{})
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected swagger route available, got %d", w.Code)
	}
	body := w.Body.String()
	legacyPaths := []string{
		"/api/question/list",
		"/api/question/{id}",
		"/api/video/upload",
		"/api/video/recommend_by_question",
		"/api/video/report_watch",
		"/api/video/list",
		"/api/video/{id}",
		"/api/video/play/{id}",
		"/api/video/similar/{id}",
		"/api/video/view_count/{id}",
		"/api/video/recommend/{id}",
		"/api/video/reaction/{id}",
		"/api/video/reaction_counts/{id}",
		"/api/video/cover/{id}",
		"/api/video/publish/{id}",
		"/api/video/status/{taskId}",
	}
	for _, path := range legacyPaths {
		if strings.Contains(body, path) {
			t.Fatalf("swagger doc unexpectedly included legacy alias path %q", path)
		}
	}
	newPaths := []string{
		"/api/recommendations/by-question",
		"/api/watch-records",
		"/api/videos",
		"/api/videos/archive",
		"/api/videos/archive/uploads",
		"/api/videos/archive/uploads/{uploadId}/complete",
		"/api/videos/uploads",
		"/api/videos/uploads/{uploadId}",
		"/api/videos/uploads/{uploadId}/chunks/{chunkIndex}",
		"/api/videos/uploads/{uploadId}/complete",
		"/api/video-segments/random-play",
		"/api/video-segments/{id}/reactions",
		"/api/video-segments/{id}/reaction-counts",
		"/api/videos/{id}/reactions",
		"/api/videos/{id}/reaction-counts",
	}
	for _, path := range newPaths {
		if !strings.Contains(body, path) {
			t.Fatalf("swagger doc missing new route %q", path)
		}
	}
}

func TestLegacyAliasRoutesAreRegistered(t *testing.T) {
	r := router.New(&appbuilder.App{Service: &videoapp.Service{StatusStore: stubStatusStore{}}})
	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{name: "list questions alias", method: http.MethodGet, path: "/api/question/list?page=0", want: http.StatusBadRequest},
		{name: "get question alias", method: http.MethodGet, path: "/api/question/0", want: http.StatusBadRequest},
		{name: "upload video alias", method: http.MethodPost, path: "/api/video/upload", want: http.StatusBadRequest},
		{name: "recommend by question alias", method: http.MethodPost, path: "/api/video/recommend_by_question", want: http.StatusBadRequest},
		{name: "report watch alias", method: http.MethodPost, path: "/api/video/report_watch", want: http.StatusBadRequest},
		{name: "list videos alias", method: http.MethodGet, path: "/api/video/list?type=BOGUS", want: http.StatusBadRequest},
		{name: "update video alias", method: http.MethodPut, path: "/api/video/0", want: http.StatusBadRequest},
		{name: "delete video alias", method: http.MethodDelete, path: "/api/video/0", want: http.StatusBadRequest},
		{name: "play video alias", method: http.MethodGet, path: "/api/video/play/0", want: http.StatusBadRequest},
		{name: "similar videos alias", method: http.MethodGet, path: "/api/video/similar/0", want: http.StatusBadRequest},
		{name: "view count alias", method: http.MethodGet, path: "/api/video/view_count/0", want: http.StatusBadRequest},
		{name: "recommend toggle alias", method: http.MethodPost, path: "/api/video/recommend/0", want: http.StatusBadRequest},
		{name: "reaction alias", method: http.MethodPost, path: "/api/video/reaction/0", want: http.StatusBadRequest},
		{name: "reaction counts alias", method: http.MethodGet, path: "/api/video/reaction_counts/0", want: http.StatusBadRequest},
		{name: "random segment alias", method: http.MethodGet, path: "/api/video-segment/random-play?user_id=bad", want: http.StatusBadRequest},
		{name: "cover upload alias", method: http.MethodPost, path: "/api/video/cover/0", want: http.StatusBadRequest},
		{name: "publish alias", method: http.MethodPost, path: "/api/video/publish/0", want: http.StatusBadRequest},
		{name: "transcode status alias", method: http.MethodGet, path: "/api/video/status/task-404", want: http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tc.want {
				t.Fatalf("expected alias route %s %s to reuse handler and return %d, got %d", tc.method, tc.path, tc.want, w.Code)
			}
		})
	}
}

func TestVideosProxyRouteRegistered(t *testing.T) {
	r := router.New(&appbuilder.App{Service: &videoapp.Service{StatusStore: stubStatusStore{}}})
	req := httptest.NewRequest(http.MethodGet, "/videos/hls/2026/04/29/demo/master.m3u8", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatalf("expected /videos/* route to be registered, got router 404")
	}
}

func TestAPIHealthzRouteRegistered(t *testing.T) {
	r := router.New(&appbuilder.App{})
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected /api/healthz to return %d, got %d", http.StatusOK, w.Code)
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected healthz json payload: %v\nbody=%s", err, w.Body.String())
	}
	if payload.Status != "ok" {
		t.Fatalf("expected healthz status ok, got %q", payload.Status)
	}
}

func TestRecommendationAdminRoutesAreRegistered(t *testing.T) {
	r := router.New(&appbuilder.App{Service: &videoapp.Service{}})

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{name: "overview", method: http.MethodGet, path: "/api/admin/recommendation/overview", want: http.StatusOK},
		{name: "diagnostics", method: http.MethodGet, path: "/api/admin/recommendation/diagnostics", want: http.StatusOK},
		{name: "datasources", method: http.MethodGet, path: "/api/admin/recommendation/datasources", want: http.StatusOK},
		{name: "effects", method: http.MethodGet, path: "/api/admin/recommendation/effects?days=bad", want: http.StatusBadRequest},
		{name: "random trace", method: http.MethodGet, path: "/api/admin/recommendation/trace/random-play?user_id=bad", want: http.StatusBadRequest},
		{name: "question trace", method: http.MethodPost, path: "/api/admin/recommendation/trace/by-question", body: `{"question_text":"   "}`, want: http.StatusBadRequest},
		{name: "redis state", method: http.MethodGet, path: "/api/admin/recommendation/redis-state?user_id=bad", want: http.StatusBadRequest},
		{name: "random preview", method: http.MethodGet, path: "/api/admin/recommendation/preview/random-play?user_id=bad", want: http.StatusBadRequest},
		{name: "question preview", method: http.MethodPost, path: "/api/admin/recommendation/preview/by-question", body: `{"question_text":"   "}`, want: http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tc.want {
				t.Fatalf("expected route %s %s to return %d, got %d: %s", tc.method, tc.path, tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestRouterAddsCORSHeadersToNormalRequests(t *testing.T) {
	r := router.New(&appbuilder.App{})
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("allow origin = %q, want *", got)
	}
}
