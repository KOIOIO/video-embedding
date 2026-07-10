package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/application/videoapp"
	domainvideo "nlp-video-analysis/internal/domain/video"
	"nlp-video-analysis/internal/http/handler"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type stubVideoApp struct {
	listVideosFunc               func(context.Context, videoapp.ListFilter) ([]domainvideo.Video, error)
	updateMetadataFunc           func(context.Context, uint64, string, string) (bool, error)
	deleteVideoFunc              func(context.Context, uint64, string) (bool, error)
	playVideoFunc                func(context.Context, uint64) (string, domainvideo.Video, bool, error)
	resolvePlaybackURLFunc       func(context.Context, domainvideo.Video) string
	getSimilarVideosFunc         func(context.Context, uint64, int) ([]domainvideo.Video, error)
	getViewCountFunc             func(context.Context, uint64) (int64, bool, error)
	setVideoPublishedFunc        func(context.Context, uint64, bool) (bool, error)
	setVideoRecommendFunc        func(context.Context, uint64, bool, uint64, int16, float64) (bool, error)
	getTranscodeStatusFunc       func(context.Context, string) (videoapp.TranscodeStatus, bool, error)
	submitReactionFunc           func(context.Context, uint64, uint64, videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error)
	getReactionCountsFunc        func(context.Context, uint64) (videoapp.VideoReactionCounts, bool, error)
	submitSegmentReactionFunc    func(context.Context, uint64, uint64, videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error)
	getSegmentReactionCountsFunc func(context.Context, uint64) (videoapp.VideoReactionCounts, bool, error)
	randomPlaySegmentFunc        func(context.Context, videoapp.RandomPlayVideoSegmentInput) (videoapp.RecommendResultItem, bool, error)
	externalRecBoleFunc         func(context.Context, videoapp.RandomPlayVideoSegmentInput) ([]uint64, error)

	listVideosCalls            int
	listVideosFilter           videoapp.ListFilter
	updateMetadataVideoID      uint64
	updateMetadataTitle        string
	updateMetadataDesc         string
	deleteVideoID              uint64
	playVideoID                uint64
	getSimilarVideosID         uint64
	getSimilarVideosLimit      int
	getViewCountID             uint64
	setVideoPublishedID        uint64
	setVideoPublishedValue     bool
	setVideoRecommendID        uint64
	setVideoRecommendValue     bool
	getTranscodeTaskID         string
	submitReactionVideoID      uint64
	submitReactionSegmentID    uint64
	submitReactionUserID       uint64
	submitReactionType         videoapp.VideoReactionType
	getReactionCountsID        uint64
	getSegmentReactionCountsID uint64
	randomPlaySegmentCalls     int
	randomPlaySegmentInput     videoapp.RandomPlayVideoSegmentInput
	externalRecBoleCalls      int
	externalRecBoleInput      videoapp.RandomPlayVideoSegmentInput
}

func (s *stubVideoApp) ListVideos(ctx context.Context, filter videoapp.ListFilter) ([]domainvideo.Video, error) {
	s.listVideosCalls++
	s.listVideosFilter = filter
	if s.listVideosFunc != nil {
		return s.listVideosFunc(ctx, filter)
	}
	return nil, nil
}

func (s *stubVideoApp) UpdateVideoMetadata(ctx context.Context, videoID uint64, title string, description string) (bool, error) {
	s.updateMetadataVideoID = videoID
	s.updateMetadataTitle = title
	s.updateMetadataDesc = description
	if s.updateMetadataFunc != nil {
		return s.updateMetadataFunc(ctx, videoID, title, description)
	}
	return false, nil
}

func (s *stubVideoApp) DeleteVideo(ctx context.Context, videoID uint64, operator string) (bool, error) {
	s.deleteVideoID = videoID
	if s.deleteVideoFunc != nil {
		return s.deleteVideoFunc(ctx, videoID, operator)
	}
	return false, nil
}

func (s *stubVideoApp) PlayVideo(ctx context.Context, videoID uint64) (string, domainvideo.Video, bool, error) {
	s.playVideoID = videoID
	if s.playVideoFunc != nil {
		return s.playVideoFunc(ctx, videoID)
	}
	return "", domainvideo.Video{}, false, nil
}

