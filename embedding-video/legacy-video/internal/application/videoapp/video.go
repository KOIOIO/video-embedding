package videoapp

import (
	"context"
	"errors"
	"strings"

	domainvideo "legacy-video/internal/domain/video"
)

// ListVideos 按过滤条件列出视频。
func (s *Service) ListVideos(ctx context.Context, filter ListFilter) ([]domainvideo.Video, error) {
	return s.Repo.List(ctx, filter)
}

// ListRecommendPoolVideos 返回被标记为推荐的视频列表。
func (s *Service) ListRecommendPoolVideos(ctx context.Context) ([]domainvideo.Video, error) {
	return s.Repo.ListRecommendPool(ctx)
}

// DeleteVideo 按视频 ID 删除视频记录。
func (s *Service) DeleteVideo(ctx context.Context, videoID uint64, _ string) (bool, error) {
	if videoID == 0 {
		return false, errors.New("video_id is required")
	}
	return s.Repo.DeleteByID(ctx, videoID)
}

// UpdateVideoMetadata 更新视频标题和描述。
func (s *Service) UpdateVideoMetadata(ctx context.Context, videoID uint64, title string, description string) (bool, error) {
	if videoID == 0 {
		return false, errors.New("video_id is required")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return false, errors.New("title is required")
	}
	return s.Repo.UpdateMetadata(ctx, videoID, title, description)
}

// SetVideoPublished 更新视频发布状态。
func (s *Service) SetVideoPublished(ctx context.Context, videoID uint64, isPublished bool) (bool, error) {
	return s.Repo.UpdatePublished(ctx, videoID, isPublished)
}

// SetVideoRecommend 更新视频是否进入推荐池及其推荐元数据。
func (s *Service) SetVideoRecommend(ctx context.Context, videoID uint64, isRecommend bool, userID uint64, recommendLevel int16, recommendScore float64) (bool, error) {
	return s.Repo.UpdateRecommend(ctx, videoID, isRecommend, userID, recommendLevel, recommendScore)
}

// GetSimilarVideos 根据视频向量近邻查找相似视频。
func (s *Service) GetSimilarVideos(ctx context.Context, videoID uint64, limit int) ([]domainvideo.Video, error) {
	return s.Repo.FindSimilar(ctx, videoID, limit)
}

// SetVideoCover 更新视频封面 URL。
func (s *Service) SetVideoCover(ctx context.Context, videoID uint64, coverURL string) (bool, error) {
	if strings.TrimSpace(coverURL) == "" {
		return false, errors.New("cover_url is required")
	}
	return s.Repo.UpdateCoverByID(ctx, videoID, coverURL)
}
