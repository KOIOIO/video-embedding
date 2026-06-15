package video

import (
	"errors"
	"strings"
	"time"
)

// Status 表示视频在上传、转码与失败恢复过程中的状态机阶段。
type Status int

// 视频处理状态常量。
const (
	StatusUploaded   Status = 1
	StatusProcessing Status = 2
	StatusDone       Status = 3
	StatusFailed     Status = 4
)

// Video 是视频领域对象，承载应用层真正关心的业务字段。
type Video struct {
	ID          uint64
	Title       string
	Description string
	VideoURL    string
	Duration    int
	CoverURL    string
	Status      Status
	ErrorMsg    string
	IsPublished bool
	IsRecommend bool
	ViewCount   int
	CreateTime  time.Time
	UpdateTime  time.Time
	Deleted     int16
}

// NewUploaded 创建一个刚完成原视频上传、尚未进入转码的 Video。
func NewUploaded(title string, description string, rawVideoURL string, now time.Time) (*Video, error) {
	if strings.TrimSpace(title) == "" {
		return nil, errors.New("title is required")
	}
	if strings.TrimSpace(rawVideoURL) == "" {
		return nil, errors.New("video_url is required")
	}

	return &Video{
		Title:       title,
		Description: description,
		VideoURL:    rawVideoURL,
		Status:      StatusUploaded,
		IsPublished: true,
		IsRecommend: false,
		ViewCount:   0,
		CreateTime:  now,
		UpdateTime:  now,
		Deleted:     0,
	}, nil
}

// MarkProcessing 把视频状态推进到处理中。
func (v *Video) MarkProcessing(now time.Time) error {
	if v.Status != StatusUploaded {
		return errors.New("invalid status transition to processing")
	}
	v.Status = StatusProcessing
	v.UpdateTime = now
	return nil
}

// MarkDone 把视频状态推进到处理完成，并清空错误信息。
func (v *Video) MarkDone(now time.Time) error {
	if v.Status != StatusProcessing {
		return errors.New("invalid status transition to done")
	}
	v.Status = StatusDone
	v.ErrorMsg = ""
	v.UpdateTime = now
	return nil
}

// MarkFailed 把视频状态推进到处理失败，并记录错误信息。
func (v *Video) MarkFailed(errMsg string, now time.Time) error {
	if v.Status != StatusProcessing && v.Status != StatusUploaded {
		return errors.New("invalid status transition to failed")
	}
	v.Status = StatusFailed
	v.ErrorMsg = errMsg
	v.UpdateTime = now
	return nil
}
