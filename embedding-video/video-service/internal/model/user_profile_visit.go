package model

import "time"

// EduUserProfileVisit 用户主页访问记录表。
type EduUserProfileVisit struct {
	ID          uint64    `gorm:"primaryKey;column:id" json:"id"`
	VisitorID   uint64    `gorm:"column:visitor_id;not null" json:"visitor_id"`
	OwnerID     uint64    `gorm:"column:owner_id;not null;index" json:"owner_id"`
	VisitDate   time.Time `gorm:"column:visit_date;type:date;not null" json:"visit_date"`
	VisitCount  int       `gorm:"column:visit_count;default:1" json:"visit_count"`
	CreateTime  time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime  time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted     int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduUserProfileVisit) TableName() string { return "edu_user_profile_visit" }
