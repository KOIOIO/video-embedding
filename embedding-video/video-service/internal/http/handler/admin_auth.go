package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/adminauth"
	"video-service/internal/http/dto"
	"video-service/middleware"
)

type adminLoginService interface {
	Login(ctx context.Context, username, password string) (adminauth.LoginResult, error)
}

type AdminAuthHandler struct {
	service adminLoginService
}

func NewAdminAuthHandler(service adminLoginService) *AdminAuthHandler {
	return &AdminAuthHandler{service: service}
}

type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Login godoc
// @Summary 管理员登录
// @Tags 管理员认证
// @Accept json
// @Produce json
// @Param request body AdminLoginRequest true "管理员凭据"
// @Success 200 {object} adminauth.LoginResult
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Router /api/auth/login [post]
func (h *AdminAuthHandler) Login(c *gin.Context) {
	var request AdminLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeAuthError(c, http.StatusBadRequest, "invalid_argument", "username and password are required")
		return
	}
	result, err := h.service.Login(c.Request.Context(), strings.TrimSpace(request.Username), request.Password)
	if errors.Is(err, adminauth.ErrInvalidCredentials) {
		writeAuthError(c, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}
	if err != nil {
		writeAuthError(c, http.StatusInternalServerError, "internal", "login failed")
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse[adminauth.LoginResult]{Success: true, Data: result})
}

// Me godoc
// @Summary 查询当前管理员
// @Tags 管理员认证
// @Produce json
// @Success 200 {object} adminauth.Admin
// @Failure 401 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/auth/me [get]
func (h *AdminAuthHandler) Me(c *gin.Context) {
	admin, ok := middleware.CurrentAdmin(c)
	if !ok {
		writeAuthError(c, http.StatusUnauthorized, "unauthorized", "administrator authentication required")
		return
	}
	c.JSON(http.StatusOK, dto.SuccessResponse[adminauth.Admin]{Success: true, Data: admin})
}

func writeAuthError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, dto.ErrorResponse{Success: false, Error: dto.ErrorBody{Code: code, Message: message}})
}
