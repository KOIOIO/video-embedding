package recommendation

import (
	"strconv"
	"strings"
	"time"
)

type GorseUser struct {
	UserID  string   `json:"UserId"`
	Labels  []string `json:"Labels,omitempty"`
	Comment string   `json:"Comment,omitempty"`
}

type GorseItem struct {
	ItemID     string         `json:"ItemId"`
	IsHidden   bool           `json:"IsHidden"`
	Categories []string       `json:"Categories,omitempty"`
	Labels     map[string]any `json:"Labels,omitempty"`
	Comment    string         `json:"Comment,omitempty"`
	Timestamp  time.Time      `json:"Timestamp,omitempty"`
}

type GorseFeedback struct {
	FeedbackType string    `json:"FeedbackType"`
	UserID       string    `json:"UserId"`
	ItemID       string    `json:"ItemId"`
	Timestamp    time.Time `json:"Timestamp"`
	Value        float64   `json:"Value,omitempty"`
}

type GorseUserSource struct {
	UserID          uint64
	GradeID         uint64
	ClassID         uint64
	UserType        string
	RecentSubjects  []string
	RecentKnowledge []string
	LearningLabels  []string
}

type GorseItemSource struct {
	VideoSegmentID  uint64
	VideoID         uint64
	Title           string
	Summary         string
	Subject         string
	KnowledgeTags   []string
	DurationSec     int
	ViewCount       int
	LikeCount       int
	DoubleLikeCount int
	DislikeCount    int
	Embedding       []float32
	IsDeleted       bool
	IsPublished     bool
	IsPlayable      bool
	IsRecommend     bool
	PublishedAt     time.Time
}

type GorseFeedbackKind string

const (
	GorseFeedbackLike            GorseFeedbackKind = "like"
	GorseFeedbackDoubleLike      GorseFeedbackKind = "double_like"
	GorseFeedbackDislike         GorseFeedbackKind = "dislike"
	GorseFeedbackWatch           GorseFeedbackKind = "watch"
	GorseFeedbackExposure        GorseFeedbackKind = "exposure"
	GorseFeedbackExposureNoClick GorseFeedbackKind = "exposure_no_click"
)

type GorseFeedbackSource struct {
	UserID         uint64
	VideoSegmentID uint64
	Kind           GorseFeedbackKind
	WatchRatio     float64
	EventTime      time.Time
}

func MapGorseUser(src GorseUserSource) GorseUser {
	labels := make([]string, 0, 4+len(src.RecentSubjects)+len(src.RecentKnowledge))
	if src.GradeID > 0 {
		labels = append(labels, "grade:"+strconv.FormatUint(src.GradeID, 10))
	}
	if src.ClassID > 0 {
		labels = append(labels, "class:"+strconv.FormatUint(src.ClassID, 10))
	}
	if userType := strings.TrimSpace(src.UserType); userType != "" {
		labels = append(labels, "type:"+userType)
	}
	for _, subject := range uniqueTrimmed(src.RecentSubjects) {
		labels = append(labels, "subject:"+subject)
	}
	for _, knowledge := range uniqueTrimmed(src.RecentKnowledge) {
		labels = append(labels, "knowledge:"+knowledge)
	}
	for _, label := range uniqueTrimmed(src.LearningLabels) {
		labels = append(labels, label)
	}
	return GorseUser{
		UserID: strconv.FormatUint(src.UserID, 10),
		Labels: labels,
	}
}

func MapGorseItem(src GorseItemSource) GorseItem {
	categories := make([]string, 0, 1+len(src.KnowledgeTags))
	if subject := strings.TrimSpace(src.Subject); subject != "" {
		categories = append(categories, "subject:"+subject)
	}
	for _, knowledge := range uniqueTrimmed(src.KnowledgeTags) {
		categories = append(categories, "knowledge:"+knowledge)
	}

	labels := map[string]any{
		"video_id":          strconv.FormatUint(src.VideoID, 10),
		"title":             strings.TrimSpace(src.Title),
		"summary":           strings.TrimSpace(src.Summary),
		"duration_sec":      float64(src.DurationSec),
		"view_count":        float64(src.ViewCount),
		"like_count":        float64(src.LikeCount),
		"double_like_count": float64(src.DoubleLikeCount),
		"dislike_count":     float64(src.DislikeCount),
	}
	if len(src.Embedding) > 0 {
		labels["embedding"] = src.Embedding
	}
	comment := strings.TrimSpace(src.Title)
	if summary := strings.TrimSpace(src.Summary); summary != "" {
		if comment != "" {
			comment += " "
		}
		comment += summary
	}
	return GorseItem{
		ItemID:     strconv.FormatUint(src.VideoSegmentID, 10),
		IsHidden:   src.IsDeleted || !src.IsPublished || !src.IsPlayable,
		Categories: categories,
		Labels:     labels,
		Comment:    comment,
		Timestamp:  src.PublishedAt,
	}
}

func MapGorseFeedback(src GorseFeedbackSource) (GorseFeedback, bool) {
	if src.UserID == 0 || src.VideoSegmentID == 0 {
		return GorseFeedback{}, false
	}
	feedback := GorseFeedback{
		UserID:    strconv.FormatUint(src.UserID, 10),
		ItemID:    strconv.FormatUint(src.VideoSegmentID, 10),
		Timestamp: src.EventTime,
	}
	switch src.Kind {
	case GorseFeedbackDoubleLike:
		feedback.FeedbackType = "double_like"
		feedback.Value = 3
	case GorseFeedbackLike:
		feedback.FeedbackType = "like"
		feedback.Value = 2
	case GorseFeedbackWatch:
		feedback.FeedbackType = "watch"
		feedback.Value = clamp01(src.WatchRatio)
	case GorseFeedbackExposure:
		feedback.FeedbackType = "exposure"
		feedback.Value = 1
	case GorseFeedbackDislike:
		feedback.FeedbackType = "dislike"
		feedback.Value = 1
	default:
		return GorseFeedback{}, false
	}
	return feedback, true
}

func uniqueTrimmed(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
