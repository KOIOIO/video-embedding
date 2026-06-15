package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/http/dto"
	"nlp-video-analysis/internal/http/handler"
)

type stubSystemApp struct {
	metricsFunc func(context.Context) (dto.SystemMetricsData, error)
}

func (s *stubSystemApp) GetSystemMetrics(ctx context.Context) (dto.SystemMetricsData, error) {
	if s.metricsFunc != nil {
		return s.metricsFunc(ctx)
	}
	return dto.SystemMetricsData{}, nil
}

func TestGetSystemMetrics_Success(t *testing.T) {
	stub := &stubSystemApp{
		metricsFunc: func(context.Context) (dto.SystemMetricsData, error) {
			return dto.SystemMetricsData{
				CPUPercent:         37.5,
				MemoryUsedBytes:    8 * 1024 * 1024,
				MemoryTotalBytes:   16 * 1024 * 1024,
				MemoryUsedPercent:  50.0,
				ProcessMemoryBytes: 64 * 1024,
				Goroutines:         42,
				ActiveCounts: map[string]int{
					"transcode_tasks_active": 2,
				},
				Timestamp: "2026-05-15T10:00:00Z",
			}, nil
		},
	}
	h := handler.NewSystemHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/metrics", nil)
	r := gin.New()
	r.GET("/api/system/metrics", h.GetSystemMetrics)

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"cpu_percent":37.5`)
	assertBodyContains(t, w.Body.Bytes(), `"memory_used_percent":50`)
	assertBodyContains(t, w.Body.Bytes(), `"goroutines":42`)
	assertBodyContains(t, w.Body.Bytes(), `"transcode_tasks_active":2`)
	assertBodyContains(t, w.Body.Bytes(), `"active_counts"`)
	assertBodyContains(t, w.Body.Bytes(), `"timestamp":"2026-05-15T10:00:00Z"`)
	assertBodyContains(t, w.Body.Bytes(), `"success":true`)
}
