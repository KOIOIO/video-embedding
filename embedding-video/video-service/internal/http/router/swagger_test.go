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
	"github.com/golang-jwt/jwt/v5"
	_ "video-service/docs/swagger"

	"video-service/internal/application/adminauth"
	"video-service/internal/application/videoapp"
	domainvideo "video-service/internal/domain/video"
	appbuilder "video-service/internal/http/app"
	"video-service/internal/http/router"
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
	r, token := authenticatedTestRouter(t, &appbuilder.App{})
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	req.Header.Set("Authorization", "Bearer "+token)
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
	r, token := authenticatedTestRouter(t, &appbuilder.App{})
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	req.Header.Set("Authorization", "Bearer "+token)
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
		"/api/knowledge-videos/tree",
		"/api/admin/knowledge-videos/batches",
		"/api/admin/knowledge-videos/batches/{batchId}",
		"/api/knowledge-points/{knowledgePointId}/video",
		"/api/knowledge-points/{knowledgePointId}/videos",
		"/api/knowledge-videos/{knowledgeVideoId}/playbacks",
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

func TestSwaggerDocGroupsAllStandardRoutes(t *testing.T) {
	r, token := authenticatedTestRouter(t, &appbuilder.App{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("swagger status = %d", w.Code)
	}
	var document struct {
		Paths map[string]map[string]struct {
			Tags []string `json:"tags"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"GET /api/healthz": "系统与健康", "GET /api/system/metrics": "系统与健康",
		"GET /api/videos": "视频资源", "PATCH /api/videos/{id}": "视频资源", "DELETE /api/videos/{id}": "视频资源",
		"POST /api/videos/{id}/publish": "视频资源", "POST /api/videos/{id}/recommend": "视频资源", "GET /api/videos/{id}/similar": "视频资源", "GET /api/videos/{id}/view-count": "视频资源",
		"POST /api/videos": "视频上传", "POST /api/videos/archive": "视频上传", "POST /api/videos/{id}/cover": "视频上传",
		"POST /api/videos/uploads": "视频上传", "GET /api/videos/uploads/{uploadId}": "视频上传", "PUT /api/videos/uploads/{uploadId}/chunks/{chunkIndex}": "视频上传", "POST /api/videos/uploads/{uploadId}/complete": "视频上传",
		"POST /api/videos/archive/uploads": "视频上传", "POST /api/videos/archive/uploads/{uploadId}/complete": "视频上传", "GET /api/videos/archive/batches/{batchId}/progress": "视频上传",
		"GET /api/videos/{id}/play": "视频播放与转码", "GET /api/transcode-tasks/{taskId}": "视频播放与转码",
		"POST /api/videos/{id}/reactions": "视频互动", "GET /api/videos/{id}/reaction-counts": "视频互动", "POST /api/watch-records": "视频互动",
		"GET /api/video-segments/random-play": "视频片段", "POST /api/video-segments/{id}/reactions": "视频片段", "GET /api/video-segments/{id}/reaction-counts": "视频片段",
		"GET /api/video-segments/{id}/comments": "视频评论", "GET /api/video-segments/{id}/comment-counts": "视频评论", "POST /api/video-segments/{id}/comments": "视频评论",
		"GET /api/comments/{id}/replies": "视频评论", "POST /api/comments/{id}/replies": "视频评论", "POST /api/comments/{id}/reactions": "视频评论",
		"GET /api/questions": "题目", "GET /api/questions/{id}": "题目",
		"GET /api/recommendations": "推荐", "POST /api/recommendations/by-question": "推荐",
		"GET /api/admin/recommendation/overview": "推荐管理", "GET /api/admin/recommendation/diagnostics": "推荐管理", "GET /api/admin/recommendation/datasources": "推荐管理", "GET /api/admin/recommendation/effects": "推荐管理", "GET /api/admin/recommendation/recbole/performance": "推荐管理",
		"GET /api/admin/recommendation/trace/random-play": "推荐管理", "POST /api/admin/recommendation/trace/by-question": "推荐管理", "GET /api/admin/recommendation/redis-state": "推荐管理", "GET /api/admin/recommendation/preview/random-play": "推荐管理", "POST /api/admin/recommendation/preview/by-question": "推荐管理",
		"GET /api/knowledge-videos/tree": "知识点视频", "POST /api/admin/knowledge-videos/batches": "知识点视频", "GET /api/admin/knowledge-videos/batches/{batchId}": "知识点视频", "GET /api/knowledge-points/{knowledgePointId}/video": "知识点视频", "GET /api/knowledge-points/{knowledgePointId}/videos": "知识点视频", "POST /api/knowledge-videos/{knowledgeVideoId}/playbacks": "知识点视频",
		"GET /api/internal/recommendations/external/recbole":  "内部接口",
		"GET /knowledge-video-media/hls/{videoId}/{filepath}": "媒体访问", "GET /videos/{filepath}": "媒体访问",
	}
	for operation, wantTag := range want {
		parts := strings.SplitN(operation, " ", 2)
		path, method := parts[1], strings.ToLower(parts[0])
		got, ok := document.Paths[path][method]
		if !ok {
			t.Errorf("swagger missing %s", operation)
			continue
		}
		if len(got.Tags) != 1 || got.Tags[0] != wantTag {
			t.Errorf("%s tags = %v, want [%s]", operation, got.Tags, wantTag)
		}
	}
}

func TestSwaggerDocumentsAdministratorSecurityBoundary(t *testing.T) {
	r, token := authenticatedTestRouter(t, &appbuilder.App{})
	req := httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("swagger status=%d", w.Code)
	}
	var document struct {
		SecurityDefinitions map[string]any `json:"securityDefinitions"`
		Paths               map[string]map[string]struct {
			Security []map[string][]string `json:"security"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if _, ok := document.SecurityDefinitions["BearerAuth"]; !ok {
		t.Fatal("swagger missing BearerAuth security definition")
	}
	if _, ok := document.Paths["/api/auth/login"]["post"]; !ok {
		t.Fatal("swagger missing administrator login")
	}
	if len(document.Paths["/api/videos"]["post"].Security) == 0 {
		t.Fatal("video upload is not marked as protected")
	}
	if len(document.Paths["/api/watch-records"]["post"].Security) != 0 {
		t.Fatal("watch records must remain public")
	}
}

func TestLegacyAliasRoutesAreRegistered(t *testing.T) {
	r, token := authenticatedTestRouter(t, &appbuilder.App{Service: &videoapp.Service{StatusStore: stubStatusStore{}}})
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
			req.Header.Set("Authorization", "Bearer "+token)
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
	r, token := authenticatedTestRouter(t, &appbuilder.App{Service: &videoapp.Service{}})

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
		{name: "recbole performance", method: http.MethodGet, path: "/api/admin/recommendation/recbole/performance?metric=NDCG%4020&begin=bad&end=2026-07-16T00:00:00Z", want: http.StatusBadRequest},
		{name: "random trace", method: http.MethodGet, path: "/api/admin/recommendation/trace/random-play?user_id=bad", want: http.StatusBadRequest},
		{name: "question trace", method: http.MethodPost, path: "/api/admin/recommendation/trace/by-question", body: `{"question_text":"   "}`, want: http.StatusBadRequest},
		{name: "redis state", method: http.MethodGet, path: "/api/admin/recommendation/redis-state?user_id=bad", want: http.StatusBadRequest},
		{name: "random preview", method: http.MethodGet, path: "/api/admin/recommendation/preview/random-play?user_id=bad", want: http.StatusBadRequest},
		{name: "question preview", method: http.MethodPost, path: "/api/admin/recommendation/preview/by-question", body: `{"question_text":"   "}`, want: http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+token)
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

type routerAuthRepository struct{}

func (routerAuthRepository) FindActiveAdminByUsername(context.Context, string) (adminauth.Admin, bool, error) {
	return adminauth.Admin{ID: 7, Username: "admin"}, true, nil
}

func (routerAuthRepository) FindActiveAdminByID(_ context.Context, id uint64) (adminauth.Admin, bool, error) {
	return adminauth.Admin{ID: id, Username: "admin"}, id == 7, nil
}

func authenticatedTestRouter(t *testing.T, app *appbuilder.App) (*gin.Engine, string) {
	t.Helper()
	const secret = "01234567890123456789012345678901"
	app.AdminAuth = adminauth.NewService(routerAuthRepository{}, secret, time.Hour)
	claims := jwt.RegisteredClaims{
		Subject:   "7",
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return router.New(app), token
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
