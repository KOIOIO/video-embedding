package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Healthz godoc
// @Summary 查询服务健康状态
// @Tags 系统与健康
// @Produce json
// @Success 200 {object} dto.HealthResponse
// @Router /api/healthz [get]
func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
