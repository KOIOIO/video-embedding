package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
)

type TextArray []string

func (a TextArray) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	var b strings.Builder
	b.WriteString("{")
	for i, s := range a {
		if i > 0 {
			b.WriteString(",")
		}
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		b.WriteString("\"")
		b.WriteString(s)
		b.WriteString("\"")
	}
	b.WriteString("}")
	return b.String(), nil
}

func (a *TextArray) Scan(src any) error {
	if a == nil {
		return errors.New("TextArray: Scan on nil receiver")
	}
	if src == nil {
		*a = nil
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("TextArray: unsupported Scan type %T", src)
	}
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		*a = TextArray{}
		return nil
	}
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return fmt.Errorf("TextArray: invalid array literal: %q", s)
	}
	s = s[1 : len(s)-1]
	if strings.TrimSpace(s) == "" {
		*a = TextArray{}
		return nil
	}

	out := make([]string, 0, 8)
	var cur strings.Builder
	inQuotes := false
	escape := false
	flush := func() {
		out = append(out, cur.String())
		cur.Reset()
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escape {
			cur.WriteByte(ch)
			escape = false
			continue
		}
		if ch == '\\' {
			escape = true
			continue
		}
		if ch == '"' {
			inQuotes = !inQuotes
			continue
		}
		if ch == ',' && !inQuotes {
			flush()
			continue
		}
		cur.WriteByte(ch)
	}
	flush()
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	*a = TextArray(out)
	return nil
}

type EduVideoResource struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	UserID uint64 `gorm:"column:user_id;not null;default:1;index" json:"user_id"`

	Title       string `gorm:"column:title;size:200;not null" json:"title"`
	Description string `gorm:"column:description;type:text" json:"description"`

	VideoURL  string `gorm:"column:video_url;size:500;not null" json:"video_url"`
	Duration  int    `gorm:"column:duration" json:"duration"`
	CoverURL  string `gorm:"column:cover_url;size:500" json:"cover_url"`
	Status    int16  `gorm:"column:status;default:1" json:"status"`
	ErrorMsg  string `gorm:"column:error_msg;type:text" json:"error_msg"`
	IsPublish bool   `gorm:"column:is_published;default:true;index" json:"is_published"`
	IsRec     bool   `gorm:"column:is_recommend;default:false;index" json:"is_recommend"`
	ViewCount int    `gorm:"column:view_count;default:0" json:"view_count"`

	LikeCount       int `gorm:"column:like_count;default:0" json:"like_count"`
	DoubleLikeCount int `gorm:"column:double_like_count;default:0" json:"double_like_count"`
	DislikeCount    int `gorm:"column:dislike_count;default:0" json:"dislike_count"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduVideoResource) TableName() string { return "edu_video_resource" }

type EduVideoUserReaction struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	UserID       uint64 `gorm:"column:user_id;not null;index" json:"user_id"`
	VideoID      uint64 `gorm:"column:video_id;not null;index" json:"video_id"`
	ReactionType string `gorm:"column:reaction_type;type:text;not null" json:"reaction_type"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduVideoUserReaction) TableName() string { return "edu_video_user_reaction" }

