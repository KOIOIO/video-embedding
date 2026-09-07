package persistence

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"video-service/internal/model"
)

// GormUserProfileRepository 用户资料仓储的 GORM 实现。
type GormUserProfileRepository struct {
	db *gorm.DB
}

// NewGormUserProfileRepository 创建用户资料仓储。
func NewGormUserProfileRepository(db *gorm.DB) *GormUserProfileRepository {
	return &GormUserProfileRepository{db: db}
}

// GetByUserID 按用户 ID 查询未删除的资料记录，查不到返回 (nil, nil)。
func (r *GormUserProfileRepository) GetByUserID(userID uint64) (*model.EduUserProfile, error) {
	var m model.EduUserProfile
	err := r.db.Where("user_id = ? AND deleted = ?", userID, 0).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// UpsertDefault 当用户资料不存在时插入默认记录，已存在则直接返回现有记录。
func (r *GormUserProfileRepository) UpsertDefault(userID uint64) (*model.EduUserProfile, error) {
	defaultNickname := fmt.Sprintf("用户%d", userID)
	record := &model.EduUserProfile{
		UserID:   userID,
		Nickname: defaultNickname,
		Deleted:  0,
	}
	// ON CONFLICT DO NOTHING：依赖部分唯一索引 uk_user_profile_user_active
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error; err != nil {
		return nil, err
	}
	return r.GetByUserID(userID)
}

// Update 按用户 ID 更新资料字段。
func (r *GormUserProfileRepository) Update(userID uint64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.Model(&model.EduUserProfile{}).
		Where("user_id = ? AND deleted = ?", userID, 0).
		Updates(updates).Error
}

// UpdateAvatar 更新用户头像地址。
func (r *GormUserProfileRepository) UpdateAvatar(userID uint64, avatarURL string) error {
	return r.Update(userID, map[string]interface{}{
		"avatar_url": avatarURL,
	})
}

// IncrementFollowCount 原子增减关注数，delta 可为负数。
func (r *GormUserProfileRepository) IncrementFollowCount(userID uint64, delta int) error {
	if delta == 0 {
		return nil
	}
	return r.db.Model(&model.EduUserProfile{}).
		Where("user_id = ? AND deleted = ?", userID, 0).
		UpdateColumn("follow_count", gorm.Expr("follow_count + ?", delta)).Error
}

// IncrementFansCount 原子增减粉丝数，delta 可为负数。
func (r *GormUserProfileRepository) IncrementFansCount(userID uint64, delta int) error {
	if delta == 0 {
		return nil
	}
	return r.db.Model(&model.EduUserProfile{}).
		Where("user_id = ? AND deleted = ?", userID, 0).
		UpdateColumn("fans_count", gorm.Expr("fans_count + ?", delta)).Error
}
