package persistence

import (
	"context"

	"gorm.io/gorm"

	"video-service/internal/model"
)

// GormUserPublishRepository 用户发布视频仓储的 GORM 实现。
type GormUserPublishRepository struct {
	db *gorm.DB
}

// NewGormUserPublishRepository 创建用户发布视频仓储。
func NewGormUserPublishRepository(db *gorm.DB) *GormUserPublishRepository {
	return &GormUserPublishRepository{db: db}
}

// CreateVideo 写入一条新的视频资源记录。
func (r *GormUserPublishRepository) CreateVideo(ctx context.Context, video *model.EduVideoResource) error {
	return r.db.WithContext(ctx).Create(video).Error
}

// GetByID 按主键读取一条未删除的视频记录。
func (r *GormUserPublishRepository) GetByID(ctx context.Context, id uint64) (*model.EduVideoResource, error) {
	var m model.EduVideoResource
	err := r.db.WithContext(ctx).Where("id = ? AND deleted = ?", id, 0).First(&m).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// ListByUserID 按用户ID分页查询已发布的用户作品，按创建时间倒序。
func (r *GormUserPublishRepository) ListByUserID(ctx context.Context, userID uint64, page, pageSize int) ([]model.EduVideoResource, int64, error) {
	var total int64
	query := r.db.WithContext(ctx).Model(&model.EduVideoResource{}).
		Where("user_id = ? AND source_type = ? AND deleted = 0 AND is_published = ?", userID, model.VideoSourceUserPublish, true)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []model.EduVideoResource
	offset := (page - 1) * pageSize
	if err := query.Order("create_time DESC").Limit(pageSize).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ListLikedVideos 分页查询用户点赞/双赞的视频，按最近点赞时间倒序，同一视频只出现一次。
func (r *GormUserPublishRepository) ListLikedVideos(ctx context.Context, userID uint64, page, pageSize int) ([]model.EduVideoResource, int64, error) {
	baseQuery := `
		FROM edu_user_reaction ur
		JOIN edu_video_resource r ON r.id = ur.video_id AND r.deleted = 0 AND r.is_published = true
		WHERE ur.user_id = ? AND ur.reaction_type IN ('like', 'double_like') AND ur.deleted = 0
		GROUP BY r.id
	`
	var total int64
	countSQL := `SELECT COUNT(DISTINCT r.id) ` + baseQuery
	if err := r.db.WithContext(ctx).Raw(countSQL, userID).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []model.EduVideoResource
	offset := (page - 1) * pageSize
	listSQL := `
		SELECT r.id, r.user_id, r.source_type, r.title, r.description, r.video_url,
		       r.cover_url, r.duration, r.status, r.is_published, r.is_recommend,
		       r.view_count, r.like_count, r.double_like_count, r.dislike_count,
		       r.error_msg, r.create_time, r.update_time, r.deleted
		` + baseQuery + `
		ORDER BY MAX(ur.update_time) DESC
		LIMIT ? OFFSET ?
	`
	if err := r.db.WithContext(ctx).Raw(listSQL, userID, pageSize, offset).Scan(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateStatus 更新视频处理状态及错误信息。
func (r *GormUserPublishRepository) UpdateStatus(ctx context.Context, id uint64, status int16, errMsg string) error {
	updates := map[string]interface{}{"status": status}
	if errMsg != "" {
		updates["error_msg"] = errMsg
	} else {
		updates["error_msg"] = ""
	}
	return r.db.WithContext(ctx).Model(&model.EduVideoResource{}).
		Where("id = ? AND deleted = ?", id, 0).Updates(updates).Error
}
