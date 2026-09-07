package persistence

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-service/internal/model"
)

func newUserMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduUserMessage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedMessage(t *testing.T, db *gorm.DB, senderID, receiverID uint64, content string, isRead bool) *model.EduUserMessage {
	t.Helper()
	msg := &model.EduUserMessage{
		SenderID:       senderID,
		ReceiverID:     receiverID,
		ConversationID: model.ConversationID(senderID, receiverID),
		Content:        content,
		IsRead:         isRead,
		Deleted:        0,
	}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}
	return msg
}

func TestUserMessageCreateAndList(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	msg := &model.EduUserMessage{
		SenderID:       1001,
		ReceiverID:     2002,
		ConversationID: model.ConversationID(1001, 2002),
		Content:        "hello",
		IsRead:         false,
		Deleted:        0,
	}
	if err := repo.CreateMessage(ctx, msg); err != nil {
		t.Fatalf("CreateMessage error: %v", err)
	}
	if msg.ID == 0 {
		t.Fatal("CreateMessage should set ID")
	}

	convID := model.ConversationID(1001, 2002)
	list, err := repo.ListMessages(ctx, convID, 1, 20)
	if err != nil {
		t.Fatalf("ListMessages error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if list[0].Content != "hello" {
		t.Fatalf("content = %q, want hello", list[0].Content)
	}
}

func TestUserMessageListPagination(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		seedMessage(t, db, 1001, 2002, "msg", false)
		// 稍微间隔确保 create_time 不同
		time.Sleep(2 * time.Millisecond)
	}

	convID := model.ConversationID(1001, 2002)
	page1, err := repo.ListMessages(ctx, convID, 1, 2)
	if err != nil {
		t.Fatalf("ListMessages page1 error: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}

	page2, err := repo.ListMessages(ctx, convID, 2, 2)
	if err != nil {
		t.Fatalf("ListMessages page2 error: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2 len = %d, want 2", len(page2))
	}

	page3, err := repo.ListMessages(ctx, convID, 3, 2)
	if err != nil {
		t.Fatalf("ListMessages page3 error: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("page3 len = %d, want 1", len(page3))
	}
}

func TestUserMessageListOrderDesc(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	seedMessage(t, db, 1001, 2002, "first", false)
	time.Sleep(5 * time.Millisecond)
	seedMessage(t, db, 2002, 1001, "second", false)
	time.Sleep(5 * time.Millisecond)
	seedMessage(t, db, 1001, 2002, "third", false)

	convID := model.ConversationID(1001, 2002)
	list, err := repo.ListMessages(ctx, convID, 1, 20)
	if err != nil {
		t.Fatalf("ListMessages error: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("list len = %d, want 3", len(list))
	}
	// 倒序：最新的在前
	if list[0].Content != "third" {
		t.Fatalf("first item content = %q, want third", list[0].Content)
	}
	if list[2].Content != "first" {
		t.Fatalf("last item content = %q, want first", list[2].Content)
	}
}

func TestUserMessageMarkAsRead(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	seedMessage(t, db, 2002, 1001, "msg1", false)
	seedMessage(t, db, 2002, 1001, "msg2", false)
	seedMessage(t, db, 1001, 2002, "msg3", false) // 1001 发的，不应被标记

	convID := model.ConversationID(1001, 2002)
	if err := repo.MarkAsRead(ctx, convID, 1001); err != nil {
		t.Fatalf("MarkAsRead error: %v", err)
	}

	// 验证：1001 收到的消息（receiver_id=1001）应已读
	var unreadCount int64
	db.Model(&model.EduUserMessage{}).
		Where("conversation_id = ? AND receiver_id = ? AND is_read = ? AND deleted = ?", convID, 1001, false, 0).
		Count(&unreadCount)
	if unreadCount != 0 {
		t.Fatalf("unread count for 1001 = %d, want 0", unreadCount)
	}

	// 验证：1001 发的消息（receiver_id=2002）仍未读
	var senderUnread int64
	db.Model(&model.EduUserMessage{}).
		Where("conversation_id = ? AND receiver_id = ? AND is_read = ? AND deleted = ?", convID, 2002, false, 0).
		Count(&senderUnread)
	if senderUnread != 1 {
		t.Fatalf("unread count for 2002 = %d, want 1", senderUnread)
	}
}

func TestUserMessageCountUnread(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	seedMessage(t, db, 2002, 1001, "u1", false)
	seedMessage(t, db, 3003, 1001, "u2", false)
	seedMessage(t, db, 1001, 2002, "u3", false) // 1001 发的
	seedMessage(t, db, 2002, 1001, "u4", true)   // 已读

	count, err := repo.CountUnread(ctx, 1001)
	if err != nil {
		t.Fatalf("CountUnread error: %v", err)
	}
	if count != 2 {
		t.Fatalf("CountUnread = %d, want 2", count)
	}
}

func TestUserMessageCountUnreadByConversation(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	seedMessage(t, db, 2002, 1001, "conv1-1", false)
	seedMessage(t, db, 2002, 1001, "conv1-2", false)
	seedMessage(t, db, 3003, 1001, "conv2-1", false)

	convID := model.ConversationID(1001, 2002)
	count, err := repo.CountUnreadByConversation(ctx, convID, 1001)
	if err != nil {
		t.Fatalf("CountUnreadByConversation error: %v", err)
	}
	if count != 2 {
		t.Fatalf("CountUnreadByConversation = %d, want 2", count)
	}
}

func TestUserMessageListConversations(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	// 会话1: 1001 <-> 2002
	seedMessage(t, db, 1001, 2002, "conv1-first", false)
	time.Sleep(5 * time.Millisecond)
	seedMessage(t, db, 2002, 1001, "conv1-last", false)

	// 会话2: 1001 <-> 3003（更新的会话）
	time.Sleep(5 * time.Millisecond)
	seedMessage(t, db, 3003, 1001, "conv2-msg1", false)
	time.Sleep(5 * time.Millisecond)
	seedMessage(t, db, 1001, 3003, "conv2-last", false)

	conversations, err := repo.ListConversations(ctx, 1001)
	if err != nil {
		t.Fatalf("ListConversations error: %v", err)
	}
	if len(conversations) != 2 {
		t.Fatalf("conversations len = %d, want 2", len(conversations))
	}

	// 按最新消息时间倒序：会话2应在前
	if conversations[0].OtherUserID != 3003 {
		t.Fatalf("first conversation other_user_id = %d, want 3003", conversations[0].OtherUserID)
	}
	if conversations[0].LastMessage != "conv2-last" {
		t.Fatalf("first conversation last_message = %q, want conv2-last", conversations[0].LastMessage)
	}
	if conversations[1].OtherUserID != 2002 {
		t.Fatalf("second conversation other_user_id = %d, want 2002", conversations[1].OtherUserID)
	}
	if conversations[1].LastMessage != "conv1-last" {
		t.Fatalf("second conversation last_message = %q, want conv1-last", conversations[1].LastMessage)
	}
}

func TestUserMessageListConversationsUnreadCount(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	// 1001 收到 2 条未读，1 条已读
	seedMessage(t, db, 2002, 1001, "unread1", false)
	seedMessage(t, db, 2002, 1001, "unread2", false)
	seedMessage(t, db, 2002, 1001, "read1", true)
	// 1001 发的消息不应计入未读
	seedMessage(t, db, 1001, 2002, "sent1", false)

	conversations, err := repo.ListConversations(ctx, 1001)
	if err != nil {
		t.Fatalf("ListConversations error: %v", err)
	}
	if len(conversations) != 1 {
		t.Fatalf("conversations len = %d, want 1", len(conversations))
	}
	if conversations[0].UnreadCount != 2 {
		t.Fatalf("unread_count = %d, want 2", conversations[0].UnreadCount)
	}
}

func TestUserMessageListConversationsOtherUserID(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	// 最新消息是 1001 发给 2002 的，other_user_id 应为 2002
	seedMessage(t, db, 2002, 1001, "from 2002", false)
	time.Sleep(5 * time.Millisecond)
	seedMessage(t, db, 1001, 2002, "from 1001", false)

	conversations, err := repo.ListConversations(ctx, 1001)
	if err != nil {
		t.Fatalf("ListConversations error: %v", err)
	}
	if len(conversations) != 1 {
		t.Fatalf("conversations len = %d, want 1", len(conversations))
	}
	if conversations[0].OtherUserID != 2002 {
		t.Fatalf("other_user_id = %d, want 2002", conversations[0].OtherUserID)
	}

	// 从 2002 的视角看，other_user_id 应为 1001
	conversations2, err := repo.ListConversations(ctx, 2002)
	if err != nil {
		t.Fatalf("ListConversations for 2002 error: %v", err)
	}
	if conversations2[0].OtherUserID != 1001 {
		t.Fatalf("other_user_id for 2002 = %d, want 1001", conversations2[0].OtherUserID)
	}
}

func TestUserMessageSoftDeleteExcluded(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	msg := seedMessage(t, db, 1001, 2002, "active", false)
	// 软删除一条
	db.Model(&model.EduUserMessage{}).Where("id = ?", msg.ID).Update("deleted", 1)
	seedMessage(t, db, 1001, 2002, "also active", false)

	convID := model.ConversationID(1001, 2002)
	list, err := repo.ListMessages(ctx, convID, 1, 20)
	if err != nil {
		t.Fatalf("ListMessages error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1 (soft-deleted excluded)", len(list))
	}
}

func TestUserMessageWithTx(t *testing.T) {
	db := newUserMessageTestDB(t)
	repo := NewGormUserMessageRepository(db)
	ctx := context.Background()

	err := db.Transaction(func(tx *gorm.DB) error {
		txRepo := repo.WithTx(tx)
		msg := &model.EduUserMessage{
			SenderID:       1001,
			ReceiverID:     2002,
			ConversationID: model.ConversationID(1001, 2002),
			Content:        "tx msg",
			IsRead:         false,
			Deleted:        0,
		}
		return txRepo.CreateMessage(ctx, msg)
	})
	if err != nil {
		t.Fatalf("transaction error: %v", err)
	}

	convID := model.ConversationID(1001, 2002)
	list, err := repo.ListMessages(ctx, convID, 1, 20)
	if err != nil {
		t.Fatalf("ListMessages after tx error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len after tx = %d, want 1", len(list))
	}
}
