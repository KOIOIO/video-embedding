package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterObjectProxyRoutesUsesConfiguredPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerObjectProxyRoutes(r, "/media", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/media/hls/demo/master.m3u8", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("configured media route status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestRegisterObjectProxyRoutesKeepsVideosCompatibilityRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerObjectProxyRoutes(r, "/media", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/videos/hls/demo/master.m3u8", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("compat media route status = %d, want %d", w.Code, http.StatusNoContent)
	}
}
