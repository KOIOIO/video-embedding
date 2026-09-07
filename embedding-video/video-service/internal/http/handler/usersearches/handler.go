package usersearches

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/usersearch"
	"video-service/internal/http/dto"
	httperrors "video-service/internal/http/errors"
	"video-service/middleware"
)

// Handler 用户搜索 HTTP 处理器内层实现。
type Handler struct {
	service *usersearch.Service
}

// New 创建用户搜索处理器。
func New(service *usersearch.Service) *Handler {
	return &Handler{service: service}
}

// SearchUsers godoc
// @Summary 搜索用户（评论 @提及用）
// @Description 按昵称或用户名模糊搜索用户，已关注用户优先排序，不返回当前用户自己。需要 JWT 认证。
// @Tags 用户搜索
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param q query string true "搜索关键词"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量（上限50）" default(20)
// @Success 200 {object} dto.SuccessResponse[map[string]interface{}]
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/users/search [get]
func (h *Handler) SearchUsers(c *gin.Context) {
	currentUserID, ok := middleware.UserID(c)
	if !ok {
		httperrors.Write(c, &httperrors.APIError{
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "user authentication required",
		})
		return
	}

	keyword := strings.TrimSpace(c.Query("q"))
	if keyword == "" {
		httperrors.Write(c, httperrors.InvalidArgument("q is required"))
		return
	}

	page, pageSize := parsePagination(c)

	list, total, err := h.service.Search(c.Request.Context(), currentUserID, keyword, page, pageSize)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("search users failed"))
		return
	}

	writeSuccess(c, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func parsePagination(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	if v := strings.TrimSpace(c.Query("page")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := strings.TrimSpace(c.Query("page_size")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > 50 {
		pageSize = 50
	}
	return page, pageSize
}

func writeSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.SuccessResponse[T]{Success: true, Data: data})
}
