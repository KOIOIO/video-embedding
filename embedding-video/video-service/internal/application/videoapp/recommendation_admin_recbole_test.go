package videoapp

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRecBolePerformanceRepository struct {
	*stubVideoRepository
	points []RecommendationRecBolePerformancePoint
	metric string
	calls  int
}

func (r *fakeRecBolePerformanceRepository) ListRecommendationRecBolePerformance(_ context.Context, metric string, _, _ time.Time) ([]RecommendationRecBolePerformancePoint, error) {
	r.calls++
	r.metric = metric
	return r.points, nil
}

func TestRecommendationRecBolePerformanceDefaultsToNDCG(t *testing.T) {
	repo := &fakeRecBolePerformanceRepository{
		stubVideoRepository: &stubVideoRepository{},
		points: []RecommendationRecBolePerformancePoint{{
			Timestamp:    time.Date(2026, 7, 21, 9, 15, 0, 0, time.UTC),
			Value:        0.35,
			ModelVersion: "recbole_v2",
		}},
	}
	svc := &Service{Repo: repo}

	result, err := svc.RecommendationRecBolePerformance(context.Background(), RecommendationRecBolePerformanceInput{
		Begin: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RecommendationRecBolePerformance returned error: %v", err)
	}
	if repo.calls != 1 || repo.metric != "NDCG@20" {
		t.Fatalf("repository calls/metric = %d/%q", repo.calls, repo.metric)
	}
	if result.Metric != "NDCG@20" || result.Label != "NDCG@20" || len(result.AvailableMetrics) != 4 {
		t.Fatalf("result metadata = %+v", result)
	}
	if len(result.Points) != 1 || result.Points[0].ModelVersion != "recbole_v2" {
		t.Fatalf("result points = %+v", result.Points)
	}
}

func TestRecommendationRecBolePerformanceRejectsUnknownMetric(t *testing.T) {
	repo := &fakeRecBolePerformanceRepository{stubVideoRepository: &stubVideoRepository{}}
	svc := &Service{Repo: repo}

	_, err := svc.RecommendationRecBolePerformance(context.Background(), RecommendationRecBolePerformanceInput{Metric: "AUC"})
	if !errors.Is(err, ErrInvalidRecBolePerformanceMetric) {
		t.Fatalf("error = %v, want ErrInvalidRecBolePerformanceMetric", err)
	}
	if repo.calls != 0 {
		t.Fatalf("repository calls = %d, want 0", repo.calls)
	}
}
