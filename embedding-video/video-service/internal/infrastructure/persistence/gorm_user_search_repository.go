package persistence

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// UserSearchResult 用户搜索结果，关联资料与关注状态。
type UserSearchResult struct {
	ID          uint64 `gorm:"column:id" json:"id"`
	Username    string `gorm:"column:username" json:"username"`
	Nickname    string `gorm:"column:nickname" json:"nickname"`
	AvatarURL   string `gorm:"column:avatar_url" json:"avatar_url"`
	IsFollowing bool   `gorm:"column:is_following" json:"is_following"`
}

// GormUserSearchRepository 用户搜索仓储的 GORM 实现。
type GormUserSearchRepository struct {
	db *gorm.DB
}

// NewGormUserSearchRepository 创建用户搜索仓储。
func NewGormUserSearchRepository(db *gorm.DB) *GormUserSearchRepository {
	return &GormUserSearchRepository{db: db}
}

// SearchUsers 按昵称或用户名模糊搜索用户，已关注用户优先，不返回当前用户自己。
// 使用 LOWER() 兼容 PostgreSQL 与 SQLite 的大小写不敏感匹配。
func (r *GormUserSearchRepository) SearchUsers(
	ctx context.Context,
	currentUserID uint64,
	keyword string,
	page, pageSize int,
) ([]UserSearchResult, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	likePattern := "%" + keyword + "%"

	baseWhere := `
		FROM sys_user u
		LEFT JOIN edu_user_profile p ON p.user_id = u.id AND p.deleted = 0
		LEFT JOIN edu_user_follow f ON f.following_id = u.id AND f.follower_id = ? AND f.deleted = 0
		WHERE u.status = 1 AND u.deleted = 0 AND u.id != ?
		  AND (LOWER(COALESCE(p.nickname, '')) LIKE LOWER(?) OR LOWER(u.username) LIKE LOWER(?))
	`
	countSQL := "SELECT COUNT(*) " + baseWhere
	var total int64
	if err := r.db.WithContext(ctx).Raw(countSQL, currentUserID, currentUserID, likePattern, likePattern).
		Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	querySQL := `
		SELECT u.id, u.username,
		       COALESCE(p.nickname, u.real_name, u.username) AS nickname,
		       COALESCE(p.avatar_url, '') AS avatar_url,
		       (f.id IS NOT NULL) AS is_following
		` + baseWhere + `
		ORDER BY is_following DESC, nickname ASC
		LIMIT ? OFFSET ?
	`
	var results []UserSearchResult
	if err := r.db.WithContext(ctx).Raw(querySQL,
		currentUserID, currentUserID, likePattern, likePattern,
		pageSize, offset,
	).Scan(&results).Error; err != nil {
		return nil, 0, fmt.Errorf("search users: %w", err)
	}
	return results, total, nil
}
