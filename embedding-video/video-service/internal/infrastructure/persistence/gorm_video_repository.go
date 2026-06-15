package persistence

import (
	"context"
	"math"
	"time"

	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nlp-video-analysis/internal/application/videoapp"
	domainvideo "nlp-video-analysis/internal/domain/video"
	"nlp-video-analysis/internal/infrastructure/persistence/sqlqueries"
	"nlp-video-analysis/internal/model"
)

type GormVideoRepository struct {
	db *gorm.DB
}

// NewGormVideoRepository 创建基于 GORM 的视频仓储实现。
func NewGormVideoRepository(db *gorm.DB) *GormVideoRepository {
	return &GormVideoRepository{db: db}
}

// Create 写入一条新的视频资源记录。
func (r *GormVideoRepository) Create(ctx context.Context, v *domainvideo.Video) error {
	m := &model.EduVideoResource{
		Title:       v.Title,
		Description: v.Description,
		VideoURL:    v.VideoURL,
		Duration:    v.Duration,
		CoverURL:    v.CoverURL,
		Status:      int16(v.Status),
		ErrorMsg:    v.ErrorMsg,
		IsPublish:   v.IsPublished,
		IsRec:       v.IsRecommend,
		ViewCount:   v.ViewCount,
		Deleted:     v.Deleted,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	v.ID = m.ID
	return nil
}

// List 按过滤条件查询视频列表。
func (r *GormVideoRepository) List(ctx context.Context, filter videoapp.ListFilter) ([]domainvideo.Video, error) {
	var models []model.EduVideoResource
	q := r.db.WithContext(ctx).Model(&model.EduVideoResource{}).Where("deleted = ?", 0)
	switch filter {
	case videoapp.ListRawOnly:
		q = q.Where("status <> ?", int16(domainvideo.StatusDone))
	case videoapp.ListHLSOnly:
		q = q.Where("status = ?", int16(domainvideo.StatusDone))
	}

	if err := q.Find(&models).Error; err != nil {
		return nil, err
	}

	out := make([]domainvideo.Video, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainVideoResource(m))
	}
	return out, nil
}

// ListRecommendPool 返回推荐池中的视频。
func (r *GormVideoRepository) ListRecommendPool(ctx context.Context) ([]domainvideo.Video, error) {
	var models []model.EduVideoResource
	if err := r.db.WithContext(ctx).
		Model(&model.EduVideoResource{}).
		Where("deleted = ? AND is_recommend = ?", 0, true).
		Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]domainvideo.Video, 0, len(models))
	for _, m := range models {
		out = append(out, toDomainVideoResource(m))
	}
	return out, nil
}

// GetByID 按主键读取一条未删除的视频记录。
func (r *GormVideoRepository) GetByID(ctx context.Context, id uint64) (domainvideo.Video, bool, error) {
	var m model.EduVideoResource
	err := r.db.WithContext(ctx).Where("id = ? AND deleted = ?", id, 0).First(&m).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domainvideo.Video{}, false, nil
		}
		return domainvideo.Video{}, false, err
	}
	return toDomainVideoResource(m), true, nil
}

// DeleteByID 软删除视频，并联动软删除相关分段和推荐记录。
func (r *GormVideoRepository) DeleteByID(ctx context.Context, id uint64) (bool, error) {
	var deleted bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.EduVideoResource{}).Where("id = ? AND deleted = ?", id, 0).Update("deleted", 1)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			deleted = false
			return nil
		}
		deleted = true
		if err := tx.Model(&model.EduVideoSegment{}).Where("video_id = ? AND deleted = 0", id).Update("deleted", 1).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.EduUserVideoRecommend{}).Where("video_id = ? AND deleted = 0", id).Update("deleted", 1).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return deleted, nil
}

