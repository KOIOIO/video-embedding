package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/application/videoapp"
	domainvideo "nlp-video-analysis/internal/domain/video"
	"nlp-video-analysis/internal/http/handler"
)

type stubRecommendationAdminApp struct {
	overviewFunc           func(context.Context) (videoapp.RecommendationAdminOverview, error)
	diagnosticsFunc        func(context.Context, videoapp.RecommendationDiagnosticsInput) (videoapp.RecommendationDiagnostics, error)
	datasourceStatsFunc    func(context.Context) (videoapp.RecommendationDatasourceStats, error)
	effectMetricsFunc      func(context.Context, videoapp.RecommendationEffectMetricsInput) (videoapp.RecommendationEffectMetrics, error)
	traceRandomPlayFunc    func(context.Context, videoapp.RandomPlayVideoSegmentInput) (videoapp.RecommendationTrace, error)
	traceQuestionFunc      func(context.Context, videoapp.RecommendByQuestionInput) (videoapp.RecommendationTrace, error)
	redisStateFunc         func(context.Context, videoapp.RecommendationRedisStateInput) (videoapp.RecommendationRedisState, error)
	previewRandomPlayFunc  func(context.Context, videoapp.RandomPlayVideoSegmentInput) ([]videoapp.RecommendResultItem, error)
	previewQuestionFunc    func(context.Context, videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error)
	resolvePlaybackURLFunc func(context.Context, domainvideo.Video) string

	previewRandomPlayInput videoapp.RandomPlayVideoSegmentInput
	previewQuestionInput   videoapp.RecommendByQuestionInput
	effectMetricsInput     videoapp.RecommendationEffectMetricsInput
	diagnosticsInput       videoapp.RecommendationDiagnosticsInput
	redisStateInput        videoapp.RecommendationRedisStateInput
}

func (s *stubRecommendationAdminApp) RecommendationAdminOverview(ctx context.Context) (videoapp.RecommendationAdminOverview, error) {
	if s.overviewFunc != nil {
		return s.overviewFunc(ctx)
	}
	return videoapp.RecommendationAdminOverview{}, nil
}

func (s *stubRecommendationAdminApp) RecommendationDatasourceStats(ctx context.Context) (videoapp.RecommendationDatasourceStats, error) {
	if s.datasourceStatsFunc != nil {
		return s.datasourceStatsFunc(ctx)
	}
	return videoapp.RecommendationDatasourceStats{}, nil
}

func (s *stubRecommendationAdminApp) RecommendationDiagnostics(ctx context.Context, input videoapp.RecommendationDiagnosticsInput) (videoapp.RecommendationDiagnostics, error) {
	s.diagnosticsInput = input
	if s.diagnosticsFunc != nil {
		return s.diagnosticsFunc(ctx, input)
	}
	return videoapp.RecommendationDiagnostics{}, nil
}

func (s *stubRecommendationAdminApp) RecommendationEffectMetrics(ctx context.Context, input videoapp.RecommendationEffectMetricsInput) (videoapp.RecommendationEffectMetrics, error) {
	s.effectMetricsInput = input
	if s.effectMetricsFunc != nil {
		return s.effectMetricsFunc(ctx, input)
	}
	return videoapp.RecommendationEffectMetrics{}, nil
}

