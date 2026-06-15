package system

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/http/dto"
	httperrors "nlp-video-analysis/internal/http/errors"
)

type Handler struct {
	app systemApp
}

type systemApp interface {
	GetSystemMetrics(ctx context.Context) (dto.SystemMetricsData, error)
}

func New(app any) *Handler {
	switch v := app.(type) {
	case systemApp:
		return &Handler{app: v}
	default:
		panic("unsupported system app")
	}
}

func (h *Handler) GetSystemMetrics(c *gin.Context) {
	metrics, err := h.app.GetSystemMetrics(c.Request.Context())
	if err != nil {
		httperrors.Write(c, httperrors.Internal("get system metrics failed"))
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse[dto.SystemMetricsData]{Success: true, Data: metrics})
}
