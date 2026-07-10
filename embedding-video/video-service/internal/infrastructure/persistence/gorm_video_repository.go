package persistence

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
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

const readyVideoSegmentStatus int16 = 1

// NewGormVideoRepository 创建基于 GORM 的视频仓储实现。
func NewGormVideoRepository(db *gorm.DB) *GormVideoRepository {
	return &GormVideoRepository{db: db}
}

// Create 写入一条新的视频资源记录。
func (r *GormVideoRepository) Create(ctx context.Context, v *domainvideo.Video) error {
	m := &model.EduVideoResource{
		UserID:      v.UserID,
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

func (r *GormVideoRepository) CanUploadVideo(ctx context.Context, userID uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("sys_user").
		Where("id = ? AND user_type IN ?", userID, []int{2, 3}).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
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

func (r *GormVideoRepository) GetArchiveProcessingProgress(ctx context.Context, videoIDs []uint64) (videoapp.ArchiveProcessingProgress, error) {
	uniqueIDs := uniqueUint64s(videoIDs)
	progress := videoapp.ArchiveProcessingProgress{Total: len(uniqueIDs)}
	if len(uniqueIDs) == 0 {
		return progress, nil
	}
	var transcoded int64
	if err := r.db.WithContext(ctx).
		Model(&model.EduVideoResource{}).
		Where("id IN ? AND deleted = ? AND status = ?", uniqueIDs, 0, int16(domainvideo.StatusDone)).
		Count(&transcoded).Error; err != nil {
		return videoapp.ArchiveProcessingProgress{}, err
	}
	var vectorized int64
	if err := r.db.WithContext(ctx).
		Model(&model.EduVideoVectorStage{}).
		Distinct("video_id").
		Where("video_id IN ? AND stage = ? AND status = ?", uniqueIDs, "vector.finalize", int16(2)).
		Count(&vectorized).Error; err != nil {
		return videoapp.ArchiveProcessingProgress{}, err
	}
	progress.Transcoded = int(transcoded)
	progress.Vectorized = int(vectorized)
	return progress, nil
}

func uniqueUint64s(values []uint64) []uint64 {
	seen := make(map[uint64]bool, len(values))
	out := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
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

func (r *GormVideoRepository) FindRecommendedSegmentsByWeakKnowledgeVector(ctx context.Context, input videoapp.WeakKnowledgeVectorQuery) ([]videoapp.RecommendCandidate, error) {
	if input.Limit <= 0 {
		input.Limit = 1
	}
	rows := make([]videoapp.RecommendCandidate, 0, input.Limit)
	err := r.db.WithContext(ctx).Raw(
		sqlqueries.RecommendByWeakKnowledgeVectorQuery,
		input.Query,
		input.UserID,
		input.UserID,
		input.UserID,
		input.RequireRecommend,
		input.Query,
		input.Limit,
	).Scan(&rows).Error
	return rows, err
}

func (r *GormVideoRepository) GetUserVideoProfile(ctx context.Context, userID uint64, modelVersion string) (videoapp.UserVideoProfile, bool, error) {
	var row struct {
		UserID        uint64 `gorm:"column:user_id"`
		ProfileVector string `gorm:"column:profile_vector"`
		PositiveCount int    `gorm:"column:positive_count"`
		ModelVersion  string `gorm:"column:model_version"`
		Status        int16  `gorm:"column:status"`
	}
	if err := r.db.WithContext(ctx).Raw(sqlqueries.GetUserVideoProfileQuery, userID, modelVersion).Scan(&row).Error; err != nil {
		return videoapp.UserVideoProfile{}, false, err
	}
	if row.UserID == 0 {
		return videoapp.UserVideoProfile{}, false, nil
	}
	vector, err := parseVectorText(row.ProfileVector)
	if err != nil {
		return videoapp.UserVideoProfile{}, false, err
	}
	return videoapp.UserVideoProfile{
		UserID:        row.UserID,
		ProfileVector: vector,
		ModelVersion:  row.ModelVersion,
		Status:        row.Status,
		PositiveCount: row.PositiveCount,
	}, true, nil
}

func (r *GormVideoRepository) FindRecommendedSegmentsForProfileRerank(ctx context.Context, input videoapp.ProfileRerankQuery) ([]videoapp.ProfileRerankCandidate, error) {
	rows := make([]videoapp.ProfileRerankCandidate, 0, input.Limit)
	err := r.db.WithContext(ctx).Raw(
		sqlqueries.RecommendByQuestionWithProfileQuery,
		input.QuestionVector,
		input.ProfileVector,
		input.UserID,
		input.UserID,
		input.UserID,
		input.QuestionVector,
		input.Limit,
	).Scan(&rows).Error
	return rows, err
}

func (r *GormVideoRepository) GetUserRecBoleEmbedding(ctx context.Context, userID uint64, modelVersion string) (videoapp.UserRecBoleEmbedding, bool, error) {
	var row struct {
		UserID       uint64 `gorm:"column:user_id"`
		Embedding    string `gorm:"column:embedding"`
		ModelVersion string `gorm:"column:model_version"`
		Status       int16  `gorm:"column:status"`
	}
	if err := r.db.WithContext(ctx).Raw(sqlqueries.GetUserRecBoleEmbeddingQuery, userID, "recbole", modelVersion).Scan(&row).Error; err != nil {
		return videoapp.UserRecBoleEmbedding{}, false, err
	}
	if row.UserID == 0 {
		return videoapp.UserRecBoleEmbedding{}, false, nil
	}
	vector, err := parseVectorText(row.Embedding)
	if err != nil {
		return videoapp.UserRecBoleEmbedding{}, false, err
	}
	return videoapp.UserRecBoleEmbedding{
		UserID:       row.UserID,
		Vector:       vector,
		ModelVersion: row.ModelVersion,
		Status:       row.Status,
	}, true, nil
}

func (r *GormVideoRepository) FindRecommendedSegmentsForRecBole(ctx context.Context, input videoapp.RecBoleQuery) ([]videoapp.RecBoleCandidate, error) {
	rows := make([]videoapp.RecBoleCandidate, 0, input.Limit)
	err := r.db.WithContext(ctx).Raw(
		sqlqueries.RecommendByRecBoleQuery,
		input.UserVector,
		"recbole",
		input.ModelVersion,
		input.UserVector,
		input.Limit,
	).Scan(&rows).Error
	return rows, err
}

func (r *GormVideoRepository) HydrateRecommendedSegmentsByID(ctx context.Context, userID uint64, ids []uint64) ([]videoapp.RecommendCandidate, error) {
	ids = uniqueUint64s(ids)
	if len(ids) == 0 {
		return nil, nil
	}
	rows := make([]videoapp.RecommendCandidate, 0, len(ids))
	query := r.db.WithContext(ctx).
		Table("edu_video_segment AS s").
		Select(`
			s.id AS video_segment_id,
			s.video_id AS video_id,
			s.start_time AS start_time_sec,
			s.end_time AS end_time_sec,
			CASE
				WHEN TRIM(COALESCE(s.content_summary, '')) <> '' THEN s.content_summary
				ELSE r.title
			END AS segment_title,
			r.video_url AS video_url,
			r.cover_url AS cover_url,
			r.status AS status,
			r.is_published AS is_published,
			r.is_recommend AS is_recommend,
			r.view_count AS view_count,
			r.create_time AS create_time,
			r.update_time AS update_time`).
		Joins("JOIN edu_video_resource AS r ON r.id = s.video_id").
		Where("s.id IN ? AND s.deleted = ? AND s.status = ? AND r.deleted = ? AND r.is_published = ? AND r.is_recommend = ? AND r.status = ? AND TRIM(COALESCE(r.video_url, '')) <> ''",
			ids,
			0,
			1,
			0,
			true,
			true,
			int16(domainvideo.StatusDone),
		)
	if userID > 0 {
		query = query.
			Joins("LEFT JOIN edu_user_reaction AS ur ON ur.user_id = ? AND ur.video_segment_id = s.id AND ur.reaction_type = ? AND ur.deleted = 0", userID, "dislike").
			Joins("LEFT JOIN edu_video_user_reaction AS vur ON vur.user_id = ? AND vur.video_id = s.video_id AND vur.reaction_type = ? AND vur.deleted = 0", userID, "dislike").
			Joins("LEFT JOIN edu_user_video_recommend AS watched ON watched.user_id = ? AND watched.video_segment_id = s.id AND watched.is_watched = ? AND watched.deleted = 0", userID, true).
			Where("ur.id IS NULL AND vur.id IS NULL AND watched.id IS NULL")
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint64]videoapp.RecommendCandidate, len(rows))
	for _, row := range rows {
		byID[row.VideoSegmentID] = row
	}
	ordered := make([]videoapp.RecommendCandidate, 0, len(rows))
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			ordered = append(ordered, row)
		}
	}
	return ordered, nil
}

func (r *GormVideoRepository) FindRecommendedSegmentsByWeakKnowledge(ctx context.Context, userID uint64, limit int, weakLimit int) ([]videoapp.RecommendCandidate, error) {
	if userID == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 1
	}
	if weakLimit <= 0 {
		weakLimit = 10
	}
	weakKnowledge, err := r.loadWeakKnowledge(ctx, userID, weakLimit)
	if err != nil {
		return nil, err
	}
	if len(weakKnowledge) == 0 {
		return nil, nil
	}
	candidates, err := r.loadKnowledgeMatchCandidatePool(ctx, userID)
	if err != nil {
		return nil, err
	}
	scored := make([]struct {
		candidate videoapp.RecommendCandidate
		score     float64
		strength  int
		index     int
	}, 0, len(candidates))
	for i, candidate := range candidates {
		bestMastery, matchStrength, ok := matchWeakKnowledge(candidate, weakKnowledge)
		if !ok {
			continue
		}
		recommendCandidate := candidate.RecommendCandidate
		recommendCandidate.Distance = bestMastery
		scored = append(scored, struct {
			candidate videoapp.RecommendCandidate
			score     float64
			strength  int
			index     int
		}{candidate: recommendCandidate, score: bestMastery, strength: matchStrength, index: i})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			if scored[i].strength != scored[j].strength {
				return scored[i].strength > scored[j].strength
			}
			return scored[i].index < scored[j].index
		}
		return scored[i].score < scored[j].score
	})
	if limit > len(scored) {
		limit = len(scored)
	}
	out := make([]videoapp.RecommendCandidate, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, scored[i].candidate)
	}
	return out, nil
}

