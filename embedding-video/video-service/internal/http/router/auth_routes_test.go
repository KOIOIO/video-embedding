package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appbuilder "video-service/internal/http/app"
	"video-service/internal/http/router"
)

func TestAdminRoutesRequireAuthentication(t *testing.T) {
	r := router.New(&appbuilder.App{})
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/system/metrics"},
		{http.MethodPost, "/api/videos"},
		{http.MethodPost, "/api/video/upload"},
		{http.MethodPost, "/api/videos/uploads"},
		{http.MethodPatch, "/api/videos/1"},
		{http.MethodPut, "/api/video/1"},
		{http.MethodDelete, "/api/videos/1"},
		{http.MethodPost, "/api/videos/1/cover"},
		{http.MethodPost, "/api/videos/1/publish"},
		{http.MethodPost, "/api/videos/1/recommend"},
		{http.MethodGet, "/api/transcode-tasks/task-1"},
		{http.MethodGet, "/api/admin/recommendation/overview"},
		{http.MethodGet, "/api/auth/me"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestPublicRoutesDoNotRequireAuthentication(t *testing.T) {
	r := router.New(&appbuilder.App{})
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/api/healthz"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/recommendations/by-question"},
		{http.MethodPost, "/api/watch-records"},
		{http.MethodGet, "/api/videos"},
		{http.MethodGet, "/api/videos/1/play"},
		{http.MethodPost, "/api/videos/1/reactions"},
		{http.MethodGet, "/api/video-segments/random-play?user_id=bad"},
		{http.MethodGet, "/swagger/index.html"},
		{http.MethodGet, "/swagger/doc.json"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
			if recorder.Code == http.StatusUnauthorized {
				t.Fatalf("public route returned 401: body=%s", recorder.Body.String())
			}
		})
	}
}
