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

type stubRecommendApp struct {
	recommendByQuestionFunc func(context.Context, videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error)
	listRecommendationsFunc func(context.Context, videoapp.ListRecommendationsInput) ([]videoapp.RecommendResultItem, error)
	reportWatchFunc         func(context.Context, videoapp.ReportWatchInput) error
	resolvePlaybackURLFunc  func(context.Context, domainvideo.Video) string

	recommendInput         videoapp.RecommendByQuestionInput
	listRecommendationsIn  videoapp.ListRecommendationsInput
	reportWatchInput       videoapp.ReportWatchInput
	resolvePlaybackURLCalls int
	resolvedVideo           domainvideo.Video
}

func (s *stubRecommendApp) RecommendByQuestion(ctx context.Context, input videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error) {
	s.recommendInput = input
	if s.recommendByQuestionFunc != nil {
		return s.recommendByQuestionFunc(ctx, input)
	}
	return nil, nil
}

func (s *stubRecommendApp) ListRecommendations(ctx context.Context, input videoapp.ListRecommendationsInput) ([]videoapp.RecommendResultItem, error) {
	s.listRecommendationsIn = input
	if s.listRecommendationsFunc != nil {
		return s.listRecommendationsFunc(ctx, input)
	}
	return nil, nil
}

func (s *stubRecommendApp) ReportWatch(ctx context.Context, input videoapp.ReportWatchInput) error {
	s.reportWatchInput = input
	if s.reportWatchFunc != nil {
		return s.reportWatchFunc(ctx, input)
	}
	return nil
}

func (s *stubRecommendApp) ResolvePlaybackURL(ctx context.Context, video domainvideo.Video) string {
	s.resolvePlaybackURLCalls++
	s.resolvedVideo = video
	if s.resolvePlaybackURLFunc != nil {
		return s.resolvePlaybackURLFunc(ctx, video)
	}
	return ""
}