func TestRecommendationAdminDiagnostics_ReturnsHealthAndRecentRequests(t *testing.T) {
	generatedAt := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	latestAt := generatedAt.Add(-2 * time.Hour)
	stub := &stubRecommendationAdminApp{
		diagnosticsFunc: func(_ context.Context, input videoapp.RecommendationDiagnosticsInput) (videoapp.RecommendationDiagnostics, error) {
			if input.Days != 14 || input.Limit != 5 {
				t.Fatalf("input = %+v, want days 14 limit 5", input)
			}
			return videoapp.RecommendationDiagnostics{
				GeneratedAt:  generatedAt,
				Days:         14,
				RequestLimit: 5,
				Health: []videoapp.RecommendationDiagnosticCheck{{
					Key:    "recbole_model",
					Name:   "RecBole 模型",
					Status: "ok",
					Value:  "recbole_v2",
					Detail: "active model recbole_v2",
				}},
				Freshness: []videoapp.RecommendationDataFreshness{{
					Source:            "recbole_user_embedding",
					Label:             "RecBole 用户向量",
					Status:            "ok",
					LatestAt:          latestAt,
					HasData:           true,
					AgeSeconds:        int64((2 * time.Hour).Seconds()),
					StaleAfterSeconds: int64((48 * time.Hour).Seconds()),
					Detail:            "2h ago",
				}},
				RecentRequests: []videoapp.RecommendationRecentRequest{{
					RequestID:     "req-1",
					UserID:        7,
					QuestionID:    3,
					Exposures:     4,
					Watched:       2,
					WatchRate:     0.5,
					Strategy:      videoapp.RecommendStrategyRecBole,
					ModelVersion:  "recbole_v2",
					LastEventTime: latestAt,
				}},
				StrategyEffects: []videoapp.RecommendationStrategyEffectMetric{{
					Strategy:     videoapp.RecommendStrategyRecBole,
					ModelVersion: "recbole_v2",
					Exposures:    10,
					Watched:      4,
					WatchRate:    0.4,
				}},
				Tasks: []videoapp.RecommendationTaskStatus{{
					Name:       "RecBole Embedding 导入",
					Status:     "ok",
					Detail:     "user/item embeddings available",
					LastRunAt:  latestAt,
					HasRunTime: true,
				}},
			}, nil
		},
	}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/recommendation/diagnostics?days=14&limit=5", nil)
	router := gin.New()
	router.GET("/api/admin/recommendation/diagnostics", h.Diagnostics)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"days":14`)
	assertBodyContains(t, w.Body.Bytes(), `"request_limit":5`)
	assertBodyContains(t, w.Body.Bytes(), `"key":"recbole_model"`)
	assertBodyContains(t, w.Body.Bytes(), `"source":"recbole_user_embedding"`)
	assertBodyContains(t, w.Body.Bytes(), `"request_id":"req-1"`)
	assertBodyContains(t, w.Body.Bytes(), `"strategy_effects"`)
	assertBodyContains(t, w.Body.Bytes(), `"tasks"`)
}

func (s *stubRecommendationAdminApp) RecommendationTraceRandomPlay(ctx context.Context, input videoapp.RandomPlayVideoSegmentInput) (videoapp.RecommendationTrace, error) {
	s.previewRandomPlayInput = input
	if s.traceRandomPlayFunc != nil {
		return s.traceRandomPlayFunc(ctx, input)
	}
	return videoapp.RecommendationTrace{}, nil
}

func (s *stubRecommendationAdminApp) RecommendationTraceByQuestion(ctx context.Context, input videoapp.RecommendByQuestionInput) (videoapp.RecommendationTrace, error) {
	s.previewQuestionInput = input
	if s.traceQuestionFunc != nil {
		return s.traceQuestionFunc(ctx, input)
	}
	return videoapp.RecommendationTrace{}, nil
}

func (s *stubRecommendationAdminApp) RecommendationRedisState(ctx context.Context, input videoapp.RecommendationRedisStateInput) (videoapp.RecommendationRedisState, error) {
	s.redisStateInput = input
	if s.redisStateFunc != nil {
		return s.redisStateFunc(ctx, input)
	}
	return videoapp.RecommendationRedisState{}, nil
}

func (s *stubRecommendationAdminApp) PreviewRandomPlayVideoSegments(ctx context.Context, input videoapp.RandomPlayVideoSegmentInput) ([]videoapp.RecommendResultItem, error) {
	s.previewRandomPlayInput = input
	if s.previewRandomPlayFunc != nil {
		return s.previewRandomPlayFunc(ctx, input)
	}
	return nil, nil
}

func (s *stubRecommendationAdminApp) PreviewRecommendByQuestion(ctx context.Context, input videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error) {
	s.previewQuestionInput = input
	if s.previewQuestionFunc != nil {
		return s.previewQuestionFunc(ctx, input)
	}
	return nil, nil
}

func (s *stubRecommendationAdminApp) ResolvePlaybackURL(ctx context.Context, video domainvideo.Video) string {
	if s.resolvePlaybackURLFunc != nil {
		return s.resolvePlaybackURLFunc(ctx, video)
	}
	return video.VideoURL
}

func TestRecommendationAdminOverview_ReturnsRuntimeState(t *testing.T) {
	generatedAt := time.Date(2026, 7, 9, 10, 30, 0, 0, time.UTC)
	stub := &stubRecommendationAdminApp{
		overviewFunc: func(context.Context) (videoapp.RecommendationAdminOverview, error) {
			return videoapp.RecommendationAdminOverview{
				Engine:       "recbole",
				GeneratedAt:  generatedAt,
				PreviewOnly:  true,
				Gorse:        videoapp.RecommendationAdminGorseOverview{Configured: true, CandidateLimit: 100, MinRecommendItems: 10, WriteBackEnabled: true, ShadowMode: true},
				RecBole:      videoapp.RecommendationAdminRecBoleOverview{ActiveModelVersion: "recbole_v2", ActiveModelFound: true},
				RedisRuntime: videoapp.RecommendationAdminRedisOverview{RecentTTLSeconds: 1800, RecentMaxSize: 200, BucketEnabled: true, BucketTTLSeconds: 1800},
			}, nil
		},
	}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/recommendation/overview", nil)
	router := gin.New()
	router.GET("/api/admin/recommendation/overview", h.Overview)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"engine":"recbole"`)
	assertBodyContains(t, w.Body.Bytes(), `"preview_only":true`)
	assertBodyContains(t, w.Body.Bytes(), `"configured":true`)
	assertBodyContains(t, w.Body.Bytes(), `"candidate_limit":100`)
	assertBodyContains(t, w.Body.Bytes(), `"active_model_version":"recbole_v2"`)
	assertBodyContains(t, w.Body.Bytes(), `"recent_max_size":200`)
	assertBodyContains(t, w.Body.Bytes(), `"bucket_enabled":true`)
}

