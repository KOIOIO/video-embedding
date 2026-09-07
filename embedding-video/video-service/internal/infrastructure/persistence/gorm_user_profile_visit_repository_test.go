package persistence

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-service/internal/model"
)

func newProfileVisitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EduUserProfileVisit{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 创建部分唯一索引，与 migration 保持一致
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uk_user_profile_visit_daily ON edu_user_profile_visit(visitor_id, owner_id, visit_date) WHERE deleted = 0;`).Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	return db
}

func TestProfileVisitUpsert_FirstVisit(t *testing.T) {
	db := newProfileVisitTestDB(t)
	repo := NewGormUserProfileVisitRepository(db)
	ctx := context.Background()
	today := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	if err := repo.UpsertVisit(ctx, 1001, 2002, today); err != nil {
		t.Fatalf("UpsertVisit error: %v", err)
	}
	var record model.EduUserProfileVisit
	if err := db.Where("visitor_id = ? AND owner_id = ? AND visit_date = ?", 1001, 2002, today).First(&record).Error; err != nil {
		t.Fatalf("query record: %v", err)
	}
	if record.VisitCount != 1 {
		t.Fatalf("expected visit_count=1, got %d", record.VisitCount)
	}
}

func TestProfileVisitUpsert_RepeatVisitIncrements(t *testing.T) {
	db := newProfileVisitTestDB(t)
	repo := NewGormUserProfileVisitRepository(db)
	ctx := context.Background()
	today := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		if err := repo.UpsertVisit(ctx, 1001, 2002, today); err != nil {
			t.Fatalf("UpsertVisit #%d error: %v", i+1, err)
		}
	}
	var count int64
	db.Model(&model.EduUserProfileVisit{}).Where("visitor_id = ? AND owner_id = ? AND visit_date = ?", 1001, 2002, today).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 row after upserts, got %d", count)
	}
	var record model.EduUserProfileVisit
	db.Where("visitor_id = ? AND owner_id = ? AND visit_date = ?", 1001, 2002, today).First(&record)
	if record.VisitCount != 3 {
		t.Fatalf("expected visit_count=3, got %d", record.VisitCount)
	}
}

func TestProfileVisitUpsert_AnonymousVisit(t *testing.T) {
	db := newProfileVisitTestDB(t)
	repo := NewGormUserProfileVisitRepository(db)
	ctx := context.Background()
	today := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	if err := repo.UpsertVisit(ctx, 0, 2002, today); err != nil {
		t.Fatalf("anonymous UpsertVisit error: %v", err)
	}
	if err := repo.UpsertVisit(ctx, 0, 2002, today); err != nil {
		t.Fatalf("anonymous UpsertVisit #2 error: %v", err)
	}
	var record model.EduUserProfileVisit
	db.Where("visitor_id = 0 AND owner_id = ? AND visit_date = ?", 2002, today).First(&record)
	if record.VisitCount != 2 {
		t.Fatalf("expected visit_count=2 for anonymous, got %d", record.VisitCount)
	}
}

func TestProfileVisitUpsert_DifferentVisitorsSeparateRows(t *testing.T) {
	db := newProfileVisitTestDB(t)
	repo := NewGormUserProfileVisitRepository(db)
	ctx := context.Background()
	today := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	_ = repo.UpsertVisit(ctx, 1001, 2002, today)
	_ = repo.UpsertVisit(ctx, 1003, 2002, today)

	var count int64
	db.Model(&model.EduUserProfileVisit{}).Where("owner_id = ? AND visit_date = ?", 2002, today).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 rows for different visitors, got %d", count)
	}
}

func TestProfileVisitGetDailyStats(t *testing.T) {
	db := newProfileVisitTestDB(t)
	repo := NewGormUserProfileVisitRepository(db)
	ctx := context.Background()
	today := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	// visitor 1001 visits 3 times
	_ = repo.UpsertVisit(ctx, 1001, 2002, today)
	_ = repo.UpsertVisit(ctx, 1001, 2002, today)
	_ = repo.UpsertVisit(ctx, 1001, 2002, today)
	// visitor 1003 visits 1 time
	_ = repo.UpsertVisit(ctx, 1003, 2002, today)
	// anonymous visits 2 times
	_ = repo.UpsertVisit(ctx, 0, 2002, today)
	_ = repo.UpsertVisit(ctx, 0, 2002, today)

	unique, total, err := repo.GetDailyStats(ctx, 2002, today)
	if err != nil {
		t.Fatalf("GetDailyStats error: %v", err)
	}
	if unique != 3 {
		t.Fatalf("expected 3 unique visitors (1001, 1003, anonymous), got %d", unique)
	}
	if total != 6 {
		t.Fatalf("expected 6 total visits (3+1+2), got %d", total)
	}
}

func TestProfileVisitGetDailyStats_NoData(t *testing.T) {
	db := newProfileVisitTestDB(t)
	repo := NewGormUserProfileVisitRepository(db)
	ctx := context.Background()
	today := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	unique, total, err := repo.GetDailyStats(ctx, 9999, today)
	if err != nil {
		t.Fatalf("GetDailyStats error: %v", err)
	}
	if unique != 0 || total != 0 {
		t.Fatalf("expected 0/0 for no data, got %d/%d", unique, total)
	}
}