type weakKnowledgeRow struct {
	KnowledgePointID uint64  `gorm:"column:knowledge_point_id"`
	Mastery          float64 `gorm:"column:mastery"`
	Name             string  `gorm:"column:name"`
	Description      string  `gorm:"column:description"`
}

func (r *GormVideoRepository) ListWeakKnowledge(ctx context.Context, userID uint64, limit int) ([]videoapp.WeakKnowledge, error) {
	rows, err := r.loadWeakKnowledge(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]videoapp.WeakKnowledge, 0, len(rows))
	for _, row := range rows {
		out = append(out, videoapp.WeakKnowledge{
			KnowledgePointID: row.KnowledgePointID,
			Mastery:          row.Mastery,
			Name:             row.Name,
			Description:      row.Description,
		})
	}
	return out, nil
}

func (r *GormVideoRepository) loadWeakKnowledge(ctx context.Context, userID uint64, limit int) ([]weakKnowledgeRow, error) {
	descriptionExpr := "''"
	hasDiscription := r.db.Migrator().HasColumn("dict_knowledge_point", "discription")
	hasDescription := r.db.Migrator().HasColumn("dict_knowledge_point", "description")
	switch {
	case hasDiscription && hasDescription:
		descriptionExpr = "COALESCE(dkp.discription, dkp.description, '')"
	case hasDiscription:
		descriptionExpr = "COALESCE(dkp.discription, '')"
	case hasDescription:
		descriptionExpr = "COALESCE(dkp.description, '')"
	}
	knowledgePointColumn := "ukm.knowledge_point_id"
	if !r.db.Migrator().HasColumn("edu_user_knowledge_mastery", "knowledge_point_id") && r.db.Migrator().HasColumn("edu_user_knowledge_mastery", "Knowledge_point_id") {
		knowledgePointColumn = `ukm."Knowledge_point_id"`
	}
	where := []string{"ukm.user_id = ?"}
	if r.db.Migrator().HasColumn("edu_user_knowledge_mastery", "deleted") {
		where = append(where, "COALESCE(ukm.deleted, 0) = 0")
	}
	if r.db.Migrator().HasColumn("dict_knowledge_point", "deleted") {
		where = append(where, "COALESCE(dkp.deleted, 0) = 0")
	}
	query := `
SELECT ` + knowledgePointColumn + ` AS knowledge_point_id,
       COALESCE(ukm.mastery, 0) AS mastery,
       COALESCE(dkp.name, '') AS name,
       ` + descriptionExpr + ` AS description
FROM edu_user_knowledge_mastery ukm
JOIN dict_knowledge_point dkp ON dkp.id = ` + knowledgePointColumn + `
WHERE ` + strings.Join(where, "\n  AND ") + `
ORDER BY COALESCE(ukm.mastery, 0) ASC, ` + knowledgePointColumn + ` ASC
LIMIT ?`
	rows := make([]weakKnowledgeRow, 0, limit)
	if err := r.db.WithContext(ctx).Raw(query, userID, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

type knowledgeMatchCandidateRow struct {
	videoapp.RecommendCandidate
	KnowledgeTags string `gorm:"column:knowledge_tags"`
	VideoTitle    string `gorm:"column:video_title"`
	Description   string `gorm:"column:description"`
}

func (r *GormVideoRepository) loadKnowledgeMatchCandidatePool(ctx context.Context, userID uint64) ([]knowledgeMatchCandidateRow, error) {
	rows := make([]knowledgeMatchCandidateRow, 0, 300)
	query := r.db.WithContext(ctx).
		Table("edu_video_segment AS s").
		Select(`
			s.id AS video_segment_id,
			s.video_id AS video_id,
			s.start_time AS start_time_sec,
			s.end_time AS end_time_sec,
			CASE
				WHEN TRIM(COALESCE(s.content_summary, '')) <> '' THEN s.content_summary
				ELSE r.title
			END AS segment_title,
			r.video_url AS video_url,
			r.cover_url AS cover_url,
			r.status AS status,
			r.is_published AS is_published,
			r.is_recommend AS is_recommend,
			r.view_count AS view_count,
			r.create_time AS create_time,
			r.update_time AS update_time,
			r.title AS video_title,
			r.description AS description,
			COALESCE(CAST(s.knowledge_tags AS TEXT), '') AS knowledge_tags`).
		Joins("JOIN edu_video_resource AS r ON r.id = s.video_id").
		Where("s.deleted = ? AND s.status = ? AND r.deleted = ? AND r.is_published = ? AND r.is_recommend = ? AND r.status = ? AND TRIM(COALESCE(r.video_url, '')) <> ''",
			0,
			1,
			0,
			true,
			true,
			int16(domainvideo.StatusDone),
		).
		Order("s.like_count DESC, s.double_like_count DESC, r.view_count DESC, s.id DESC").
		Limit(300)
	if userID > 0 {
		query = query.
			Joins("LEFT JOIN edu_user_reaction AS ur ON ur.user_id = ? AND ur.video_segment_id = s.id AND ur.reaction_type = ? AND ur.deleted = 0", userID, "dislike").
			Joins("LEFT JOIN edu_video_user_reaction AS vur ON vur.user_id = ? AND vur.video_id = s.video_id AND vur.reaction_type = ? AND vur.deleted = 0", userID, "dislike").
			Joins("LEFT JOIN edu_user_video_recommend AS watched ON watched.user_id = ? AND watched.video_segment_id = s.id AND watched.is_watched = ? AND watched.deleted = 0", userID, true).
			Where("ur.id IS NULL AND vur.id IS NULL AND watched.id IS NULL")
	}
	return rows, query.Scan(&rows).Error
}

func matchWeakKnowledge(candidate knowledgeMatchCandidateRow, weakKnowledge []weakKnowledgeRow) (float64, int, bool) {
	primaryHaystack := strings.ToLower(strings.Join([]string{
		candidate.SegmentTitle,
		candidate.KnowledgeTags,
	}, " "))
	secondaryHaystack := strings.ToLower(strings.Join([]string{
		candidate.VideoTitle,
		candidate.Description,
	}, " "))
	best := 0.0
	bestStrength := 0
	found := false
	for _, knowledge := range weakKnowledge {
		for _, token := range knowledgeMatchTokens(knowledge) {
			strength := 0
			switch {
			case strings.Contains(primaryHaystack, token):
				strength = 2
			case strings.Contains(secondaryHaystack, token):
				strength = 1
			default:
				continue
			}
			if !found || knowledge.Mastery < best || (knowledge.Mastery == best && strength > bestStrength) {
				best = knowledge.Mastery
				bestStrength = strength
			}
			found = true
		}
	}
	return best, bestStrength, found
}

func knowledgeMatchTokens(knowledge weakKnowledgeRow) []string {
	parts := strings.FieldsFunc(knowledge.Name+" "+knowledge.Description, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ',', '，', ';', '；', '/', '|', '、', '.', '。', ':', '：':
			return true
		default:
			return false
		}
	})
	tokens := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		tokens = append(tokens, part)
	}
	return tokens
}