func (s *stubVideoApp) ResolvePlaybackURL(ctx context.Context, video domainvideo.Video) string {
	if s.resolvePlaybackURLFunc != nil {
		return s.resolvePlaybackURLFunc(ctx, video)
	}
	if video.Status != domainvideo.StatusDone {
		return video.VideoURL
	}
	value := strings.TrimPrefix(video.VideoURL, "/videos/")
	value = strings.TrimPrefix(value, "raw/")
	value = strings.TrimPrefix(value, "/")
	parts := strings.Split(value, "/")
	if len(parts) < 4 {
		return video.VideoURL
	}
	datePath := strings.Join(parts[:3], "/")
	fileName := parts[3]
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	return "/videos/hls/" + datePath + "/" + base + "/master.m3u8"
}

func (s *stubVideoApp) GetSimilarVideos(ctx context.Context, videoID uint64, limit int) ([]domainvideo.Video, error) {
	s.getSimilarVideosID = videoID
	s.getSimilarVideosLimit = limit
	if s.getSimilarVideosFunc != nil {
		return s.getSimilarVideosFunc(ctx, videoID, limit)
	}
	return nil, nil
}

func (s *stubVideoApp) GetViewCount(ctx context.Context, videoID uint64) (int64, bool, error) {
	s.getViewCountID = videoID
	if s.getViewCountFunc != nil {
		return s.getViewCountFunc(ctx, videoID)
	}
	return 0, false, nil
}

func (s *stubVideoApp) SetVideoPublished(ctx context.Context, videoID uint64, isPublished bool) (bool, error) {
	s.setVideoPublishedID = videoID
	s.setVideoPublishedValue = isPublished
	if s.setVideoPublishedFunc != nil {
		return s.setVideoPublishedFunc(ctx, videoID, isPublished)
	}
	return false, nil
}

func (s *stubVideoApp) SetVideoRecommend(ctx context.Context, videoID uint64, isRecommend bool, userID uint64, recommendLevel int16, recommendScore float64) (bool, error) {
	s.setVideoRecommendID = videoID
	s.setVideoRecommendValue = isRecommend
	if s.setVideoRecommendFunc != nil {
		return s.setVideoRecommendFunc(ctx, videoID, isRecommend, userID, recommendLevel, recommendScore)
	}
	return false, nil
}

func (s *stubVideoApp) GetTranscodeStatus(ctx context.Context, taskID string) (videoapp.TranscodeStatus, bool, error) {
	s.getTranscodeTaskID = taskID
	if s.getTranscodeStatusFunc != nil {
		return s.getTranscodeStatusFunc(ctx, taskID)
	}
	return videoapp.TranscodeStatus{}, false, nil
}

func (s *stubVideoApp) SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error) {
	s.submitReactionVideoID = videoID
	s.submitReactionUserID = userID
	s.submitReactionType = reactionType
	if s.submitReactionFunc != nil {
		return s.submitReactionFunc(ctx, videoID, userID, reactionType)
	}
	return videoapp.VideoReactionResult{}, false, nil
}

func (s *stubVideoApp) GetVideoReactionCounts(ctx context.Context, videoID uint64) (videoapp.VideoReactionCounts, bool, error) {
	s.getReactionCountsID = videoID
	if s.getReactionCountsFunc != nil {
		return s.getReactionCountsFunc(ctx, videoID)
	}
	return videoapp.VideoReactionCounts{}, false, nil
}

func (s *stubVideoApp) SubmitSegmentReaction(ctx context.Context, segmentID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error) {
	s.submitReactionSegmentID = segmentID
	s.submitReactionUserID = userID
	s.submitReactionType = reactionType
	if s.submitSegmentReactionFunc != nil {
		return s.submitSegmentReactionFunc(ctx, segmentID, userID, reactionType)
	}
	return videoapp.VideoReactionResult{}, false, nil
}

func (s *stubVideoApp) GetSegmentReactionCounts(ctx context.Context, segmentID uint64) (videoapp.VideoReactionCounts, bool, error) {
	s.getSegmentReactionCountsID = segmentID
	if s.getSegmentReactionCountsFunc != nil {
		return s.getSegmentReactionCountsFunc(ctx, segmentID)
	}
	return videoapp.VideoReactionCounts{}, false, nil
}

func (s *stubVideoApp) RandomPlayVideoSegment(ctx context.Context, input videoapp.RandomPlayVideoSegmentInput) (videoapp.RecommendResultItem, bool, error) {
	s.randomPlaySegmentCalls++
	s.randomPlaySegmentInput = input
	if s.randomPlaySegmentFunc != nil {
		return s.randomPlaySegmentFunc(ctx, input)
	}
	return videoapp.RecommendResultItem{}, false, nil
}