type EduUserReaction struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	UserID         uint64 `gorm:"column:user_id;not null;index" json:"user_id"`
	VideoID        uint64 `gorm:"column:video_id;not null;index" json:"video_id"`
	VideoSegmentID uint64 `gorm:"column:video_segment_id;not null;index" json:"video_segment_id"`
	ReactionType   string `gorm:"column:reaction_type;type:text;not null" json:"reaction_type"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduUserReaction) TableName() string { return "edu_user_reaction" }

type EduVideoSegment struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	VideoID        uint64 `gorm:"column:video_id;not null;index" json:"video_id"`
	SegmentIndex   int    `gorm:"column:segment_index;not null" json:"segment_index"`
	StartTimeSec   int    `gorm:"column:start_time;not null" json:"start_time"`
	EndTimeSec     int    `gorm:"column:end_time;not null" json:"end_time"`
	ContentSummary string `gorm:"column:content_summary;type:text" json:"content_summary"`

	Embedding     pgvector.Vector `gorm:"column:embedding;type:vector(1536)" json:"-"`
	KnowledgeTags TextArray       `gorm:"column:knowledge_tags;type:text[]" json:"knowledge_tags"`

	LikeCount       int `gorm:"column:like_count;default:0" json:"like_count"`
	DoubleLikeCount int `gorm:"column:double_like_count;default:0" json:"double_like_count"`
	DislikeCount    int `gorm:"column:dislike_count;default:0" json:"dislike_count"`

	Status     int16     `gorm:"column:status;default:1" json:"status"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduVideoSegment) TableName() string { return "edu_video_segment" }

type EduVideoVectorStage struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	TaskID       string `gorm:"column:task_id;type:text;not null;index:idx_video_vector_stage_unique,unique,priority:1" json:"task_id"`
	VideoID      uint64 `gorm:"column:video_id;not null;index" json:"video_id"`
	Stage        string `gorm:"column:stage;type:text;not null;index:idx_video_vector_stage_unique,unique,priority:2" json:"stage"`
	SegmentIndex int    `gorm:"column:segment_index;not null;default:0;index:idx_video_vector_stage_unique,unique,priority:3" json:"segment_index"`
	SegmentID    uint64 `gorm:"column:segment_id;not null;default:0;index:idx_video_vector_stage_unique,unique,priority:4" json:"segment_id"`

	Status       int16  `gorm:"column:status;not null;default:0;index" json:"status"`
	ObjectKey    string `gorm:"column:object_key;type:text;not null;default:''" json:"object_key"`
	Text         string `gorm:"column:text;type:text;not null;default:''" json:"text"`
	ErrorMessage string `gorm:"column:error_message;type:text;not null;default:''" json:"error_message"`
	RetryCount   int    `gorm:"column:retry_count;not null;default:0" json:"retry_count"`
	StartTimeSec int    `gorm:"column:start_time;not null;default:0" json:"start_time"`
	EndTimeSec   int    `gorm:"column:end_time;not null;default:0" json:"end_time"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

func (EduVideoVectorStage) TableName() string { return "edu_video_vector_stage" }

type EduUserVideoRecommend struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	UserID         uint64  `gorm:"column:user_id;not null;index" json:"user_id"`
	VideoID        uint64  `gorm:"column:video_id;not null;index" json:"video_id"`
	RecommendLevel int16   `gorm:"column:recommend_level;default:0" json:"recommend_level"`
	QuestionID     uint64  `gorm:"column:question_id;index" json:"question_id"`
	VideoSegmentID uint64  `gorm:"column:video_segment_id;index" json:"video_segment_id"`
	RecommendScore float64 `gorm:"column:recommend_score;type:numeric(5,4)" json:"recommend_score"`
	IsWatched      bool    `gorm:"column:is_watched;default:false" json:"is_watched"`
	WatchDuration  int     `gorm:"column:watch_duration" json:"watch_duration"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduUserVideoRecommend) TableName() string { return "edu_user_video_recommend" }