func TestRecommendationAdminDatasources_ReturnsRecommendationInputs(t *testing.T) {
	stub := &stubRecommendationAdminApp{
		datasourceStatsFunc: func(context.Context) (videoapp.RecommendationDatasourceStats, error) {
			return videoapp.RecommendationDatasourceStats{
				VideoTotal:             10,
				PublishedVideos:        8,
				RecommendVideos:        5,
				SegmentTotal:           30,
				PlayableSegments:       25,
				EmbeddedSegments:       20,
				ExposureTotal:          100,
				WatchedExposures:       40,
				RecommendationRows:     70,
				WatchedRecommendations: 35,
				RecBoleUsers:           12,
				RecBoleItems:           21,
				ReactionRows:           9,
			}, nil
		},
	}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/recommendation/datasources", nil)
	router := gin.New()
	router.GET("/api/admin/recommendation/datasources", h.Datasources)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_total":10`)
	assertBodyContains(t, w.Body.Bytes(), `"published_videos":8`)
	assertBodyContains(t, w.Body.Bytes(), `"segment_total":30`)
	assertBodyContains(t, w.Body.Bytes(), `"embedded_segments":20`)
	assertBodyContains(t, w.Body.Bytes(), `"exposure_total":100`)
	assertBodyContains(t, w.Body.Bytes(), `"watched_exposures":40`)
	assertBodyContains(t, w.Body.Bytes(), `"recbole_users":12`)
	assertBodyContains(t, w.Body.Bytes(), `"recbole_items":21`)
}

func TestRecommendationAdminEffects_ReturnsDailyAndStrategyMetrics(t *testing.T) {
	stub := &stubRecommendationAdminApp{
		effectMetricsFunc: func(_ context.Context, input videoapp.RecommendationEffectMetricsInput) (videoapp.RecommendationEffectMetrics, error) {
			if input.Days != 14 {
				t.Fatalf("days = %d, want 14", input.Days)
			}
			return videoapp.RecommendationEffectMetrics{
				Daily: []videoapp.RecommendationDailyEffectMetric{{
					Day:       "2026-07-09",
					Exposures: 20,
					Watched:   8,
					WatchRate: 0.4,
				}},
				Strategies: []videoapp.RecommendationStrategyEffectMetric{{
					Strategy:     videoapp.RecommendStrategyGorse,
					ModelVersion: "gorse",
					Exposures:    12,
					Watched:      6,
					WatchRate:    0.5,
					AverageRank:  1.5,
					AverageScore: 0.75,
				}},
			}, nil
		},
	}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/recommendation/effects?days=14", nil)
	router := gin.New()
	router.GET("/api/admin/recommendation/effects", h.Effects)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"days":14`)
	assertBodyContains(t, w.Body.Bytes(), `"day":"2026-07-09"`)
	assertBodyContains(t, w.Body.Bytes(), `"watch_rate":0.4`)
	assertBodyContains(t, w.Body.Bytes(), `"strategy":"gorse"`)
	assertBodyContains(t, w.Body.Bytes(), `"average_rank":1.5`)
}

func TestRecommendationAdminEffects_RejectsInvalidDays(t *testing.T) {
	stub := &stubRecommendationAdminApp{}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/recommendation/effects?days=bad", nil)
	router := gin.New()
	router.GET("/api/admin/recommendation/effects", h.Effects)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
}

func TestRecommendationAdminTraceRandomPlay_ReturnsStagesAndItems(t *testing.T) {
	generatedAt := time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC)
	stub := &stubRecommendationAdminApp{
		traceRandomPlayFunc: func(_ context.Context, input videoapp.RandomPlayVideoSegmentInput) (videoapp.RecommendationTrace, error) {
			if input.UserID != 7 || input.Limit != 2 {
				t.Fatalf("input = %+v, want user 7 limit 2", input)
			}
			return videoapp.RecommendationTrace{
				Mode:        "random_play",
				Engine:      videoapp.RecommendStrategyKnowledgeMatch,
				UserID:      7,
				Limit:       2,
				GeneratedAt: generatedAt,
				PreviewOnly: true,
				Stages: []videoapp.RecommendationTraceStage{{
					Name:   "preview",
					Status: "ok",
					Detail: "2 candidates returned without persistence",
				}},
				Items: []videoapp.RecommendationTraceItem{{
					Rank:           1,
					VideoID:        11,
					VideoSegmentID: 22,
					RecommendScore: 0.8,
					Strategy:       videoapp.RecommendStrategyKnowledgeMatch,
					ModelVersion:   "knowledge_match_v1",
					Status:         "returned",
					Reasons:        []string{"preview_only"},
				}},
			}, nil
		},
	}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/recommendation/trace/random-play?user_id=7&limit=2", nil)
	router := gin.New()
	router.GET("/api/admin/recommendation/trace/random-play", h.TraceRandomPlay)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"mode":"random_play"`)
	assertBodyContains(t, w.Body.Bytes(), `"engine":"knowledge_match"`)
	assertBodyContains(t, w.Body.Bytes(), `"name":"preview"`)
	assertBodyContains(t, w.Body.Bytes(), `"video_segment_id":22`)
	assertBodyContains(t, w.Body.Bytes(), `"reasons":["preview_only"]`)
}

