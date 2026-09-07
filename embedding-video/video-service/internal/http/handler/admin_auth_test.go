package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/adminauth"
	"video-service/middleware"
)

func TestAdminAuthHandlerLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAdminLoginService{result: adminauth.LoginResult{
		Token: "token-1", ExpiresAt: time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC),
		Admin: adminauth.Admin{ID: 7, Username: "admin", RealName: "System Admin", PasswordHash: "must-not-leak"},
	}}
	router := gin.New()
	router.POST("/api/auth/login", NewAdminAuthHandler(service).Login)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"access_token":"token-1"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "must-not-leak") {
		t.Fatalf("response leaked password hash: %s", recorder.Body.String())
	}
}

func TestAdminAuthHandlerRejectsInvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/auth/login", NewAdminAuthHandler(&stubAdminLoginService{err: adminauth.ErrInvalidCredentials}).Login)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "invalid username or password") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminAuthHandlerMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/auth/me", func(c *gin.Context) {
		c.Set(middleware.AdminContextKey, adminauth.Admin{ID: 7, Username: "admin", RealName: "System Admin"})
	}, NewAdminAuthHandler(&stubAdminLoginService{}).Me)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"username":"admin"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminAuthHandlerRegisterSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &stubAdminLoginService{result: adminauth.LoginResult{
		Token: "new-token", ExpiresAt: time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC),
		Admin: adminauth.Admin{ID: 42, Username: "newuser", RealName: "New User", UserType: 2},
	}}
	router := gin.New()
	router.POST("/api/auth/register", NewAdminAuthHandler(service).Register)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"username":"newuser","password":"secret123","nickname":"New User"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"access_token":"new-token"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"user_type":2`) {
		t.Fatalf("response missing user_type: %s", recorder.Body.String())
	}
}

func TestAdminAuthHandlerRegisterUsernameExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/auth/register", NewAdminAuthHandler(&stubAdminLoginService{err: adminauth.ErrUsernameExists}).Register)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"username":"existing","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "username_exists") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminAuthHandlerRegisterInvalidUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/auth/register", NewAdminAuthHandler(&stubAdminLoginService{err: adminauth.ErrInvalidUsername}).Register)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"username":"ab","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "invalid_username") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAdminAuthHandlerRegisterInvalidPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/auth/register", NewAdminAuthHandler(&stubAdminLoginService{err: adminauth.ErrInvalidPassword}).Register)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(`{"username":"validuser","password":"short"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "invalid_password") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

type stubAdminLoginService struct {
	result adminauth.LoginResult
	err    error
}

func (s *stubAdminLoginService) Login(context.Context, string, string) (adminauth.LoginResult, error) {
	return s.result, s.err
}

func (s *stubAdminLoginService) Register(context.Context, string, string, string) (adminauth.LoginResult, error) {
	return s.result, s.err
}
