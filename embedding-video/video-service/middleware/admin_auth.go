package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/adminauth"
	"video-service/internal/http/dto"
)

const AdminContextKey = "authenticated_admin"

type AdminAuthenticator interface {
	Authenticate(ctx context.Context, token string) (adminauth.Admin, error)
}

func RequireAdmin(authenticator AdminAuthenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || authenticator == nil {
			writeUnauthorized(c)
			return
		}
		admin, err := authenticator.Authenticate(c.Request.Context(), parts[1])
		if err != nil {
			writeUnauthorized(c)
			return
		}
		c.Set(AdminContextKey, admin)
		c.Next()
	}
}

func AdminID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(AdminContextKey)
	if !ok {
		return 0, false
	}
	admin, ok := value.(adminauth.Admin)
	return admin.ID, ok && admin.ID > 0
}

func CurrentAdmin(c *gin.Context) (adminauth.Admin, bool) {
	value, ok := c.Get(AdminContextKey)
	if !ok {
		return adminauth.Admin{}, false
	}
	admin, ok := value.(adminauth.Admin)
	return admin, ok && admin.ID > 0
}

// RequireUser authenticates any active user (normal or admin) via Bearer token.
// TODO: distinguish admin-only routes by checking UserType == adminauth.UserTypeAdmin.
func RequireUser(authenticator AdminAuthenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || authenticator == nil {
			writeUserUnauthorized(c)
			return
		}
		admin, err := authenticator.Authenticate(c.Request.Context(), parts[1])
		if err != nil {
			writeUserUnauthorized(c)
			return
		}
		c.Set(AdminContextKey, admin)
		c.Next()
	}
}

func UserID(c *gin.Context) (uint64, bool) {
	return AdminID(c)
}

func CurrentUser(c *gin.Context) (adminauth.Admin, bool) {
	return CurrentAdmin(c)
}

func writeUserUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
		Success: false,
		Error:   dto.ErrorBody{Code: "unauthorized", Message: "user authentication required"},
	})
}

func writeUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, dto.ErrorResponse{
		Success: false,
		Error:   dto.ErrorBody{Code: "unauthorized", Message: "administrator authentication required"},
	})
}