func TestRecommendationAdminTraceByQuestion_MapsBody(t *testing.T) {
	stub := &stubRecommendationAdminApp{
		traceQuestionFunc: func(_ context.Context, input videoapp.RecommendByQuestionInput) (videoapp.RecommendationTrace, error) {
			if input.QuestionID != 8 || input.QuestionText != "factor" || input.UserID != 7 || input.Limit != 3 {
				t.Fatalf("input = %+v", input)
			}
			return videoapp.RecommendationTrace{
				Mode:         "by_question",
				Engine:       videoapp.RecommendStrategyQuestionVector,
				UserID:       7,
				QuestionID:   8,
				QuestionText: "factor",
				Limit:        3,
				PreviewOnly:  true,
			}, nil
		},
	}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/recommendation/trace/by-question", bytes.NewBufferString(`{"question_id":8,"question_text":"  factor  ","user_id":7,"limit":3}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/admin/recommendation/trace/by-question", h.TraceByQuestion)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"mode":"by_question"`)
	assertBodyContains(t, w.Body.Bytes(), `"question_text":"factor"`)
}

func TestRecommendationAdminRedisState_ReturnsBucketAndRecentDiagnostics(t *testing.T) {
	stub := &stubRecommendationAdminApp{
		redisStateFunc: func(_ context.Context, input videoapp.RecommendationRedisStateInput) (videoapp.RecommendationRedisState, error) {
			if input.UserID != 7 {
				t.Fatalf("input = %+v, want user 7", input)
			}
			return videoapp.RecommendationRedisState{
				UserID: 7,
				Bucket: videoapp.RecommendationRedisBucketState{
					Enabled:    true,
					Exists:     true,
					TTLSeconds: 1200,
					Count:      2,
					MaxSize:    5,
					MinSize:    3,
					Items: []videoapp.RecommendationRedisBucketItem{{
						Rank:           1,
						VideoID:        11,
						VideoSegmentID: 22,
						Strategy:       videoapp.RecommendStrategyGorse,
					}},
				},
				Recent: videoapp.RecommendationRedisRecentState{
					Enabled:    true,
					Exists:     true,
					TTLSeconds: 1200,
					Count:      3,
					MaxSize:    200,
					SegmentIDs: []uint64{22, 23, 24},
				},
			}, nil
		},
	}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/recommendation/redis-state?user_id=7", nil)
	router := gin.New()
	router.GET("/api/admin/recommendation/redis-state", h.RedisState)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"user_id":7`)
	assertBodyContains(t, w.Body.Bytes(), `"bucket"`)
	assertBodyContains(t, w.Body.Bytes(), `"ttl_seconds":1200`)
	assertBodyContains(t, w.Body.Bytes(), `"segment_ids":[22,23,24]`)
}

