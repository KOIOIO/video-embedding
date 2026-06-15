package handler

import (
	"github.com/gin-gonic/gin"

	systemhandler "nlp-video-analysis/internal/http/handler/system"
)

type SystemHandler struct {
	inner *systemhandler.Handler
}

func NewSystemHandler(app any) *SystemHandler {
	return &SystemHandler{inner: systemhandler.New(app)}
}

func (h *SystemHandler) GetSystemMetrics(c *gin.Context) {
	h.inner.GetSystemMetrics(c)
}
