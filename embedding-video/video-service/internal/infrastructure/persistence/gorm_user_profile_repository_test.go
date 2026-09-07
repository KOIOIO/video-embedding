package persistence

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-service/internal/model"
)

func newUserProfileTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduUserProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestUserProfileGetByUserIDReturnsNilWhenMissing(t *testing.T) {
	db := newUserProfileTestDB(t)
	repo := NewGormUserProfileRepository(db)

	got, err := repo.GetByUserID(9999)
	if err != nil {
		t.Fatalf("GetByUserID error: %v", err)
	}
	if got != nil {
		t.Fatalf("GetByUserID = %+v, want nil", got)
	}
}

func TestUserProfileUpsertDefaultCreatesRecord(t *testing.T) {
	db := newUserProfileTestDB(t)
	repo := NewGormUserProfileRepository(db)

	profile, err := repo.UpsertDefault(1001)
	if err != nil {
		t.Fatalf("UpsertDefault error: %v", err)
	}
	if profile == nil {
		t.Fatal("UpsertDefault returned nil")
	}
	if profile.UserID != 1001 {
		t.Fatalf("UserID = %d, want 1001", profile.UserID)
	}
	if profile.Nickname != "用户1001" {
		t.Fatalf("Nickname = %q, want %q", profile.Nickname, "用户1001")
	}
	if profile.ID == 0 {
		t.Fatal("ID should be auto-generated")
	}
}

func TestUserProfileUpsertDefaultIdempotent(t *testing.T) {
	db := newUserProfileTestDB(t)
	repo := NewGormUserProfileRepository(db)

	first, err := repo.UpsertDefault(2002)
	if err != nil {
		t.Fatalf("first UpsertDefault error: %v", err)
	}
	second, err := repo.UpsertDefault(2002)
	if err != nil {
		t.Fatalf("second UpsertDefault error: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("UpsertDefault not idempotent: first.ID=%d second.ID=%d", first.ID, second.ID)
	}
}

func TestUserProfileUpdateFields(t *testing.T) {
	db := newUserProfileTestDB(t)
	repo := NewGormUserProfileRepository(db)

	if _, err := repo.UpsertDefault(3003); err != nil {
		t.Fatalf("UpsertDefault error: %v", err)
	}
	err := repo.Update(3003, map[string]interface{}{
		"nickname": "新昵称",
		"bio":      "这是签名",
		"gender":   int16(1),
		"location": "郑州",
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	got, err := repo.GetByUserID(3003)
	if err != nil {
		t.Fatalf("GetByUserID error: %v", err)
	}
	if got.Nickname != "新昵称" {
		t.Fatalf("Nickname = %q, want %q", got.Nickname, "新昵称")
	}
	if got.Bio != "这是签名" {
		t.Fatalf("Bio = %q, want %q", got.Bio, "这是签名")
	}
	if got.Gender != 1 {
		t.Fatalf("Gender = %d, want 1", got.Gender)
	}
	if got.Location != "郑州" {
		t.Fatalf("Location = %q, want %q", got.Location, "郑州")
	}
}

func TestUserProfileUpdateAvatar(t *testing.T) {
	db := newUserProfileTestDB(t)
	repo := NewGormUserProfileRepository(db)

	if _, err := repo.UpsertDefault(4004); err != nil {
		t.Fatalf("UpsertDefault error: %v", err)
	}
	if err := repo.UpdateAvatar(4004, "https://example.com/avatar.jpg"); err != nil {
		t.Fatalf("UpdateAvatar error: %v", err)
	}
	got, err := repo.GetByUserID(4004)
	if err != nil {
		t.Fatalf("GetByUserID error: %v", err)
	}
	if got.AvatarURL != "https://example.com/avatar.jpg" {
		t.Fatalf("AvatarURL = %q, want %q", got.AvatarURL, "https://example.com/avatar.jpg")
	}
}

func TestUserProfileIncrementFollowCount(t *testing.T) {
	db := newUserProfileTestDB(t)
	repo := NewGormUserProfileRepository(db)

	if _, err := repo.UpsertDefault(5005); err != nil {
		t.Fatalf("UpsertDefault error: %v", err)
	}
	if err := repo.IncrementFollowCount(5005, 5); err != nil {
		t.Fatalf("IncrementFollowCount error: %v", err)
	}
	if err := repo.IncrementFollowCount(5005, -2); err != nil {
		t.Fatalf("IncrementFollowCount negative error: %v", err)
	}
	got, err := repo.GetByUserID(5005)
	if err != nil {
		t.Fatalf("GetByUserID error: %v", err)
	}
	if got.FollowCount != 3 {
		t.Fatalf("FollowCount = %d, want 3", got.FollowCount)
	}
}

func TestUserProfileIncrementFansCount(t *testing.T) {
	db := newUserProfileTestDB(t)
	repo := NewGormUserProfileRepository(db)

	if _, err := repo.UpsertDefault(6006); err != nil {
		t.Fatalf("UpsertDefault error: %v", err)
	}
	if err := repo.IncrementFansCount(6006, 10); err != nil {
		t.Fatalf("IncrementFansCount error: %v", err)
	}
	got, err := repo.GetByUserID(6006)
	if err != nil {
		t.Fatalf("GetByUserID error: %v", err)
	}
	if got.FansCount != 10 {
		t.Fatalf("FansCount = %d, want 10", got.FansCount)
	}
}
