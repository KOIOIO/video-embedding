package handler

import (
	"github.com/gin-gonic/gin"

	"video-service/internal/application/profilevisit"
	profilevisithandler "video-service/internal/http/handler/profilevisits"
)

// ProfileVisitHandler 用户主页访问 HTTP 处理器外层代理。
type ProfileVisitHandler struct {
	inner *profilevisithandler.Handler
}

// NewProfileVisitHandler 创建用户主页访问处理器。
func NewProfileVisitHandler(service *profilevisit.Service) *ProfileVisitHandler {
	return &ProfileVisitHandler{inner: profilevisithandler.New(service)}
}

// RecordVisit 记录主页访问。
func (h *ProfileVisitHandler) RecordVisit(c *gin.Context) {
	h.inner.RecordVisit(c)
}

// GetMyVisits 获取我的主页访问统计。
func (h *ProfileVisitHandler) GetMyVisits(c *gin.Context) {
	h.inner.GetMyVisits(c)
}