// UpdateMetadata 更新视频标题与描述。
func (r *GormVideoRepository) UpdateMetadata(ctx context.Context, id uint64, title string, description string) (bool, error) {
	res := r.db.WithContext(ctx).
		Model(&model.EduVideoResource{}).
		Where("id = ? AND deleted = ?", id, 0).
		Updates(map[string]any{
			"title":       title,
			"description": description,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// UpdatePublished 更新视频发布状态。
func (r *GormVideoRepository) UpdatePublished(ctx context.Context, id uint64, isPublished bool) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.EduVideoResource{}).Where("id = ? AND deleted = ?", id, 0).Update("is_published", isPublished)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// UpdateRecommend 更新视频推荐状态，并维护测试阶段使用的推荐表数据。
func (r *GormVideoRepository) UpdateRecommend(ctx context.Context, id uint64, isRecommend bool, userID uint64, recommendLevel int16, recommendScore float64) (bool, error) {
	var rowsAffected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.EduVideoResource{}).Where("id = ? AND deleted = ?", id, 0).Update("is_recommend", isRecommend)
		if res.Error != nil {
			return res.Error
		}
		rowsAffected = res.RowsAffected
		if rowsAffected == 0 {
			return nil
		}

		if isRecommend {
			var seg model.EduVideoSegment
			err := tx.Model(&model.EduVideoSegment{}).Where("video_id = ? AND deleted = ?", id, 0).First(&seg).Error
			var segmentID uint64 = 0
			if err == nil {
				segmentID = seg.ID
			}

			var existing model.EduUserVideoRecommend
			err = tx.Model(&model.EduUserVideoRecommend{}).Where("video_id = ? AND user_id = ?", id, userID).First(&existing).Error
			if err != nil {
				rec := model.EduUserVideoRecommend{
					UserID:         userID,
					VideoID:        id,
					QuestionID:     id, // Use video_id as question_id for testing to avoid unique constraint violations
					VideoSegmentID: segmentID,
					RecommendScore: recommendScore,
					IsWatched:      false,
					RecommendLevel: recommendLevel,
					Deleted:        0,
				}
				if err := tx.Create(&rec).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Model(&existing).Updates(map[string]any{
					"deleted":         0,
					"recommend_level": recommendLevel,
					"recommend_score": recommendScore,
				}).Error; err != nil {
					return err
				}
			}
		} else {
			if err := tx.Model(&model.EduUserVideoRecommend{}).Where("video_id = ? AND user_id = ?", id, userID).Update("deleted", 1).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

// UpdateCoverByID 更新视频封面地址。
func (r *GormVideoRepository) UpdateCoverByID(ctx context.Context, id uint64, coverURL string) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.EduVideoResource{}).Where("id = ? AND deleted = ?", id, 0).Update("cover_url", coverURL)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// IncrementViewCount 原子增加视频观看次数，并返回最新计数。
func (r *GormVideoRepository) IncrementViewCount(ctx context.Context, id uint64) (int, bool, error) {
	res := r.db.WithContext(ctx).Model(&model.EduVideoResource{}).Where("id = ? AND deleted = ?", id, 0).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1))
	if res.Error != nil {
		return 0, false, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, false, nil
	}
	vc, ok, err := r.GetViewCount(ctx, id)
	if err != nil {
		return 0, false, err
	}
	return vc, ok, nil
}

// GetViewCount 查询视频观看次数。
func (r *GormVideoRepository) GetViewCount(ctx context.Context, id uint64) (int, bool, error) {
	var m model.EduVideoResource
	err := r.db.WithContext(ctx).Model(&model.EduVideoResource{}).Select("id", "view_count").Where("id = ? AND deleted = ?", id, 0).First(&m).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, false, nil
		}
		return 0, false, err
	}
	return m.ViewCount, true, nil
}

func reactionCounterColumn(reactionType videoapp.VideoReactionType) string {
	switch reactionType {
	case videoapp.VideoReactionLike:
		return "like_count"
	case videoapp.VideoReactionDoubleLike:
		return "double_like_count"
	case videoapp.VideoReactionDislike:
		return "dislike_count"
	default:
		return ""
	}
}

