package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"video-service/internal/model"
)

// ConversationSummary 会话摘要查询结果。
type ConversationSummary struct {
	ConversationID  string    `gorm:"column:conversation_id" json:"conversation_id"`
	OtherUserID     uint64    `gorm:"column:other_user_id" json:"other_user_id"`
	LastMessage     string    `gorm:"column:last_message" json:"last_message"`
	LastMessageTime time.Time `gorm:"column:last_message_time" json:"last_message_time"`
	UnreadCount     int64     `gorm:"column:unread_count" json:"unread_count"`
}

// GormUserMessageRepository 用户私信仓储的 GORM 实现。
type GormUserMessageRepository struct {
	db *gorm.DB
}

// NewGormUserMessageRepository 创建用户私信仓储。
func NewGormUserMessageRepository(db *gorm.DB) *GormUserMessageRepository {
	return &GormUserMessageRepository{db: db}
}

// WithTx 返回绑定到指定事务的仓储实例。
func (r *GormUserMessageRepository) WithTx(tx *gorm.DB) *GormUserMessageRepository {
	return &GormUserMessageRepository{db: tx}
}

// CreateMessage 插入一条私信消息。
func (r *GormUserMessageRepository) CreateMessage(ctx context.Context, msg *model.EduUserMessage) error {
	return r.db.WithContext(ctx).Create(msg).Error
}

// ListMessages 按会话查询消息列表，按 create_time 倒序分页。
func (r *GormUserMessageRepository) ListMessages(ctx context.Context, conversationID string, page, pageSize int) ([]model.EduUserMessage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	var results []model.EduUserMessage
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND deleted = ?", conversationID, 0).
		Order("create_time DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&results).Error
	return results, err
}

// MarkAsRead 将指定会话中 receiver_id=userID 的未读消息标记为已读。
func (r *GormUserMessageRepository) MarkAsRead(ctx context.Context, conversationID string, userID uint64) error {
	return r.db.WithContext(ctx).
		Model(&model.EduUserMessage{}).
		Where("conversation_id = ? AND receiver_id = ? AND is_read = ? AND deleted = ?", conversationID, userID, false, 0).
		Update("is_read", true).Error
}

// CountUnread 查询用户总未读数。
func (r *GormUserMessageRepository) CountUnread(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.EduUserMessage{}).
		Where("receiver_id = ? AND is_read = ? AND deleted = ?", userID, false, 0).
		Count(&count).Error
	return count, err
}

// CountUnreadByConversation 查询指定会话中用户的未读数。
func (r *GormUserMessageRepository) CountUnreadByConversation(ctx context.Context, conversationID string, userID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.EduUserMessage{}).
		Where("conversation_id = ? AND receiver_id = ? AND is_read = ? AND deleted = ?", conversationID, userID, false, 0).
		Count(&count).Error
	return count, err
}

// conversationRow 会话列表查询的内部扫描结构。
// SQLite 的 MAX(create_time) 返回字符串，用 string 接收后再转换为 time.Time，
// 兼容 PostgreSQL 与 SQLite。
type conversationRow struct {
	ConversationID  string `gorm:"column:conversation_id"`
	LastMessageTime string `gorm:"column:last_message_time"`
	LastMessage     string `gorm:"column:last_message"`
	OtherUserID     uint64 `gorm:"column:other_user_id"`
	UnreadCount     int64  `gorm:"column:unread_count"`
}

// ListConversations 查询用户参与的所有会话，每个会话返回最新消息、时间和未读数，按最新消息时间倒序。
// 使用 GROUP BY + 子查询实现，兼容 PostgreSQL 与 SQLite。
func (r *GormUserMessageRepository) ListConversations(ctx context.Context, userID uint64) ([]ConversationSummary, error) {
	var rows []conversationRow
	err := r.db.WithContext(ctx).
		Table("edu_user_message AS m").
		Select(`
			m.conversation_id AS conversation_id,
			MAX(m.create_time) AS last_message_time,
			(SELECT content FROM edu_user_message AS lm
				WHERE lm.conversation_id = m.conversation_id AND lm.deleted = 0
				ORDER BY lm.create_time DESC LIMIT 1) AS last_message,
			(SELECT CASE WHEN sender_id = ? THEN receiver_id ELSE sender_id END
				FROM edu_user_message AS om
				WHERE om.conversation_id = m.conversation_id AND om.deleted = 0
				ORDER BY om.create_time DESC LIMIT 1) AS other_user_id,
			COUNT(CASE WHEN m.receiver_id = ? AND m.is_read = ? THEN 1 END) AS unread_count
		`, userID, userID, false).
		Where("(m.sender_id = ? OR m.receiver_id = ?) AND m.deleted = ?", userID, userID, 0).
		Group("m.conversation_id").
		Order("last_message_time DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	results := make([]ConversationSummary, 0, len(rows))
	for _, row := range rows {
		lastTime, parseErr := time.Parse(time.RFC3339Nano, row.LastMessageTime)
		if parseErr != nil {
			// SQLite 可能返回 "2006-01-02 15:04:05.999999999-07:00" 格式
			lastTime, parseErr = time.Parse("2006-01-02 15:04:05.999999999-07:00", row.LastMessageTime)
		}
		if parseErr != nil {
			lastTime, parseErr = time.Parse("2006-01-02 15:04:05-07:00", row.LastMessageTime)
		}
		if parseErr != nil {
			lastTime = time.Time{}
		}
		results = append(results, ConversationSummary{
			ConversationID:  row.ConversationID,
			OtherUserID:     row.OtherUserID,
			LastMessage:     row.LastMessage,
			LastMessageTime: lastTime,
			UnreadCount:     row.UnreadCount,
		})
	}
	return results, nil
}
