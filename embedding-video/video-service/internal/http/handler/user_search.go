package handler

import (
	"github.com/gin-gonic/gin"

	"video-service/internal/application/usersearch"
	usersearchhandler "video-service/internal/http/handler/usersearches"
)

// UserSearchHandler 用户搜索 HTTP 处理器外层代理。
type UserSearchHandler struct {
	inner *usersearchhandler.Handler
}

// NewUserSearchHandler 创建用户搜索处理器。
func NewUserSearchHandler(service *usersearch.Service) *UserSearchHandler {
	return &UserSearchHandler{inner: usersearchhandler.New(service)}
}

// SearchUsers 搜索用户（评论 @提及用）。
func (h *UserSearchHandler) SearchUsers(c *gin.Context) {
	h.inner.SearchUsers(c)
}
