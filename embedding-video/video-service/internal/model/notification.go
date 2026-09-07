package model

import "time"

// EduUserNotification 用户通知表，存储评论 @提及 等通知。
type EduUserNotification struct {
	ID         uint64 `gorm:"primaryKey;column:id" json:"id"`
	UserID     uint64 `gorm:"column:user_id;not null;index" json:"user_id"`
	Type       string `gorm:"column:type;type:varchar(32);not null;default:'mention';index" json:"type"`
	FromUserID uint64 `gorm:"column:from_user_id;not null;default:0" json:"from_user_id"`
	VideoID    uint64 `gorm:"column:video_id;not null;default:0" json:"video_id"`
	VideoSegmentID uint64 `gorm:"column:video_segment_id;not null;default:0" json:"video_segment_id"`
	CommentID  uint64 `gorm:"column:comment_id;not null;default:0" json:"comment_id"`
	Content    string `gorm:"column:content;type:varchar(500);not null;default:''" json:"content"`
	IsRead     int16  `gorm:"column:is_read;not null;default:0" json:"is_read"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduUserNotification) TableName() string { return "edu_user_notification" }

// NotificationTypeMention 评论 @提及 通知类型。
const NotificationTypeMention = "mention"
