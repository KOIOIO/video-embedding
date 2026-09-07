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
	service := NewService(repo, testJWTSecret(), time.Hour)
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

func TestServiceRegisterSuccess(t *testing.T) {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	repo := &authTestRepository{createdID: 42}
	service := NewService(repo, testJWTSecret(), 8*time.Hour)
	service.now = func() time.Time { return now }

	result, err := service.Register(context.Background(), "newuser", "password123", "New User")
	if err != nil {
		t.Fatalf("Register() error=%v", err)
	}
	if result.Token == "" || result.Admin.ID != 42 || result.Admin.Username != "newuser" || result.Admin.RealName != "New User" {
		t.Fatalf("Register() result=%+v", result)
	}
	if result.Admin.UserType != UserTypeNormal {
		t.Fatalf("expected UserTypeNormal, got %d", result.Admin.UserType)
	}
	if result.Admin.PasswordHash != "" {
		t.Fatal("password hash leaked in result")
	}
	if !result.ExpiresAt.Equal(now.Add(8 * time.Hour)) {
		t.Fatalf("ExpiresAt=%s", result.ExpiresAt)
	}
	// Verify token is valid
	authenticated, err := service.Authenticate(context.Background(), result.Token)
	if err != nil || authenticated.ID != 42 {
		t.Fatalf("Authenticate() after register: admin=%+v err=%v", authenticated, err)
	}
}

func TestServiceRegisterDefaultsNicknameToUsername(t *testing.T) {
	repo := &authTestRepository{createdID: 10}
	service := NewService(repo, testJWTSecret(), time.Hour)

	result, err := service.Register(context.Background(), "alice", "secret123", "")
	if err != nil {
		t.Fatalf("Register() error=%v", err)
	}
	if result.Admin.RealName != "alice" {
		t.Fatalf("expected nickname default to username, got %q", result.Admin.RealName)
	}
}

func TestServiceRegisterValidatesUsername(t *testing.T) {
	repo := &authTestRepository{}
	service := NewService(repo, testJWTSecret(), time.Hour)

	for _, username := range []string{"ab", "user name", "user@name", strings.Repeat("x", 21), ""} {
		_, err := service.Register(context.Background(), username, "password123", "")
		if !errors.Is(err, ErrInvalidUsername) {
			t.Fatalf("Register(%q) expected ErrInvalidUsername, got %v", username, err)
		}
	}
	if repo.createCalls != 0 {
		t.Fatal("CreateUser should not be called for invalid username")
	}
}

func TestServiceRegisterValidatesPassword(t *testing.T) {
	repo := &authTestRepository{}
	service := NewService(repo, testJWTSecret(), time.Hour)

	for _, password := range []string{"short", strings.Repeat("x", 33)} {
		_, err := service.Register(context.Background(), "validuser", password, "")
		if !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("Register(password len=%d) expected ErrInvalidPassword, got %v", len(password), err)
		}
	}
	if repo.createCalls != 0 {
		t.Fatal("CreateUser should not be called for invalid password")
	}
}

func TestServiceRegisterDuplicateUsername(t *testing.T) {
	repo := &authTestRepository{createErr: ErrUsernameExists}
	service := NewService(repo, testJWTSecret(), time.Hour)

	_, err := service.Register(context.Background(), "existing", "password123", "")
	if !errors.Is(err, ErrUsernameExists) {
		t.Fatalf("expected ErrUsernameExists, got %v", err)
	}
}

func TestServiceLoginAcceptsNormalUser(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("userpass"), bcrypt.MinCost)
	repo := &authTestRepository{
		admin:  Admin{ID: 99, Username: "normaluser", PasswordHash: string(hash), UserType: UserTypeNormal, Status: 1},
		active: true,
	}
	service := NewService(repo, testJWTSecret(), time.Hour)

	result, err := service.Login(context.Background(), "normaluser", "userpass")
	if err != nil || result.Admin.ID != 99 || result.Admin.UserType != UserTypeNormal {
		t.Fatalf("Login() for normal user result=%+v err=%v", result, err)
	}
}

type authTestRepository struct {
	admin       Admin
	active      bool
	createdID   uint64
	createErr   error
	createCalls int
}

func (r *authTestRepository) FindActiveAdminByUsername(_ context.Context, username string) (Admin, bool, error) {
	return r.admin, r.active && username == r.admin.Username && r.admin.UserType == UserTypeAdmin, nil
}

func (r *authTestRepository) FindActiveAdminByID(_ context.Context, id uint64) (Admin, bool, error) {
	return r.admin, r.active && id == r.admin.ID && r.admin.UserType == UserTypeAdmin, nil
}

func (r *authTestRepository) FindActiveUserByUsername(_ context.Context, username string) (Admin, bool, error) {
	return r.admin, r.active && username == r.admin.Username, nil
}

func (r *authTestRepository) FindActiveUserByID(_ context.Context, id uint64) (Admin, bool, error) {
	return r.admin, r.active && id == r.admin.ID, nil
}

func (r *authTestRepository) CreateUser(_ context.Context, admin Admin) (uint64, error) {
	r.createCalls++
	if r.createErr != nil {
		return 0, r.createErr
	}
	admin.ID = r.createdID
	r.admin = admin
	r.active = true
	return r.createdID, nil
}
