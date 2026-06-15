package videoapp

import (
	"context"

	runtimeapp "nlp-video-analysis/internal/application/videoapp/runtime"
	"nlp-video-analysis/internal/http/dto"
)

func (s *Service) GetSystemMetrics(ctx context.Context) (dto.SystemMetricsData, error) {
	return runtimeapp.MetricsService{Counters: runtimeCounters}.GetSystemMetrics(ctx)
}
