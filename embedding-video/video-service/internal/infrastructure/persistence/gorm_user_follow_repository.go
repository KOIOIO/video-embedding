package persistence

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"video-service/internal/model"
)

// GormUserFollowRepository 用户关注关系仓储的 GORM 实现。
type GormUserFollowRepository struct {
	db *gorm.DB
}

// NewGormUserFollowRepository 创建用户关注关系仓储。
func NewGormUserFollowRepository(db *gorm.DB) *GormUserFollowRepository {
	return &GormUserFollowRepository{db: db}
}

// WithTx 返回绑定到指定事务的仓储实例，用于事务内操作。
func (r *GormUserFollowRepository) WithTx(tx *gorm.DB) *GormUserFollowRepository {
	return &GormUserFollowRepository{db: tx}
}

// CreateFollow 插入一条关注关系，依赖唯一索引 uk_user_follow_pair_active 防止重复关注。
func (r *GormUserFollowRepository) CreateFollow(ctx context.Context, followerID, followingID uint64) error {
	record := &model.EduUserFollow{
		FollowerID:  followerID,
		FollowingID: followingID,
		Deleted:     0,
	}
	return r.db.WithContext(ctx).Create(record).Error
}

// DeleteFollow 软删除指定关注关系（UPDATE deleted=1）。
func (r *GormUserFollowRepository) DeleteFollow(ctx context.Context, followerID, followingID uint64) error {
	result := r.db.WithContext(ctx).
		Model(&model.EduUserFollow{}).
		Where("follower_id = ? AND following_id = ? AND deleted = ?", followerID, followingID, 0).
		Update("deleted", 1)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("follow relation not found")
	}
	return nil
}

// IsFollowing 查询 followerID 是否关注了 followingID。
func (r *GormUserFollowRepository) IsFollowing(ctx context.Context, followerID, followingID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.EduUserFollow{}).
		Where("follower_id = ? AND following_id = ? AND deleted = ?", followerID, followingID, 0).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountFollowers 查询指定用户的粉丝数。
func (r *GormUserFollowRepository) CountFollowers(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.EduUserFollow{}).
		Where("following_id = ? AND deleted = ?", userID, 0).
		Count(&count).Error
	return count, err
}

// CountFollowing 查询指定用户的关注数。
func (r *GormUserFollowRepository) CountFollowing(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.EduUserFollow{}).
		Where("follower_id = ? AND deleted = ?", userID, 0).
		Count(&count).Error
	return count, err
}

// ListFollowing 查询指定用户的关注列表，JOIN user_profile 取昵称头像，按关注时间倒序。
func (r *GormUserFollowRepository) ListFollowing(ctx context.Context, userID uint64, page, pageSize int) ([]model.FollowWithProfile, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	var results []model.FollowWithProfile
	err := r.db.WithContext(ctx).
		Table("edu_user_follow AS f").
		Select("f.id, f.follower_id, f.following_id, f.create_time, p.nickname, p.avatar_url, p.bio").
		Joins("LEFT JOIN edu_user_profile AS p ON p.user_id = f.following_id AND p.deleted = ?", 0).
		Where("f.follower_id = ? AND f.deleted = ?", userID, 0).
		Order("f.create_time DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(&results).Error
	return results, err
}

// ListFollowers 查询指定用户的粉丝列表，JOIN user_profile 取昵称头像，按关注时间倒序。
func (r *GormUserFollowRepository) ListFollowers(ctx context.Context, userID uint64, page, pageSize int) ([]model.FollowWithProfile, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	var results []model.FollowWithProfile
	err := r.db.WithContext(ctx).
		Table("edu_user_follow AS f").
		Select("f.id, f.follower_id, f.following_id, f.create_time, p.nickname, p.avatar_url, p.bio").
		Joins("LEFT JOIN edu_user_profile AS p ON p.user_id = f.follower_id AND p.deleted = ?", 0).
		Where("f.following_id = ? AND f.deleted = ?", userID, 0).
		Order("f.create_time DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(&results).Error
	return results, err
}