// ApplyVideoReactionState 按最终状态幂等写入用户评价，供 Redis Stream 消费者落库使用。
func (r *GormVideoRepository) ApplyVideoReactionState(ctx context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType, active bool) (bool, error) {
	var found bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var video model.EduVideoResource
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted = ?", videoID, 0).
			First(&video).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				found = false
				return nil
			}
			return err
		}
		found = true

		newColumn := reactionCounterColumn(reactionType)
		if newColumn == "" {
			return nil
		}

		var reaction model.EduVideoUserReaction
		res := tx.Where("user_id = ? AND video_id = ?", userID, videoID).Limit(1).Find(&reaction)
		if res.Error != nil {
			return res.Error
		}

		var err error
		if res.RowsAffected == 0 {
			err = gorm.ErrRecordNotFound
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if err == gorm.ErrRecordNotFound {
			if !active {
				return nil
			}
			if err := tx.Create(&model.EduVideoUserReaction{
				UserID:       userID,
				VideoID:      videoID,
				ReactionType: string(reactionType),
				Deleted:      0,
			}).Error; err != nil {
				return err
			}
			return tx.Model(&model.EduVideoResource{}).
				Where("id = ?", videoID).
				UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error
		}

		oldType := videoapp.VideoReactionType(reaction.ReactionType)
		oldColumn := reactionCounterColumn(oldType)
		isActive := reaction.Deleted == 0

		if !active {
			if !isActive || oldType != reactionType {
				return nil
			}
			if err := tx.Model(&model.EduVideoUserReaction{}).
				Where("id = ?", reaction.ID).
				Updates(map[string]any{"deleted": 1}).Error; err != nil {
				return err
			}
			if oldColumn == "" {
				return nil
			}
			return tx.Model(&model.EduVideoResource{}).
				Where("id = ?", videoID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error
		}

		if isActive && oldType == reactionType {
			return nil
		}
		if err := tx.Model(&model.EduVideoUserReaction{}).
			Where("id = ?", reaction.ID).
			Updates(map[string]any{
				"reaction_type": string(reactionType),
				"deleted":       0,
			}).Error; err != nil {
			return err
		}
		if isActive && oldColumn != "" {
			if err := tx.Model(&model.EduVideoResource{}).
				Where("id = ?", videoID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.EduVideoResource{}).
			Where("id = ?", videoID).
			UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// GetVideoUserReaction 读取单个用户对单个视频的当前评价状态。
func (r *GormVideoRepository) GetVideoUserReaction(ctx context.Context, videoID uint64, userID uint64) (videoapp.VideoReactionType, bool, bool, error) {
	var video model.EduVideoResource
	if err := r.db.WithContext(ctx).
		Model(&model.EduVideoResource{}).
		Select("id").
		Where("id = ? AND deleted = ?", videoID, 0).
		First(&video).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", false, false, nil
		}
		return "", false, false, err
	}

	var reaction model.EduVideoUserReaction
	res := r.db.WithContext(ctx).
		Model(&model.EduVideoUserReaction{}).
		Where("user_id = ? AND video_id = ?", userID, videoID).
		Limit(1).
		Find(&reaction)
	if res.Error != nil {
		return "", false, false, res.Error
	}
	if res.RowsAffected == 0 {
		return "", false, true, nil
	}
	return videoapp.VideoReactionType(reaction.ReactionType), reaction.Deleted == 0, true, nil
}

// SubmitVideoReaction 原子提交、切换或取消用户对视频的评价。
func (r *GormVideoRepository) SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType) (bool, bool, error) {
	var active bool
	var found bool

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var video model.EduVideoResource
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted = ?", videoID, 0).
			First(&video).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				found = false
				return nil
			}
			return err
		}
		found = true

		newColumn := reactionCounterColumn(reactionType)
		var reaction model.EduVideoUserReaction
		res := tx.Where("user_id = ? AND video_id = ?", userID, videoID).Limit(1).Find(&reaction)
		if res.Error != nil {
			return res.Error
		}
		err := error(nil)
		if res.RowsAffected == 0 {
			err = gorm.ErrRecordNotFound
		}
		if err == gorm.ErrRecordNotFound {
			if err := tx.Create(&model.EduVideoUserReaction{
				UserID:       userID,
				VideoID:      videoID,
				ReactionType: string(reactionType),
				Deleted:      0,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.EduVideoResource{}).
				Where("id = ?", videoID).
				UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error; err != nil {
				return err
			}
			active = true
			return nil
		}

		oldType := videoapp.VideoReactionType(reaction.ReactionType)
		oldColumn := reactionCounterColumn(oldType)
		isActive := reaction.Deleted == 0

		if isActive && oldType == reactionType {
			if err := tx.Model(&model.EduVideoUserReaction{}).
				Where("id = ?", reaction.ID).
				Updates(map[string]any{"deleted": 1}).Error; err != nil {
				return err
			}
			if oldColumn != "" {
				if err := tx.Model(&model.EduVideoResource{}).
					Where("id = ?", videoID).
					UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error; err != nil {
					return err
				}
			}
			active = false
			return nil
		}

		if err := tx.Model(&model.EduVideoUserReaction{}).
			Where("id = ?", reaction.ID).
			Updates(map[string]any{
				"reaction_type": string(reactionType),
				"deleted":       0,
			}).Error; err != nil {
			return err
		}
		if isActive && oldColumn != "" {
			if err := tx.Model(&model.EduVideoResource{}).
				Where("id = ?", videoID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.EduVideoResource{}).
			Where("id = ?", videoID).
			UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error; err != nil {
			return err
		}
		active = true
		return nil
	})
	if err != nil {
		return false, false, err
	}
	return active, found, nil
}

// GetVideoReactionCounts 查询视频点赞和双赞聚合计数。
func (r *GormVideoRepository) GetVideoReactionCounts(ctx context.Context, videoID uint64) (videoapp.VideoReactionCounts, bool, error) {
	var row model.EduVideoResource
	err := r.db.WithContext(ctx).
		Model(&model.EduVideoResource{}).
		Select("id", "like_count", "double_like_count").
		Where("id = ? AND deleted = ?", videoID, 0).
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return videoapp.VideoReactionCounts{}, false, nil
		}
		return videoapp.VideoReactionCounts{}, false, err
	}
	return videoapp.VideoReactionCounts{
		LikeCount:       int64(row.LikeCount),
		DoubleLikeCount: int64(row.DoubleLikeCount),
	}, true, nil
}

