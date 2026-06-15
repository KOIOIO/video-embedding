package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetupRouterHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()

	SetupRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid json response, got error: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("expected status ok, got %q", payload.Status)
	}
	contentType := rec.Header().Get("Content-Type")
	if contentType == "" {
		t.Fatal("expected content type header to be set")
	}
	if len(contentType) < len("application/json") || contentType[:len("application/json")] != "application/json" {
		t.Fatalf("expected json content type, got %q", contentType)
	}
}
