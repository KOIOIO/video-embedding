package handler

import (
	"github.com/gin-gonic/gin"

	"video-service/internal/application/usermessage"
	usermessagehandler "video-service/internal/http/handler/usermessages"
)

// UserMessageHandler 用户私信 HTTP 处理器外层代理。
type UserMessageHandler struct {
	inner *usermessagehandler.Handler
}

// NewUserMessageHandler 创建用户私信处理器。
func NewUserMessageHandler(service *usermessage.Service) *UserMessageHandler {
	return &UserMessageHandler{inner: usermessagehandler.New(service)}
}

// SendMessage 发送私信。
func (h *UserMessageHandler) SendMessage(c *gin.Context) {
	h.inner.SendMessage(c)
}

// GetConversations 获取会话列表。
func (h *UserMessageHandler) GetConversations(c *gin.Context) {
	h.inner.GetConversations(c)
}

// GetMessages 获取会话消息列表。
func (h *UserMessageHandler) GetMessages(c *gin.Context) {
	h.inner.GetMessages(c)
}

// MarkAsRead 标记会话消息已读。
func (h *UserMessageHandler) MarkAsRead(c *gin.Context) {
	h.inner.MarkAsRead(c)
}