type EduUserVideoProfile struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	UserID           uint64          `gorm:"column:user_id;not null;index" json:"user_id"`
	ProfileVector    pgvector.Vector `gorm:"column:profile_vector;type:vector(1536)" json:"-"`
	PositiveCount    int             `gorm:"column:positive_count;default:0" json:"positive_count"`
	NegativeCount    int             `gorm:"column:negative_count;default:0" json:"negative_count"`
	WatchCount       int             `gorm:"column:watch_count;default:0" json:"watch_count"`
	SourceEventCount int             `gorm:"column:source_event_count;default:0" json:"source_event_count"`
	LastEventTime    time.Time       `gorm:"column:last_event_time" json:"last_event_time"`
	ModelVersion     string          `gorm:"column:model_version;type:text;not null;index" json:"model_version"`
	Status           int16           `gorm:"column:status;default:1;index" json:"status"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduUserVideoProfile) TableName() string { return "edu_user_video_profile" }

type EduRecommendExposure struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	RequestID      string    `gorm:"column:request_id;type:text;not null;index" json:"request_id"`
	UserID         uint64    `gorm:"column:user_id;not null;index" json:"user_id"`
	QuestionID     uint64    `gorm:"column:question_id;index" json:"question_id"`
	VideoID        uint64    `gorm:"column:video_id;not null;index" json:"video_id"`
	VideoSegmentID uint64    `gorm:"column:video_segment_id;not null;index" json:"video_segment_id"`
	Rank           int       `gorm:"column:rank;not null" json:"rank"`
	Score          float64   `gorm:"column:score;type:numeric(8,6)" json:"score"`
	Strategy       string    `gorm:"column:strategy;type:text;not null;index" json:"strategy"`
	ModelVersion   string    `gorm:"column:model_version;type:text;index" json:"model_version"`
	Clicked        bool      `gorm:"column:clicked;default:false;index" json:"clicked"`
	Watched        bool      `gorm:"column:watched;default:false;index" json:"watched"`
	ClickedTime    time.Time `gorm:"column:clicked_time" json:"clicked_time"`
	WatchedTime    time.Time `gorm:"column:watched_time" json:"watched_time"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime;index" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduRecommendExposure) TableName() string { return "edu_recommend_exposure" }

type EduVideoItemEmbedding struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	VideoSegmentID uint64          `gorm:"column:video_segment_id;not null;index" json:"video_segment_id"`
	VideoID        uint64          `gorm:"column:video_id;not null;index" json:"video_id"`
	Embedding      pgvector.Vector `gorm:"column:embedding;type:vector(64)" json:"-"`
	ModelVersion   string          `gorm:"column:model_version;type:text;not null;index" json:"model_version"`
	Status         int16           `gorm:"column:status;default:1;index" json:"status"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduVideoItemEmbedding) TableName() string { return "edu_video_item_embedding" }

type EduUserTowerEmbedding struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	UserID       uint64          `gorm:"column:user_id;not null;index" json:"user_id"`
	TowerVector  pgvector.Vector `gorm:"column:tower_vector;type:vector(64)" json:"-"`
	ModelVersion string          `gorm:"column:model_version;type:text;not null;index" json:"model_version"`
	Status       int16           `gorm:"column:status;default:1;index" json:"status"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduUserTowerEmbedding) TableName() string { return "edu_user_tower_embedding" }

type EduRecommendModelVersion struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	ModelName    string    `gorm:"column:model_name;type:text;not null;index" json:"model_name"`
	ModelVersion string    `gorm:"column:model_version;type:text;not null;index" json:"model_version"`
	ArtifactPath string    `gorm:"column:artifact_path;type:text" json:"artifact_path"`
	MetricsJSON  string    `gorm:"column:metrics_json;type:jsonb;not null;default:'{}'" json:"metrics_json"`
	IsActive     bool      `gorm:"column:is_active;default:false;index" json:"is_active"`
	Status       int16     `gorm:"column:status;default:1;index" json:"status"`
	PublishedAt  time.Time `gorm:"column:published_at" json:"published_at"`

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduRecommendModelVersion) TableName() string { return "edu_recommend_model_version" }

type SysUser struct {
	ID       uint64 `gorm:"primaryKey;column:id" json:"id"`
	UserType int16  `gorm:"column:user_type;not null;default:0" json:"user_type"`
	GradeID  int64  `gorm:"column:grade_id;default:0" json:"grade_id"`
	ClassID  int64  `gorm:"column:class_id;default:0" json:"class_id"`
	Deleted  int16  `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (SysUser) TableName() string { return "sys_user" }
