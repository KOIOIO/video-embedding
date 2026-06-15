package videos

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/application/videoapp"
	domainvideo "nlp-video-analysis/internal/domain/video"
	"nlp-video-analysis/internal/http/dto"
	httperrors "nlp-video-analysis/internal/http/errors"
)

type Handler struct {
	app videoApp
}

type videoApp interface {
	ListVideos(ctx context.Context, filter videoapp.ListFilter) ([]domainvideo.Video, error)
	UpdateVideoMetadata(ctx context.Context, videoID uint64, title string, description string) (bool, error)
	DeleteVideo(ctx context.Context, videoID uint64, operator string) (bool, error)
	PlayVideo(ctx context.Context, videoID uint64) (string, domainvideo.Video, bool, error)
	ResolvePlaybackURL(ctx context.Context, video domainvideo.Video) string
	GetSimilarVideos(ctx context.Context, videoID uint64, limit int) ([]domainvideo.Video, error)
	GetViewCount(ctx context.Context, videoID uint64) (int64, bool, error)
	SetVideoPublished(ctx context.Context, videoID uint64, isPublished bool) (bool, error)
	SetVideoRecommend(ctx context.Context, videoID uint64, isRecommend bool, userID uint64, recommendLevel int16, recommendScore float64) (bool, error)
	SubmitVideoReaction(ctx context.Context, videoID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error)
	GetVideoReactionCounts(ctx context.Context, videoID uint64) (videoapp.VideoReactionCounts, bool, error)
	SubmitSegmentReaction(ctx context.Context, segmentID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, bool, error)
	GetSegmentReactionCounts(ctx context.Context, segmentID uint64) (videoapp.VideoReactionCounts, bool, error)
	RandomPlayVideoSegment(ctx context.Context) (videoapp.RecommendResultItem, bool, error)
	GetTranscodeStatus(ctx context.Context, taskID string) (videoapp.TranscodeStatus, bool, error)
}

func New(app any) *Handler {
	switch v := app.(type) {
	case videoApp:
		return &Handler{app: v}
	default:
		panic("unsupported video app")
	}
}

func (h *Handler) ListVideos(c *gin.Context) {
	filterType := strings.ToUpper(strings.TrimSpace(c.DefaultQuery("type", "ALL")))
	filter := videoapp.ListAll
	switch filterType {
	case "ALL":
	case "RAW":
		filter = videoapp.ListRawOnly
	case "HLS":
		filter = videoapp.ListHLSOnly
	default:
		httperrors.Write(c, httperrors.InvalidArgument("type must be one of ALL, RAW, HLS"))
		return
	}

	videos, err := h.app.ListVideos(c.Request.Context(), filter)
	if err != nil {
		httperrors.Write(c, httperrors.Internal("list videos failed"))
		return
	}

	writeSuccess(c, dto.VideoListData{
		Videos: h.mapVideos(c.Request.Context(), videos),
		Total:  len(videos),
		Type:   filterType,
	})
}

func (h *Handler) UpdateVideoMetadata(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	var req dto.UpdateVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}

	normalizedTitle := strings.TrimSpace(req.Title)
	updated, err := h.app.UpdateVideoMetadata(c.Request.Context(), videoID, normalizedTitle, req.Description)
	if err != nil {
		writeAppError(c, err, "update video metadata failed")
		return
	}
	if !updated {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.UpdateVideoData{VideoID: videoID, Title: normalizedTitle, Description: req.Description, Updated: true})
}

func (h *Handler) DeleteVideo(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	deleted, err := h.app.DeleteVideo(c.Request.Context(), videoID, "")
	if err != nil {
		writeAppError(c, err, "delete video failed")
		return
	}
	if !deleted {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.DeleteVideoData{VideoID: videoID, Deleted: true})
}

func (h *Handler) PlayVideo(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	playURL, video, found, err := h.app.PlayVideo(c.Request.Context(), videoID)
	if err != nil {
		writeAppError(c, err, "play video failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.PlayVideoData{PlayURL: playURL, Video: mapVideoForPlayback(video, playURL)})
}

func (h *Handler) GetSimilarVideos(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	limit := 6
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			httperrors.Write(c, httperrors.InvalidArgument("limit must be a positive integer"))
			return
		}
		limit = parsed
	}

	videos, err := h.app.GetSimilarVideos(c.Request.Context(), videoID, limit)
	if err != nil {
		writeAppError(c, err, "get similar videos failed")
		return
	}

	writeSuccess(c, dto.SimilarVideosData{Videos: h.mapVideos(c.Request.Context(), videos), Total: len(videos)})
}

