package notifications

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/notification"
	"video-service/internal/http/dto"
	httperrors "video-service/internal/http/errors"
	"video-service/internal/infrastructure/persistence"
	"video-service/middleware"
)

// Handler 用户通知 HTTP 处理器内层实现。
type Handler struct {
	service *notification.Service
}

// New 创建用户通知处理器。
func New(service *notification.Service) *Handler {
	return &Handler{service: service}
}

// NotificationData 通知返回数据。
type NotificationData struct {
	ID               uint64 `json:"id"`
	Type             string `json:"type"`
	FromUserID       uint64 `json:"from_user_id"`
	FromUserNickname string `json:"from_user_nickname"`
	FromUserAvatar   string `json:"from_user_avatar"`
	VideoID          uint64 `json:"video_id"`
	VideoSegmentID   uint64 `json:"video_segment_id"`
	CommentID        uint64 `json:"comment_id"`
	Content          string `json:"content"`
	IsRead           int16  `json:"is_read"`
	CreatedAtUnix    int64  `json:"created_at_unix"`
}

// NotificationListData 通知列表返回数据。
type NotificationListData struct {
	Total         int64              `json:"total"`
	Page          int                `json:"page"`
	PageSize      int                `json:"page_size"`
	Notifications []NotificationData `json:"notifications"`
}

// ListNotifications godoc
// @Summary 查询通知列表
// @Tags 用户通知
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} dto.SuccessResponse[NotificationListData]
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/notifications [get]
func (h *Handler) ListNotifications(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	page := parsePositiveIntQuery(c, "page", 1)
	pageSize := parsePositiveIntQuery(c, "page_size", notification.DefaultPageSize)
	if pageSize > notification.MaxPageSize {
		pageSize = notification.MaxPageSize
	}
	result, err := h.service.ListNotifications(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("list notifications failed"))
		return
	}
	writeSuccess(c, NotificationListData{
		Total:         result.Total,
		Page:          page,
		PageSize:      pageSize,
		Notifications: mapNotificationViews(result.Notifications),
	})
}

// GetUnreadCount godoc
// @Summary 获取未读通知数
// @Tags 用户通知
// @Produce json
// @Success 200 {object} dto.SuccessResponse[map[string]int64]
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/notifications/unread-count [get]
func (h *Handler) GetUnreadCount(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	count, err := h.service.GetUnreadCount(c.Request.Context(), userID)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("get unread count failed"))
		return
	}
	writeSuccess(c, gin.H{"unread_count": count})
}

// MarkAsRead godoc
// @Summary 标记单条通知已读
// @Tags 用户通知
// @Produce json
// @Param id path int true "通知ID"
// @Success 200 {object} dto.SuccessResponse[map[string]bool]
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/notifications/{id}/read [post]
func (h *Handler) MarkAsRead(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	err := h.service.MarkAsRead(c.Request.Context(), id, userID)
	if err != nil {
		if errors.Is(err, notification.ErrNotificationNotFound) {
			httperrors.Write(c, httperrors.NotFound("notification_not_found", "notification does not exist"))
			return
		}
		httperrors.Write(c, httperrors.Internal("mark notification as read failed"))
		return
	}
	writeSuccess(c, gin.H{"success": true})
}

// MarkAllAsRead godoc
// @Summary 标记所有通知已读
// @Tags 用户通知
// @Produce json
// @Success 200 {object} dto.SuccessResponse[map[string]bool]
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/notifications/read-all [post]
func (h *Handler) MarkAllAsRead(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	if err := h.service.MarkAllAsRead(c.Request.Context(), userID); err != nil {
		httperrors.Write(c, httperrors.Internal("mark all notifications as read failed"))
		return
	}
	writeSuccess(c, gin.H{"success": true})
}

// --- helpers ---

func currentUserID(c *gin.Context) (uint64, bool) {
	userID, ok := middleware.UserID(c)
	if !ok {
		httperrors.Write(c, httperrors.InvalidArgument("user_id is required"))
		return 0, false
	}
	return userID, true
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

func parsePositiveIntQuery(c *gin.Context, name string, fallback int) int {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func writeSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.SuccessResponse[T]{Success: true, Data: data})
}

func mapNotificationViews(views []persistence.NotificationView) []NotificationData {
	out := make([]NotificationData, 0, len(views))
	for _, v := range views {
		out = append(out, NotificationData{
			ID:               v.ID,
			Type:             v.Type,
			FromUserID:       v.FromUserID,
			FromUserNickname: v.FromUserNickname,
			FromUserAvatar:   v.FromUserAvatar,
			VideoID:          v.VideoID,
			VideoSegmentID:   v.VideoSegmentID,
			CommentID:        v.CommentID,
			Content:          v.Content,
			IsRead:           v.IsRead,
			CreatedAtUnix:    v.CreatedAt.Unix(),
		})
	}
	return out
}