func TestRecommendByQuestion_EmptyQuestionText(t *testing.T) {
	h := handler.NewRecommendHandler(&stubRecommendApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/by-question", bytes.NewBufferString(`{"question_id":0,"question_text":"   ","user_id":2}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/recommendations/by-question", h.RecommendByQuestion)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"question_text is required when question_id is absent"`)
}

func TestRecommendByQuestion_AllowsQuestionIDWithoutQuestionText(t *testing.T) {
	stub := &stubRecommendApp{
		recommendByQuestionFunc: func(_ context.Context, input videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error) {
			if input.QuestionID != 8 {
				t.Fatalf("expected question_id 8, got %d", input.QuestionID)
			}
			if input.QuestionText != "" {
				t.Fatalf("expected empty question_text, got %q", input.QuestionText)
			}
			return nil, nil
		},
	}
	h := handler.NewRecommendHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/by-question", bytes.NewBufferString(`{"question_id":8,"question_text":"   ","user_id":2}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/recommendations/by-question", h.RecommendByQuestion)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRecommendByQuestion_DefaultsLimitToLegacyValueAndEnrichesPlayURL(t *testing.T) {
	stub := &stubRecommendApp{
		recommendByQuestionFunc: func(_ context.Context, input videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error) {
			if input.Limit != 3 {
				t.Fatalf("expected legacy default limit 3, got %d", input.Limit)
			}
			if input.QuestionText != "how to factor quadratic equations" {
				t.Fatalf("expected trimmed question text, got %q", input.QuestionText)
			}
			return []videoapp.RecommendResultItem{{
				QuestionID:     8,
				VideoID:        15,
				VideoSegmentID: 101,
				RecommendScore: 0.98,
				StartTimeSec:   12,
				EndTimeSec:     34,
				Video: domainvideo.Video{
					ID:         15,
					Title:      "Quadratics",
					CoverURL:   "/covers/15.jpg",
					CreateTime: time.Unix(1714300000, 0),
					UpdateTime: time.Unix(1714300100, 0),
				},
			}}, nil
		},
		resolvePlaybackURLFunc: func(_ context.Context, video domainvideo.Video) string {
			if video.ID != 15 {
				t.Fatalf("unexpected resolved video id %d", video.ID)
			}
			return "/videos/hls/15/master.m3u8"
		},
	}
	h := handler.NewRecommendHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/by-question", bytes.NewBufferString(`{"question_id":8,"question_text":"  how to factor quadratic equations  ","user_id":2,"limit":0}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/recommendations/by-question", h.RecommendByQuestion)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.resolvePlaybackURLCalls != 1 {
		t.Fatalf("expected one read-only playback resolution call, got %d", stub.resolvePlaybackURLCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"success":true`)
	assertBodyContains(t, w.Body.Bytes(), `"video_id":15`)
	assertBodyContains(t, w.Body.Bytes(), `"video_segment_id":101`)
	assertBodyContains(t, w.Body.Bytes(), `"start_time_sec":12`)
	assertBodyContains(t, w.Body.Bytes(), `"end_time_sec":34`)
	assertBodyContains(t, w.Body.Bytes(), `"play_url":"/videos/hls/15/master.m3u8"`)
}

func TestRecommendByQuestion_ReturnsDegradedResponseWhenAIIsDown(t *testing.T) {
	stub := &stubRecommendApp{
		recommendByQuestionFunc: func(_ context.Context, input videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error) {
			return nil, videoapp.DegradedError{Reason: "provider_unavailable", Items: []videoapp.RecommendResultItem{{
				QuestionID:     input.QuestionID,
				VideoID:        15,
				VideoSegmentID: 101,
				RecommendScore: 0.88,
				StartTimeSec:   12,
				EndTimeSec:     34,
				Video: domainvideo.Video{
					ID:    15,
					Title: "Quadratics",
				},
			}}}
		},
		resolvePlaybackURLFunc: func(_ context.Context, _ domainvideo.Video) string {
			return "/videos/hls/15/master.m3u8"
		},
	}
	h := handler.NewRecommendHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/recommendations/by-question", bytes.NewBufferString(`{"question_id":8,"question_text":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/recommendations/by-question", h.RecommendByQuestion)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"degraded":true`) {
		t.Fatalf("expected degraded response, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"video_id":15`) {
		t.Fatalf("expected fallback items in response, got %s", w.Body.String())
	}
}

func TestListRecommendations_Success(t *testing.T) {
	stub := &stubRecommendApp{
		listRecommendationsFunc: func(_ context.Context, input videoapp.ListRecommendationsInput) ([]videoapp.RecommendResultItem, error) {
			if input.UserID != 7 || input.QuestionID != 3 {
				t.Fatalf("unexpected list input: %+v", input)
			}
			return []videoapp.RecommendResultItem{{
				QuestionID:     3,
				VideoID:        17,
				VideoSegmentID: 204,
				RecommendScore: 0.88,
				IsWatched:      true,
				WatchDuration:  45,
				StartTimeSec:   120,
				EndTimeSec:     165,
				Video: domainvideo.Video{ID: 17, Title: "History"},
			}}, nil
		},
	}
	h := handler.NewRecommendHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/recommendations?user_id=7&question_id=3&limit=5", nil)
	router := gin.New()
	router.GET("/api/recommendations", h.ListRecommendations)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":17`)
	assertBodyContains(t, w.Body.Bytes(), `"question_id":3`)
	assertBodyContains(t, w.Body.Bytes(), `"is_watched":true`)
	assertBodyContains(t, w.Body.Bytes(), `"watch_duration":45`)
	assertBodyContains(t, w.Body.Bytes(), `"start_time_sec":120`)
	assertBodyContains(t, w.Body.Bytes(), `"end_time_sec":165`)
}

func TestListRecommendations_RejectsMissingQuestionID(t *testing.T) {
	h := handler.NewRecommendHandler(&stubRecommendApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/recommendations?user_id=7", nil)
	router := gin.New()
	router.GET("/api/recommendations", h.ListRecommendations)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"question_id is required"`)
}

func TestListRecommendations_RejectsInvalidQuestionID(t *testing.T) {
	h := handler.NewRecommendHandler(&stubRecommendApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/recommendations?question_id=abc", nil)
	router := gin.New()
	router.GET("/api/recommendations", h.ListRecommendations)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"question_id must be a positive integer"`)
}

func TestReportWatch_Success(t *testing.T) {
	stub := &stubRecommendApp{
		reportWatchFunc: func(_ context.Context, input videoapp.ReportWatchInput) error {
			if input.QuestionID != 9 || input.UserID != 2 || input.VideoSegmentID != 88 || !input.IsWatched || input.WatchDuration != 67 {
				t.Fatalf("unexpected report watch input: %+v", input)
			}
			return nil
		},
	}
	h := handler.NewRecommendHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/watch-records", bytes.NewBufferString(`{"question_id":9,"user_id":2,"video_segment_id":88,"is_watched":true,"watch_duration":67}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/watch-records", h.ReportWatch)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"success":true`)
	assertBodyContains(t, w.Body.Bytes(), `"recorded":true`)
}

func TestReportWatch_RejectsMissingVideoSegmentID(t *testing.T) {
	h := handler.NewRecommendHandler(&stubRecommendApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/watch-records", bytes.NewBufferString(`{"question_id":9,"user_id":2,"video_segment_id":0,"is_watched":true,"watch_duration":67}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/watch-records", h.ReportWatch)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"video_segment_id is required"`)
}

func TestReportWatch_RejectsNegativeWatchDuration(t *testing.T) {
	h := handler.NewRecommendHandler(&stubRecommendApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/watch-records", bytes.NewBufferString(`{"question_id":9,"user_id":2,"video_segment_id":88,"is_watched":true,"watch_duration":-1}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/watch-records", h.ReportWatch)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"watch_duration must be greater than or equal to 0"`)
}
