package model

import (
	"fmt"
	"time"
)

// EduUserMessage 用户私信消息表。
type EduUserMessage struct {
	ID             uint64    `gorm:"primaryKey;column:id" json:"id"`
	SenderID       uint64    `gorm:"column:sender_id;not null;index" json:"sender_id"`
	ReceiverID     uint64    `gorm:"column:receiver_id;not null;index" json:"receiver_id"`
	ConversationID string    `gorm:"column:conversation_id;type:text;not null;index" json:"conversation_id"`
	Content        string    `gorm:"column:content;type:text;not null" json:"content"`
	IsRead         bool      `gorm:"column:is_read;default:false;index" json:"is_read"`
	CreateTime     time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	Deleted        int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduUserMessage) TableName() string { return "edu_user_message" }

// ConversationID 生成会话ID，用 min_max 拼接确保双向一致。
func ConversationID(userA, userB uint64) string {
	if userA < userB {
		return fmt.Sprintf("%d_%d", userA, userB)
	}
	return fmt.Sprintf("%d_%d", userB, userA)
}
