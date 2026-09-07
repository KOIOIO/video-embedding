package userprofile

import (
	"errors"
	"strings"
	"testing"

	"video-service/internal/model"
)

type mockUserProfileRepo struct {
	profiles    map[uint64]*model.EduUserProfile
	getErr      error
	upsertErr   error
	updateErr   error
	avatarErr   error
	upsertCalls int
	updateCalls int
}

func newMockRepo() *mockUserProfileRepo {
	return &mockUserProfileRepo{profiles: make(map[uint64]*model.EduUserProfile)}
}

func (m *mockUserProfileRepo) GetByUserID(userID uint64) (*model.EduUserProfile, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if p, ok := m.profiles[userID]; ok {
		return p, nil
	}
	return nil, nil
}

func (m *mockUserProfileRepo) UpsertDefault(userID uint64) (*model.EduUserProfile, error) {
	m.upsertCalls++
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	if p, ok := m.profiles[userID]; ok {
		return p, nil
	}
	p := &model.EduUserProfile{
		ID:       uint64(len(m.profiles) + 1),
		UserID:   userID,
		Nickname: "用户" + itoa(userID),
	}
	m.profiles[userID] = p
	return p, nil
}

func (m *mockUserProfileRepo) Update(userID uint64, updates map[string]interface{}) error {
	m.updateCalls++
	if m.updateErr != nil {
		return m.updateErr
	}
	p, ok := m.profiles[userID]
	if !ok {
		return errors.New("profile not found")
	}
	if v, ok := updates["nickname"]; ok {
		p.Nickname = v.(string)
	}
	if v, ok := updates["bio"]; ok {
		p.Bio = v.(string)
	}
	if v, ok := updates["location"]; ok {
		p.Location = v.(string)
	}
	if v, ok := updates["gender"]; ok {
		p.Gender = v.(int16)
	}
	return nil
}

func (m *mockUserProfileRepo) UpdateAvatar(userID uint64, avatarURL string) error {
	if m.avatarErr != nil {
		return m.avatarErr
	}
	p, ok := m.profiles[userID]
	if !ok {
		return errors.New("profile not found")
	}
	p.AvatarURL = avatarURL
	return nil
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func TestServiceGetProfileLazyInit(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	profile, err := svc.GetProfile(1001)
	if err != nil {
		t.Fatalf("GetProfile error: %v", err)
	}
	if profile == nil {
		t.Fatal("GetProfile returned nil")
	}
	if profile.Nickname != "用户1001" {
		t.Fatalf("Nickname = %q, want %q", profile.Nickname, "用户1001")
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("upsertCalls = %d, want 1", repo.upsertCalls)
	}

	// 第二次调用不应再触发 UpsertDefault
	profile2, err := svc.GetProfile(1001)
	if err != nil {
		t.Fatalf("second GetProfile error: %v", err)
	}
	if profile2.ID != profile.ID {
		t.Fatal("second GetProfile returned different record")
	}
	if repo.upsertCalls != 1 {
		t.Fatalf("upsertCalls after second call = %d, want 1", repo.upsertCalls)
	}
}

func TestServiceUpdateProfileValidation(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	// 空昵称
	if err := svc.UpdateProfile(1001, "", "bio", "loc", 1); err == nil {
		t.Fatal("expected error for empty nickname")
	}

	// 昵称超长
	longNick := strings.Repeat("a", 101)
	if err := svc.UpdateProfile(1001, longNick, "bio", "loc", 1); err == nil {
		t.Fatal("expected error for nickname > 100 chars")
	}

	// bio 超长
	longBio := strings.Repeat("a", 501)
	if err := svc.UpdateProfile(1001, "nick", longBio, "loc", 1); err == nil {
		t.Fatal("expected error for bio > 500 chars")
	}

	// gender 非法
	if err := svc.UpdateProfile(1001, "nick", "bio", "loc", 3); err == nil {
		t.Fatal("expected error for invalid gender")
	}
}

func TestServiceUpdateProfileSuccess(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	if err := svc.UpdateProfile(2002, "测试用户", "这是签名", "郑州", 1); err != nil {
		t.Fatalf("UpdateProfile error: %v", err)
	}
	profile, err := svc.GetProfile(2002)
	if err != nil {
		t.Fatalf("GetProfile error: %v", err)
	}
	if profile.Nickname != "测试用户" {
		t.Fatalf("Nickname = %q, want %q", profile.Nickname, "测试用户")
	}
	if profile.Bio != "这是签名" {
		t.Fatalf("Bio = %q, want %q", profile.Bio, "这是签名")
	}
	if profile.Gender != 1 {
		t.Fatalf("Gender = %d, want 1", profile.Gender)
	}
}

func TestServiceUpdateAvatar(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)

	// 空 URL
	if err := svc.UpdateAvatar(3003, "  "); err == nil {
		t.Fatal("expected error for empty avatar URL")
	}

	if err := svc.UpdateAvatar(3003, "https://example.com/avatar.png"); err != nil {
		t.Fatalf("UpdateAvatar error: %v", err)
	}
	profile, err := svc.GetProfile(3003)
	if err != nil {
		t.Fatalf("GetProfile error: %v", err)
	}
	if profile.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("AvatarURL = %q, want %q", profile.AvatarURL, "https://example.com/avatar.png")
	}
}
