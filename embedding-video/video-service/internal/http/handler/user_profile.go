package handler

import (
	"github.com/gin-gonic/gin"

	userprofilehandler "video-service/internal/http/handler/userprofiles"
	"video-service/internal/application/userprofile"
	"video-service/internal/infrastructure/objectstorage"
)

// UserProfileHandler 用户资料 HTTP 处理器外层代理。
type UserProfileHandler struct {
	inner *userprofilehandler.Handler
}

// NewUserProfileHandler 创建用户资料处理器。
func NewUserProfileHandler(service *userprofile.Service, store *objectstorage.RustFS) *UserProfileHandler {
	return &UserProfileHandler{inner: userprofilehandler.New(service, store)}
}

// GetProfile 获取指定用户的资料。
func (h *UserProfileHandler) GetProfile(c *gin.Context) {
	h.inner.GetProfile(c)
}

// UpdateProfile 更新当前用户资料。
func (h *UserProfileHandler) UpdateProfile(c *gin.Context) {
	h.inner.UpdateProfile(c)
}

// UploadAvatar 上传当前用户头像。
func (h *UserProfileHandler) UploadAvatar(c *gin.Context) {
	h.inner.UploadAvatar(c)
}
