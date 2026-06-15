package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestShouldLogAccessSuppressesRoutineHealthyEndpoints(t *testing.T) {
	if shouldLogAccess("/healthz", http.StatusOK, 10*time.Millisecond, "", time.Second) {
		t.Fatal("expected /healthz 200 to be suppressed")
	}
	if shouldLogAccess("/api/healthz", http.StatusOK, 10*time.Millisecond, "", time.Second) {
		t.Fatal("expected /api/healthz 200 to be suppressed")
	}
	if shouldLogAccess("/swagger/index.html", http.StatusOK, 10*time.Millisecond, "", time.Second) {
		t.Fatal("expected swagger route to be suppressed")
	}
}

func TestShouldLogAccessKeepsFailuresAndSlowRequests(t *testing.T) {
	if !shouldLogAccess("/api/videos", http.StatusInternalServerError, 10*time.Millisecond, "", time.Second) {
		t.Fatal("expected 500 response to be logged")
	}
	if !shouldLogAccess("/api/videos", http.StatusOK, 1500*time.Millisecond, "", time.Second) {
		t.Fatal("expected slow request to be logged")
	}
	if !shouldLogAccess("/api/videos", http.StatusOK, 10*time.Millisecond, "handler failed", time.Second) {
		t.Fatal("expected request with private error to be logged")
	}
}

func TestAccessLogMiddlewareStillServesRequestWhenSuppressed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AccessLogMiddleware())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
