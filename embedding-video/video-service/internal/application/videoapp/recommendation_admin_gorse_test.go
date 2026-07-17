package videoapp

import (
	"context"
	"errors"
	"testing"
	"time"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
)

type fakeGorseDashboardClient struct {
	feedbackTypes []string
	points        []recommendationapp.GorseDashboardPoint
	metric        string
	calls         int
}

func (c *fakeGorseDashboardClient) PositiveFeedbackTypes(context.Context) ([]string, error) {
	return c.feedbackTypes, nil
}

func (c *fakeGorseDashboardClient) Timeseries(_ context.Context, metric string, _, _ time.Time) ([]recommendationapp.GorseDashboardPoint, error) {
	c.calls++
	c.metric = metric
	return c.points, nil
}

func TestRecommendationGorsePerformanceUsesConfiguredFeedbackMetric(t *testing.T) {
	client := &fakeGorseDashboardClient{
		feedbackTypes: []string{"like", "double_like", "watch>=0.6"},
		points: []recommendationapp.GorseDashboardPoint{{
			Timestamp: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
			Value:     0.375,
		}},
	}
	svc := &Service{GorseDashboardClient: client}
	begin := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 16, 23, 59, 59, 0, time.UTC)

	result, err := svc.RecommendationGorsePerformance(context.Background(), RecommendationGorsePerformanceInput{
		Metric: "positive_feedback_ratio_watch>=0.6",
		Begin:  begin,
		End:    end,
	})
	if err != nil {
		t.Fatalf("RecommendationGorsePerformance returned error: %v", err)
	}
	if client.metric != "positive_feedback_ratio_watch>=0.6" || client.calls != 1 {
		t.Fatalf("client metric/calls = %q/%d", client.metric, client.calls)
	}
	if result.Label != "正向反馈率（Watch>=0.6）" || len(result.AvailableMetrics) != 10 || len(result.Points) != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestRecommendationGorsePerformanceRejectsUnknownMetricBeforeTimeseries(t *testing.T) {
	client := &fakeGorseDashboardClient{feedbackTypes: []string{"like"}}
	svc := &Service{GorseDashboardClient: client}

	_, err := svc.RecommendationGorsePerformance(context.Background(), RecommendationGorsePerformanceInput{
		Metric: "positive_feedback_ratio_not_configured",
		Begin:  time.Now().Add(-time.Hour),
		End:    time.Now(),
	})
	if !errors.Is(err, ErrInvalidGorsePerformanceMetric) {
		t.Fatalf("error = %v, want ErrInvalidGorsePerformanceMetric", err)
	}
	if client.calls != 0 {
		t.Fatalf("timeseries calls = %d, want 0", client.calls)
	}
}

func TestRecommendationAdminOverviewTreatsDashboardClientAsConfigured(t *testing.T) {
	svc := &Service{GorseDashboardClient: &fakeGorseDashboardClient{}}

	overview, err := svc.RecommendationAdminOverview(context.Background())
	if err != nil {
		t.Fatalf("RecommendationAdminOverview returned error: %v", err)
	}
	if !overview.Gorse.Configured {
		t.Fatal("Gorse should be configured when the dashboard client is available")
	}
}