// ApplySegmentReactionState 按最终状态幂等写入用户对视频片段的评价。
func (r *GormVideoRepository) ApplySegmentReactionState(ctx context.Context, segmentID uint64, userID uint64, reactionType videoapp.VideoReactionType, active bool) (bool, error) {
	var found bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var segment model.EduVideoSegment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "video_id").
			Where("id = ? AND deleted = ?", segmentID, 0).
			First(&segment).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				found = false
				return nil
			}
			return err
		}
		found = true

		newColumn := reactionCounterColumn(reactionType)
		if newColumn == "" {
			return nil
		}

		var reaction model.EduUserReaction
		res := tx.Where("user_id = ? AND video_segment_id = ?", userID, segmentID).Limit(1).Find(&reaction)
		if res.Error != nil {
			return res.Error
		}
		err := error(nil)
		if res.RowsAffected == 0 {
			err = gorm.ErrRecordNotFound
		}

		if err == gorm.ErrRecordNotFound {
			if !active {
				return nil
			}
			if err := tx.Create(&model.EduUserReaction{
				UserID:         userID,
				VideoID:        segment.VideoID,
				VideoSegmentID: segmentID,
				ReactionType:   string(reactionType),
				Deleted:        0,
			}).Error; err != nil {
				return err
			}
			return tx.Model(&model.EduVideoSegment{}).
				Where("id = ?", segmentID).
				UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error
		}

		oldType := videoapp.VideoReactionType(reaction.ReactionType)
		oldColumn := reactionCounterColumn(oldType)
		isActive := reaction.Deleted == 0

		if !active {
			if !isActive || oldType != reactionType {
				return nil
			}
			if err := tx.Model(&model.EduUserReaction{}).
				Where("id = ?", reaction.ID).
				Updates(map[string]any{"deleted": 1, "video_id": segment.VideoID}).Error; err != nil {
				return err
			}
			if oldColumn == "" {
				return nil
			}
			return tx.Model(&model.EduVideoSegment{}).
				Where("id = ?", segmentID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error
		}

		if isActive && oldType == reactionType {
			return nil
		}
		if err := tx.Model(&model.EduUserReaction{}).
			Where("id = ?", reaction.ID).
			Updates(map[string]any{
				"video_id":      segment.VideoID,
				"reaction_type": string(reactionType),
				"deleted":       0,
			}).Error; err != nil {
			return err
		}
		if isActive && oldColumn != "" {
			if err := tx.Model(&model.EduVideoSegment{}).
				Where("id = ?", segmentID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.EduVideoSegment{}).
			Where("id = ?", segmentID).
			UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// GetSegmentUserReaction 读取单个用户对单个视频片段的当前评价状态。
func (r *GormVideoRepository) GetSegmentUserReaction(ctx context.Context, segmentID uint64, userID uint64) (videoapp.VideoReactionType, bool, bool, error) {
	var segment model.EduVideoSegment
	if err := r.db.WithContext(ctx).
		Model(&model.EduVideoSegment{}).
		Select("id").
		Where("id = ? AND deleted = ?", segmentID, 0).
		First(&segment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", false, false, nil
		}
		return "", false, false, err
	}

	var reaction model.EduUserReaction
	res := r.db.WithContext(ctx).
		Model(&model.EduUserReaction{}).
		Where("user_id = ? AND video_segment_id = ?", userID, segmentID).
		Limit(1).
		Find(&reaction)
	if res.Error != nil {
		return "", false, false, res.Error
	}
	if res.RowsAffected == 0 {
		return "", false, true, nil
	}
	return videoapp.VideoReactionType(reaction.ReactionType), reaction.Deleted == 0, true, nil
}

// SubmitSegmentReaction 原子提交、切换或取消用户对视频片段的评价。
func (r *GormVideoRepository) SubmitSegmentReaction(ctx context.Context, segmentID uint64, userID uint64, reactionType videoapp.VideoReactionType) (bool, bool, error) {
	var active bool
	var found bool

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var segment model.EduVideoSegment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "video_id").
			Where("id = ? AND deleted = ?", segmentID, 0).
			First(&segment).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				found = false
				return nil
			}
			return err
		}
		found = true

		newColumn := reactionCounterColumn(reactionType)
		var reaction model.EduUserReaction
		res := tx.Where("user_id = ? AND video_segment_id = ?", userID, segmentID).Limit(1).Find(&reaction)
		if res.Error != nil {
			return res.Error
		}
		err := error(nil)
		if res.RowsAffected == 0 {
			err = gorm.ErrRecordNotFound
		}
		if err == gorm.ErrRecordNotFound {
			if err := tx.Create(&model.EduUserReaction{
				UserID:         userID,
				VideoID:        segment.VideoID,
				VideoSegmentID: segmentID,
				ReactionType:   string(reactionType),
				Deleted:        0,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.EduVideoSegment{}).
				Where("id = ?", segmentID).
				UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error; err != nil {
				return err
			}
			active = true
			return nil
		}

		oldType := videoapp.VideoReactionType(reaction.ReactionType)
		oldColumn := reactionCounterColumn(oldType)
		isActive := reaction.Deleted == 0

		if isActive && oldType == reactionType {
			if err := tx.Model(&model.EduUserReaction{}).
				Where("id = ?", reaction.ID).
				Updates(map[string]any{"deleted": 1, "video_id": segment.VideoID}).Error; err != nil {
				return err
			}
			if oldColumn != "" {
				if err := tx.Model(&model.EduVideoSegment{}).
					Where("id = ?", segmentID).
					UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error; err != nil {
					return err
				}
			}
			active = false
			return nil
		}

		if err := tx.Model(&model.EduUserReaction{}).
			Where("id = ?", reaction.ID).
			Updates(map[string]any{
				"video_id":      segment.VideoID,
				"reaction_type": string(reactionType),
				"deleted":       0,
			}).Error; err != nil {
			return err
		}
		if isActive && oldColumn != "" {
			if err := tx.Model(&model.EduVideoSegment{}).
				Where("id = ?", segmentID).
				UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - ?", 1)).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.EduVideoSegment{}).
			Where("id = ?", segmentID).
			UpdateColumn(newColumn, gorm.Expr(newColumn+" + ?", 1)).Error; err != nil {
			return err
		}
		active = true
		return nil
	})
	if err != nil {
		return false, false, err
	}
	return active, found, nil
}

