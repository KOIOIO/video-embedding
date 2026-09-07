package handler

import (
	"github.com/gin-gonic/gin"

	"video-service/internal/application/notification"
	notificationhandler "video-service/internal/http/handler/notifications"
)

// NotificationHandler 用户通知处理器外层代理。
type NotificationHandler struct {
	inner *notificationhandler.Handler
}

// NewNotificationHandler 创建用户通知处理器。
func NewNotificationHandler(service *notification.Service) *NotificationHandler {
	return &NotificationHandler{inner: notificationhandler.New(service)}
}

// ListNotifications 查询通知列表。
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	h.inner.ListNotifications(c)
}

// GetUnreadCount 获取未读通知数。
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	h.inner.GetUnreadCount(c)
}

// MarkAsRead 标记单条通知已读。
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	h.inner.MarkAsRead(c)
}

// MarkAllAsRead 标记所有通知已读。
func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	h.inner.MarkAllAsRead(c)
}
