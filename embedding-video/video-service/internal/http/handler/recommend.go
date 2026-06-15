package handler

import (
	"github.com/gin-gonic/gin"

	recommendhandler "nlp-video-analysis/internal/http/handler/recommendations"
)

type RecommendHandler struct {
	inner *recommendhandler.Handler
}

func NewRecommendHandler(app any) *RecommendHandler {
	return &RecommendHandler{inner: recommendhandler.New(app)}
}

// RecommendByQuestion godoc
// @Summary Recommend videos by question
// @Tags recommendations
// @Accept json
// @Produce json
// @Param request body dto.RecommendByQuestionRequest true "Recommendation request"
// @Success 200 {object} dto.RecommendationListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/recommendations/by-question [post]
func (h *RecommendHandler) RecommendByQuestion(c *gin.Context) {
	h.inner.RecommendByQuestion(c)
}

// ListRecommendations godoc
// @Summary List recommendations
// @Tags recommendations
// @Produce json
// @Param question_id query int true "Question ID"
// @Param user_id query int false "User ID"
// @Param limit query int false "Result limit"
// @Success 200 {object} dto.RecommendationListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/recommendations [get]
func (h *RecommendHandler) ListRecommendations(c *gin.Context) {
	h.inner.ListRecommendations(c)
}

// ReportWatch godoc
// @Summary Report watch progress
// @Tags recommendations
// @Accept json
// @Produce json
// @Param request body dto.ReportWatchRequest true "Watch report request"
// @Success 200 {object} dto.WatchRecordResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/watch-records [post]
func (h *RecommendHandler) ReportWatch(c *gin.Context) {
	h.inner.ReportWatch(c)
}
