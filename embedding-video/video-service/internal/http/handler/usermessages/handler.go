package usermessages

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/usermessage"
	"video-service/internal/http/dto"
	httperrors "video-service/internal/http/errors"
)

// Handler 用户私信 HTTP 处理器内层实现。
type Handler struct {
	service *usermessage.Service
}

// New 创建用户私信处理器。
func New(service *usermessage.Service) *Handler {
	return &Handler{service: service}
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

// SendMessage godoc
// @Summary 发送私信
// @Tags 用户私信
// @Accept json
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Param userId path int true "接收方用户ID"
// @Param body body sendMessageRequest true "消息内容"
// @Success 200 {object} dto.SuccessResponse[map[string]interface{}]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/messages/{userId} [post]
func (h *Handler) SendMessage(c *gin.Context) {
	// TODO: replace with real user authentication
	senderID, ok := currentUserID(c)
	if !ok {
		return
	}
	receiverID, ok := parsePositiveUintParam(c, "userId")
	if !ok {
		return
	}
	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}
	msg, err := h.service.SendMessage(c.Request.Context(), senderID, receiverID, req.Content)
	if err != nil {
		switch err {
		case usermessage.ErrCannotMessageSelf:
			httperrors.Write(c, httperrors.InvalidArgument("cannot message yourself"))
		case usermessage.ErrEmptyContent:
			httperrors.Write(c, httperrors.InvalidArgument("message content cannot be empty"))
		case usermessage.ErrContentTooLong:
			httperrors.Write(c, httperrors.InvalidArgument("message content exceeds 2000 characters"))
		default:
			httperrors.Write(c, httperrors.Internal("send message failed"))
		}
		return
	}
	writeSuccess(c, gin.H{"message": msg})
}

// GetConversations godoc
// @Summary 获取会话列表
// @Tags 用户私信
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Success 200 {object} dto.SuccessResponse[map[string]interface{}]
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/messages/conversations [get]
func (h *Handler) GetConversations(c *gin.Context) {
	// TODO: replace with real user authentication
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	list, err := h.service.GetConversations(c.Request.Context(), userID)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("get conversations failed"))
		return
	}
	writeSuccess(c, gin.H{"list": list})
}

// GetMessages godoc
// @Summary 获取会话消息列表
// @Tags 用户私信
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Param userId path int true "对方用户ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} dto.SuccessResponse[map[string]interface{}]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/messages/{userId} [get]
func (h *Handler) GetMessages(c *gin.Context) {
	// TODO: replace with real user authentication
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	otherUserID, ok := parsePositiveUintParam(c, "userId")
	if !ok {
		return
	}
	page, pageSize := parsePagination(c)
	list, err := h.service.GetConversationMessages(c.Request.Context(), userID, otherUserID, page, pageSize)
	if err != nil {
		if err == usermessage.ErrNotConversationParticipant {
			httperrors.Write(c, httperrors.InvalidArgument("invalid conversation"))
			return
		}
		httperrors.Write(c, httperrors.Internal("get messages failed"))
		return
	}
	writeSuccess(c, gin.H{
		"list":      list,
		"page":      page,
		"page_size": pageSize,
	})
}

// MarkAsRead godoc
// @Summary 标记会话消息已读
// @Tags 用户私信
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Param userId path int true "对方用户ID"
// @Success 200 {object} dto.SuccessResponse[map[string]bool]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/messages/{userId}/read [post]
func (h *Handler) MarkAsRead(c *gin.Context) {
	// TODO: replace with real user authentication
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	otherUserID, ok := parsePositiveUintParam(c, "userId")
	if !ok {
		return
	}
	if err := h.service.MarkConversationAsRead(c.Request.Context(), userID, otherUserID); err != nil {
		if err == usermessage.ErrNotConversationParticipant {
			httperrors.Write(c, httperrors.InvalidArgument("invalid conversation"))
			return
		}
		httperrors.Write(c, httperrors.Internal("mark as read failed"))
		return
	}
	writeSuccess(c, gin.H{"success": true})
}

// --- helpers ---

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
