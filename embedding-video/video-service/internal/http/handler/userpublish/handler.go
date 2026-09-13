package userpublish

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/userpublish"
	domainvideo "video-service/internal/domain/video"
	"video-service/internal/http/dto"
	httperrors "video-service/internal/http/errors"
	"video-service/internal/infrastructure/objectstorage"
	"video-service/internal/model"
)

const (
	videoMaxBytes    = 500 << 20 // 500MB
	videoObjectDir   = "raw/user-publish"
	hlsObjectDir     = "hls/user-publish"
	hlsMasterName    = "master.m3u8"
)

var allowedVideoExtensions = map[string]bool{
	".mp4":  true,
	".mov":  true,
	".avi":  true,
	".webm": true,
}

// Handler 用户发布视频 HTTP 处理器内层实现。
type Handler struct {
	service      *userpublish.Service
	store        *objectstorage.RustFS
	rawURLPrefix string
	hlsURLPrefix string
}

// New 创建用户发布视频处理器。
func New(service *userpublish.Service, store *objectstorage.RustFS, rawURLPrefix string) *Handler {
	hlsURLPrefix := strings.Replace(strings.TrimRight(rawURLPrefix, "/"), "/raw", "/hls", 1)
	return &Handler{
		service:      service,
		store:        store,
		rawURLPrefix: strings.TrimRight(rawURLPrefix, "/"),
		hlsURLPrefix: hlsURLPrefix,
	}
}

type publishVideoResponse struct {
	VideoID uint64 `json:"video_id"`
	Status  int16  `json:"status"`
}

type videoStatusResponse struct {
	ID       uint64 `json:"id"`
	Status   int16  `json:"status"`
	ErrorMsg string `json:"error_msg"`
}

type userVideoItem struct {
	ID              uint64 `json:"id"`
	UserID          uint64 `json:"author_id"`
	AuthorAvatarURL string `json:"author_avatar_url"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	CoverURL        string `json:"cover_url"`
	Duration        int    `json:"duration"`
	Status          int16  `json:"status"`
	ViewCount       int    `json:"view_count"`
	PlayURL         string `json:"play_url"`
	CreateTime      string `json:"create_time"`
}

type listUserVideosResponse struct {
	List     []userVideoItem `json:"list"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// PublishVideo 接收 multipart 视频文件，保存到对象存储并创建发布记录。
func (h *Handler) PublishVideo(c *gin.Context) {
	// TODO: replace with real user authentication
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	file, header, ok := readVideoFile(c)
	if !ok {
		return
	}
	defer file.Close()

	if !validateVideoFile(c, header) {
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		httperrors.Write(c, httperrors.InvalidArgument("title is required"))
		return
	}
	description := c.PostForm("description")

	timestamp := time.Now().UnixNano()
	safeName := sanitizeFilename(header.Filename)
	objectKey := fmt.Sprintf("%s/%d/%d_%s", videoObjectDir, userID, timestamp, safeName)

	if err := h.store.Put(c.Request.Context(), objectKey, file, header.Size, header.Header.Get("Content-Type")); err != nil {
		httperrors.Write(c, httperrors.Internal("upload video failed"))
		return
	}

	hlsObjectPrefix := fmt.Sprintf("%s/%d/%d", hlsObjectDir, userID, timestamp)
	hlsURL := fmt.Sprintf("%s/%s/%d/%d/%s", h.hlsURLPrefix, "user-publish", userID, timestamp, hlsMasterName)
	// VideoURL 直接存 HLS 播放地址（用户主页作品/喜欢列表可直接播放），原始文件由 RawKey 指向 raw 对象
	videoURL := hlsURL

	video, err := h.service.PublishVideo(
		c.Request.Context(),
		userID,
		title,
		description,
		videoURL,
		0, // duration will be probed by transcode worker
		objectKey,
		hlsObjectPrefix,
		hlsURL,
	)
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument(err.Error()))
		return
	}

	writeSuccess(c, publishVideoResponse{VideoID: video.ID, Status: video.Status})
}

// GetVideoStatus 查询当前用户视频的处理状态。
func (h *Handler) GetVideoStatus(c *gin.Context) {
	// TODO: replace with real user authentication
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	video, err := h.service.Repo.GetByID(c.Request.Context(), videoID)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("get video status failed"))
		return
	}
	if video == nil {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}
	if video.UserID != userID {
		httperrors.Write(c, &httperrors.APIError{
			Status:  http.StatusForbidden,
			Code:    "forbidden",
			Message: "you do not have permission to view this video",
		})
		return
	}

	writeSuccess(c, videoStatusResponse{
		ID:       video.ID,
		Status:   video.Status,
		ErrorMsg: video.ErrorMsg,
	})
}

