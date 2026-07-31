package handler

import (
	"github.com/gin-gonic/gin"

	recommendationadminhandler "video-service/internal/http/handler/recommendationadmin"
)

type RecommendationAdminHandler struct {
	inner *recommendationadminhandler.Handler
}

func NewRecommendationAdminHandler(app any) *RecommendationAdminHandler {
	return &RecommendationAdminHandler{inner: recommendationadminhandler.New(app)}
}

// Overview godoc
// @Summary 查询推荐系统概览
// @Tags 推荐管理
// @Produce json
// @Success 200 {object} dto.RecommendationAdminOverviewResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/overview [get]
func (h *RecommendationAdminHandler) Overview(c *gin.Context) {
	h.inner.Overview(c)
}

// Diagnostics godoc
// @Summary 查询推荐系统诊断信息
// @Tags 推荐管理
// @Produce json
// @Param days query int false "统计天数" default(14)
// @Param limit query int false "最近请求数量"
// @Success 200 {object} dto.RecommendationDiagnosticsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/diagnostics [get]
func (h *RecommendationAdminHandler) Diagnostics(c *gin.Context) {
	h.inner.Diagnostics(c)
}

// Datasources godoc
// @Summary 查询推荐数据源统计
// @Tags 推荐管理
// @Produce json
// @Success 200 {object} dto.RecommendationDatasourceStatsResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/datasources [get]
func (h *RecommendationAdminHandler) Datasources(c *gin.Context) {
	h.inner.Datasources(c)
}

// Effects godoc
// @Summary 查询推荐命中效果
// @Tags 推荐管理
// @Produce json
// @Param days query int false "统计天数" default(14)
// @Success 200 {object} dto.RecommendationEffectMetricsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/effects [get]
func (h *RecommendationAdminHandler) Effects(c *gin.Context) {
	h.inner.Effects(c)
}

// RecBolePerformance godoc
// @Summary 查询 RecBole 性能指标
// @Tags 推荐管理
// @Produce json
// @Param metric query string true "指标名称"
// @Param begin query string true "开始时间，RFC3339"
// @Param end query string true "结束时间，RFC3339"
// @Success 200 {object} dto.RecommendationRecBolePerformanceResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/recbole/performance [get]
func (h *RecommendationAdminHandler) RecBolePerformance(c *gin.Context) {
	h.inner.RecBolePerformance(c)
}

// TraceRandomPlay godoc
// @Summary 追踪随机播放推荐链路
// @Tags 推荐管理
// @Produce json
// @Param user_id query int true "用户ID"
// @Param limit query int false "返回数量"
// @Success 200 {object} dto.RecommendationTraceResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/trace/random-play [get]
func (h *RecommendationAdminHandler) TraceRandomPlay(c *gin.Context) {
	h.inner.TraceRandomPlay(c)
}

// TraceByQuestion godoc
// @Summary 追踪按题目推荐链路
// @Tags 推荐管理
// @Accept json
// @Produce json
// @Param request body dto.RecommendByQuestionRequest true "推荐请求参数"
// @Success 200 {object} dto.RecommendationTraceResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/trace/by-question [post]
func (h *RecommendationAdminHandler) TraceByQuestion(c *gin.Context) {
	h.inner.TraceByQuestion(c)
}

// RedisState godoc
// @Summary 查询用户推荐 Redis 状态
// @Tags 推荐管理
// @Produce json
// @Param user_id query int true "用户ID"
// @Success 200 {object} dto.RecommendationRedisStateResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/redis-state [get]
func (h *RecommendationAdminHandler) RedisState(c *gin.Context) {
	h.inner.RedisState(c)
}

// PreviewRandomPlay godoc
// @Summary 预览随机播放推荐结果
// @Tags 推荐管理
// @Produce json
// @Param user_id query int true "用户ID"
// @Param limit query int false "返回数量"
// @Success 200 {object} dto.RecommendationAdminPreviewListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/preview/random-play [get]
func (h *RecommendationAdminHandler) PreviewRandomPlay(c *gin.Context) {
	h.inner.PreviewRandomPlay(c)
}

// PreviewByQuestion godoc
// @Summary 预览按题目推荐结果
// @Tags 推荐管理
// @Accept json
// @Produce json
// @Param request body dto.RecommendByQuestionRequest true "推荐请求参数"
// @Success 200 {object} dto.RecommendationAdminPreviewListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/admin/recommendation/preview/by-question [post]
func (h *RecommendationAdminHandler) PreviewByQuestion(c *gin.Context) {
	h.inner.PreviewByQuestion(c)
}
