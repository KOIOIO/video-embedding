package usermessage

import (
	"context"
	"errors"
	"testing"
	"time"

	"video-service/internal/infrastructure/persistence"
	"video-service/internal/model"
)

// --- mock message repository ---

type mockMessageRepo struct {
	messages          []model.EduUserMessage
	createErr         error
	markAsReadErr     error
	countUnread       map[uint64]int64
	countUnreadConv   map[string]int64
	conversations     []persistence.ConversationSummary
	createCalls       int
	markAsReadCalls   int
	lastMarkConvID    string
	lastMarkUserID    uint64
}

func newMockMessageRepo() *mockMessageRepo {
	return &mockMessageRepo{
		countUnread:     make(map[uint64]int64),
		countUnreadConv: make(map[string]int64),
	}
}

func (m *mockMessageRepo) CreateMessage(_ context.Context, msg *model.EduUserMessage) error {
	m.createCalls++
	if m.createErr != nil {
		return m.createErr
	}
	msg.ID = uint64(len(m.messages) + 1)
	msg.CreateTime = time.Now()
	m.messages = append(m.messages, *msg)
	return nil
}

func (m *mockMessageRepo) ListMessages(_ context.Context, conversationID string, page, pageSize int) ([]model.EduUserMessage, error) {
	var result []model.EduUserMessage
	for _, msg := range m.messages {
		if msg.ConversationID == conversationID && msg.Deleted == 0 {
			result = append(result, msg)
		}
	}
	return result, nil
}

func (m *mockMessageRepo) MarkAsRead(_ context.Context, conversationID string, userID uint64) error {
	m.markAsReadCalls++
	m.lastMarkConvID = conversationID
	m.lastMarkUserID = userID
	if m.markAsReadErr != nil {
		return m.markAsReadErr
	}
	return nil
}

func (m *mockMessageRepo) CountUnread(_ context.Context, userID uint64) (int64, error) {
	return m.countUnread[userID], nil
}

func (m *mockMessageRepo) CountUnreadByConversation(_ context.Context, conversationID string, userID uint64) (int64, error) {
	return m.countUnreadConv[conversationID], nil
}

func (m *mockMessageRepo) ListConversations(_ context.Context, userID uint64) ([]persistence.ConversationSummary, error) {
	return m.conversations, nil
}

// --- tests ---

func TestSendMessageCannotMessageSelf(t *testing.T) {
	svc := NewService(newMockMessageRepo())
	_, err := svc.SendMessage(context.Background(), 1001, 1001, "hello")
	if !errors.Is(err, ErrCannotMessageSelf) {
		t.Fatalf("SendMessage(self) error = %v, want ErrCannotMessageSelf", err)
	}
}

func TestSendMessageEmptyContent(t *testing.T) {
	svc := NewService(newMockMessageRepo())
	_, err := svc.SendMessage(context.Background(), 1001, 2002, "")
	if !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("SendMessage(empty) error = %v, want ErrEmptyContent", err)
	}
	_, err = svc.SendMessage(context.Background(), 1001, 2002, "   ")
	if !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("SendMessage(whitespace) error = %v, want ErrEmptyContent", err)
	}
}

func TestSendMessageContentTooLong(t *testing.T) {
	svc := NewService(newMockMessageRepo())
	longContent := make([]byte, 2001)
	for i := range longContent {
		longContent[i] = 'a'
	}
	_, err := svc.SendMessage(context.Background(), 1001, 2002, string(longContent))
	if !errors.Is(err, ErrContentTooLong) {
		t.Fatalf("SendMessage(too long) error = %v, want ErrContentTooLong", err)
	}
}

func TestSendMessageSuccess(t *testing.T) {
	repo := newMockMessageRepo()
	svc := NewService(repo)
	msg, err := svc.SendMessage(context.Background(), 1001, 2002, "hello world")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if msg.SenderID != 1001 {
		t.Fatalf("SenderID = %d, want 1001", msg.SenderID)
	}
	if msg.ReceiverID != 2002 {
		t.Fatalf("ReceiverID = %d, want 2002", msg.ReceiverID)
	}
	if msg.Content != "hello world" {
		t.Fatalf("Content = %q, want %q", msg.Content, "hello world")
	}
	if msg.ConversationID != model.ConversationID(1001, 2002) {
		t.Fatalf("ConversationID = %q, want %q", msg.ConversationID, model.ConversationID(1001, 2002))
	}
	if msg.IsRead {
		t.Fatal("IsRead should be false for new message")
	}
	if repo.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", repo.createCalls)
	}
}

