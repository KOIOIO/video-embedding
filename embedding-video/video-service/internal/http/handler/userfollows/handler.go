package userfollows

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/userfollow"
	"video-service/internal/http/dto"
	httperrors "video-service/internal/http/errors"
)

// Handler 用户关注 HTTP 处理器内层实现。
type Handler struct {
	service *userfollow.Service
}

// New 创建用户关注处理器。
func New(service *userfollow.Service) *Handler {
	return &Handler{service: service}
}

// Follow godoc
// @Summary 关注用户
// @Tags 用户关注
// @Accept json
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Param id path int true "被关注用户ID"
// @Success 200 {object} dto.SuccessResponse[map[string]string]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/follow [post]
func (h *Handler) Follow(c *gin.Context) {
	// TODO: replace with real user authentication
	followerID, ok := currentUserID(c)
	if !ok {
		return
	}
	followingID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.Follow(c.Request.Context(), followerID, followingID); err != nil {
		if err == userfollow.ErrAlreadyFollowing {
			httperrors.Write(c, httperrors.InvalidArgument("already following"))
			return
		}
		if err == userfollow.ErrCannotFollowSelf {
			httperrors.Write(c, httperrors.InvalidArgument("cannot follow yourself"))
			return
		}
		httperrors.Write(c, httperrors.Internal("follow failed"))
		return
	}
	writeSuccess(c, gin.H{"status": "following"})
}

// Unfollow godoc
// @Summary 取消关注用户
// @Tags 用户关注
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Param id path int true "被取消关注用户ID"
// @Success 200 {object} dto.SuccessResponse[map[string]string]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/follow [delete]
func (h *Handler) Unfollow(c *gin.Context) {
	// TODO: replace with real user authentication
	followerID, ok := currentUserID(c)
	if !ok {
		return
	}
	followingID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.Unfollow(c.Request.Context(), followerID, followingID); err != nil {
		if err == userfollow.ErrCannotFollowSelf {
			httperrors.Write(c, httperrors.InvalidArgument("cannot unfollow yourself"))
			return
		}
		httperrors.Write(c, httperrors.Internal("unfollow failed"))
		return
	}
	writeSuccess(c, gin.H{"status": "none"})
}

// ListFollowing godoc
// @Summary 获取用户关注列表
// @Tags 用户关注
// @Produce json
// @Param id path int true "用户ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} dto.SuccessResponse[map[string]interface{}]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/following [get]
func (h *Handler) ListFollowing(c *gin.Context) {
	userID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	page, pageSize := parsePagination(c)
	list, err := h.service.ListFollowing(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("list following failed"))
		return
	}
	total, _ := h.service.CountFollowing(c.Request.Context(), userID)
	writeSuccess(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListFollowers godoc
// @Summary 获取用户粉丝列表
// @Tags 用户关注
// @Produce json
// @Param id path int true "用户ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} dto.SuccessResponse[map[string]interface{}]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/followers [get]
func (h *Handler) ListFollowers(c *gin.Context) {
	userID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	page, pageSize := parsePagination(c)
	list, err := h.service.ListFollowers(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("list followers failed"))
		return
	}
	total, _ := h.service.CountFollowers(c.Request.Context(), userID)
	writeSuccess(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetRelation godoc
// @Summary 获取当前用户与目标用户的关注关系
// @Tags 用户关注
// @Produce json
// @Param X-User-ID header string false "当前用户ID（未登录时可为空）"
// @Param id path int true "目标用户ID"
// @Success 200 {object} dto.SuccessResponse[map[string]string]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/{id}/relation [get]
func (h *Handler) GetRelation(c *gin.Context) {
	// TODO: replace with real user authentication
	userID := optionalUserID(c)
	targetID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	relation, err := h.service.GetRelation(c.Request.Context(), userID, targetID)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("get relation failed"))
		return
	}
	writeSuccess(c, gin.H{"relation": relation})
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

func optionalUserID(c *gin.Context) uint64 {
	raw := strings.TrimSpace(c.GetHeader("X-User-ID"))
	if raw == "" {
		return 0
	}
	userID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return userID
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

func parsePagination(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	if v := strings.TrimSpace(c.Query("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := strings.TrimSpace(c.Query("page_size")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func writeSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.SuccessResponse[T]{Success: true, Data: data})
}