// GetSegmentReactionCounts 查询视频片段点赞和双赞聚合计数。
func (r *GormVideoRepository) GetSegmentReactionCounts(ctx context.Context, segmentID uint64) (videoapp.VideoReactionCounts, bool, error) {
	var row model.EduVideoSegment
	err := r.db.WithContext(ctx).
		Model(&model.EduVideoSegment{}).
		Select("id", "like_count", "double_like_count").
		Where("id = ? AND deleted = ?", segmentID, 0).
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return videoapp.VideoReactionCounts{}, false, nil
		}
		return videoapp.VideoReactionCounts{}, false, err
	}
	return videoapp.VideoReactionCounts{
		LikeCount:       int64(row.LikeCount),
		DoubleLikeCount: int64(row.DoubleLikeCount),
	}, true, nil
}

// GetSegmentEmbeddingDim 查询当前片段向量的维度。
func (r *GormVideoRepository) GetSegmentEmbeddingDim(ctx context.Context) (int, error) {
	var d int
	err := r.db.WithContext(ctx).Raw("SELECT vector_dims(embedding) FROM edu_video_segment WHERE deleted = 0 AND status = 1 AND embedding IS NOT NULL LIMIT 1;").Scan(&d).Error
	return d, err
}

// GetQuestionEmbeddingTextByID 读取题库表中预计算的题目向量文本。
func (r *GormVideoRepository) GetQuestionEmbeddingTextByID(ctx context.Context, questionID uint64) (string, error) {
	var row struct {
		Embedding string `gorm:"column:embedding"`
	}
	err := r.db.WithContext(ctx).Raw(sqlqueries.GetQuestionEmbeddingByIDQuery, questionID).Scan(&row).Error
	return row.Embedding, err
}

func (r *GormVideoRepository) ListQuestions(ctx context.Context, page int, pageSize int) (videoapp.QuestionPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := r.db.WithContext(ctx).Raw(sqlqueries.CountQuestionBankQuery).Scan(&total).Error; err != nil {
		return videoapp.QuestionPage{}, err
	}

	rows := make([]videoapp.QuestionItem, 0, pageSize)
	if err := r.db.WithContext(ctx).Raw(sqlqueries.ListQuestionBankQuery, pageSize, offset).Scan(&rows).Error; err != nil {
		return videoapp.QuestionPage{}, err
	}

	return videoapp.QuestionPage{Total: total, Page: page, PageSize: pageSize, Items: rows}, nil
}

func (r *GormVideoRepository) GetQuestionByID(ctx context.Context, id uint64) (videoapp.QuestionItem, bool, error) {
	var row videoapp.QuestionItem
	if err := r.db.WithContext(ctx).Raw(sqlqueries.GetQuestionByIDQuery, id).Scan(&row).Error; err != nil {
		return videoapp.QuestionItem{}, false, err
	}
	if row.ID == 0 {
		return videoapp.QuestionItem{}, false, nil
	}
	return row, true, nil
}