func TestSendMessageConversationIDConsistent(t *testing.T) {
	// 双向发送应生成相同的 conversation_id
	repo := newMockMessageRepo()
	svc := NewService(repo)
	msg1, _ := svc.SendMessage(context.Background(), 1001, 2002, "from 1001")
	msg2, _ := svc.SendMessage(context.Background(), 2002, 1001, "from 2002")
	if msg1.ConversationID != msg2.ConversationID {
		t.Fatalf("conversation IDs differ: %q vs %q", msg1.ConversationID, msg2.ConversationID)
	}
	if msg1.ConversationID != "1001_2002" {
		t.Fatalf("ConversationID = %q, want 1001_2002", msg1.ConversationID)
	}
}

func TestSendMessageRepositoryError(t *testing.T) {
	repo := newMockMessageRepo()
	repo.createErr = errors.New("db connection lost")
	svc := NewService(repo)
	_, err := svc.SendMessage(context.Background(), 1001, 2002, "hello")
	if err == nil {
		t.Fatal("SendMessage should return error when repo fails")
	}
}

func TestGetConversationMessagesInvalidParticipant(t *testing.T) {
	svc := NewService(newMockMessageRepo())
	_, err := svc.GetConversationMessages(context.Background(), 0, 2002, 1, 20)
	if !errors.Is(err, ErrNotConversationParticipant) {
		t.Fatalf("zero userID error = %v, want ErrNotConversationParticipant", err)
	}
	_, err = svc.GetConversationMessages(context.Background(), 1001, 1001, 1, 20)
	if !errors.Is(err, ErrNotConversationParticipant) {
		t.Fatalf("same user error = %v, want ErrNotConversationParticipant", err)
	}
}

func TestGetConversationMessagesSuccess(t *testing.T) {
	repo := newMockMessageRepo()
	svc := NewService(repo)
	// 插入一些消息
	svc.SendMessage(context.Background(), 1001, 2002, "msg1")
	svc.SendMessage(context.Background(), 2002, 1001, "msg2")

	list, err := svc.GetConversationMessages(context.Background(), 1001, 2002, 1, 20)
	if err != nil {
		t.Fatalf("GetConversationMessages error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
}

func TestMarkConversationAsRead(t *testing.T) {
	repo := newMockMessageRepo()
	svc := NewService(repo)
	err := svc.MarkConversationAsRead(context.Background(), 1001, 2002)
	if err != nil {
		t.Fatalf("MarkConversationAsRead error: %v", err)
	}
	if repo.markAsReadCalls != 1 {
		t.Fatalf("markAsReadCalls = %d, want 1", repo.markAsReadCalls)
	}
	expectedConvID := model.ConversationID(1001, 2002)
	if repo.lastMarkConvID != expectedConvID {
		t.Fatalf("lastMarkConvID = %q, want %q", repo.lastMarkConvID, expectedConvID)
	}
	if repo.lastMarkUserID != 1001 {
		t.Fatalf("lastMarkUserID = %d, want 1001", repo.lastMarkUserID)
	}
}

func TestMarkConversationAsReadInvalid(t *testing.T) {
	svc := NewService(newMockMessageRepo())
	err := svc.MarkConversationAsRead(context.Background(), 0, 2002)
	if !errors.Is(err, ErrNotConversationParticipant) {
		t.Fatalf("zero userID error = %v, want ErrNotConversationParticipant", err)
	}
}

func TestGetTotalUnread(t *testing.T) {
	repo := newMockMessageRepo()
	repo.countUnread[1001] = 5
	svc := NewService(repo)
	count, err := svc.GetTotalUnread(context.Background(), 1001)
	if err != nil {
		t.Fatalf("GetTotalUnread error: %v", err)
	}
	if count != 5 {
		t.Fatalf("count = %d, want 5", count)
	}
}

func TestGetConversationsConversion(t *testing.T) {
	repo := newMockMessageRepo()
	now := time.Now()
	repo.conversations = []persistence.ConversationSummary{
		{
			ConversationID:  "1001_2002",
			OtherUserID:     2002,
			LastMessage:     "hello",
			LastMessageTime: now,
			UnreadCount:     3,
		},
	}
	svc := NewService(repo)
	list, err := svc.GetConversations(context.Background(), 1001)
	if err != nil {
		t.Fatalf("GetConversations error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if list[0].ConversationID != "1001_2002" {
		t.Fatalf("ConversationID = %q, want 1001_2002", list[0].ConversationID)
	}
	if list[0].OtherUserID != 2002 {
		t.Fatalf("OtherUserID = %d, want 2002", list[0].OtherUserID)
	}
	if list[0].LastMessage != "hello" {
		t.Fatalf("LastMessage = %q, want hello", list[0].LastMessage)
	}
	if list[0].UnreadCount != 3 {
		t.Fatalf("UnreadCount = %d, want 3", list[0].UnreadCount)
	}
}
