package handler

import (
	"github.com/gin-gonic/gin"

	recommendationadminhandler "nlp-video-analysis/internal/http/handler/recommendationadmin"
)

type RecommendationAdminHandler struct {
	inner *recommendationadminhandler.Handler
}

func NewRecommendationAdminHandler(app any) *RecommendationAdminHandler {
	return &RecommendationAdminHandler{inner: recommendationadminhandler.New(app)}
}

func (h *RecommendationAdminHandler) Overview(c *gin.Context) {
	h.inner.Overview(c)
}

func (h *RecommendationAdminHandler) Diagnostics(c *gin.Context) {
	h.inner.Diagnostics(c)
}

func (h *RecommendationAdminHandler) Datasources(c *gin.Context) {
	h.inner.Datasources(c)
}

func (h *RecommendationAdminHandler) Effects(c *gin.Context) {
	h.inner.Effects(c)
}

func (h *RecommendationAdminHandler) TraceRandomPlay(c *gin.Context) {
	h.inner.TraceRandomPlay(c)
}

func (h *RecommendationAdminHandler) TraceByQuestion(c *gin.Context) {
	h.inner.TraceByQuestion(c)
}

func (h *RecommendationAdminHandler) RedisState(c *gin.Context) {
	h.inner.RedisState(c)
}

func (h *RecommendationAdminHandler) PreviewRandomPlay(c *gin.Context) {
	h.inner.PreviewRandomPlay(c)
}

func (h *RecommendationAdminHandler) PreviewByQuestion(c *gin.Context) {
	h.inner.PreviewByQuestion(c)
}
