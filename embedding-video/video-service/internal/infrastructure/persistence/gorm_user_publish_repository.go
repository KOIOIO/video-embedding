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
