package profilevisits

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/profilevisit"
	"video-service/internal/http/dto"
	httperrors "video-service/internal/http/errors"
)

// Handler 用户主页访问 HTTP 处理器内层实现。
type Handler struct {
	service *profilevisit.Service
}

// New 创建用户主页访问处理器。
func New(service *profilevisit.Service) *Handler {
	return &Handler{service: service}
}

// RecordVisit godoc
// @Summary 记录主页访问
// @Tags 主页访问
// @Produce json
// @Param X-User-ID header string false "访问者用户ID（匿名可不传，TODO: replace with real user authentication）"
// @Param id path int true "被访问者用户ID"
// @Success 200 {object} dto.SuccessResponse[map[string]bool]
// @Router /api/users/{id}/visit [post]
func (h *Handler) RecordVisit(c *gin.Context) {
	// TODO: replace with real user authentication
	visitorID := optionalUserID(c)
	ownerID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	// fire and forget：即使记录失败也不影响前端，始终返回成功
	_ = h.service.RecordVisit(c.Request.Context(), visitorID, ownerID)
	writeSuccess(c, gin.H{"success": true})
}

// GetMyVisits godoc
// @Summary 获取我的主页访问统计
// @Tags 主页访问
// @Produce json
// @Param X-User-ID header string true "用户ID（TODO: replace with real user authentication）"
// @Param date query string false "日期 YYYY-MM-DD，默认今天"
// @Success 200 {object} dto.SuccessResponse[map[string]interface{}]
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/me/visits [get]
func (h *Handler) GetMyVisits(c *gin.Context) {
	// TODO: replace with real user authentication
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	dateStr := strings.TrimSpace(c.Query("date"))
	uniqueVisitors, totalVisits, err := h.service.GetDailyStats(c.Request.Context(), userID, dateStr)
	if err != nil {
		switch err {
		case profilevisit.ErrInvalidDate:
			httperrors.Write(c, httperrors.InvalidArgument("invalid date format, expected YYYY-MM-DD"))
		default:
			httperrors.Write(c, httperrors.Internal("get visit stats failed"))
		}
		return
	}
	if dateStr == "" {
		now := time.Now()
		dateStr = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	writeSuccess(c, gin.H{
		"date":            dateStr,
		"unique_visitors": uniqueVisitors,
		"total_visits":    totalVisits,
	})
}

// --- helpers ---

// optionalUserID 从 X-User-ID 读取访问者 ID，缺失或无效时返回 0（匿名）。
func optionalUserID(c *gin.Context) uint64 {
	raw := strings.TrimSpace(c.GetHeader("X-User-ID"))
	if raw == "" {
		return 0
	}
	userID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return userID
}

func currentUserID(c *gin.Context) (uint64, bool) {
	raw := strings.TrimSpace(c.GetHeader("X-User-ID"))
	if raw == "" {
		httperrors.Write(c, &httperrors.APIError{
			Status:  http.StatusUnauthorized,
			Code:    "unauthorized",
			Message: "X-User-ID header is required",
		})
		return 0, false
	}
	userID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || userID == 0 {
		httperrors.Write(c, httperrors.InvalidArgument("X-User-ID must be a positive integer"))
		return 0, false
	}
	return userID, true
}

func parsePositiveUintParam(c *gin.Context, name string) (uint64, bool) {
	raw := strings.TrimSpace(c.Param(name))
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		httperrors.Write(c, httperrors.InvalidArgument(name+" must be a positive integer"))
		return 0, false
	}
	return value, true
}

func writeSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.SuccessResponse[T]{Success: true, Data: data})
}
