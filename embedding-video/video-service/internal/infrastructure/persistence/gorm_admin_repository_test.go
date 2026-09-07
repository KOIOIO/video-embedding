package persistence

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"video-service/internal/application/adminauth"
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

func TestGormAdminRepositoryFindActiveUserByUsername(t *testing.T) {
	db := adminRepositoryTestDB(t)
	repo := NewGormAdminRepository(db)

	// FindActiveUserByUsername should find both admin (type 3) and normal user (type 2)
	for _, tc := range []struct {
		username string
		wantID   uint64
		wantType int16
	}{
		{"admin", 7, 3},
		{"teacher", 8, 2},
	} {
		t.Run(tc.username, func(t *testing.T) {
			admin, found, err := repo.FindActiveUserByUsername(context.Background(), tc.username)
			if err != nil || !found {
				t.Fatalf("FindActiveUserByUsername(%q) found=%v err=%v", tc.username, found, err)
			}
			if admin.ID != tc.wantID || admin.UserType != tc.wantType {
				t.Fatalf("admin = %+v", admin)
			}
		})
	}
}

func TestGormAdminRepositoryFindActiveUserByID(t *testing.T) {
	db := adminRepositoryTestDB(t)
	repo := NewGormAdminRepository(db)

	admin, found, err := repo.FindActiveUserByID(context.Background(), 8)
	if err != nil || !found || admin.Username != "teacher" || admin.UserType != 2 {
		t.Fatalf("FindActiveUserByID() admin=%+v found=%v err=%v", admin, found, err)
	}
}

func TestGormAdminRepositoryFindActiveUserRejectsInactive(t *testing.T) {
	db := adminRepositoryTestDB(t)
	repo := NewGormAdminRepository(db)

	for _, username := range []string{"disabled", "deleted", "missing"} {
		t.Run(username, func(t *testing.T) {
			_, found, err := repo.FindActiveUserByUsername(context.Background(), username)
			if err != nil {
				t.Fatalf("FindActiveUserByUsername() error = %v", err)
			}
			if found {
				t.Fatal("FindActiveUserByUsername() found inactive account")
			}
		})
	}
}

func TestGormAdminRepositoryCreateUser(t *testing.T) {
	db := adminRepositoryTestDB(t)
	repo := NewGormAdminRepository(db)

	id, err := repo.CreateUser(context.Background(), adminauth.Admin{
		Username:     "newuser",
		PasswordHash: "$2a$10$newhash",
		RealName:     "New User",
		UserType:     adminauth.UserTypeNormal,
		Status:       1,
		Deleted:      0,
	})
	if err != nil {
		t.Fatalf("CreateUser() error=%v", err)
	}
	if id == 0 {
		t.Fatal("CreateUser() returned zero ID")
	}

	// Verify the user was created and can be found
	admin, found, err := repo.FindActiveUserByUsername(context.Background(), "newuser")
	if err != nil || !found {
		t.Fatalf("FindActiveUserByUsername after create: found=%v err=%v", found, err)
	}
	if admin.ID != id || admin.RealName != "New User" || admin.UserType != adminauth.UserTypeNormal {
		t.Fatalf("created admin = %+v", admin)
	}
}

func TestGormAdminRepositoryCreateUserDuplicateUsername(t *testing.T) {
	db := adminRepositoryTestDB(t)
	repo := NewGormAdminRepository(db)

	// First create should succeed
	_, err := repo.CreateUser(context.Background(), adminauth.Admin{
		Username:     "dupuser",
		PasswordHash: "$2a$10$hash",
		RealName:     "Dup",
		UserType:     adminauth.UserTypeNormal,
		Status:       1,
		Deleted:      0,
	})
	if err != nil {
		t.Fatalf("first CreateUser() error=%v", err)
	}

	// Second create with same username should fail with ErrUsernameExists
	_, err = repo.CreateUser(context.Background(), adminauth.Admin{
		Username:     "dupuser",
		PasswordHash: "$2a$10$hash2",
		RealName:     "Dup2",
		UserType:     adminauth.UserTypeNormal,
		Status:       1,
		Deleted:      0,
	})
	if !errors.Is(err, adminauth.ErrUsernameExists) {
		t.Fatalf("expected ErrUsernameExists, got %v", err)
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
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_user_username ON sys_user(username) WHERE deleted = 0;`).Error; err != nil {
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
