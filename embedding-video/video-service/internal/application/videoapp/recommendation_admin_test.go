package videoapp

import (
	"context"
	"testing"
	"time"

	domainvideo "nlp-video-analysis/internal/domain/video"
)

func TestRecommendationTraceRandomPlayUsesPreviewOnlyPath(t *testing.T) {
	saveCalls := 0
	exposureCalls := 0
	svc := NewService(&stubVideoRepository{
		hydrateSegmentsFunc: func(_ context.Context, userID uint64, ids []uint64) ([]RecommendCandidate, error) {
			if userID != 7 || len(ids) != 1 || ids[0] != 101 {
				t.Fatalf("hydrate input userID=%d ids=%v", userID, ids)
			}
			return []RecommendCandidate{{
				VideoID:        11,
				VideoSegmentID: 101,
				StartTimeSec:   10,
				EndTimeSec:     40,
				VideoURL:       "/videos/raw/2026/07/09/algebra.mp4",
				CoverURL:       "/covers/11.jpg",
				Status:         int16(domainvideo.StatusDone),
				IsPublished:    true,
			}}, nil
		},
		saveUserVideoRecommendationFunc: func(context.Context, uint64, uint64, uint64, uint64, float64, time.Time) error {
			saveCalls++
			return nil
		},
		saveRecommendationExposuresFunc: func(context.Context, []RecommendationExposure) error {
			exposureCalls++
			return nil
		},
	}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.RecommendationEngine = RecommendStrategyGorse
	svc.GorseClient = &videoAppFakeGorseClient{ids: []uint64{101}}
	svc.Now = func() time.Time { return time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC) }

	trace, err := svc.RecommendationTraceRandomPlay(context.Background(), RandomPlayVideoSegmentInput{UserID: 7, Limit: 1})
	if err != nil {
		t.Fatalf("RecommendationTraceRandomPlay returned error: %v", err)
	}
	if !trace.PreviewOnly || trace.Mode != "random_play" || trace.Engine != RecommendStrategyGorse || trace.UserID != 7 || trace.Limit != 1 {
		t.Fatalf("unexpected trace metadata: %+v", trace)
	}
	if len(trace.Stages) == 0 || trace.Stages[len(trace.Stages)-1].Name != "preview" || trace.Stages[len(trace.Stages)-1].Status != "ok" {
		t.Fatalf("unexpected stages: %+v", trace.Stages)
	}
	if len(trace.Items) != 1 || trace.Items[0].VideoSegmentID != 101 || trace.Items[0].Strategy != RecommendStrategyGorse || trace.Items[0].Status != "returned" {
		t.Fatalf("unexpected trace items: %+v", trace.Items)
	}
	if saveCalls != 0 || exposureCalls != 0 {
		t.Fatalf("trace must be preview-only, save=%d exposure=%d", saveCalls, exposureCalls)
	}
}

func TestRecommendationRedisStateReadsBucketAndRecentDiagnostics(t *testing.T) {
	bucket := &stubRandomPlayBucket{
		items: []RecommendResultItem{{
			VideoID:               11,
			VideoSegmentID:        101,
			RecommendScore:        0.7,
			RecommendStrategy:     RecommendStrategyKnowledgeMatch,
			RecommendModelVersion: "knowledge_match_v1",
		}},
		ttl: 25 * time.Minute,
	}
	recent := &stubRecentSegmentStore{
		recent: []uint64{101, 102, 103},
		ttl:    20 * time.Minute,
	}
	svc := NewService(&stubVideoRepository{}, nil, nil, nil, nil, nil, nil, Paths{})
	svc.RandomPlayBucket = bucket
	svc.RecentSegments = recent
	svc.RecentSegmentMaxSize = 2
	svc.Now = func() time.Time { return time.Date(2026, 7, 9, 12, 30, 0, 0, time.UTC) }

	state, err := svc.RecommendationRedisState(context.Background(), RecommendationRedisStateInput{UserID: 7})
	if err != nil {
		t.Fatalf("RecommendationRedisState returned error: %v", err)
	}
	if state.UserID != 7 || state.GeneratedAt.IsZero() {
		t.Fatalf("unexpected state metadata: %+v", state)
	}
	if !state.Bucket.Enabled || !state.Bucket.Exists || state.Bucket.Count != 1 || state.Bucket.TTLSeconds != 1500 || state.Bucket.MaxSize != 5 || state.Bucket.MinSize != 3 {
		t.Fatalf("unexpected bucket state: %+v", state.Bucket)
	}
	if len(state.Bucket.Items) != 1 || state.Bucket.Items[0].VideoSegmentID != 101 {
		t.Fatalf("unexpected bucket items: %+v", state.Bucket.Items)
	}
	if !state.Recent.Enabled || !state.Recent.Exists || state.Recent.Count != 3 || state.Recent.TTLSeconds != 1200 || state.Recent.MaxSize != 2 || !state.Recent.OverLimit {
		t.Fatalf("unexpected recent state: %+v", state.Recent)
	}
}
