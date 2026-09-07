package handler

import (
	"github.com/gin-gonic/gin"

	"video-service/internal/application/userfollow"
	userfollowhandler "video-service/internal/http/handler/userfollows"
)

// UserFollowHandler 用户关注 HTTP 处理器外层代理。
type UserFollowHandler struct {
	inner *userfollowhandler.Handler
}

// NewUserFollowHandler 创建用户关注处理器。
func NewUserFollowHandler(service *userfollow.Service) *UserFollowHandler {
	return &UserFollowHandler{inner: userfollowhandler.New(service)}
}

// Follow 关注用户。
func (h *UserFollowHandler) Follow(c *gin.Context) {
	h.inner.Follow(c)
}

// Unfollow 取消关注用户。
func (h *UserFollowHandler) Unfollow(c *gin.Context) {
	h.inner.Unfollow(c)
}

// ListFollowing 获取用户关注列表。
func (h *UserFollowHandler) ListFollowing(c *gin.Context) {
	h.inner.ListFollowing(c)
}

// ListFollowers 获取用户粉丝列表。
func (h *UserFollowHandler) ListFollowers(c *gin.Context) {
	h.inner.ListFollowers(c)
}

// GetRelation 获取当前用户与目标用户的关注关系。
func (h *UserFollowHandler) GetRelation(c *gin.Context) {
	h.inner.GetRelation(c)
}
