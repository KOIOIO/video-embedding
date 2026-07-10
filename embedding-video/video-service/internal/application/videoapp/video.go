package videoapp

import (
	"context"
	"strings"

	domainvideo "nlp-video-analysis/internal/domain/video"
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
		return false, InvalidArgumentError("video_id is required")
	}
	return s.Repo.DeleteByID(ctx, videoID)
}

// UpdateVideoMetadata 更新视频标题和描述。
func (s *Service) UpdateVideoMetadata(ctx context.Context, videoID uint64, title string, description string) (bool, error) {
	if videoID == 0 {
		return false, InvalidArgumentError("video_id is required")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return false, InvalidArgumentError("title is required")
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
		return false, InvalidArgumentError("cover_url is required")
	}
	return s.Repo.UpdateCoverByID(ctx, videoID, coverURL)
}

// SubmitVideoReaction 提交或取消用户对视频的评价。
func (s *Service) SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType VideoReactionType) (VideoReactionResult, bool, error) {
	if videoID == 0 {
		return VideoReactionResult{}, false, InvalidArgumentError("video_id is required")
	}
	if userID == 0 {
		return VideoReactionResult{}, false, InvalidArgumentError("user_id is required")
	}
	if !reactionType.IsValid() {
		return VideoReactionResult{}, false, InvalidArgumentError("reaction_type must be one of like, double_like, dislike")
	}
	if s.ReactionStore == nil {
		active, found, err := s.Repo.SubmitVideoReaction(ctx, videoID, userID, reactionType)
		if err != nil || !found {
			return VideoReactionResult{}, found, err
		}
		counts, _, err := s.Repo.GetVideoReactionCounts(ctx, videoID)
		if err != nil {
			return VideoReactionResult{}, true, err
		}
		return VideoReactionResult{Active: active, ReactionType: reactionType, Counts: counts}, true, nil
	}

	var seedCounts VideoReactionCounts
	countsSeeded, err := s.ReactionStore.HasCounts(ctx, videoID)
	if err != nil {
		return VideoReactionResult{}, false, err
	}
	if !countsSeeded {
		counts, found, err := s.Repo.GetVideoReactionCounts(ctx, videoID)
		if err != nil || !found {
			return VideoReactionResult{}, found, err
		}
		seedCounts = counts
	}

	var seedReaction VideoReactionType
	var seedActive bool
	userSeeded, err := s.ReactionStore.HasUserReaction(ctx, videoID, userID)
	if err != nil {
		return VideoReactionResult{}, false, err
	}
	if !userSeeded {
		var found bool
		seedReaction, seedActive, found, err = s.Repo.GetVideoUserReaction(ctx, videoID, userID)
		if err != nil || !found {
			return VideoReactionResult{}, found, err
		}
	}

	result, err := s.ReactionStore.Submit(ctx, videoID, userID, reactionType, seedCounts, seedReaction, seedActive)
	if err != nil {
		return VideoReactionResult{}, false, err
	}
	return result, true, nil
}

// GetVideoReactionCounts 查询视频点赞和双赞数量。
func (s *Service) GetVideoReactionCounts(ctx context.Context, videoID uint64) (VideoReactionCounts, bool, error) {
	if videoID == 0 {
		return VideoReactionCounts{}, false, InvalidArgumentError("video_id is required")
	}
	if s.ReactionStore != nil {
		hasCounts, err := s.ReactionStore.HasCounts(ctx, videoID)
		if err != nil {
			return VideoReactionCounts{}, false, err
		}
		if hasCounts {
			counts, err := s.ReactionStore.GetCounts(ctx, videoID, VideoReactionCounts{})
			return counts, true, err
		}
	}
	dbCounts, found, err := s.Repo.GetVideoReactionCounts(ctx, videoID)
	if err != nil || !found || s.ReactionStore == nil {
		return dbCounts, found, err
	}
	counts, err := s.ReactionStore.GetCounts(ctx, videoID, dbCounts)
	if err != nil {
		return VideoReactionCounts{}, false, err
	}
	return counts, true, nil
}