// FindRecommendedSegments 按向量近邻搜索推荐片段。
func (r *GormVideoRepository) FindRecommendedSegments(ctx context.Context, query pgvector.Vector, limit int) ([]videoapp.RecommendCandidate, error) {
	rows := make([]videoapp.RecommendCandidate, 0, limit)
	err := r.db.WithContext(ctx).Raw(sqlqueries.RecommendByQuestionQuery, query, query, limit).Scan(&rows).Error
	return rows, err
}

// FindRandomPlayableSegment 随机返回一个未删除、已发布且已转码完成的视频片段。
func (r *GormVideoRepository) FindRandomPlayableSegment(ctx context.Context) (videoapp.RecommendResultItem, bool, error) {
	type row struct {
		VideoSegmentID uint64    `gorm:"column:video_segment_id"`
		VideoID        uint64    `gorm:"column:video_id"`
		StartTimeSec   int       `gorm:"column:start_time_sec"`
		EndTimeSec     int       `gorm:"column:end_time_sec"`
		Title          string    `gorm:"column:title"`
		VideoTitle     string    `gorm:"column:video_title"`
		Description    string    `gorm:"column:description"`
		VideoURL       string    `gorm:"column:video_url"`
		Duration       int       `gorm:"column:duration"`
		CoverURL       string    `gorm:"column:cover_url"`
		Status         int16     `gorm:"column:status"`
		ErrorMsg       string    `gorm:"column:error_msg"`
		IsPublished    bool      `gorm:"column:is_published"`
		IsRecommend    bool      `gorm:"column:is_recommend"`
		ViewCount      int       `gorm:"column:view_count"`
		CreateTime     time.Time `gorm:"column:create_time"`
		UpdateTime     time.Time `gorm:"column:update_time"`
		Deleted        int16     `gorm:"column:deleted"`
	}

	var item row
	err := r.db.WithContext(ctx).
		Table("edu_video_segment AS s").
		Select(`
			s.id AS video_segment_id,
			s.video_id AS video_id,
			s.start_time AS start_time_sec,
			s.end_time AS end_time_sec,
			CASE
				WHEN TRIM(COALESCE(s.content_summary, '')) <> '' THEN s.content_summary
				ELSE r.title
			END AS title,
			r.title AS video_title,
			r.description AS description,
			r.video_url AS video_url,
			r.duration AS duration,
			r.cover_url AS cover_url,
			r.status AS status,
			r.error_msg AS error_msg,
			r.is_published AS is_published,
			r.is_recommend AS is_recommend,
			r.view_count AS view_count,
			r.create_time AS create_time,
			r.update_time AS update_time,
			r.deleted AS deleted`).
		Joins("JOIN edu_video_resource AS r ON r.id = s.video_id").
		Where("s.deleted = ? AND s.status = ? AND r.deleted = ? AND r.is_published = ? AND r.status = ? AND TRIM(COALESCE(r.video_url, '')) <> ''", 0, 1, 0, true, int16(domainvideo.StatusDone)).
		Order("RANDOM()").
		Limit(1).
		Scan(&item).Error
	if err != nil {
		return videoapp.RecommendResultItem{}, false, err
	}
	if item.VideoSegmentID == 0 {
		return videoapp.RecommendResultItem{}, false, nil
	}

	return videoapp.RecommendResultItem{
		VideoID:        item.VideoID,
		VideoSegmentID: item.VideoSegmentID,
		StartTimeSec:   item.StartTimeSec,
		EndTimeSec:     item.EndTimeSec,
		TitleOverride:  item.Title,
		Video: domainvideo.Video{
			ID:          item.VideoID,
			Title:       item.VideoTitle,
			Description: item.Description,
			VideoURL:    item.VideoURL,
			Duration:    item.Duration,
			CoverURL:    item.CoverURL,
			Status:      domainvideo.Status(item.Status),
			ErrorMsg:    item.ErrorMsg,
			IsPublished: item.IsPublished,
			IsRecommend: item.IsRecommend,
			ViewCount:   item.ViewCount,
			CreateTime:  item.CreateTime,
			UpdateTime:  item.UpdateTime,
			Deleted:     item.Deleted,
		},
	}, true, nil
}

// SaveUserVideoRecommendation 写入或更新用户推荐记录。
func (r *GormVideoRepository) SaveUserVideoRecommendation(ctx context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, now time.Time) error {
	return r.db.WithContext(ctx).Exec(sqlqueries.UpsertUserVideoRecommendQuery, userID, videoID, questionID, segmentID, score, now, now).Error
}

