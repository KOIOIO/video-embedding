package model

import "time"

// EduUserFollow 用户关注关系表。
type EduUserFollow struct {
	ID          uint64    `gorm:"primaryKey;column:id" json:"id"`
	FollowerID  uint64    `gorm:"column:follower_id;not null;index" json:"follower_id"`
	FollowingID uint64    `gorm:"column:following_id;not null;index" json:"following_id"`
	CreateTime  time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	Deleted     int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduUserFollow) TableName() string { return "edu_user_follow" }

// FollowWithProfile 关注关系关联用户资料的查询结果。
type FollowWithProfile struct {
	ID          uint64    `gorm:"column:id" json:"id"`
	FollowerID  uint64    `gorm:"column:follower_id" json:"follower_id"`
	FollowingID uint64    `gorm:"column:following_id" json:"following_id"`
	CreateTime  time.Time `gorm:"column:create_time" json:"create_time"`
	Nickname    string    `gorm:"column:nickname" json:"nickname"`
	AvatarURL   string    `gorm:"column:avatar_url" json:"avatar_url"`
	Bio         string    `gorm:"column:bio" json:"bio"`
}
