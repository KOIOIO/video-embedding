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

	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduVideoResource) TableName() string { return "edu_video_resource" }

type EduVideoSegment struct {
	ID uint64 `gorm:"primaryKey;column:id" json:"id"`

	VideoID        uint64 `gorm:"column:video_id;not null;index" json:"video_id"`
	SegmentIndex   int    `gorm:"column:segment_index;not null" json:"segment_index"`
	StartTimeSec   int    `gorm:"column:start_time;not null" json:"start_time"`
	EndTimeSec     int    `gorm:"column:end_time;not null" json:"end_time"`
	ContentSummary string `gorm:"column:content_summary;type:text" json:"content_summary"`

	Embedding     pgvector.Vector `gorm:"column:embedding;type:vector(1536)" json:"-"`
	KnowledgeTags TextArray       `gorm:"column:knowledge_tags;type:text[]" json:"knowledge_tags"`

	Status     int16     `gorm:"column:status;default:1" json:"status"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	Deleted    int16     `gorm:"column:deleted;default:0;index" json:"deleted"`
}

func (EduVideoSegment) TableName() string { return "edu_video_segment" }

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
