package userprofiles

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/userprofile"
	"video-service/internal/http/dto"
	httperrors "video-service/internal/http/errors"
	"video-service/internal/infrastructure/objectstorage"
)

const (
	avatarMaxBytes   = 2 << 20 // 2MB
	avatarObjectDir  = "avatars"
	avatarURLPrefix  = "/videos" // 复用现有对象存储代理路由
)

var allowedAvatarContentTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// Handler 用户资料 HTTP 处理器内层实现。
type Handler struct {
	service *userprofile.Service
	store   *objectstorage.RustFS
}

// New 创建用户资料处理器。
func New(service *userprofile.Service, store *objectstorage.RustFS) *Handler {
	return &Handler{service: service, store: store}
}

type updateProfileRequest struct {
	Nickname string `json:"nickname"`
	Bio      string `json:"bio"`
	Location string `json:"location"`
	Gender   int16  `json:"gender"`
}

// GetProfile godoc
// @Summary 获取用户资料
// @Tags 用户资料
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} dto.SuccessResponse[model.EduUserProfile]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/profile [get]
func (h *Handler) GetProfile(c *gin.Context) {
	userID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	profile, err := h.service.GetProfile(userID)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("get user profile failed"))
		return
	}
	writeSuccess(c, profile)
}

// GetMe godoc
// @Summary 获取当前用户资料
// @Tags 用户资料
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Success 200 {object} dto.SuccessResponse[model.EduUserProfile]
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/me [get]
func (h *Handler) GetMe(c *gin.Context) {
	// TODO: replace with real user authentication
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	profile, err := h.service.GetProfile(userID)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("get current user profile failed"))
		return
	}
	writeSuccess(c, profile)
}

// UpdateProfile godoc
// @Summary 更新当前用户资料
// @Tags 用户资料
// @Accept json
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Param request body updateProfileRequest true "资料更新请求"
// @Success 200 {object} dto.SuccessResponse[map[string]interface{}]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/me/profile [put]
func (h *Handler) UpdateProfile(c *gin.Context) {
	// TODO: replace with real user authentication
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}
	if err := h.service.UpdateProfile(userID, req.Nickname, req.Bio, req.Location, req.Gender); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument(err.Error()))
		return
	}
	writeSuccess(c, gin.H{"updated": true})
}

// UploadAvatar godoc
// @Summary 上传当前用户头像
// @Tags 用户资料
// @Accept multipart/form-data
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Param avatar formData file true "头像图片（jpg/png/gif/webp，≤2MB）"
// @Success 200 {object} dto.SuccessResponse[map[string]string]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/me/avatar [post]
func (h *Handler) UploadAvatar(c *gin.Context) {
	// TODO: replace with real user authentication
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	file, header, ok := readAvatarFile(c)
	if !ok {
		return
	}
	defer file.Close()

	ext, ok := validateAvatar(c, header)
	if !ok {
		return
	}

	objectKey := fmt.Sprintf("%s/%d/%d%s", avatarObjectDir, userID, time.Now().UnixNano(), ext)
	if err := h.store.Put(c.Request.Context(), objectKey, file, header.Size, header.Header.Get("Content-Type")); err != nil {
		httperrors.Write(c, httperrors.Internal("upload avatar failed"))
		return
	}

	avatarURL := strings.TrimRight(avatarURLPrefix, "/") + "/" + objectKey
	if err := h.service.UpdateAvatar(userID, avatarURL); err != nil {
		httperrors.Write(c, httperrors.Internal("update avatar failed"))
		return
	}
	writeSuccess(c, gin.H{"avatar_url": avatarURL})
}

func currentUserID(c *gin.Context) (uint64, bool) {
	raw := strings.TrimSpace(c.GetHeader("X-User-ID"))
	if raw == "" {
		httperrors.Write(c, &httperrors.APIError{
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "X-User-ID header is required",
		})
		return 0, false
	}
	userID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || userID == 0 {
		httperrors.Write(c, httperrors.InvalidArgument("X-User-ID must be a positive integer"))
		return 0, false
	}
	return userID, true
}

func readAvatarFile(c *gin.Context) (multipart.File, *multipart.FileHeader, bool) {
	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		if err == http.ErrMissingFile || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			httperrors.Write(c, httperrors.InvalidArgument("avatar file is required"))
			return nil, nil, false
		}
		httperrors.Write(c, httperrors.InvalidArgument("invalid multipart form"))
		return nil, nil, false
	}
	if header == nil || strings.TrimSpace(header.Filename) == "" {
		_ = file.Close()
		httperrors.Write(c, httperrors.InvalidArgument("avatar file is required"))
		return nil, nil, false
	}
	return file, header, true
}

func validateAvatar(c *gin.Context, header *multipart.FileHeader) (string, bool) {
	if header.Size > avatarMaxBytes {
		httperrors.Write(c, httperrors.InvalidArgument("avatar size must be at most 2MB"))
		return "", false
	}
	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	ext, ok := allowedAvatarContentTypes[contentType]
	if !ok {
		// fallback to file extension
		ext = strings.ToLower(filepath.Ext(header.Filename))
		if _, ok := allowedAvatarContentTypes["image"+ext]; !ok {
			httperrors.Write(c, httperrors.InvalidArgument("avatar must be jpg, png, gif, or webp"))
			return "", false
		}
	}
	return ext, true
}

func parsePositiveUintParam(c *gin.Context, name string) (uint64, bool) {
	raw := strings.TrimSpace(c.Param(name))
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		httperrors.Write(c, httperrors.InvalidArgument(name+" must be a positive integer"))
		return 0, false
	}
	return value, true
}

func writeSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.SuccessResponse[T]{Success: true, Data: data})
}
