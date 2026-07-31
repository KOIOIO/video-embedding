package model

import "time"

type EduKnowledgeVideoBatch struct {
	ID           uint64 `gorm:"primaryKey;column:id" json:"id"`
	UploadUserID uint64 `gorm:"column:upload_user_id;not null" json:"upload_user_id"`

	ZipFileName   string    `gorm:"column:zip_file_name;size:255;not null" json:"zip_file_name"`
	XLSXFileName  string    `gorm:"column:xlsx_file_name;size:255;not null" json:"xlsx_file_name"`
	XLSXObjectKey string    `gorm:"column:xlsx_object_key;size:500;not null" json:"xlsx_object_key"`
	TotalCount    int       `gorm:"column:total_count;not null;default:0" json:"total_count"`
	ReadyCount    int       `gorm:"column:ready_count;not null;default:0" json:"ready_count"`
	FailedCount   int       `gorm:"column:failed_count;not null;default:0" json:"failed_count"`
	Status        int16     `gorm:"column:status;not null;default:1" json:"status"`
	ErrorMessage  string    `gorm:"column:error_message;type:text;not null;default:''" json:"error_message"`
	CreateTime    time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime    time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

func (EduKnowledgeVideoBatch) TableName() string { return "edu_knowledge_video_batch" }

type EduKnowledgeVideo struct {
	ID                 uint64     `gorm:"primaryKey;column:id" json:"id"`
	BatchID            uint64     `gorm:"column:batch_id;not null" json:"batch_id"`
	KnowledgePointID   uint64     `gorm:"column:knowledge_point_id;not null" json:"knowledge_point_id"`
	KnowledgePointName string     `gorm:"column:knowledge_point_name;size:255;not null" json:"knowledge_point_name"`
	SourceFileName     string     `gorm:"column:source_file_name;size:255;not null" json:"source_file_name"`
	SourceObjectKey    string     `gorm:"column:source_object_key;size:500;not null" json:"source_object_key"`
	HLSObjectPrefix    string     `gorm:"column:hls_object_prefix;size:500;not null" json:"hls_object_prefix"`
	HLSMasterObjectKey string     `gorm:"column:hls_master_object_key;size:500;not null" json:"hls_master_object_key"`
	Duration           int        `gorm:"column:duration;not null;default:0" json:"duration"`
	Status             int16      `gorm:"column:status;not null;default:0" json:"status"`
	ErrorMessage       string     `gorm:"column:error_message;type:text;not null;default:''" json:"error_message"`
	EnqueueTime        *time.Time `gorm:"column:enqueue_time" json:"enqueue_time"`
	CreateTime         time.Time  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime         time.Time  `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted            int16      `gorm:"column:deleted;not null;default:0" json:"deleted"`
}

func (EduKnowledgeVideo) TableName() string { return "edu_knowledge_video" }

type EduKnowledgeVideoPlayRecord struct {
	ID               uint64    `gorm:"primaryKey;column:id" json:"id"`
	UserID           uint64    `gorm:"column:user_id;not null" json:"user_id"`
	KnowledgePointID uint64    `gorm:"column:knowledge_point_id;not null" json:"knowledge_point_id"`
	KnowledgeVideoID uint64    `gorm:"column:knowledge_video_id;not null" json:"knowledge_video_id"`
	CreateTime       time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

func (EduKnowledgeVideoPlayRecord) TableName() string { return "edu_knowledge_video_play_record" }