func (r *GormVideoRepository) GetActiveRecBoleModelVersion(ctx context.Context) (string, bool, error) {
	var row struct {
		ModelVersion string `gorm:"column:model_version"`
	}
	if err := r.db.WithContext(ctx).Raw(sqlqueries.GetActiveRecommendModelVersionQuery, "recbole").Scan(&row).Error; err != nil {
		return "", false, err
	}
	version := strings.TrimSpace(row.ModelVersion)
	if version == "" {
		return "", false, nil
	}
	return version, true, nil
}

// FindRandomPlayableSegment 随机返回一个未删除、已发布且已转码完成的视频片段。
func (r *GormVideoRepository) FindRandomPlayableSegment(ctx context.Context) (videoapp.RecommendResultItem, bool, error) {
	return r.findRandomPlayableSegment(ctx, nil)
}

func (r *GormVideoRepository) FindRandomPlayableSegmentExcluding(ctx context.Context, excludedSegmentIDs []uint64) (videoapp.RecommendResultItem, bool, error) {
	return r.findRandomPlayableSegment(ctx, excludedSegmentIDs)
}

func (r *GormVideoRepository) findRandomPlayableSegment(ctx context.Context, excludedSegmentIDs []uint64) (videoapp.RecommendResultItem, bool, error) {
	type row struct {
		VideoSegmentID uint64    `gorm:"column:video_segment_id"`
		VideoID        uint64    `gorm:"column:video_id"`
		StartTimeSec   int       `gorm:"column:start_time_sec"`
		EndTimeSec     int       `gorm:"column:end_time_sec"`
		Title          string    `gorm:"column:title"`
		UserID         uint64    `gorm:"column:user_id"`
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
	baseQuery := func() *gorm.DB {
		query := r.db.WithContext(ctx).
			Table("edu_video_segment AS s").
			Joins("JOIN edu_video_resource AS r ON r.id = s.video_id").
			Where("s.deleted = ? AND s.status = ? AND r.deleted = ? AND r.is_published = ? AND r.status = ? AND TRIM(COALESCE(r.video_url, '')) <> ''", 0, 1, 0, true, int16(domainvideo.StatusDone))
		if len(excludedSegmentIDs) > 0 {
			query = query.Where("s.id NOT IN ?", excludedSegmentIDs)
		}
		return query
	}

	var maxSegment struct {
		ID uint64 `gorm:"column:id"`
	}
	if err := baseQuery().Select("s.id").Order("s.id DESC").Limit(1).Scan(&maxSegment).Error; err != nil {
		return videoapp.RecommendResultItem{}, false, err
	}
	if maxSegment.ID == 0 {
		return videoapp.RecommendResultItem{}, false, nil
	}

	pivot := randomUint64InRange(1, maxSegment.ID)
	scanCandidate := func(query *gorm.DB) error {
		return query.
			Select(`
				s.id AS video_segment_id,
				s.video_id AS video_id,
				s.start_time AS start_time_sec,
				s.end_time AS end_time_sec,
				CASE
					WHEN TRIM(COALESCE(s.content_summary, '')) <> '' THEN s.content_summary
					ELSE r.title
				END AS title,
				r.user_id AS user_id,
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
			Limit(1).
			Scan(&item).Error
	}
	err := scanCandidate(baseQuery().Where("s.id >= ?", pivot).Order("s.id ASC"))
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
			UserID:      item.UserID,
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

func randomUint64InRange(min uint64, max uint64) uint64 {
	if max <= min {
		return min
	}
	span := max - min + 1
	n, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetUint64(span))
	if err != nil {
		return min + uint64(time.Now().UnixNano())%span
	}
	return min + n.Uint64()
}

// SaveUserVideoRecommendation 写入或更新用户推荐记录。
func (r *GormVideoRepository) SaveUserVideoRecommendation(ctx context.Context, userID uint64, questionID uint64, videoID uint64, segmentID uint64, score float64, now time.Time) error {
	return r.db.WithContext(ctx).Exec(sqlqueries.UpsertUserVideoRecommendQuery, userID, videoID, questionID, segmentID, score, now, now).Error
}

// SaveRecommendationExposures 批量记录一次推荐结果的实际曝光列表。
func (r *GormVideoRepository) SaveRecommendationExposures(ctx context.Context, exposures []videoapp.RecommendationExposure) error {
	if len(exposures) == 0 {
		return nil
	}
	rows := make([]model.EduRecommendExposure, 0, len(exposures))
	for _, exposure := range exposures {
		rows = append(rows, model.EduRecommendExposure{
			RequestID:      exposure.RequestID,
			UserID:         exposure.UserID,
			QuestionID:     exposure.QuestionID,
			VideoID:        exposure.VideoID,
			VideoSegmentID: exposure.VideoSegmentID,
			Rank:           exposure.Rank,
			Score:          exposure.Score,
			Strategy:       exposure.Strategy,
			ModelVersion:   exposure.ModelVersion,
			Clicked:        false,
			Watched:        false,
			CreateTime:     exposure.Now,
			UpdateTime:     exposure.Now,
			Deleted:        0,
		})
	}
	return r.db.WithContext(ctx).CreateInBatches(rows, 100).Error
}

func (r *GormVideoRepository) GetRecommendationDatasourceStats(ctx context.Context) (videoapp.RecommendationDatasourceStats, error) {
	db := r.db.WithContext(ctx)
	var stats videoapp.RecommendationDatasourceStats
	if err := db.Model(&model.EduVideoResource{}).Where("deleted = ?", 0).Count(&stats.VideoTotal).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduVideoResource{}).Where("deleted = ? AND is_published = ?", 0, true).Count(&stats.PublishedVideos).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduVideoResource{}).Where("deleted = ? AND is_recommend = ?", 0, true).Count(&stats.RecommendVideos).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduVideoSegment{}).Where("deleted = ?", 0).Count(&stats.SegmentTotal).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduVideoSegment{}).Where("deleted = ? AND status = ?", 0, readyVideoSegmentStatus).Count(&stats.PlayableSegments).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduVideoSegment{}).Where("deleted = ? AND embedding IS NOT NULL", 0).Count(&stats.EmbeddedSegments).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduRecommendExposure{}).Where("deleted = ?", 0).Count(&stats.ExposureTotal).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduRecommendExposure{}).Where("deleted = ? AND watched = ?", 0, true).Count(&stats.WatchedExposures).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduUserVideoRecommend{}).Where("deleted = ?", 0).Count(&stats.RecommendationRows).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduUserVideoRecommend{}).Where("deleted = ? AND is_watched = ?", 0, true).Count(&stats.WatchedRecommendations).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Table("recsys.recommend_user_embedding").Where("deleted = ? AND status = ?", 0, 1).Distinct("user_id").Count(&stats.RecBoleUsers).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Table("recsys.recommend_item_embedding").Where("deleted = ? AND status = ?", 0, 1).Distinct("video_segment_id").Count(&stats.RecBoleItems).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	if err := db.Model(&model.EduUserReaction{}).Where("deleted = ?", 0).Count(&stats.ReactionRows).Error; err != nil {
		return videoapp.RecommendationDatasourceStats{}, err
	}
	return stats, nil
}

func (r *GormVideoRepository) ListRecommendationRecentRequests(ctx context.Context, limit int) ([]videoapp.RecommendationRecentRequest, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	var rows []struct {
		RequestID     string         `gorm:"column:request_id"`
		UserID        uint64         `gorm:"column:user_id"`
		QuestionID    uint64         `gorm:"column:question_id"`
		Exposures     int64          `gorm:"column:exposures"`
		Watched       int64          `gorm:"column:watched"`
		Strategy      string         `gorm:"column:strategy"`
		ModelVersion  string         `gorm:"column:model_version"`
		LastEventTime sql.NullString `gorm:"column:last_event_time"`
	}
	err := r.db.WithContext(ctx).
		Model(&model.EduRecommendExposure{}).
		Select(`
request_id AS request_id,
MAX(user_id) AS user_id,
MAX(question_id) AS question_id,
COUNT(*) AS exposures,
SUM(CASE WHEN watched THEN 1 ELSE 0 END) AS watched,
MAX(strategy) AS strategy,
MAX(model_version) AS model_version,
MAX(create_time) AS last_event_time`).
		Where("deleted = ? AND request_id <> ?", 0, "").
		Group("request_id").
		Order("last_event_time DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]videoapp.RecommendationRecentRequest, 0, len(rows))
	for _, row := range rows {
		lastEventTime, _ := parseDBTime(row.LastEventTime.String)
		out = append(out, videoapp.RecommendationRecentRequest{
			RequestID:     row.RequestID,
			UserID:        row.UserID,
			QuestionID:    row.QuestionID,
			Exposures:     row.Exposures,
			Watched:       row.Watched,
			WatchRate:     ratioFloat(row.Watched, row.Exposures),
			Strategy:      row.Strategy,
			ModelVersion:  row.ModelVersion,
			LastEventTime: lastEventTime,
		})
	}
	return out, nil
}

func (r *GormVideoRepository) ListRecommendationDataFreshness(ctx context.Context) ([]videoapp.RecommendationDataFreshness, error) {
	definitions := []struct {
		source string
		label  string
		query  string
	}{
		{source: "recommendation_exposure", label: "推荐曝光", query: `SELECT MAX(create_time) AS latest_at FROM edu_recommend_exposure WHERE deleted = 0`},
		{source: "watch_records", label: "观看记录", query: `SELECT MAX(update_time) AS latest_at FROM edu_user_video_recommend WHERE deleted = 0`},
		{source: "segment_reactions", label: "片段 Reaction", query: `SELECT MAX(update_time) AS latest_at FROM edu_user_reaction WHERE deleted = 0`},
		{source: "video_reactions", label: "视频 Reaction", query: `SELECT MAX(update_time) AS latest_at FROM edu_video_user_reaction WHERE deleted = 0`},
		{source: "recbole_user_embedding", label: "RecBole 用户向量", query: `SELECT MAX(update_time) AS latest_at FROM recsys.recommend_user_embedding WHERE deleted = 0 AND status = 1`},
		{source: "recbole_item_embedding", label: "RecBole 片段向量", query: `SELECT MAX(update_time) AS latest_at FROM recsys.recommend_item_embedding WHERE deleted = 0 AND status = 1`},
	}
	out := make([]videoapp.RecommendationDataFreshness, 0, len(definitions))
	for _, definition := range definitions {
		latest, found, err := r.latestRecommendationTime(ctx, definition.query)
		if err != nil {
			return nil, err
		}
		out = append(out, videoapp.RecommendationDataFreshness{
			Source:   definition.source,
			Label:    definition.label,
			LatestAt: latest,
			HasData:  found,
		})
	}
	return out, nil
}

func (r *GormVideoRepository) latestRecommendationTime(ctx context.Context, query string) (time.Time, bool, error) {
	var row struct {
		LatestAt sql.NullString `gorm:"column:latest_at"`
	}
	if err := r.db.WithContext(ctx).Raw(query).Scan(&row).Error; err != nil {
		return time.Time{}, false, err
	}
	if !row.LatestAt.Valid {
		return time.Time{}, false, nil
	}
	latestAt, ok := parseDBTime(row.LatestAt.String)
	return latestAt, ok, nil
}

func ratioFloat(numerator int64, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func parseDBTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (r *GormVideoRepository) ListRecommendationEffectMetrics(ctx context.Context, days int) (videoapp.RecommendationEffectMetrics, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	now := time.Now()
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -days+1)
	var daily []videoapp.RecommendationDailyEffectMetric
	if err := r.db.WithContext(ctx).Raw(`
SELECT
  DATE(create_time) AS day,
  COUNT(*) AS exposures,
  COALESCE(SUM(CASE WHEN watched THEN 1 ELSE 0 END), 0) AS watched,
  CASE WHEN COUNT(*) = 0 THEN 0 ELSE CAST(SUM(CASE WHEN watched THEN 1 ELSE 0 END) AS REAL) / COUNT(*) END AS watch_rate
FROM edu_recommend_exposure
WHERE deleted = 0 AND create_time >= ?
GROUP BY DATE(create_time)
ORDER BY day ASC
`, cutoff).Scan(&daily).Error; err != nil {
		return videoapp.RecommendationEffectMetrics{}, err
	}

	var strategies []videoapp.RecommendationStrategyEffectMetric
	if err := r.db.WithContext(ctx).Raw(`
SELECT
  strategy AS strategy,
  COALESCE(model_version, '') AS model_version,
  COUNT(*) AS exposures,
  COALESCE(SUM(CASE WHEN watched THEN 1 ELSE 0 END), 0) AS watched,
  CASE WHEN COUNT(*) = 0 THEN 0 ELSE CAST(SUM(CASE WHEN watched THEN 1 ELSE 0 END) AS REAL) / COUNT(*) END AS watch_rate,
  COALESCE(AVG(rank), 0) AS average_rank,
  COALESCE(AVG(score), 0) AS average_score
FROM edu_recommend_exposure
WHERE deleted = 0 AND create_time >= ?
GROUP BY strategy, COALESCE(model_version, '')
ORDER BY exposures DESC, strategy ASC, model_version ASC
`, cutoff).Scan(&strategies).Error; err != nil {
		return videoapp.RecommendationEffectMetrics{}, err
	}
	return videoapp.RecommendationEffectMetrics{Daily: daily, Strategies: strategies}, nil
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

// MarkRecommendationExposureWatched 将最近一次匹配的曝光标记为已点击/已观看。
func (r *GormVideoRepository) MarkRecommendationExposureWatched(ctx context.Context, userID uint64, questionID uint64, segmentID uint64, now time.Time) error {
	var row struct {
		ID uint64 `gorm:"column:id"`
	}
	err := r.db.WithContext(ctx).
		Model(&model.EduRecommendExposure{}).
		Select("id").
		Where("user_id = ? AND question_id = ? AND video_segment_id = ? AND deleted = ?", userID, questionID, segmentID, 0).
		Order("create_time DESC, id DESC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return err
	}
	if row.ID == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&model.EduRecommendExposure{}).
		Where("id = ?", row.ID).
		Updates(map[string]interface{}{
			"clicked":      true,
			"watched":      true,
			"clicked_time": now,
			"watched_time": now,
			"update_time":  now,
		}).Error
}

func parseVectorText(text string) ([]float32, error) {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	trimmed = strings.TrimPrefix(trimmed, "(")
	trimmed = strings.TrimSuffix(trimmed, ")")
	if strings.TrimSpace(trimmed) == "" {
		return nil, nil
	}
	parts := strings.Split(trimmed, ",")
	values := make([]float32, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
		if err != nil {
			return nil, err
		}
		values = append(values, float32(value))
	}
	return values, nil
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
		UserID:      m.UserID,
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
	if err := db.Exec(sqlqueries.AddVideoResourceUserIDQuery).Error; err != nil {
		return err
	}
	if err := EnsureRecSysSchema(db); err != nil {
		return err
	}
	_ = db.Exec(sqlqueries.CreateUserQuestionIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateSegmentIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateVideoReactionUserIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateVideoReactionVideoIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateUserReactionUserIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateUserReactionVideoIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateUserReactionSegmentIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateVideoResourceUserIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateRecommendExposureLookupIndexQuery).Error
	_ = db.Exec(sqlqueries.CreateRecommendExposureRequestIndexQuery).Error

	if err := db.Exec(sqlqueries.CreateUserQuestionSegmentUniqueConstraintQuery).Error; err != nil {
		return err
	}
	if err := db.Exec(sqlqueries.CreateVideoReactionUniqueConstraintQuery).Error; err != nil {
		return err
	}
	if err := db.Exec(sqlqueries.CreateUserReactionUniqueConstraintQuery).Error; err != nil {
		return err
	}
	if err := db.Exec(sqlqueries.CreateUserVideoProfileUniqueConstraintQuery).Error; err != nil {
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