// ListUserVideos 分页查询指定用户的已发布作品。
func (h *Handler) ListUserVideos(c *gin.Context) {
	userID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	page := parseIntQuery(c, "page", 1)
	pageSize := parseIntQuery(c, "page_size", 12)

	videos, total, err := h.service.ListUserVideos(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("list user videos failed"))
		return
	}

	list := make([]userVideoItem, 0, len(videos))
	avatarMap, _ := h.service.ListAvatarURLs(c.Request.Context(), collectUserIDs(videos))
	for _, v := range videos {
		list = append(list, userVideoItem{
			ID:              v.ID,
			UserID:          v.UserID,
			AuthorAvatarURL: avatarMap[v.UserID],
			Title:           v.Title,
			Description:     v.Description,
			CoverURL:        v.CoverURL,
			Duration:        v.Duration,
			Status:          v.Status,
			ViewCount:       v.ViewCount,
			PlayURL:         h.publishedPlayURL(v),
			CreateTime:      v.CreateTime.Format(time.RFC3339),
		})
	}

	writeSuccess(c, listUserVideosResponse{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// ListLikedVideos 分页查询指定用户点赞/双赞的视频。
func (h *Handler) ListLikedVideos(c *gin.Context) {
	userID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	page := parseIntQuery(c, "page", 1)
	pageSize := parseIntQuery(c, "page_size", 12)

	videos, total, err := h.service.ListLikedVideos(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("list liked videos failed"))
		return
	}

	list := make([]userVideoItem, 0, len(videos))
	avatarMap, _ := h.service.ListAvatarURLs(c.Request.Context(), collectUserIDs(videos))
	for _, v := range videos {
		list = append(list, userVideoItem{
			ID:              v.ID,
			UserID:          v.UserID,
			AuthorAvatarURL: avatarMap[v.UserID],
			Title:           v.Title,
			Description:     v.Description,
			CoverURL:        v.CoverURL,
			Duration:        v.Duration,
			Status:          v.Status,
			ViewCount:       v.ViewCount,
			PlayURL:         h.publishedPlayURL(v),
			CreateTime:      v.CreateTime.Format(time.RFC3339),
		})
	}

	writeSuccess(c, listUserVideosResponse{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// collectUserIDs 收集视频作者 ID 并去重。
func collectUserIDs(videos []model.EduVideoResource) []uint64 {
	seen := make(map[uint64]struct{}, len(videos))
	ids := make([]uint64, 0, len(videos))
	for _, v := range videos {
		if v.UserID == 0 {
			continue
		}
		if _, ok := seen[v.UserID]; ok {
			continue
		}
		seen[v.UserID] = struct{}{}
		ids = append(ids, v.UserID)
	}
	return ids
}

// publishedPlayURL 仅当视频已发布（转码完成）时返回可播放地址：
// 用户发布视频的 VideoURL 已直接存 HLS；知识视频（admin_import）的 VideoURL
// 是 raw mp4，按与 feed 一致的规则推导出对应 HLS 播放地址。
func (h *Handler) publishedPlayURL(v model.EduVideoResource) string {
	if v.Status != int16(domainvideo.StatusDone) {
		return ""
	}
	value := strings.TrimSpace(v.VideoURL)
	if value == "" {
		return ""
	}
	if strings.Contains(value, ".m3u8") {
		return value
	}
	rawPrefix := strings.TrimRight(h.rawURLPrefix, "/")
	prefix := strings.TrimRight(h.hlsURLPrefix, "/")
	if strings.HasPrefix(value, rawPrefix+"/") {
		value = strings.TrimPrefix(value, rawPrefix+"/")
	} else {
		value = strings.TrimPrefix(value, "/videos/")
		value = strings.TrimPrefix(value, "raw/")
	}
	value = strings.TrimPrefix(value, "/")
	parts := strings.Split(value, "/")
	if len(parts) < 4 {
		return ""
	}
	datePath := strings.Join(parts[:3], "/")
	fileName := parts[3]
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	return prefix + "/" + datePath + "/" + base + "/" + hlsMasterName
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

func readVideoFile(c *gin.Context) (multipart.File, *multipart.FileHeader, bool) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if err == http.ErrMissingFile || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			httperrors.Write(c, httperrors.InvalidArgument("video file is required"))
			return nil, nil, false
		}
		httperrors.Write(c, httperrors.InvalidArgument("invalid multipart form"))
		return nil, nil, false
	}
	if header == nil || strings.TrimSpace(header.Filename) == "" {
		_ = file.Close()
		httperrors.Write(c, httperrors.InvalidArgument("video file is required"))
		return nil, nil, false
	}
	return file, header, true
}

func validateVideoFile(c *gin.Context, header *multipart.FileHeader) bool {
	if header.Size > videoMaxBytes {
		httperrors.Write(c, httperrors.InvalidArgument("video size must be at most 500MB"))
		return false
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedVideoExtensions[ext] {
		httperrors.Write(c, httperrors.InvalidArgument("video must be mp4, mov, avi, or webm"))
		return false
	}
	return true
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "_")
	return name
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

func parseIntQuery(c *gin.Context, name string, defaultValue int) int {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultValue
	}
	return value
}

func writeSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.SuccessResponse[T]{Success: true, Data: data})
}
