package videoapp

import (
	"context"

	runtimeapp "video-service/internal/application/videoapp/runtime"
	"video-service/internal/http/dto"
)

func (s *Service) GetSystemMetrics(ctx context.Context) (dto.SystemMetricsData, error) {
	return runtimeapp.MetricsService{Counters: runtimeCounters}.GetSystemMetrics(ctx)
}