func (h *Handler) GetViewCount(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	viewCount, found, err := h.app.GetViewCount(c.Request.Context(), videoID)
	if err != nil {
		writeAppError(c, err, "get view count failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.ViewCountData{VideoID: videoID, ViewCount: viewCount})
}

func (h *Handler) SetVideoPublished(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	var req dto.PublishVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}
	if req.IsPublished == nil {
		httperrors.Write(c, httperrors.InvalidArgument("is_published is required"))
		return
	}

	updated, err := h.app.SetVideoPublished(c.Request.Context(), videoID, *req.IsPublished)
	if err != nil {
		writeAppError(c, err, "set video published failed")
		return
	}
	if !updated {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.PublishVideoData{VideoID: videoID, IsPublished: *req.IsPublished, Updated: true})
}

func (h *Handler) SetVideoRecommend(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	var req dto.RecommendVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}
	if err := validateRecommendRequest(req); err != nil {
		writeAppError(c, err, "set video recommend failed")
		return
	}

	updated, err := h.app.SetVideoRecommend(c.Request.Context(), videoID, *req.IsRecommend, req.UserID, req.RecommendLevel, req.RecommendScore)
	if err != nil {
		writeAppError(c, err, "set video recommend failed")
		return
	}
	if !updated {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.RecommendVideoData{VideoID: videoID, IsRecommend: *req.IsRecommend, Updated: true})
}

func (h *Handler) SubmitVideoReaction(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	var req dto.VideoReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}
	reactionType, err := validateVideoReactionRequest(req)
	if err != nil {
		writeAppError(c, err, "submit video reaction failed")
		return
	}

	result, found, err := h.app.SubmitVideoReaction(c.Request.Context(), videoID, req.UserID, reactionType)
	if err != nil {
		writeAppError(c, err, "submit video reaction failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.VideoReactionData{
		VideoID:         videoID,
		UserID:          req.UserID,
		ReactionType:    string(reactionType),
		Active:          result.Active,
		LikeCount:       result.Counts.LikeCount,
		DoubleLikeCount: result.Counts.DoubleLikeCount,
		Updated:         true,
	})
}