func (s *stubVideoApp) ExternalRecBoleItemIDs(ctx context.Context, input videoapp.RandomPlayVideoSegmentInput) ([]uint64, error) {
	s.externalRecBoleCalls++
	s.externalRecBoleInput = input
	if s.externalRecBoleFunc != nil {
		return s.externalRecBoleFunc(ctx, input)
	}
	return nil, nil
}

func TestListVideos_UsesApplicationService(t *testing.T) {
	stub := &stubVideoApp{
		listVideosFunc: func(_ context.Context, filter videoapp.ListFilter) ([]domainvideo.Video, error) {
			return []domainvideo.Video{{ID: 7, Title: "demo", ViewCount: 5}}, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos?type=ALL", nil)
	router := gin.New()
	router.GET("/api/videos", h.ListVideos)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.listVideosCalls != 1 {
		t.Fatalf("expected service call, got %d", stub.listVideosCalls)
	}
	if stub.listVideosFilter != videoapp.ListAll {
		t.Fatalf("expected ALL filter, got %v", stub.listVideosFilter)
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":7`)
	assertBodyContains(t, w.Body.Bytes(), `"success":true`)
	assertBodyContains(t, w.Body.Bytes(), `"total":1`)
	assertBodyContains(t, w.Body.Bytes(), `"type":"ALL"`)
}

func TestListVideos_DefaultsTypeToAll(t *testing.T) {
	stub := &stubVideoApp{}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	router := gin.New()
	router.GET("/api/videos", h.ListVideos)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.listVideosFilter != videoapp.ListAll {
		t.Fatalf("expected default ALL filter, got %v", stub.listVideosFilter)
	}
}

func TestListVideos_DoneVideoMapsHLSURLFromRawPath(t *testing.T) {
	stub := &stubVideoApp{
		listVideosFunc: func(_ context.Context, filter videoapp.ListFilter) ([]domainvideo.Video, error) {
			return []domainvideo.Video{{
				ID:       88,
				Title:    "done-video",
				VideoURL: "/videos/raw/2026/04/29/1777428805644133000.mp4",
				Status:   domainvideo.StatusDone,
			}}, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos?type=ALL", nil)
	router := gin.New()
	router.GET("/api/videos", h.ListVideos)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":88`)
	assertBodyContains(t, w.Body.Bytes(), `"raw_url":""`)
	assertBodyContains(t, w.Body.Bytes(), `"hls_url":"/videos/hls/2026/04/29/1777428805644133000/master.m3u8"`)
	if bytes.Contains(w.Body.Bytes(), []byte(`"hls_url":"/videos/raw/2026/04/29/1777428805644133000.mp4"`)) {
		t.Fatalf("done video should not expose raw mp4 as hls_url: %s", w.Body.String())
	}
}

func TestListVideos_RejectsUnsupportedType(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos?type=BOGUS", nil)
	router := gin.New()
	router.GET("/api/videos", h.ListVideos)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"type must be one of ALL, RAW, HLS"`)
}

func TestUpdateVideoMetadata_RejectsInvalidID(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/videos/0", bytes.NewBufferString(`{"title":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.PATCH("/api/videos/:id", h.UpdateVideoMetadata)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"id must be a positive integer"`)
}

func TestUpdateVideoMetadata_Success(t *testing.T) {
	stub := &stubVideoApp{
		updateMetadataFunc: func(_ context.Context, videoID uint64, title string, description string) (bool, error) {
			if videoID != 11 || title != "new title" || description != "new desc" {
				t.Fatalf("unexpected input: id=%d title=%q description=%q", videoID, title, description)
			}
			return true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/videos/11", bytes.NewBufferString(`{"title":"new title","description":"new desc"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.PATCH("/api/videos/:id", h.UpdateVideoMetadata)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":11`)
	assertBodyContains(t, w.Body.Bytes(), `"title":"new title"`)
	assertBodyContains(t, w.Body.Bytes(), `"description":"new desc"`)
}

func TestUpdateVideoMetadata_ResponseUsesNormalizedTitle(t *testing.T) {
	stub := &stubVideoApp{
		updateMetadataFunc: func(_ context.Context, videoID uint64, title string, description string) (bool, error) {
			if title != "normalized title" {
				t.Fatalf("expected normalized title, got %q", title)
			}
			return true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/videos/11", bytes.NewBufferString(`{"title":"  normalized title  ","description":"new desc"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.PATCH("/api/videos/:id", h.UpdateVideoMetadata)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"title":"normalized title"`)
	if bytes.Contains(w.Body.Bytes(), []byte(`"title":"  normalized title  "`)) {
		t.Fatalf("response should not contain raw untrimmed title: %s", w.Body.String())
	}
}

func TestDeleteVideo_Success(t *testing.T) {
	stub := &stubVideoApp{
		deleteVideoFunc: func(_ context.Context, videoID uint64, operator string) (bool, error) {
			return videoID == 12, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/videos/12", nil)
	router := gin.New()
	router.DELETE("/api/videos/:id", h.DeleteVideo)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":12`)
	assertBodyContains(t, w.Body.Bytes(), `"deleted":true`)
}

func TestPlayVideo_NotFound(t *testing.T) {
	stub := &stubVideoApp{
		playVideoFunc: func(_ context.Context, videoID uint64) (string, domainvideo.Video, bool, error) {
			return "", domainvideo.Video{}, false, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos/13/play", nil)
	router := gin.New()
	router.GET("/api/videos/:id/play", h.PlayVideo)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"video_not_found"`)
}

func TestPlayVideo_UsesResolvedPlaybackURLInVideoPayload(t *testing.T) {
	stub := &stubVideoApp{
		playVideoFunc: func(_ context.Context, videoID uint64) (string, domainvideo.Video, bool, error) {
			return "/videos/hls/14/master.m3u8", domainvideo.Video{
				ID:       14,
				Title:    "playable",
				VideoURL: "/videos/raw/2026/04/28/demo.mp4",
				Status:   domainvideo.StatusDone,
			}, true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos/14/play", nil)
	router := gin.New()
	router.GET("/api/videos/:id/play", h.PlayVideo)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"play_url":"/videos/hls/14/master.m3u8"`)
	assertBodyContains(t, w.Body.Bytes(), `"hls_url":"/videos/hls/14/master.m3u8"`)
	if bytes.Contains(w.Body.Bytes(), []byte(`"hls_url":"/videos/raw/2026/04/28/demo.mp4"`)) {
		t.Fatalf("video payload should use resolved playback URL: %s", w.Body.String())
	}
}

func TestGetSimilarVideos_UsesLimitQuery(t *testing.T) {
	stub := &stubVideoApp{
		getSimilarVideosFunc: func(_ context.Context, videoID uint64, limit int) ([]domainvideo.Video, error) {
			return []domainvideo.Video{{ID: 21, Title: "similar"}}, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos/15/similar?limit=9", nil)
	router := gin.New()
	router.GET("/api/videos/:id/similar", h.GetSimilarVideos)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.getSimilarVideosLimit != 9 {
		t.Fatalf("expected limit 9, got %d", stub.getSimilarVideosLimit)
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":21`)
	assertBodyContains(t, w.Body.Bytes(), `"total":1`)
}

func TestGetViewCount_Success(t *testing.T) {
	stub := &stubVideoApp{
		getViewCountFunc: func(_ context.Context, videoID uint64) (int64, bool, error) {
			return 99, true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos/17/view-count", nil)
	router := gin.New()
	router.GET("/api/videos/:id/view-count", h.GetViewCount)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":17`)
	assertBodyContains(t, w.Body.Bytes(), `"view_count":99`)
}

func TestSetVideoPublished_Success(t *testing.T) {
	stub := &stubVideoApp{
		setVideoPublishedFunc: func(_ context.Context, videoID uint64, isPublished bool) (bool, error) {
			return true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/18/publish", bytes.NewBufferString(`{"is_published":true}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/publish", h.SetVideoPublished)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !stub.setVideoPublishedValue {
		t.Fatal("expected is_published true")
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":18`)
	assertBodyContains(t, w.Body.Bytes(), `"is_published":true`)
}

func TestSetVideoPublished_RejectsMissingBooleanField(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/18/publish", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/publish", h.SetVideoPublished)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"is_published is required"`)
}

func TestSetVideoRecommend_Success(t *testing.T) {
	stub := &stubVideoApp{
		setVideoRecommendFunc: func(_ context.Context, videoID uint64, isRecommend bool, userID uint64, recommendLevel int16, recommendScore float64) (bool, error) {
			if userID != 23 || recommendLevel != 3 || recommendScore != 0.75 {
				t.Fatalf("unexpected recommend payload: user=%d level=%d score=%v", userID, recommendLevel, recommendScore)
			}
			return true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/19/recommend", bytes.NewBufferString(`{"is_recommend":true,"user_id":23,"recommend_level":3,"recommend_score":0.75}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/recommend", h.SetVideoRecommend)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !stub.setVideoRecommendValue {
		t.Fatal("expected is_recommend true")
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":19`)
	assertBodyContains(t, w.Body.Bytes(), `"is_recommend":true`)
}

func TestSetVideoRecommend_RejectsMissingBooleanField(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/19/recommend", bytes.NewBufferString(`{"user_id":23,"recommend_level":3,"recommend_score":0.75}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/recommend", h.SetVideoRecommend)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"is_recommend is required"`)
}

func TestSetVideoRecommend_RejectsMissingUserIDWhenMetadataUsed(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/19/recommend", bytes.NewBufferString(`{"is_recommend":true,"recommend_level":3,"recommend_score":0.75}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/recommend", h.SetVideoRecommend)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"user_id must be a positive integer when recommend metadata is provided"`)
}

func TestSetVideoRecommend_RejectsInvalidRecommendLevel(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/19/recommend", bytes.NewBufferString(`{"is_recommend":true,"user_id":23,"recommend_level":0,"recommend_score":0.75}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/recommend", h.SetVideoRecommend)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"recommend_level must be greater than 0"`)
}

func TestSetVideoRecommend_RejectsNegativeRecommendScore(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/19/recommend", bytes.NewBufferString(`{"is_recommend":true,"user_id":23,"recommend_level":3,"recommend_score":-0.1}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/recommend", h.SetVideoRecommend)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"recommend_score must be greater than or equal to 0"`)
}

func TestGetTranscodeStatus_Success(t *testing.T) {
	stub := &stubVideoApp{
		getTranscodeStatusFunc: func(_ context.Context, taskID string) (videoapp.TranscodeStatus, bool, error) {
			return videoapp.TranscodeStatus{Status: domainvideo.StatusDone, HLSURL: "/videos/hls/20/master.m3u8"}, true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/transcode-tasks/task-20", nil)
	router := gin.New()
	router.GET("/api/transcode-tasks/:taskId", h.GetTranscodeStatus)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"task_id":"task-20"`)
	assertBodyContains(t, w.Body.Bytes(), `"status":"DONE"`)
	assertBodyContains(t, w.Body.Bytes(), `"hls_url":"/videos/hls/20/master.m3u8"`)
}

func TestGetTranscodeStatus_RequiresTaskID(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/transcode-tasks/", nil)
	router := gin.New()
	router.GET("/api/transcode-tasks/:taskId", h.GetTranscodeStatus)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected router 404 for missing param, got %d", w.Code)
	}
}

func TestSubmitVideoReaction_SetsReaction(t *testing.T) {
	stub := &stubVideoApp{
		submitReactionFunc: func(_ context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error) {
			if videoID != 21 || userID != 7 || reactionType != videoapp.VideoReactionDoubleLike {
				t.Fatalf("unexpected reaction payload: video=%d user=%d type=%q", videoID, userID, reactionType)
			}
			return videoapp.VideoReactionResult{
				Active:       true,
				ReactionType: videoapp.VideoReactionDoubleLike,
				Counts:       videoapp.VideoReactionCounts{LikeCount: 1, DoubleLikeCount: 3},
			}, true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/21/reactions", bytes.NewBufferString(`{"user_id":7,"reaction_type":"double_like"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/reactions", h.SubmitVideoReaction)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":21`)
	assertBodyContains(t, w.Body.Bytes(), `"user_id":7`)
	assertBodyContains(t, w.Body.Bytes(), `"reaction_type":"double_like"`)
	assertBodyContains(t, w.Body.Bytes(), `"active":true`)
	assertBodyContains(t, w.Body.Bytes(), `"like_count":1`)
	assertBodyContains(t, w.Body.Bytes(), `"double_like_count":3`)
	assertBodyContains(t, w.Body.Bytes(), `"updated":true`)
}

func TestSubmitVideoReaction_RepeatingSameReactionCancelsIt(t *testing.T) {
	stub := &stubVideoApp{
		submitReactionFunc: func(_ context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error) {
			return videoapp.VideoReactionResult{
				Active:       false,
				ReactionType: videoapp.VideoReactionLike,
				Counts:       videoapp.VideoReactionCounts{LikeCount: 0, DoubleLikeCount: 0},
			}, true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/21/reactions", bytes.NewBufferString(`{"user_id":7,"reaction_type":"like"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/reactions", h.SubmitVideoReaction)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"reaction_type":"like"`)
	assertBodyContains(t, w.Body.Bytes(), `"active":false`)
}

func TestSubmitVideoReaction_RejectsInvalidReactionType(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/21/reactions", bytes.NewBufferString(`{"user_id":7,"reaction_type":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/reactions", h.SubmitVideoReaction)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"message":"reaction_type must be one of like, double_like, dislike"`)
}

func TestSubmitVideoReaction_RejectsMissingUserID(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/21/reactions", bytes.NewBufferString(`{"reaction_type":"like"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/reactions", h.SubmitVideoReaction)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"message":"user_id is required"`)
}

func TestSubmitVideoReaction_ReturnsNotFound(t *testing.T) {
	stub := &stubVideoApp{
		submitReactionFunc: func(_ context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error) {
			return videoapp.VideoReactionResult{}, false, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/videos/21/reactions", bytes.NewBufferString(`{"user_id":7,"reaction_type":"like"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/videos/:id/reactions", h.SubmitVideoReaction)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"video_not_found"`)
}

func TestGetVideoReactionCounts_Success(t *testing.T) {
	stub := &stubVideoApp{
		getReactionCountsFunc: func(_ context.Context, videoID uint64) (videoapp.VideoReactionCounts, bool, error) {
			return videoapp.VideoReactionCounts{LikeCount: 5, DoubleLikeCount: 2}, true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos/21/reaction-counts", nil)
	router := gin.New()
	router.GET("/api/videos/:id/reaction-counts", h.GetVideoReactionCounts)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":21`)
	assertBodyContains(t, w.Body.Bytes(), `"like_count":5`)
	assertBodyContains(t, w.Body.Bytes(), `"double_like_count":2`)
	if bytes.Contains(w.Body.Bytes(), []byte(`"dislike_count"`)) {
		t.Fatalf("response must not include dislike_count: %s", w.Body.String())
	}
}

func TestSubmitSegmentReaction_SetsReaction(t *testing.T) {
	stub := &stubVideoApp{
		submitSegmentReactionFunc: func(_ context.Context, segmentID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error) {
			if segmentID != 31 || userID != 7 || reactionType != videoapp.VideoReactionDoubleLike {
				t.Fatalf("unexpected segment reaction payload: segment=%d user=%d type=%q", segmentID, userID, reactionType)
			}
			return videoapp.VideoReactionResult{
				Active:       true,
				ReactionType: videoapp.VideoReactionDoubleLike,
				Counts:       videoapp.VideoReactionCounts{LikeCount: 1, DoubleLikeCount: 3},
			}, true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/video-segments/31/reactions", bytes.NewBufferString(`{"user_id":7,"reaction_type":"double_like"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.POST("/api/video-segments/:id/reactions", h.SubmitSegmentReaction)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"segment_id":31`)
	assertBodyContains(t, w.Body.Bytes(), `"user_id":7`)
	assertBodyContains(t, w.Body.Bytes(), `"reaction_type":"double_like"`)
	assertBodyContains(t, w.Body.Bytes(), `"active":true`)
	assertBodyContains(t, w.Body.Bytes(), `"like_count":1`)
	assertBodyContains(t, w.Body.Bytes(), `"double_like_count":3`)
	assertBodyContains(t, w.Body.Bytes(), `"updated":true`)
}

func TestGetSegmentReactionCounts_Success(t *testing.T) {
	stub := &stubVideoApp{
		getSegmentReactionCountsFunc: func(_ context.Context, segmentID uint64) (videoapp.VideoReactionCounts, bool, error) {
			if segmentID != 31 {
				t.Fatalf("unexpected segment id: %d", segmentID)
			}
			return videoapp.VideoReactionCounts{LikeCount: 5, DoubleLikeCount: 2}, true, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/video-segments/31/reaction-counts", nil)
	router := gin.New()
	router.GET("/api/video-segments/:id/reaction-counts", h.GetSegmentReactionCounts)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"segment_id":31`)
	assertBodyContains(t, w.Body.Bytes(), `"like_count":5`)
	assertBodyContains(t, w.Body.Bytes(), `"double_like_count":2`)
	if bytes.Contains(w.Body.Bytes(), []byte(`"video_id"`)) {
		t.Fatalf("segment response must not include video_id: %s", w.Body.String())
	}
}

func TestRandomPlayVideoSegment_ReturnsPlayableSegment(t *testing.T) {
	stub := &stubVideoApp{
		randomPlaySegmentFunc: func(_ context.Context, input videoapp.RandomPlayVideoSegmentInput) (videoapp.RecommendResultItem, bool, error) {
			if input.UserID != 7 {
				t.Fatalf("expected user_id 7 to be passed, got %d", input.UserID)
			}
			return videoapp.RecommendResultItem{
				VideoID:          11,
				VideoSegmentID:   101,
				UserReacted:      true,
				UserReactionType: videoapp.VideoReactionDoubleLike,
				StartTimeSec:     10,
				EndTimeSec:       40,
				TitleOverride:    "segment title",
				Video: domainvideo.Video{
					ID:          11,
					Title:       "video title",
					VideoURL:    "/videos/raw/2026/06/09/playable.mp4",
					CoverURL:    "/covers/11.jpg",
					Status:      domainvideo.StatusDone,
					IsPublished: true,
				},
			}, true, nil
		},
		resolvePlaybackURLFunc: func(_ context.Context, video domainvideo.Video) string {
			if video.ID != 11 {
				t.Fatalf("unexpected resolved video id %d", video.ID)
			}
			return "/videos/hls/2026/06/09/playable/master.m3u8"
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/video-segments/random-play?user_id=7", nil)
	router := gin.New()
	router.GET("/api/video-segments/random-play", h.RandomPlayVideoSegment)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.randomPlaySegmentCalls != 1 {
		t.Fatalf("expected one random segment call, got %d", stub.randomPlaySegmentCalls)
	}
	if stub.randomPlaySegmentInput.UserID != 7 {
		t.Fatalf("expected user_id 7 to be passed, got %d", stub.randomPlaySegmentInput.UserID)
	}
	assertBodyContains(t, w.Body.Bytes(), `"video_id":11`)
	assertBodyContains(t, w.Body.Bytes(), `"video_segment_id":101`)
	assertBodyContains(t, w.Body.Bytes(), `"start_time_sec":10`)
	assertBodyContains(t, w.Body.Bytes(), `"end_time_sec":40`)
	assertBodyContains(t, w.Body.Bytes(), `"title":"segment title"`)
	assertBodyContains(t, w.Body.Bytes(), `"cover_url":"/covers/11.jpg"`)
	assertBodyContains(t, w.Body.Bytes(), `"play_url":"/videos/hls/2026/06/09/playable/master.m3u8"`)
	assertBodyContains(t, w.Body.Bytes(), `"user_reacted":true`)
	assertBodyContains(t, w.Body.Bytes(), `"user_reaction_type":"double_like"`)
}

func TestExternalRecBoleRecommendations_ReturnsStringIDs(t *testing.T) {
	stub := &stubVideoApp{
		externalRecBoleFunc: func(_ context.Context, input videoapp.RandomPlayVideoSegmentInput) ([]uint64, error) {
			if input.UserID != 7 || input.Limit != 3 {
				t.Fatalf("input = %+v, want user 7 limit 3", input)
			}
			return []uint64{102, 101, 0, 103}, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/internal/recommendations/external/recbole?user_id=7&n=3", nil)
	router := gin.New()
	router.GET("/api/internal/recommendations/external/recbole", h.ExternalRecBoleRecommendations)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.externalRecBoleCalls != 1 {
		t.Fatalf("external calls = %d, want 1", stub.externalRecBoleCalls)
	}
	var ids []string
	if err := json.Unmarshal(w.Body.Bytes(), &ids); err != nil {
		t.Fatalf("decode ids: %v body=%s", err, w.Body.String())
	}
	want := []string{"102", "101", "103"}
	if len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestExternalRecBoleRecommendations_ReturnsEmptyForMissingUserID(t *testing.T) {
	stub := &stubVideoApp{}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/internal/recommendations/external/recbole", nil)
	router := gin.New()
	router.GET("/api/internal/recommendations/external/recbole", h.ExternalRecBoleRecommendations)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.externalRecBoleCalls != 0 {
		t.Fatalf("external calls = %d, want 0", stub.externalRecBoleCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `[]`)
}

func TestRandomPlayVideoSegment_DefaultsMissingUserIDToSix(t *testing.T) {
	stub := &stubVideoApp{
		randomPlaySegmentFunc: func(_ context.Context, input videoapp.RandomPlayVideoSegmentInput) (videoapp.RecommendResultItem, bool, error) {
			if input.UserID != 6 {
				t.Fatalf("expected missing user_id to default to 6, got %d", input.UserID)
			}
			return videoapp.RecommendResultItem{
				VideoID:        11,
				VideoSegmentID: 101,
				Video: domainvideo.Video{
					ID:          11,
					Title:       "video title",
					VideoURL:    "/videos/raw/2026/06/09/playable.mp4",
					Status:      domainvideo.StatusDone,
					IsPublished: true,
				},
			}, true, nil
		},
		resolvePlaybackURLFunc: func(context.Context, domainvideo.Video) string {
			return "/videos/hls/2026/06/09/playable/master.m3u8"
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/video-segments/random-play", nil)
	router := gin.New()
	router.GET("/api/video-segments/random-play", h.RandomPlayVideoSegment)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if stub.randomPlaySegmentInput.UserID != 6 {
		t.Fatalf("expected user_id 6 to be passed, got %d", stub.randomPlaySegmentInput.UserID)
	}
}

func TestRandomPlayVideoSegment_RejectsInvalidUserID(t *testing.T) {
	stub := &stubVideoApp{}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/video-segments/random-play?user_id=bad", nil)
	router := gin.New()
	router.GET("/api/video-segments/random-play", h.RandomPlayVideoSegment)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if stub.randomPlaySegmentCalls != 0 {
		t.Fatalf("expected no random segment call, got %d", stub.randomPlaySegmentCalls)
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
}

func TestRandomPlayVideoSegment_ReturnsNotFoundWhenNoSegmentExists(t *testing.T) {
	h := handler.NewVideoHandler(&stubVideoApp{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/video-segments/random-play", nil)
	router := gin.New()
	router.GET("/api/video-segments/random-play", h.RandomPlayVideoSegment)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"video_segment_not_found"`)
}

func TestListVideos_MapsServiceError(t *testing.T) {
	stub := &stubVideoApp{
		listVideosFunc: func(_ context.Context, filter videoapp.ListFilter) ([]domainvideo.Video, error) {
			return nil, errors.New("boom")
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	router := gin.New()
	router.GET("/api/videos", h.ListVideos)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"internal"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"list videos failed"`)
}

func TestUpdateVideoMetadata_MapsTypedValidationError(t *testing.T) {
	stub := &stubVideoApp{
		updateMetadataFunc: func(_ context.Context, videoID uint64, title string, description string) (bool, error) {
			return false, videoapp.InvalidArgumentError("title is required")
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/videos/11", bytes.NewBufferString(`{"title":"hello","description":"new desc"}`))
	req.Header.Set("Content-Type", "application/json")
	router := gin.New()
	router.PATCH("/api/videos/:id", h.UpdateVideoMetadata)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertBodyContains(t, w.Body.Bytes(), `"code":"invalid_argument"`)
	assertBodyContains(t, w.Body.Bytes(), `"message":"title is required"`)
}

func assertBodyContains(t *testing.T, body []byte, want string) {
	t.Helper()
	if !bytes.Contains(body, []byte(want)) {
		t.Fatalf("response body %s does not contain %s", string(body), want)
	}
}

func TestVideoJSONTimeFields(t *testing.T) {
	stub := &stubVideoApp{
		listVideosFunc: func(_ context.Context, filter videoapp.ListFilter) ([]domainvideo.Video, error) {
			return []domainvideo.Video{{
				ID:         30,
				Title:      "time-check",
				CreateTime: time.Unix(1714300000, 0),
				UpdateTime: time.Unix(1714300100, 0),
			}}, nil
		},
	}
	h := handler.NewVideoHandler(stub)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	router := gin.New()
	router.GET("/api/videos", h.ListVideos)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected data shape: %#v", payload["data"])
	}
	videos, ok := data["videos"].([]any)
	if !ok || len(videos) != 1 {
		t.Fatalf("unexpected videos shape: %#v", data["videos"])
	}
	video, ok := videos[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected video shape: %#v", videos[0])
	}
	if video["created_at_unix"] != float64(1714300000) {
		t.Fatalf("unexpected created_at_unix: %#v", video["created_at_unix"])
	}
	if video["updated_at_unix"] != float64(1714300100) {
		t.Fatalf("unexpected updated_at_unix: %#v", video["updated_at_unix"])
	}
}
