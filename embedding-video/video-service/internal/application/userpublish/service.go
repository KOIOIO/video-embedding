package userpublish

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainvideo "video-service/internal/domain/video"
	"video-service/internal/application/videoapp"
	"video-service/internal/model"
)

const (
	MaxTitleLength       = 200
	MaxDescriptionLength = 2000
	MaxUserPublishDurationSec = 180
)

// Repository 定义用户发布视频所需的仓储能力。
type Repository interface {
	CreateVideo(ctx context.Context, video *model.EduVideoResource) error
	GetByID(ctx context.Context, id uint64) (*model.EduVideoResource, error)
	ListByUserID(ctx context.Context, userID uint64, page, pageSize int) ([]model.EduVideoResource, int64, error)
	UpdateStatus(ctx context.Context, id uint64, status int16, errMsg string) error
}

// TranscodeQueue 抽象转码任务投递能力。
type TranscodeQueue interface {
	Enqueue(ctx context.Context, task videoapp.TranscodeTask) error
}

// VectorizeQueue 抽象向量化任务投递能力（可选）。
type VectorizeQueue interface {
	Enqueue(ctx context.Context, task videoapp.VectorizeTask) error
}

// Service 用户发布视频应用服务。
type Service struct {
	Repo        Repository
	Queue       TranscodeQueue
	VectorQueue VectorizeQueue
	Now         func() time.Time
}

// NewService 创建用户发布服务。
func NewService(repo Repository, queue TranscodeQueue, vectorQueue VectorizeQueue) *Service {
	return &Service{
		Repo:        repo,
		Queue:       queue,
		VectorQueue: vectorQueue,
		Now:         time.Now,
	}
}

// PublishVideo 校验参数、创建视频记录并入转码队列。
func (s *Service) PublishVideo(ctx context.Context, userID uint64, title, description string, videoURL string, duration int, rawObjectKey, hlsObjectPrefix, hlsURL string) (*model.EduVideoResource, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("title is required")
	}
	if len([]rune(title)) > MaxTitleLength {
		return nil, fmt.Errorf("title must be at most %d characters", MaxTitleLength)
	}
	if len([]rune(description)) > MaxDescriptionLength {
		return nil, fmt.Errorf("description must be at most %d characters", MaxDescriptionLength)
	}
	if strings.TrimSpace(videoURL) == "" {
		return nil, errors.New("video_url is required")
	}
	if userID == 0 {
		return nil, errors.New("user_id is required")
	}

	now := s.Now()
	video := &model.EduVideoResource{
		UserID:      userID,
		SourceType:  model.VideoSourceUserPublish,
		Title:       title,
		Description: description,
		VideoURL:    videoURL,
		Duration:    duration,
		Status:      int16(domainvideo.StatusUploaded),
		IsPublish:   true,
		IsRec:       false,
		CreateTime:  now,
		UpdateTime:  now,
		Deleted:     0,
	}

	if err := s.Repo.CreateVideo(ctx, video); err != nil {
		return nil, err
	}

	taskID := strconv.FormatUint(video.ID, 10)
	if err := s.Queue.Enqueue(ctx, videoapp.TranscodeTask{
		VideoID:         video.ID,
		RawKey:          rawObjectKey,
		HLSObjectPrefix: hlsObjectPrefix,
		TaskID:          taskID,
		HLSURL:          hlsURL,
	}); err != nil {
		_ = s.Repo.UpdateStatus(ctx, video.ID, int16(domainvideo.StatusFailed), "enqueue transcode failed: "+err.Error())
		return nil, err
	}

	if s.VectorQueue != nil {
		_ = s.VectorQueue.Enqueue(ctx, videoapp.VectorizeTask{
			VideoID: video.ID,
			RawKey:  rawObjectKey,
			TaskID:  taskID,
		})
	}

	return video, nil
}

// GetVideoStatus 查询视频处理状态和错误信息。
func (s *Service) GetVideoStatus(ctx context.Context, id uint64) (int16, string, error) {
	video, err := s.Repo.GetByID(ctx, id)
	if err != nil {
		return 0, "", err
	}
	if video == nil {
		return 0, "", errors.New("video not found")
	}
	return video.Status, video.ErrorMsg, nil
}

// ListUserVideos 分页查询用户已发布的作品列表。
func (s *Service) ListUserVideos(ctx context.Context, userID uint64, page, pageSize int) ([]model.EduVideoResource, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 12
	}
	return s.Repo.ListByUserID(ctx, userID, page, pageSize)
}