// ListRecommendations 查询用户的推荐记录列表。
func (r *GormVideoRepository) ListRecommendations(ctx context.Context, userID uint64, questionID uint64, limit int) ([]videoapp.RecommendationRecord, error) {
	rows := make([]videoapp.RecommendationRecord, 0, limit)
	err := r.db.WithContext(ctx).Raw(sqlqueries.ListRecommendationsQuery, userID, questionID, limit).Scan(&rows).Error
	return rows, err
}

// GetVideoIDBySegmentID 根据片段 ID 反查所属视频 ID。
func (r *GormVideoRepository) GetVideoIDBySegmentID(ctx context.Context, segmentID uint64) (uint64, error) {
	var videoID uint64
	err := r.db.WithContext(ctx).Raw(sqlqueries.GetVideoIDBySegmentIDQuery, segmentID).Scan(&videoID).Error
	return videoID, err
}

// HasWatchedVideoForQuestion 判断同一用户在同一题目下是否已对该视频产生观看记录。
func (r *GormVideoRepository) HasWatchedVideoForQuestion(ctx context.Context, userID uint64, questionID uint64, videoID uint64) (bool, error) {
	var row struct {
		Exists int `gorm:"column:exists"`
	}
	err := r.db.WithContext(ctx).Raw(sqlqueries.HasWatchedVideoForQuestionQuery, userID, questionID, videoID).Scan(&row).Error
	if err != nil {
		return false, err
	}
	return row.Exists == 1, nil
}

// SaveWatchRecord 写入或更新观看记录。
func (r *GormVideoRepository) SaveWatchRecord(ctx context.Context, userID uint64, videoID uint64, questionID uint64, segmentID uint64, isWatched bool, watchDuration int, now time.Time) (bool, error) {
	var row struct {
		Inserted bool `gorm:"column:inserted"`
	}
	err := r.db.WithContext(ctx).Raw(sqlqueries.UpsertWatchRecordQuery, userID, videoID, questionID, segmentID, isWatched, watchDuration, now, now).Scan(&row).Error
	if err != nil {
		return false, err
	}
	return row.Inserted, nil
}

// FindSimilar 优先使用片段向量均值做近邻搜索。
// 如果视频还没有向量数据，则退化为按主键邻近的视频列表。
func (r *GormVideoRepository) FindSimilar(ctx context.Context, id uint64, limit int) ([]domainvideo.Video, error) {
	if limit <= 0 {
		limit = 6
	}

	baseVec, hasVec, err := r.averageEmbeddingForVideo(ctx, id, 3)
	if err != nil {
		return nil, err
	}
	if hasVec {
		type row struct {
			VideoID uint64  `gorm:"column:video_id"`
			Dist    float64 `gorm:"column:dist"`
		}
		var rows []row
		if err := r.db.WithContext(ctx).Raw(sqlqueries.FindSimilarVideosQuery, baseVec, id, limit).Scan(&rows).Error; err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			ids := make([]uint64, 0, len(rows))
			for _, it := range rows {
				ids = append(ids, it.VideoID)
			}
			var ms []model.EduVideoResource
			if err := r.db.WithContext(ctx).
				Model(&model.EduVideoResource{}).
				Where("deleted = 0 AND id IN ?", ids).
				Find(&ms).Error; err != nil {
				return nil, err
			}
			mByID := make(map[uint64]model.EduVideoResource, len(ms))
			for _, m := range ms {
				mByID[m.ID] = m
			}
			out := make([]domainvideo.Video, 0, len(ids))
			for _, vid := range ids {
				m, ok := mByID[vid]
				if !ok {
					continue
				}
				out = append(out, toDomainVideoResource(m))
			}
			return out, nil
		}
	}

	nBefore := limit / 2
	nAfter := limit - nBefore

	var before []model.EduVideoResource
	if nBefore > 0 {
		if err := r.db.WithContext(ctx).
			Model(&model.EduVideoResource{}).
			Where("deleted = 0 AND id < ? AND is_published = ?", id, true).
			Order("id desc").
			Limit(nBefore).
			Find(&before).Error; err != nil {
			return nil, err
		}
	}

	var after []model.EduVideoResource
	if nAfter > 0 {
		if err := r.db.WithContext(ctx).
			Model(&model.EduVideoResource{}).
			Where("deleted = 0 AND id > ? AND is_published = ?", id, true).
			Order("id asc").
			Limit(nAfter).
			Find(&after).Error; err != nil {
			return nil, err
		}
	}

	for i, j := 0, len(before)-1; i < j; i, j = i+1, j-1 {
		before[i], before[j] = before[j], before[i]
	}

	out := make([]domainvideo.Video, 0, len(before)+len(after))
	for _, m := range before {
		out = append(out, toDomainVideoResource(m))
	}
	for _, m := range after {
		out = append(out, toDomainVideoResource(m))
	}
	return out, nil
}

