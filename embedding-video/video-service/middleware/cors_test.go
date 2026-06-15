package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSMiddlewareHandlesOptionsPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddleware())
	r.GET("/api/videos", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/videos", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("allow origin = %q, want *", got)
	}
}

func TestCORSMiddlewareUsesConfiguredHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddlewareWithOptions(CORSOptions{
		AllowOrigin:   "https://example.com",
		AllowMethods:  "GET, OPTIONS",
		AllowHeaders:  "Content-Type",
		ExposeHeaders: "Content-Length",
		MaxAge:        "60",
	}))
	r.GET("/api/videos", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("allow origin = %q, want configured origin", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Fatalf("allow methods = %q, want configured methods", got)
	}
	if got := w.Header().Get("Access-Control-Max-Age"); got != "60" {
		t.Fatalf("max age = %q, want configured max age", got)
	}
}
