package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"video-service/internal/model"
)

// GormNotificationRepository 用户通知仓储的 GORM 实现。
type GormNotificationRepository struct {
	db *gorm.DB
}

// NewGormNotificationRepository 创建用户通知仓储。
func NewGormNotificationRepository(db *gorm.DB) *GormNotificationRepository {
	return &GormNotificationRepository{db: db}
}

// NotificationView 带发起人信息的通知视图。
type NotificationView struct {
	ID               uint64
	UserID           uint64
	Type             string
	FromUserID       uint64
	FromUserNickname string
	FromUserAvatar   string
	VideoID          uint64
	VideoSegmentID   uint64
	CommentID        uint64
	Content          string
	IsRead           int16
	CreatedAt        time.Time
}

// CreateNotification 写入一条通知，依赖部分唯一索引防重复。
func (r *GormNotificationRepository) CreateNotification(ctx context.Context, n *model.EduUserNotification) error {
	if n == nil {
		return errors.New("notification is required")
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(n).Error
}

// ListNotifications 分页查询用户通知，按创建时间倒序，JOIN 用户资料取昵称头像。
func (r *GormNotificationRepository) ListNotifications(ctx context.Context, userID uint64, page, pageSize int) ([]NotificationView, int64, error) {
	base := r.db.WithContext(ctx).Table("edu_user_notification AS n").
		Select(`n.id, n.user_id, n.type, n.from_user_id,
			COALESCE(p.nickname, '') AS from_user_nickname,
			COALESCE(p.avatar_url, '') AS from_user_avatar,
			n.video_id, n.video_segment_id, n.comment_id, n.content, n.is_read, n.create_time`).
		Joins("LEFT JOIN edu_user_profile AS p ON p.user_id = n.from_user_id AND p.deleted = 0").
		Where("n.user_id = ? AND n.deleted = 0", userID)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	type row struct {
		ID               uint64    `gorm:"column:id"`
		UserID           uint64    `gorm:"column:user_id"`
		Type             string    `gorm:"column:type"`
		FromUserID       uint64    `gorm:"column:from_user_id"`
		FromUserNickname string    `gorm:"column:from_user_nickname"`
		FromUserAvatar   string    `gorm:"column:from_user_avatar"`
		VideoID          uint64    `gorm:"column:video_id"`
		VideoSegmentID   uint64    `gorm:"column:video_segment_id"`
		CommentID        uint64    `gorm:"column:comment_id"`
		Content          string    `gorm:"column:content"`
		IsRead           int16     `gorm:"column:is_read"`
		CreateTime       time.Time `gorm:"column:create_time"`
	}
	var rows []row
	if err := base.Order("n.create_time DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	views := make([]NotificationView, 0, len(rows))
	for _, r := range rows {
		views = append(views, NotificationView{
			ID:               r.ID,
			UserID:           r.UserID,
			Type:             r.Type,
			FromUserID:       r.FromUserID,
			FromUserNickname: r.FromUserNickname,
			FromUserAvatar:   r.FromUserAvatar,
			VideoID:          r.VideoID,
			VideoSegmentID:   r.VideoSegmentID,
			CommentID:        r.CommentID,
			Content:          r.Content,
			IsRead:           r.IsRead,
			CreatedAt:        r.CreateTime,
		})
	}
	return views, total, nil
}

// CountUnread 统计用户未读通知数。
func (r *GormNotificationRepository) CountUnread(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.EduUserNotification{}).
		Where("user_id = ? AND is_read = 0 AND deleted = 0", userID).
		Count(&count).Error
	return count, err
}

// MarkAsRead 标记单条通知为已读，只能标记自己的通知。
func (r *GormNotificationRepository) MarkAsRead(ctx context.Context, id, userID uint64) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.EduUserNotification{}).
		Where("id = ? AND user_id = ? AND deleted = 0", id, userID).
		Update("is_read", 1)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// MarkAllAsRead 标记用户所有通知为已读。
func (r *GormNotificationRepository) MarkAllAsRead(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Model(&model.EduUserNotification{}).
		Where("user_id = ? AND is_read = 0 AND deleted = 0", userID).
		Update("is_read", 1).Error
}

// FindUserIDsByNicknames 按昵称批量查找用户 ID，用于评论 @提及 解析。
func (r *GormNotificationRepository) FindUserIDsByNicknames(ctx context.Context, nicknames []string) (map[string]uint64, error) {
	result := make(map[string]uint64, len(nicknames))
	if len(nicknames) == 0 {
		return result, nil
	}
	type row struct {
		UserID   uint64 `gorm:"column:user_id"`
		Nickname string `gorm:"column:nickname"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("edu_user_profile").
		Select("user_id, nickname").
		Where("nickname IN ? AND deleted = 0", nicknames).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Nickname] = r.UserID
	}
	return result, nil
}

// UserDisplayInfo 用户展示信息（昵称、头像、用户名）。
type UserDisplayInfo struct {
	UserID    uint64
	Username  string
	Nickname  string
	AvatarURL string
}

// GetUserDisplayInfoByIDs 批量查询用户展示信息，LEFT JOIN edu_user_profile，回退 sys_user.username。
func (r *GormNotificationRepository) GetUserDisplayInfoByIDs(ctx context.Context, userIDs []uint64) (map[uint64]UserDisplayInfo, error) {
	result := make(map[uint64]UserDisplayInfo, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	type row struct {
		ID        uint64 `gorm:"column:id"`
		Username  string `gorm:"column:username"`
		Nickname  string `gorm:"column:nickname"`
		AvatarURL string `gorm:"column:avatar_url"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).Table("sys_user AS u").
		Select(`u.id, u.username,
			COALESCE(p.nickname, '') AS nickname,
			COALESCE(p.avatar_url, '') AS avatar_url`).
		Joins("LEFT JOIN edu_user_profile AS p ON p.user_id = u.id AND p.deleted = 0").
		Where("u.id IN ? AND u.deleted = 0", userIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.ID] = UserDisplayInfo{
			UserID:    r.ID,
			Username:  r.Username,
			Nickname:  r.Nickname,
			AvatarURL: r.AvatarURL,
		}
	}
	return result, nil
}
