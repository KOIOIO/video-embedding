package handler

import (
	"github.com/gin-gonic/gin"

	"video-service/internal/application/userpublish"
	userpublishhandler "video-service/internal/http/handler/userpublish"
	"video-service/internal/infrastructure/objectstorage"
)

// UserPublishHandler 用户发布视频 HTTP 处理器外层代理。
type UserPublishHandler struct {
	inner *userpublishhandler.Handler
}

// NewUserPublishHandler 创建用户发布视频处理器。
func NewUserPublishHandler(service *userpublish.Service, store *objectstorage.RustFS, rawURLPrefix string) *UserPublishHandler {
	return &UserPublishHandler{inner: userpublishhandler.New(service, store, rawURLPrefix)}
}

// PublishVideo 发布视频。
func (h *UserPublishHandler) PublishVideo(c *gin.Context) {
	h.inner.PublishVideo(c)
}

// GetVideoStatus 查询视频处理状态。
func (h *UserPublishHandler) GetVideoStatus(c *gin.Context) {
	h.inner.GetVideoStatus(c)
}

// ListUserVideos 查询用户作品列表。
func (h *UserPublishHandler) ListUserVideos(c *gin.Context) {
	h.inner.ListUserVideos(c)
}