func TestRecommendationAdminPreviewRandomPlay_MapsInputAndItems(t *testing.T) {
	stub := &stubRecommendationAdminApp{
		previewRandomPlayFunc: func(_ context.Context, input videoapp.RandomPlayVideoSegmentInput) ([]videoapp.RecommendResultItem, error) {
			if input.UserID != 7 || input.Limit != 2 {
				t.Fatalf("input = %+v, want user 7 limit 2", input)
			}
			return []videoapp.RecommendResultItem{{
				VideoID:               11,
				VideoSegmentID:        22,
				RecommendScore:        0.92,
				RecommendStrategy:     videoapp.RecommendStrategyRecBole,
				RecommendModelVersion: "recbole_v2",
				StartTimeSec:          5,
				EndTimeSec:            40,
				Video: domainvideo.Video{
					ID:       11,
					Title:    "Algebra",
					VideoURL: "/videos/raw/2026/07/09/algebra.mp4",
					CoverURL: "/covers/11.jpg",
				},
			}}, nil
		},
		resolvePlaybackURLFunc: func(_ context.Context, video domainvideo.Video) string {
			if video.ID != 11 {
				t.Fatalf("unexpected video id %d", video.ID)
			}
			return "/videos/hls/2026/07/09/algebra/master.m3u8"
		},
	}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/recommendation/preview/random-play?user_id=7&limit=2", nil)
	router := gin.New()
	router.GET("/api/admin/recommendation/preview/random-play", h.PreviewRandomPlay)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"total":1`)
	assertBodyContains(t, w.Body.Bytes(), `"rank":1`)
	assertBodyContains(t, w.Body.Bytes(), `"video_segment_id":22`)
	assertBodyContains(t, w.Body.Bytes(), `"strategy":"recbole"`)
	assertBodyContains(t, w.Body.Bytes(), `"model_version":"recbole_v2"`)
	assertBodyContains(t, w.Body.Bytes(), `"play_url":"/videos/hls/2026/07/09/algebra/master.m3u8"`)
}

func TestRecommendationAdminPreviewRandomPlay_RejectsInvalidUserID(t *testing.T) {
	stub := &stubRecommendationAdminApp{}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/recommendation/preview/random-play?user_id=bad", nil)
	router := gin.New()
	router.GET("/api/admin/recommendation/preview/random-play", h.PreviewRandomPlay)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if stub.previewRandomPlayInput.UserID != 0 {
		t.Fatalf("preview input should not be populated, got %+v", stub.previewRandomPlayInput)
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
}

func TestRecommendationAdminPreviewByQuestion_MapsTrimmedInputAndItems(t *testing.T) {
	stub := &stubRecommendationAdminApp{
		previewQuestionFunc: func(_ context.Context, input videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error) {
			if input.UserID != 9 || input.Limit != 2 || input.QuestionText != "how to solve linear equations" {
				t.Fatalf("input = %+v, want trimmed question preview input", input)
			}
			return []videoapp.RecommendResultItem{{
				QuestionID:            0,
				VideoID:               31,
				VideoSegmentID:        42,
				RecommendScore:        0.75,
				RecommendStrategy:     videoapp.RecommendStrategyQuestionVector,
				RecommendModelVersion: "",
				StartTimeSec:          12,
				EndTimeSec:            50,
				TitleOverride:         "linear equations clip",
				Video: domainvideo.Video{
					ID:       31,
					Title:    "Linear equations",
					VideoURL: "/videos/raw/2026/07/09/linear.mp4",
				},
			}}, nil
		},
	}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/recommendation/preview/by-question", bytes.NewBufferString(`{"question_text":"  how to solve linear equations  ","user_id":9,"limit":2}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/admin/recommendation/preview/by-question", h.PreviewByQuestion)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"total":1`)
	assertBodyContains(t, w.Body.Bytes(), `"video_segment_id":42`)
	assertBodyContains(t, w.Body.Bytes(), `"title":"linear equations clip"`)
	assertBodyContains(t, w.Body.Bytes(), `"strategy":"question_vector"`)
}

func TestRecommendationAdminPreviewByQuestion_RejectsMissingQuestion(t *testing.T) {
	stub := &stubRecommendationAdminApp{}
	h := handler.NewRecommendationAdminHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/recommendation/preview/by-question", strings.NewReader(`{"question_text":"   ","user_id":9}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/admin/recommendation/preview/by-question", h.PreviewByQuestion)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
}
