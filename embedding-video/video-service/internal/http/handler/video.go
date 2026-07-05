package handler

import (
	"github.com/gin-gonic/gin"

	videohandler "nlp-video-analysis/internal/http/handler/videos"
)

type VideoHandler struct {
	inner *videohandler.Handler
}

func NewVideoHandler(app any) *VideoHandler {
	return &VideoHandler{inner: videohandler.New(app)}
}

// ListVideos godoc
// @Summary List videos
// @Tags videos
// @Produce json
// @Param type query string false "Filter type" Enums(ALL,RAW,HLS) default(ALL)
// @Success 200 {object} dto.VideoListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos [get]
func (h *VideoHandler) ListVideos(c *gin.Context) {
	h.inner.ListVideos(c)
}

// UpdateVideoMetadata godoc
// @Summary Update video metadata
// @Tags videos
// @Accept json
// @Produce json
// @Param id path int true "Video ID"
// @Param request body dto.UpdateVideoRequest true "Updated metadata"
// @Success 200 {object} dto.UpdateVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id} [patch]
func (h *VideoHandler) UpdateVideoMetadata(c *gin.Context) {
	h.inner.UpdateVideoMetadata(c)
}

// DeleteVideo godoc
// @Summary Delete a video
// @Tags videos
// @Produce json
// @Param id path int true "Video ID"
// @Success 200 {object} dto.DeleteVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id} [delete]
func (h *VideoHandler) DeleteVideo(c *gin.Context) {
	h.inner.DeleteVideo(c)
}

// PlayVideo godoc
// @Summary Resolve a video's playback URL
// @Tags videos
// @Produce json
// @Param id path int true "Video ID"
// @Success 200 {object} dto.PlayVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/play [get]
func (h *VideoHandler) PlayVideo(c *gin.Context) {
	h.inner.PlayVideo(c)
}

// GetSimilarVideos godoc
// @Summary List similar videos
// @Tags videos
// @Produce json
// @Param id path int true "Video ID"
// @Param limit query int false "Result limit" default(6)
// @Success 200 {object} dto.SimilarVideosResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/similar [get]
func (h *VideoHandler) GetSimilarVideos(c *gin.Context) {
	h.inner.GetSimilarVideos(c)
}

// GetViewCount godoc
// @Summary Get a video's view count
// @Tags videos
// @Produce json
// @Param id path int true "Video ID"
// @Success 200 {object} dto.ViewCountResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/view-count [get]
func (h *VideoHandler) GetViewCount(c *gin.Context) {
	h.inner.GetViewCount(c)
}

// SetVideoPublished godoc
// @Summary Set video published status
// @Tags videos
// @Accept json
// @Produce json
// @Param id path int true "Video ID"
// @Param request body dto.PublishVideoRequest true "Publish request"
// @Success 200 {object} dto.PublishVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/publish [post]
func (h *VideoHandler) SetVideoPublished(c *gin.Context) {
	h.inner.SetVideoPublished(c)
}

// SetVideoRecommend godoc
// @Summary Set video recommendation status
// @Tags videos
// @Accept json
// @Produce json
// @Param id path int true "Video ID"
// @Param request body dto.RecommendVideoRequest true "Recommendation request"
// @Success 200 {object} dto.RecommendVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/recommend [post]
func (h *VideoHandler) SetVideoRecommend(c *gin.Context) {
	h.inner.SetVideoRecommend(c)
}

// SubmitVideoReaction godoc
// @Summary Submit or cancel a video reaction
// @Tags videos
// @Accept json
// @Produce json
// @Param id path int true "Video ID"
// @Param request body dto.VideoReactionRequest true "Reaction request"
// @Success 200 {object} dto.VideoReactionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/reactions [post]
func (h *VideoHandler) SubmitVideoReaction(c *gin.Context) {
	h.inner.SubmitVideoReaction(c)
}

// GetVideoReactionCounts godoc
// @Summary Get a video's like and double-like counts
// @Tags videos
// @Produce json
// @Param id path int true "Video ID"
// @Success 200 {object} dto.VideoReactionCountsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/reaction-counts [get]
func (h *VideoHandler) GetVideoReactionCounts(c *gin.Context) {
	h.inner.GetVideoReactionCounts(c)
}

// SubmitSegmentReaction godoc
// @Summary Submit or cancel a video segment reaction
// @Tags videos
// @Accept json
// @Produce json
// @Param id path int true "Video segment ID"
// @Param request body dto.VideoReactionRequest true "Reaction request"
// @Success 200 {object} dto.SegmentReactionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/video-segments/{id}/reactions [post]
func (h *VideoHandler) SubmitSegmentReaction(c *gin.Context) {
	h.inner.SubmitSegmentReaction(c)
}

// GetSegmentReactionCounts godoc
// @Summary Get a video segment's like and double-like counts
// @Tags videos
// @Produce json
// @Param id path int true "Video segment ID"
// @Success 200 {object} dto.SegmentReactionCountsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/video-segments/{id}/reaction-counts [get]
func (h *VideoHandler) GetSegmentReactionCounts(c *gin.Context) {
	h.inner.GetSegmentReactionCounts(c)
}

// RandomPlayVideoSegment godoc
// @Summary Random play video segment
// @Tags videos
// @Produce json
// @Param user_id query int false "User ID for two-tower personalized recommendation"
// @Success 200 {object} dto.RandomVideoSegmentResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/video-segments/random-play [get]
func (h *VideoHandler) RandomPlayVideoSegment(c *gin.Context) {
	h.inner.RandomPlayVideoSegment(c)
}

// ExternalTwoTowerRecommendations godoc
// @Summary Get two-tower item IDs for Gorse external recommender
// @Tags internal
// @Produce json
// @Param user_id query int true "User ID"
// @Param n query int false "Number of item IDs, capped at 500"
// @Success 200 {array} string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/internal/recommendations/external/two-tower [get]
func (h *VideoHandler) ExternalTwoTowerRecommendations(c *gin.Context) {
	h.inner.ExternalTwoTowerRecommendations(c)
}

// GetTranscodeStatus godoc
// @Summary Get transcode task status
// @Tags videos
// @Produce json
// @Param taskId path string true "Transcode task ID"
// @Success 200 {object} dto.TranscodeStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/transcode-tasks/{taskId} [get]
func (h *VideoHandler) GetTranscodeStatus(c *gin.Context) {
	h.inner.GetTranscodeStatus(c)
}
