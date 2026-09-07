package persistence

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-service/internal/model"
)

// sysUserTestRow 用于测试中创建 sys_user 表记录。
type sysUserTestRow struct {
	ID       uint64 `gorm:"column:id;primaryKey"`
	Username string `gorm:"column:username"`
	Password string `gorm:"column:password"`
	RealName string `gorm:"column:real_name"`
	UserType int16  `gorm:"column:user_type"`
	Status   int16  `gorm:"column:status"`
	Deleted  int16  `gorm:"column:deleted"`
}

func (sysUserTestRow) TableName() string { return "sys_user" }

func newUserSearchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&sysUserTestRow{}, &model.EduUserProfile{}, &model.EduUserFollow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedUserSearchData(t *testing.T, db *gorm.DB) {
	t.Helper()
	users := []sysUserTestRow{
		{ID: 1, Username: "alice", Password: "x", RealName: "Alice", UserType: 1, Status: 1, Deleted: 0},
		{ID: 2, Username: "bob", Password: "x", RealName: "Bob", UserType: 1, Status: 1, Deleted: 0},
		{ID: 3, Username: "charlie", Password: "x", RealName: "Charlie", UserType: 1, Status: 1, Deleted: 0},
		{ID: 4, Username: "dave", Password: "x", RealName: "Dave", UserType: 1, Status: 0, Deleted: 0},  // 禁用
		{ID: 5, Username: "eve", Password: "x", RealName: "Eve", UserType: 1, Status: 1, Deleted: 1},    // 已删除
		{ID: 100, Username: "current", Password: "x", RealName: "Current", UserType: 1, Status: 1, Deleted: 0},
	}
	for _, u := range users {
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("create user %d: %v", u.ID, err)
		}
	}
	profiles := []model.EduUserProfile{
		{UserID: 1, Nickname: "爱丽丝", AvatarURL: "/alice.png", Deleted: 0},
		{UserID: 3, Nickname: "查理", AvatarURL: "/charlie.png", Deleted: 0},
		{UserID: 100, Nickname: "当前用户", AvatarURL: "", Deleted: 0},
	}
	for _, p := range profiles {
		if err := db.Create(&p).Error; err != nil {
			t.Fatalf("create profile %d: %v", p.UserID, err)
		}
	}
	// current user (100) follows alice (1) and charlie (3)
	follows := []model.EduUserFollow{
		{FollowerID: 100, FollowingID: 1, Deleted: 0},
		{FollowerID: 100, FollowingID: 3, Deleted: 0},
	}
	for _, f := range follows {
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("create follow: %v", err)
		}
	}
}

func TestUserSearchExcludesCurrentUser(t *testing.T) {
	db := newUserSearchTestDB(t)
	seedUserSearchData(t, db)
	repo := NewGormUserSearchRepository(db)

	results, total, err := repo.SearchUsers(context.Background(), 100, "a", 1, 20)
	if err != nil {
		t.Fatalf("SearchUsers error: %v", err)
	}
	for _, r := range results {
		if r.ID == 100 {
			t.Fatal("current user should not appear in results")
		}
	}
	_ = total
}

func TestUserSearchExcludesInactiveAndDeleted(t *testing.T) {
	db := newUserSearchTestDB(t)
	seedUserSearchData(t, db)
	repo := NewGormUserSearchRepository(db)

	results, _, err := repo.SearchUsers(context.Background(), 100, "dave", 1, 20)
	if err != nil {
		t.Fatalf("SearchUsers error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for disabled user dave, got %d", len(results))
	}

	results, _, err = repo.SearchUsers(context.Background(), 100, "eve", 1, 20)
	if err != nil {
		t.Fatalf("SearchUsers error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for deleted user eve, got %d", len(results))
	}
}

func TestUserSearchFollowingPriority(t *testing.T) {
	db := newUserSearchTestDB(t)
	seedUserSearchData(t, db)
	repo := NewGormUserSearchRepository(db)

	// 搜索 "a" 匹配 alice(已关注), charlie(已关注), 不匹配 bob
	// 但 charlie 不含 "a"，所以只有 alice
	results, _, err := repo.SearchUsers(context.Background(), 100, "li", 1, 20)
	if err != nil {
		t.Fatalf("SearchUsers error: %v", err)
	}
	// "li" matches alice (爱丽丝) and charlie (查理) and bob (鲍勃 - no)
	// alice and charlie are both followed, so order by nickname ASC
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	// Both are followed, sorted by nickname ASC (Unicode code point: 查 U+67E5 < 爱 U+7231)
	if results[0].Nickname != "查理" {
		t.Fatalf("first result nickname = %q, want %q", results[0].Nickname, "查理")
	}
	if !results[0].IsFollowing {
		t.Fatal("charlie should be marked as following")
	}
}

func TestUserSearchByUsername(t *testing.T) {
	db := newUserSearchTestDB(t)
	seedUserSearchData(t, db)
	repo := NewGormUserSearchRepository(db)

	results, total, err := repo.SearchUsers(context.Background(), 100, "bob", 1, 20)
	if err != nil {
		t.Fatalf("SearchUsers error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Username != "bob" {
		t.Fatalf("username = %q, want %q", results[0].Username, "bob")
	}
	// bob has no profile, nickname should fall back to real_name
	if results[0].Nickname != "Bob" {
		t.Fatalf("nickname = %q, want %q (real_name fallback)", results[0].Nickname, "Bob")
	}
	if results[0].IsFollowing {
		t.Fatal("bob should not be marked as following")
	}
}

func TestUserSearchPagination(t *testing.T) {
	db := newUserSearchTestDB(t)
	seedUserSearchData(t, db)
	repo := NewGormUserSearchRepository(db)

	// Search broad term that matches multiple users
	results, total, err := repo.SearchUsers(context.Background(), 100, "e", 1, 1)
	if err != nil {
		t.Fatalf("SearchUsers error: %v", err)
	}
	if total < 2 {
		t.Fatalf("expected total >= 2, got %d", total)
	}
	if len(results) != 1 {
		t.Fatalf("page 1 with pageSize 1 should return 1 result, got %d", len(results))
	}

	// Page 2
	results2, _, err := repo.SearchUsers(context.Background(), 100, "e", 2, 1)
	if err != nil {
		t.Fatalf("SearchUsers page 2 error: %v", err)
	}
	if len(results2) != 1 {
		t.Fatalf("page 2 with pageSize 1 should return 1 result, got %d", len(results2))
	}
	if results[0].ID == results2[0].ID {
		t.Fatal("page 1 and page 2 should return different users")
	}
}

func TestUserSearchAvatarFallback(t *testing.T) {
	db := newUserSearchTestDB(t)
	seedUserSearchData(t, db)
	repo := NewGormUserSearchRepository(db)

	results, _, err := repo.SearchUsers(context.Background(), 100, "bob", 1, 20)
	if err != nil {
		t.Fatalf("SearchUsers error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].AvatarURL != "" {
		t.Fatalf("avatar_url = %q, want empty string fallback", results[0].AvatarURL)
	}
}

func TestUserSearchCaseInsensitive(t *testing.T) {
	db := newUserSearchTestDB(t)
	seedUserSearchData(t, db)
	repo := NewGormUserSearchRepository(db)

	results, total, err := repo.SearchUsers(context.Background(), 100, "ALICE", 1, 20)
	if err != nil {
		t.Fatalf("SearchUsers error: %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1 (case insensitive)", total)
	}
	if len(results) != 1 || results[0].Username != "alice" {
		t.Fatalf("expected alice, got %+v", results)
	}
}