// UpdateStatusByID 更新视频处理状态及错误信息。
func (r *GormVideoRepository) UpdateStatusByID(ctx context.Context, id uint64, status domainvideo.Status, errMsg string) error {
	updates := map[string]interface{}{
		"status": int16(status),
	}
	if errMsg != "" {
		updates["error_msg"] = errMsg
	} else {
		updates["error_msg"] = ""
	}
	return r.db.WithContext(ctx).Model(&model.EduVideoResource{}).Where("id = ? AND deleted = ?", id, 0).Updates(updates).Error
}

// averageEmbeddingForVideo 计算视频若干分段 embedding 的均值向量，供相似视频召回使用。
func (r *GormVideoRepository) averageEmbeddingForVideo(ctx context.Context, videoID uint64, take int) (pgvector.Vector, bool, error) {
	if take <= 0 {
		take = 3
	}
	var segs []model.EduVideoSegment
	if err := r.db.WithContext(ctx).
		Model(&model.EduVideoSegment{}).
		Select("embedding").
		Where("deleted = 0 AND status = 1 AND video_id = ?", videoID).
		Order("segment_index asc").
		Limit(take).
		Find(&segs).Error; err != nil {
		return pgvector.Vector{}, false, err
	}
	if len(segs) == 0 {
		return pgvector.Vector{}, false, nil
	}
	dim := len(segs[0].Embedding.Slice())
	if dim == 0 {
		return pgvector.Vector{}, false, nil
	}
	sum := make([]float32, dim)
	n := 0
	for _, s := range segs {
		vec := s.Embedding.Slice()
		if len(vec) != dim {
			continue
		}
		for i := 0; i < dim; i++ {
			sum[i] += vec[i]
		}
		n++
	}
	if n == 0 {
		return pgvector.Vector{}, false, nil
	}
	inv := float32(1.0 / float64(n))
	for i := 0; i < dim; i++ {
		sum[i] *= inv
		if math.IsNaN(float64(sum[i])) || math.IsInf(float64(sum[i]), 0) {
			sum[i] = 0
		}
	}
	return pgvector.NewVector(sum), true, nil
}

// toDomainVideoResource 把数据库模型转换成领域对象。
func toDomainVideoResource(m model.EduVideoResource) domainvideo.Video {
	return domainvideo.Video{
		ID:          m.ID,
		Title:       m.Title,
		Description: m.Description,
		VideoURL:    m.VideoURL,
		Duration:    m.Duration,
		CoverURL:    m.CoverURL,
		Status:      domainvideo.Status(m.Status),
		ErrorMsg:    m.ErrorMsg,
		IsPublished: m.IsPublish,
		IsRecommend: m.IsRec,
		ViewCount:   m.ViewCount,
		CreateTime:  m.CreateTime,
		UpdateTime:  m.UpdateTime,
		Deleted:     m.Deleted,
	}
}

// EnsureIntegrity 创建补充字段、索引、约束和触发器，保证现有表结构满足当前代码依赖。
func EnsureIntegrity(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	if err := db.Exec(sqlqueries.AddRecommendColumnsQuery).Error; err != nil {
		return err
	}
	_ = db.Exec(sqlqueries.CreateUserQuestionIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateSegmentIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateVideoReactionUserIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateVideoReactionVideoIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateUserReactionUserIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateUserReactionVideoIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateUserReactionSegmentIndexQuery).Error

	if err := db.Exec(sqlqueries.CreateUserQuestionSegmentUniqueConstraintQuery).Error; err != nil {
		return err
	}
	if err := db.Exec(sqlqueries.CreateVideoReactionUniqueConstraintQuery).Error; err != nil {
		return err
	}
	if err := db.Exec(sqlqueries.CreateUserReactionUniqueConstraintQuery).Error; err != nil {
		return err
	}

	if err := db.Exec(sqlqueries.CleanOrphanSegmentsQuery).Error; err != nil {
		return err
	}
	if err := db.Exec(sqlqueries.CleanOrphanRecommendationsQuery).Error; err != nil {
		return err
	}

	if err := db.Exec(sqlqueries.CreateSegmentVideoForeignKeyQuery).Error; err != nil {
		return err
	}
	if err := db.Exec(sqlqueries.CreateRecommendVideoForeignKeyQuery).Error; err != nil {
		return err
	}

	if err := db.Exec(sqlqueries.CreateSyncVideoDeletedFunctionQuery).Error; err != nil {
		return err
	}

	if err := db.Exec(sqlqueries.CreateSyncVideoDeletedTriggerQuery).Error; err != nil {
		return err
	}

	return nil
}