// SubmitSegmentReaction 提交或取消用户对视频片段的评价。
func (s *Service) SubmitSegmentReaction(ctx context.Context, segmentID uint64, userID uint64, reactionType VideoReactionType) (VideoReactionResult, bool, error) {
	if segmentID == 0 {
		return VideoReactionResult{}, false, InvalidArgumentError("segment_id is required")
	}
	if userID == 0 {
		return VideoReactionResult{}, false, InvalidArgumentError("user_id is required")
	}
	if !reactionType.IsValid() {
		return VideoReactionResult{}, false, InvalidArgumentError("reaction_type must be one of like, double_like, dislike")
	}
	if s.SegmentReactionRepo == nil {
		return VideoReactionResult{}, false, InvalidArgumentError("segment reaction repository is required")
	}
	if s.SegmentReactionStore == nil {
		active, found, err := s.SegmentReactionRepo.SubmitSegmentReaction(ctx, segmentID, userID, reactionType)
		if err != nil || !found {
			return VideoReactionResult{}, found, err
		}
		counts, _, err := s.SegmentReactionRepo.GetSegmentReactionCounts(ctx, segmentID)
		if err != nil {
			return VideoReactionResult{}, true, err
		}
		return VideoReactionResult{Active: active, ReactionType: reactionType, Counts: counts}, true, nil
	}

	var seedCounts VideoReactionCounts
	countsSeeded, err := s.SegmentReactionStore.HasCounts(ctx, segmentID)
	if err != nil {
		return VideoReactionResult{}, false, err
	}
	if !countsSeeded {
		counts, found, err := s.SegmentReactionRepo.GetSegmentReactionCounts(ctx, segmentID)
		if err != nil || !found {
			return VideoReactionResult{}, found, err
		}
		seedCounts = counts
	}

	var seedReaction VideoReactionType
	var seedActive bool
	userSeeded, err := s.SegmentReactionStore.HasUserReaction(ctx, segmentID, userID)
	if err != nil {
		return VideoReactionResult{}, false, err
	}
	if !userSeeded {
		var found bool
		seedReaction, seedActive, found, err = s.SegmentReactionRepo.GetSegmentUserReaction(ctx, segmentID, userID)
		if err != nil || !found {
			return VideoReactionResult{}, found, err
		}
	}

	result, err := s.SegmentReactionStore.Submit(ctx, segmentID, userID, reactionType, seedCounts, seedReaction, seedActive)
	if err != nil {
		return VideoReactionResult{}, false, err
	}
	return result, true, nil
}

// GetSegmentReactionCounts 查询视频片段点赞和双赞数量。
func (s *Service) GetSegmentReactionCounts(ctx context.Context, segmentID uint64) (VideoReactionCounts, bool, error) {
	if segmentID == 0 {
		return VideoReactionCounts{}, false, InvalidArgumentError("segment_id is required")
	}
	if s.SegmentReactionRepo == nil {
		return VideoReactionCounts{}, false, InvalidArgumentError("segment reaction repository is required")
	}
	if s.SegmentReactionStore != nil {
		hasCounts, err := s.SegmentReactionStore.HasCounts(ctx, segmentID)
		if err != nil {
			return VideoReactionCounts{}, false, err
		}
		if hasCounts {
			counts, err := s.SegmentReactionStore.GetCounts(ctx, segmentID, VideoReactionCounts{})
			return counts, true, err
		}
	}
	dbCounts, found, err := s.SegmentReactionRepo.GetSegmentReactionCounts(ctx, segmentID)
	if err != nil || !found || s.SegmentReactionStore == nil {
		return dbCounts, found, err
	}
	counts, err := s.SegmentReactionStore.GetCounts(ctx, segmentID, dbCounts)
	if err != nil {
		return VideoReactionCounts{}, false, err
	}
	return counts, true, nil
}
