package notification

import (
	"context"
	"testing"

	"video-service/internal/infrastructure/persistence"
	"video-service/internal/model"
)

type fakeNotificationRepo struct {
	created    []model.EduUserNotification
	views      []persistence.NotificationView
	total      int64
	unread     int64
	markedIDs  map[uint64]bool
	markAllCnt int
	nicknameMap map[string]uint64
}

func (f *fakeNotificationRepo) CreateNotification(_ context.Context, n *model.EduUserNotification) error {
	f.created = append(f.created, *n)
	return nil
}

func (f *fakeNotificationRepo) ListNotifications(_ context.Context, _ uint64, _, _ int) ([]persistence.NotificationView, int64, error) {
	return f.views, f.total, nil
}

func (f *fakeNotificationRepo) CountUnread(_ context.Context, _ uint64) (int64, error) {
	return f.unread, nil
}

func (f *fakeNotificationRepo) MarkAsRead(_ context.Context, id, _ uint64) (bool, error) {
	if f.markedIDs == nil {
		f.markedIDs = map[uint64]bool{}
	}
	f.markedIDs[id] = true
	return true, nil
}

func (f *fakeNotificationRepo) MarkAllAsRead(_ context.Context, _ uint64) error {
	f.markAllCnt++
	return nil
}

func (f *fakeNotificationRepo) FindUserIDsByNicknames(_ context.Context, nicknames []string) (map[string]uint64, error) {
	result := make(map[string]uint64, len(nicknames))
	for _, n := range nicknames {
		if uid, ok := f.nicknameMap[n]; ok {
			result[n] = uid
		}
	}
	return result, nil
}

func TestParseMentionsDedupAndFilter(t *testing.T) {
	repo := &fakeNotificationRepo{nicknameMap: map[string]uint64{
		"小明": 10,
		"alice": 20,
	}}
	svc := NewService(repo)
	ctx := context.Background()

	// 重复 @小明、不存在的用户、@自己 都应被过滤
	targets, err := svc.ParseMentions(ctx, "@小明 @小明 @不存在 @alice", 10)
	if err != nil {
		t.Fatalf("parse mentions: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("targets len = %d, want 1 (only alice, 小明 is self)", len(targets))
	}
	if targets[0].Nickname != "alice" || targets[0].UserID != 20 {
		t.Fatalf("unexpected target: %+v", targets[0])
	}
}

func TestParseMentionsNoAt(t *testing.T) {
	repo := &fakeNotificationRepo{}
	svc := NewService(repo)
	targets, err := svc.ParseMentions(context.Background(), "普通评论没有提及", 1)
	if err != nil {
		t.Fatalf("parse mentions: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("targets len = %d, want 0", len(targets))
	}
}

func TestCreateMentionNotifications(t *testing.T) {
	repo := &fakeNotificationRepo{nicknameMap: map[string]uint64{
		"小红": 30,
	}}
	svc := NewService(repo)
	ctx := context.Background()

	err := svc.CreateMentionNotifications(ctx, 1, "作者", 100, 50, 200, "@小红 你好")
	if err != nil {
		t.Fatalf("create mention notifications: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created len = %d, want 1", len(repo.created))
	}
	n := repo.created[0]
	if n.UserID != 30 || n.FromUserID != 1 || n.VideoID != 100 || n.CommentID != 200 {
		t.Fatalf("unexpected notification: %+v", n)
	}
	if n.Type != model.NotificationTypeMention {
		t.Fatalf("type = %q, want %q", n.Type, model.NotificationTypeMention)
	}
	if n.Content != "作者在评论中@了你" {
		t.Fatalf("content = %q, want %q", n.Content, "作者在评论中@了你")
	}
}

func TestCreateMentionNotificationsSelfSkipped(t *testing.T) {
	repo := &fakeNotificationRepo{nicknameMap: map[string]uint64{
		"自己": 1,
	}}
	svc := NewService(repo)
	err := svc.CreateMentionNotifications(context.Background(), 1, "自己", 100, 50, 200, "@自己 你好")
	if err != nil {
		t.Fatalf("create mention notifications: %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created len = %d, want 0 (self mention skipped)", len(repo.created))
	}
}

func TestListNotificationsPagination(t *testing.T) {
	repo := &fakeNotificationRepo{total: 5}
	svc := NewService(repo)
	result, err := svc.ListNotifications(context.Background(), 1, 1, 10)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if result.Total != 5 {
		t.Fatalf("total = %d, want 5", result.Total)
	}
}

func TestListNotificationsClampsPageSize(t *testing.T) {
	repo := &fakeNotificationRepo{}
	svc := NewService(repo)
	// pageSize 超过上限应被截断，不应报错
	_, err := svc.ListNotifications(context.Background(), 1, 1, 999)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
}

func TestGetUnreadCount(t *testing.T) {
	repo := &fakeNotificationRepo{unread: 3}
	svc := NewService(repo)
	count, err := svc.GetUnreadCount(context.Background(), 1)
	if err != nil {
		t.Fatalf("get unread count: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
}

func TestMarkAllAsRead(t *testing.T) {
	repo := &fakeNotificationRepo{}
	svc := NewService(repo)
	if err := svc.MarkAllAsRead(context.Background(), 1); err != nil {
		t.Fatalf("mark all as read: %v", err)
	}
	if repo.markAllCnt != 1 {
		t.Fatalf("markAllCnt = %d, want 1", repo.markAllCnt)
	}
}

func TestValidationErrors(t *testing.T) {
	repo := &fakeNotificationRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	if _, err := svc.ListNotifications(ctx, 0, 1, 10); err == nil {
		t.Fatal("expected error for zero user_id")
	}
	if _, err := svc.GetUnreadCount(ctx, 0); err == nil {
		t.Fatal("expected error for zero user_id")
	}
	if err := svc.MarkAsRead(ctx, 0, 1); err == nil {
		t.Fatal("expected error for zero id")
	}
	if err := svc.MarkAllAsRead(ctx, 0); err == nil {
		t.Fatal("expected error for zero user_id")
	}
}
