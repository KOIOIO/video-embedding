package notification

import (
	"context"
	"regexp"
	"strings"
	"time"

	"video-service/internal/infrastructure/persistence"
	"video-service/internal/model"
)

// ErrNotificationNotFound 通知不存在。
var ErrNotificationNotFound = errNotification("notification not found")

type errNotification string

func (e errNotification) Error() string { return string(e) }

const (
	// DefaultPageSize 默认分页大小。
	DefaultPageSize = 20
	// MaxPageSize 分页上限。
	MaxPageSize = 50
)

// mentionRegex 匹配评论中的 @昵称，支持中文、字母、数字、下划线。
var mentionRegex = regexp.MustCompile(`@([\x{4e00}-\x{9fa5}a-zA-Z0-9_]+)`)

// NotificationRepository 通知仓储接口。
type NotificationRepository interface {
	CreateNotification(ctx context.Context, n *model.EduUserNotification) error
	ListNotifications(ctx context.Context, userID uint64, page, pageSize int) ([]persistence.NotificationView, int64, error)
	CountUnread(ctx context.Context, userID uint64) (int64, error)
	MarkAsRead(ctx context.Context, id, userID uint64) (bool, error)
	MarkAllAsRead(ctx context.Context, userID uint64) error
	FindUserIDsByNicknames(ctx context.Context, nicknames []string) (map[string]uint64, error)
}

// Service 用户通知应用服务。
type Service struct {
	repo NotificationRepository
	now  func() time.Time
}

// NewService 创建通知服务。
func NewService(repo NotificationRepository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// ListResult 通知列表分页结果。
type ListResult struct {
	Total         int64
	Notifications []persistence.NotificationView
}

// ListNotifications 分页查询用户通知。
func (s *Service) ListNotifications(ctx context.Context, userID uint64, page, pageSize int) (ListResult, error) {
	if userID == 0 {
		return ListResult{}, errNotification("user_id is required")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	views, total, err := s.repo.ListNotifications(ctx, userID, page, pageSize)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Total: total, Notifications: views}, nil
}

// GetUnreadCount 获取用户未读通知数。
func (s *Service) GetUnreadCount(ctx context.Context, userID uint64) (int64, error) {
	if userID == 0 {
		return 0, errNotification("user_id is required")
	}
	return s.repo.CountUnread(ctx, userID)
}

// MarkAsRead 标记单条通知为已读。
func (s *Service) MarkAsRead(ctx context.Context, id, userID uint64) error {
	if id == 0 || userID == 0 {
		return errNotification("id and user_id are required")
	}
	found, err := s.repo.MarkAsRead(ctx, id, userID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotificationNotFound
	}
	return nil
}

// MarkAllAsRead 标记用户所有通知为已读。
func (s *Service) MarkAllAsRead(ctx context.Context, userID uint64) error {
	if userID == 0 {
		return errNotification("user_id is required")
	}
	return s.repo.MarkAllAsRead(ctx, userID)
}

// MentionTarget 解析出的 @提及 目标。
type MentionTarget struct {
	Nickname string
	UserID   uint64
}

// ParseMentions 解析评论内容中的 @昵称，并查找对应用户 ID。
// 去重，忽略未匹配用户，忽略 @自己。
func (s *Service) ParseMentions(ctx context.Context, content string, fromUserID uint64) ([]MentionTarget, error) {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(matches))
	var nicknames []string
	for _, m := range matches {
		nick := strings.TrimSpace(m[1])
		if nick == "" {
			continue
		}
		if _, ok := seen[nick]; ok {
			continue
		}
		seen[nick] = struct{}{}
		nicknames = append(nicknames, nick)
	}
	if len(nicknames) == 0 {
		return nil, nil
	}
	userIDMap, err := s.repo.FindUserIDsByNicknames(ctx, nicknames)
	if err != nil {
		return nil, err
	}
	var targets []MentionTarget
	for _, nick := range nicknames {
		uid, ok := userIDMap[nick]
		if !ok || uid == 0 {
			continue
		}
		if uid == fromUserID {
			continue
		}
		targets = append(targets, MentionTarget{Nickname: nick, UserID: uid})
	}
	return targets, nil
}

// CreateMentionNotifications 为评论中 @ 的用户创建通知。
// fromNickname 用于拼接通知内容，如 "XXX在评论中@了你"。
func (s *Service) CreateMentionNotifications(ctx context.Context, fromUserID uint64, fromNickname string, videoID, videoSegmentID, commentID uint64, content string) error {
	if fromUserID == 0 || commentID == 0 {
		return nil
	}
	targets, err := s.ParseMentions(ctx, content, fromUserID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	displayName := fromNickname
	if displayName == "" {
		displayName = "有人"
	}
	notifyContent := displayName + "在评论中@了你"
	now := s.now()
	for _, t := range targets {
		n := &model.EduUserNotification{
			UserID:         t.UserID,
			Type:           model.NotificationTypeMention,
			FromUserID:     fromUserID,
			VideoID:        videoID,
			VideoSegmentID: videoSegmentID,
			CommentID:      commentID,
			Content:        notifyContent,
			IsRead:         0,
			Deleted:        0,
		}
		n.CreateTime = now
		n.UpdateTime = now
		if err := s.repo.CreateNotification(ctx, n); err != nil {
			return err
		}
	}
	return nil
}
