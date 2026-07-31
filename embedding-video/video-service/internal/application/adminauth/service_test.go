package adminauth

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func testJWTSecret() string {
	return strings.Repeat("test-secret-", 4)
}

func legacyMD5(password string) string {
	sum := md5.Sum([]byte(password))
	return hex.EncodeToString(sum[:])
}

func TestServiceLoginAndAuthenticate(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	repo := &authTestRepository{admin: Admin{ID: 7, Username: "admin", PasswordHash: string(hash), RealName: "System Admin", UserType: 3, Status: 1}, active: true}
	service := NewService(repo, testJWTSecret(), 8*time.Hour)
	service.now = func() time.Time { return now }

	result, err := service.Login(context.Background(), " admin ", "correct-password")
	if err != nil || result.Token == "" || result.Admin.ID != 7 {
		t.Fatalf("Login() result=%+v err=%v", result, err)
	}
	if !result.ExpiresAt.Equal(now.Add(8 * time.Hour)) {
		t.Fatalf("ExpiresAt=%s", result.ExpiresAt)
	}
	authenticated, err := service.Authenticate(context.Background(), result.Token)
	if err != nil || authenticated.ID != 7 {
		t.Fatalf("Authenticate() admin=%+v err=%v", authenticated, err)
	}
}

func TestServiceLoginRejectsInvalidCredentials(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	repo := &authTestRepository{admin: Admin{ID: 7, Username: "admin", PasswordHash: string(hash)}, active: true}
	service := NewService(repo, testJWTSecret(), time.Hour)

	for _, tc := range []struct{ username, password string }{{"missing", "correct-password"}, {"admin", "wrong"}, {"", ""}} {
		_, err := service.Login(context.Background(), tc.username, tc.password)
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login(%q) error=%v", tc.username, err)
		}
	}
}

func TestServiceLoginAcceptsLegacyMD5Password(t *testing.T) {
	repo := &authTestRepository{
		admin:  Admin{ID: 7, Username: "admin", PasswordHash: legacyMD5("123456")},
		active: true,
	}
	service := NewService(repo, testJWTSecret(), time.Hour)

	result, err := service.Login(context.Background(), "admin", "123456")
	if err != nil || result.Token == "" {
		t.Fatalf("Login() result=%+v err=%v", result, err)
	}
}

func TestServiceLoginRejectsInvalidLegacyPasswordAndUnknownHash(t *testing.T) {
	repo := &authTestRepository{admin: Admin{ID: 7, Username: "admin"}, active: true}
	service := NewService(repo, testJWTSecret(), time.Hour)

	for _, tc := range []struct {
		name     string
		hash     string
		password string
	}{
		{name: "wrong legacy password", hash: legacyMD5("123456"), password: "654321"},
		{name: "unknown hash format", hash: "plain-text-password", password: "plain-text-password"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo.admin.PasswordHash = tc.hash
			if _, err := service.Login(context.Background(), "admin", tc.password); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("Login() error=%v", err)
			}
		})
	}
}

func TestServiceAuthenticateRejectsExpiredAndDisabledAdmin(t *testing.T) {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	repo := &authTestRepository{admin: Admin{ID: 7, Username: "admin", PasswordHash: "$2a$04$invalid"}, active: true}
	service := NewService(repo, "01234567890123456789012345678901", time.Hour)
	service.now = func() time.Time { return now }
	token, _, err := service.issueToken(7)
	if err != nil {
		t.Fatal(err)
	}

	repo.active = false
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("disabled Authenticate() error=%v", err)
	}
	repo.active = true
	service.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := service.Authenticate(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired Authenticate() error=%v", err)
	}
}

type authTestRepository struct {
	admin  Admin
	active bool
}

func (r *authTestRepository) FindActiveAdminByUsername(_ context.Context, username string) (Admin, bool, error) {
	return r.admin, r.active && username == r.admin.Username, nil
}

func (r *authTestRepository) FindActiveAdminByID(_ context.Context, id uint64) (Admin, bool, error) {
	return r.admin, r.active && id == r.admin.ID, nil
}
