package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/adminauth"
)

func TestRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authenticator := &stubAdminAuthenticator{admin: adminauth.Admin{ID: 7, Username: "admin"}}
	router := gin.New()
	router.GET("/admin", RequireAdmin(authenticator), func(c *gin.Context) {
		id, ok := AdminID(c)
		if !ok || id != 7 {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	for _, tc := range []struct {
		name, authorization string
		want                int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"malformed", "Token abc", http.StatusUnauthorized},
		{"valid", "Bearer good-token", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			req.Header.Set("Authorization", tc.authorization)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != tc.want {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

type stubAdminAuthenticator struct {
	admin adminauth.Admin
}

func (s *stubAdminAuthenticator) Authenticate(_ context.Context, token string) (adminauth.Admin, error) {
	if token != "good-token" {
		return adminauth.Admin{}, adminauth.ErrInvalidToken
	}
	return s.admin, nil
}
