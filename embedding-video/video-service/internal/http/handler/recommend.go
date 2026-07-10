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
// @Summary 根据题目推荐视频
// @Tags 视频服务
// @Accept json
// @Produce json
// @Param request body dto.RecommendByQuestionRequest true "推荐请求参数"
// @Success 200 {object} dto.RecommendationListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/recommendations/by-question [post]
func (h *RecommendHandler) RecommendByQuestion(c *gin.Context) {
	h.inner.RecommendByQuestion(c)
}

// ListRecommendations godoc
// @Summary 查询推荐列表
// @Tags 视频服务
// @Produce json
// @Param question_id query int true "题目ID"
// @Param user_id query int false "用户ID"
// @Param limit query int false "返回数量"
// @Success 200 {object} dto.RecommendationListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/recommendations [get]
func (h *RecommendHandler) ListRecommendations(c *gin.Context) {
	h.inner.ListRecommendations(c)
}

// ReportWatch godoc
// @Summary 上报视频观看进度
// @Tags 视频服务
// @Accept json
// @Produce json
// @Param request body dto.ReportWatchRequest true "观看进度参数"
// @Success 200 {object} dto.WatchRecordResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/watch-records [post]
func (h *RecommendHandler) ReportWatch(c *gin.Context) {
	h.inner.ReportWatch(c)
}
