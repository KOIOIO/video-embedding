package usermessage

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"video-service/internal/infrastructure/persistence"
	"video-service/internal/model"
)

// ErrCannotMessageSelf 表示不能给自己发消息。
var ErrCannotMessageSelf = errors.New("cannot message yourself")

// ErrEmptyContent 表示消息内容为空。
var ErrEmptyContent = errors.New("message content cannot be empty")

// ErrContentTooLong 表示消息内容超过长度限制。
var ErrContentTooLong = errors.New("message content exceeds 2000 characters")

// ErrNotConversationParticipant 表示用户不是会话参与者。
var ErrNotConversationParticipant = errors.New("user is not a participant of this conversation")

const maxMessageContentLength = 2000

// MessageRepository 定义用户私信仓储接口。
type MessageRepository interface {
	CreateMessage(ctx context.Context, msg *model.EduUserMessage) error
	ListMessages(ctx context.Context, conversationID string, page, pageSize int) ([]model.EduUserMessage, error)
	MarkAsRead(ctx context.Context, conversationID string, userID uint64) error
	CountUnread(ctx context.Context, userID uint64) (int64, error)
	CountUnreadByConversation(ctx context.Context, conversationID string, userID uint64) (int64, error)
	ListConversations(ctx context.Context, userID uint64) ([]persistence.ConversationSummary, error)
}

// ConversationSummary 服务层会话摘要 DTO。
type ConversationSummary struct {
	ConversationID  string    `json:"conversation_id"`
	OtherUserID     uint64    `json:"other_user_id"`
	LastMessage     string    `json:"last_message"`
	LastMessageTime time.Time `json:"last_message_time"`
	UnreadCount     int64     `json:"unread_count"`
}

// Service 用户私信应用服务。
type Service struct {
	repo MessageRepository
}

// NewService 创建用户私信服务。
func NewService(repo MessageRepository) *Service {
	return &Service{repo: repo}
}

// SendMessage 发送私信消息。
func (s *Service) SendMessage(ctx context.Context, senderID, receiverID uint64, content string) (*model.EduUserMessage, error) {
	if senderID == receiverID {
		return nil, ErrCannotMessageSelf
	}
	trimmed := strings.TrimSpace(content)
	if utf8.RuneCountInString(trimmed) == 0 {
		return nil, ErrEmptyContent
	}
	if utf8.RuneCountInString(trimmed) > maxMessageContentLength {
		return nil, ErrContentTooLong
	}
	msg := &model.EduUserMessage{
		SenderID:       senderID,
		ReceiverID:     receiverID,
		ConversationID: model.ConversationID(senderID, receiverID),
		Content:        trimmed,
		IsRead:         false,
		Deleted:        0,
	}
	if err := s.repo.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// GetConversationMessages 获取会话消息列表。
func (s *Service) GetConversationMessages(ctx context.Context, userID, otherUserID uint64, page, pageSize int) ([]model.EduUserMessage, error) {
	conversationID := model.ConversationID(userID, otherUserID)
	// 校验 userID 是会话参与者：conversation_id 由 min(userID, otherUserID) 拼接，
	// 只要 userID != 0 且 otherUserID != 0，userID 必然是参与者之一。
	if userID == 0 || otherUserID == 0 || userID == otherUserID {
		return nil, ErrNotConversationParticipant
	}
	return s.repo.ListMessages(ctx, conversationID, page, pageSize)
}

// GetConversations 获取用户的会话列表。
func (s *Service) GetConversations(ctx context.Context, userID uint64) ([]ConversationSummary, error) {
	repoList, err := s.repo.ListConversations(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]ConversationSummary, 0, len(repoList))
	for _, item := range repoList {
		result = append(result, ConversationSummary{
			ConversationID:  item.ConversationID,
			OtherUserID:     item.OtherUserID,
			LastMessage:     item.LastMessage,
			LastMessageTime: item.LastMessageTime,
			UnreadCount:     item.UnreadCount,
		})
	}
	return result, nil
}

// MarkConversationAsRead 标记指定会话中用户收到的消息为已读。
func (s *Service) MarkConversationAsRead(ctx context.Context, userID, otherUserID uint64) error {
	if userID == 0 || otherUserID == 0 || userID == otherUserID {
		return ErrNotConversationParticipant
	}
	conversationID := model.ConversationID(userID, otherUserID)
	return s.repo.MarkAsRead(ctx, conversationID, userID)
}

// GetTotalUnread 获取用户总未读数。
func (s *Service) GetTotalUnread(ctx context.Context, userID uint64) (int64, error) {
	return s.repo.CountUnread(ctx, userID)
}
