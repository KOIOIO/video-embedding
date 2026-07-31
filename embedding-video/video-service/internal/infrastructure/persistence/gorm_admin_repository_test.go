package persistence

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGormAdminRepositoryFindActiveAdminByUsername(t *testing.T) {
	db := adminRepositoryTestDB(t)
	repo := NewGormAdminRepository(db)

	admin, found, err := repo.FindActiveAdminByUsername(context.Background(), "admin")
	if err != nil || !found {
		t.Fatalf("FindActiveAdminByUsername() found=%v err=%v", found, err)
	}
	if admin.ID != 7 || admin.Username != "admin" || admin.RealName != "System Admin" || admin.UserType != 3 {
		t.Fatalf("admin = %+v", admin)
	}
}

func TestGormAdminRepositoryRejectsIneligibleAccounts(t *testing.T) {
	db := adminRepositoryTestDB(t)
	repo := NewGormAdminRepository(db)

	for _, username := range []string{"teacher", "disabled", "deleted", "missing"} {
		t.Run(username, func(t *testing.T) {
			_, found, err := repo.FindActiveAdminByUsername(context.Background(), username)
			if err != nil {
				t.Fatalf("FindActiveAdminByUsername() error = %v", err)
			}
			if found {
				t.Fatal("FindActiveAdminByUsername() found ineligible account")
			}
		})
	}
}

func TestGormAdminRepositoryFindActiveAdminByID(t *testing.T) {
	db := adminRepositoryTestDB(t)
	repo := NewGormAdminRepository(db)

	admin, found, err := repo.FindActiveAdminByID(context.Background(), 7)
	if err != nil || !found || admin.Username != "admin" {
		t.Fatalf("FindActiveAdminByID() admin=%+v found=%v err=%v", admin, found, err)
	}
}

func adminRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE sys_user (
        id INTEGER PRIMARY KEY,
        username TEXT,
        password TEXT,
        real_name TEXT,
        user_type INTEGER,
        status INTEGER,
        deleted INTEGER
    )`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO sys_user VALUES
        (7, 'admin', '$2a$10$hash', 'System Admin', 3, 1, 0),
        (8, 'teacher', '$2a$10$hash', 'Teacher', 2, 1, 0),
        (9, 'disabled', '$2a$10$hash', 'Disabled', 3, 0, 0),
        (10, 'deleted', '$2a$10$hash', 'Deleted', 3, 1, 1)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