func (h *Handler) GetVideoReactionCounts(c *gin.Context) {
	videoID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	counts, found, err := h.app.GetVideoReactionCounts(c.Request.Context(), videoID)
	if err != nil {
		writeAppError(c, err, "get reaction counts failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("video_not_found", "video does not exist"))
		return
	}

	writeSuccess(c, dto.VideoReactionCountsData{
		VideoID:         videoID,
		LikeCount:       counts.LikeCount,
		DoubleLikeCount: counts.DoubleLikeCount,
	})
}

func (h *Handler) SubmitSegmentReaction(c *gin.Context) {
	segmentID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	var req dto.VideoReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}
	reactionType, err := validateVideoReactionRequest(req)
	if err != nil {
		writeAppError(c, err, "submit segment reaction failed")
		return
	}

	result, found, err := h.app.SubmitSegmentReaction(c.Request.Context(), segmentID, req.UserID, reactionType)
	if err != nil {
		writeAppError(c, err, "submit segment reaction failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("segment_not_found", "segment does not exist"))
		return
	}

	writeSuccess(c, dto.SegmentReactionData{
		SegmentID:       segmentID,
		UserID:          req.UserID,
		ReactionType:    string(reactionType),
		Active:          result.Active,
		LikeCount:       result.Counts.LikeCount,
		DoubleLikeCount: result.Counts.DoubleLikeCount,
		Updated:         true,
	})
}

func (h *Handler) GetSegmentReactionCounts(c *gin.Context) {
	segmentID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	counts, found, err := h.app.GetSegmentReactionCounts(c.Request.Context(), segmentID)
	if err != nil {
		writeAppError(c, err, "get segment reaction counts failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("segment_not_found", "segment does not exist"))
		return
	}

	writeSuccess(c, dto.SegmentReactionCountsData{
		SegmentID:       segmentID,
		LikeCount:       counts.LikeCount,
		DoubleLikeCount: counts.DoubleLikeCount,
	})
}

func (h *Handler) RandomPlayVideoSegment(c *gin.Context) {
	item, found, err := h.app.RandomPlayVideoSegment(c.Request.Context())
	if err != nil {
		writeAppError(c, err, "random play video segment failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("video_segment_not_found", "video segment does not exist"))
		return
	}

	title := item.Video.Title
	if item.TitleOverride != "" {
		title = item.TitleOverride
	}
	writeSuccess(c, dto.RandomVideoSegmentData{
		VideoID:        item.VideoID,
		VideoSegmentID: item.VideoSegmentID,
		StartTimeSec:   item.StartTimeSec,
		EndTimeSec:     item.EndTimeSec,
		Title:          title,
		CoverURL:       item.Video.CoverURL,
		PlayURL:        strings.TrimSpace(h.app.ResolvePlaybackURL(c.Request.Context(), item.Video)),
	})
}

func (h *Handler) GetTranscodeStatus(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("taskId"))
	if taskID == "" {
		httperrors.Write(c, httperrors.InvalidArgument("task_id is required"))
		return
	}

	status, found, err := h.app.GetTranscodeStatus(c.Request.Context(), taskID)
	if err != nil {
		writeAppError(c, err, "get transcode status failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("task_not_found", "transcode task does not exist"))
		return
	}

	writeSuccess(c, dto.TranscodeStatusData{TaskID: taskID, Status: statusToString(status.Status), HLSURL: status.HLSURL})
}

func writeSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.SuccessResponse[T]{Success: true, Data: data})
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

func (h *Handler) mapVideos(ctx context.Context, videos []domainvideo.Video) []dto.VideoItem {
	out := make([]dto.VideoItem, 0, len(videos))
	for _, video := range videos {
		out = append(out, h.mapVideo(ctx, video))
	}
	return out
}

func (h *Handler) mapVideo(ctx context.Context, video domainvideo.Video) dto.VideoItem {
	return mapVideoWithResolvedURL(video, strings.TrimSpace(h.app.ResolvePlaybackURL(ctx, video)))
}

func mapVideoForPlayback(video domainvideo.Video, playURL string) dto.VideoItem {
	return mapVideoWithResolvedURL(video, strings.TrimSpace(playURL))
}

func mapVideoWithResolvedURL(video domainvideo.Video, resolvedPlaybackURL string) dto.VideoItem {
	hlsURL := ""
	if video.Status == domainvideo.StatusDone {
		hlsURL = resolvedPlaybackURL
	}
	rawURL := video.VideoURL
	if video.Status == domainvideo.StatusDone {
		rawURL = ""
	}
	return dto.VideoItem{
		VideoID:       video.ID,
		Title:         video.Title,
		Description:   video.Description,
		RawURL:        rawURL,
		HLSURL:        hlsURL,
		IsPublished:   video.IsPublished,
		IsRecommend:   video.IsRecommend,
		ViewCount:     int64(video.ViewCount),
		CoverURL:      video.CoverURL,
		CreatedAtUnix: video.CreateTime.Unix(),
		UpdatedAtUnix: video.UpdateTime.Unix(),
	}
}

func writeAppError(c *gin.Context, err error, fallback string) {
	if err == nil {
		httperrors.Write(c, httperrors.Internal(fallback))
		return
	}
	var validationErr videoapp.ValidationError
	if errors.As(err, &validationErr) {
		httperrors.Write(c, httperrors.InvalidArgument(err.Error()))
		return
	}
	httperrors.Write(c, httperrors.Internal(fallback))
}

func statusToString(status domainvideo.Status) string {
	switch status {
	case domainvideo.StatusUploaded:
		return "UPLOADED"
	case domainvideo.StatusProcessing:
		return "PROCESSING"
	case domainvideo.StatusDone:
		return "DONE"
	case domainvideo.StatusFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

func validateRecommendRequest(req dto.RecommendVideoRequest) error {
	if req.IsRecommend == nil {
		return videoapp.InvalidArgumentError("is_recommend is required")
	}
	metadataUsed := req.UserID != 0 || req.RecommendLevel != 0 || req.RecommendScore != 0
	if metadataUsed && req.UserID == 0 {
		return videoapp.InvalidArgumentError("user_id must be a positive integer when recommend metadata is provided")
	}
	if req.RecommendLevel < 0 {
		return videoapp.InvalidArgumentError("recommend_level must be greater than 0")
	}
	if req.RecommendLevel == 0 && metadataUsed {
		return videoapp.InvalidArgumentError("recommend_level must be greater than 0")
	}
	if req.RecommendScore < 0 {
		return videoapp.InvalidArgumentError("recommend_score must be greater than or equal to 0")
	}
	return nil
}

func validateVideoReactionRequest(req dto.VideoReactionRequest) (videoapp.VideoReactionType, error) {
	if req.UserID == 0 {
		return "", videoapp.InvalidArgumentError("user_id is required")
	}
	reactionType := videoapp.VideoReactionType(strings.TrimSpace(req.ReactionType))
	if !reactionType.IsValid() {
		return "", videoapp.InvalidArgumentError("reaction_type must be one of like, double_like, dislike")
	}
	return reactionType, nil
}
