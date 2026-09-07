package model

import "time"

// EduUserProfile 用户资料表，存储昵称、头像、签名等公开信息。
type EduUserProfile struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	UserID   uint64 `gorm:"column:user_id;not null;uniqueIndex" json:"user_id"`
	Nickname string `gorm:"column:nickname;size:100;not null" json:"nickname"`
	AvatarURL string `gorm:"column:avatar_url;size:500" json:"avatar_url"`
	Bio      string `gorm:"column:bio;type:text" json:"bio"`
	Gender   int16  `gorm:"column:gender;default:0" json:"gender"`
	Location string `gorm:"column:location;size:200" json:"location"`

	FollowCount int `gorm:"column:follow_count;default:0" json:"follow_count"`
	FansCount   int `gorm:"column:fans_count;default:0" json:"fans_count"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduUserProfile) TableName() string { return "edu_user_profile" }
