package handler

import (
	"github.com/gin-gonic/gin"

	systemhandler "video-service/internal/http/handler/system"
)

type SystemHandler struct {
	inner *systemhandler.Handler
}

func NewSystemHandler(app any) *SystemHandler {
	return &SystemHandler{inner: systemhandler.New(app)}
}

// GetSystemMetrics godoc
// @Summary 查询系统运行指标
// @Tags 系统与健康
// @Produce json
// @Success 200 {object} dto.SystemMetricsResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/system/metrics [get]
func (h *SystemHandler) GetSystemMetrics(c *gin.Context) {
	h.inner.GetSystemMetrics(c)
}
