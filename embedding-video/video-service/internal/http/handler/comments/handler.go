package comments

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"video-service/internal/application/videoapp"
	"video-service/internal/http/dto"
	httperrors "video-service/internal/http/errors"
	"video-service/middleware"
)

type Handler struct {
	app commentApp
}

type commentApp interface {
	CreateComment(ctx context.Context, segmentID uint64, userID uint64, content string) (videoapp.CommentView, error)
	CreateReply(ctx context.Context, commentID uint64, userID uint64, content string) (videoapp.CommentView, error)
	ListSegmentComments(ctx context.Context, segmentID uint64, viewerID uint64, page int, pageSize int) (videoapp.CommentListView, error)
	ListCommentReplies(ctx context.Context, commentID uint64, viewerID uint64, page int, pageSize int) (videoapp.CommentListView, error)
	ToggleCommentLike(ctx context.Context, commentID uint64, userID uint64, reactionType videoapp.VideoReactionType) (videoapp.VideoReactionResult, error)
	GetSegmentCommentCount(ctx context.Context, segmentID uint64) (int64, error)
}

func New(app any) *Handler {
	switch v := app.(type) {
	case commentApp:
		return &Handler{app: v}
	default:
		panic("unsupported comment app")
	}
}

func (h *Handler) ListComments(c *gin.Context) {
	segmentID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	viewerID, ok := parseOptionalPositiveUintQuery(c, "user_id")
	if !ok {
		return
	}
	page, ok := parsePositiveIntQuery(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := parsePositiveIntQuery(c, "page_size", videoapp.DefaultCommentPageSize)
	if !ok {
		return
	}
	if pageSize > videoapp.MaxCommentPageSize {
		pageSize = videoapp.MaxCommentPageSize
	}

	list, err := h.app.ListSegmentComments(c.Request.Context(), segmentID, viewerID, page, pageSize)
	if err != nil {
		writeCommentError(c, err, "list comments failed")
		return
	}
	writeSuccess(c, dto.CommentListData{
		Total:    list.Total,
		Page:     page,
		PageSize: pageSize,
		Comments: mapCommentViews(list.Comments),
	})
}

func (h *Handler) CreateComment(c *gin.Context) {
	segmentID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}

	comment, err := h.app.CreateComment(c.Request.Context(), segmentID, userID, req.Content)
	if err != nil {
		writeCommentError(c, err, "create comment failed")
		return
	}
	writeSuccess(c, mapCommentView(comment))
}

func (h *Handler) GetCommentCounts(c *gin.Context) {
	segmentID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	count, err := h.app.GetSegmentCommentCount(c.Request.Context(), segmentID)
	if err != nil {
		writeCommentError(c, err, "get comment counts failed")
		return
	}
	writeSuccess(c, dto.SegmentCommentCountData{VideoSegmentID: segmentID, Total: count})
}

func (h *Handler) ListReplies(c *gin.Context) {
	commentID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	viewerID, ok := parseOptionalPositiveUintQuery(c, "user_id")
	if !ok {
		return
	}
	page, ok := parsePositiveIntQuery(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := parsePositiveIntQuery(c, "page_size", videoapp.DefaultReplyPageSize)
	if !ok {
		return
	}
	if pageSize > videoapp.MaxReplyPageSize {
		pageSize = videoapp.MaxReplyPageSize
	}

	list, err := h.app.ListCommentReplies(c.Request.Context(), commentID, viewerID, page, pageSize)
	if err != nil {
		writeCommentError(c, err, "list replies failed")
		return
	}
	writeSuccess(c, dto.CommentListData{
		Total:    list.Total,
		Page:     page,
		PageSize: pageSize,
		Comments: mapCommentViews(list.Comments),
	})
}

func (h *Handler) CreateReply(c *gin.Context) {
	commentID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}

	reply, err := h.app.CreateReply(c.Request.Context(), commentID, userID, req.Content)
	if err != nil {
		writeCommentError(c, err, "create reply failed")
		return
	}
	writeSuccess(c, mapCommentView(reply))
}

func (h *Handler) ToggleCommentLike(c *gin.Context) {
	commentID, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var req dto.CommentReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}
	reactionType := videoapp.VideoReactionType(strings.TrimSpace(req.ReactionType))
	if !reactionType.IsValid() {
		httperrors.Write(c, httperrors.InvalidArgument("reaction_type must be one of like, double_like, dislike"))
		return
	}
	result, err := h.app.ToggleCommentLike(c.Request.Context(), commentID, userID, reactionType)
	if err != nil {
		writeCommentError(c, err, "toggle comment reaction failed")
		return
	}
	writeSuccess(c, dto.CommentLikeData{
		CommentID:       commentID,
		Active:          result.Active,
		ReactionType:    string(result.ReactionType),
		LikeCount:       result.Counts.LikeCount,
		DoubleLikeCount: result.Counts.DoubleLikeCount,
	})
}

func writeCommentError(c *gin.Context, err error, fallback string) {
	if err == nil {
		httperrors.Write(c, httperrors.Internal(fallback))
		return
	}
	if errors.Is(err, videoapp.ErrSegmentNotFound) {
		httperrors.Write(c, httperrors.NotFound("video_segment_not_found", "video segment does not exist"))
		return
	}
	if errors.Is(err, videoapp.ErrCommentNotFound) {
		httperrors.Write(c, httperrors.NotFound("comment_not_found", "comment does not exist"))
		return
	}
	var validationErr videoapp.ValidationError
	if errors.As(err, &validationErr) {
		httperrors.Write(c, httperrors.InvalidArgument(err.Error()))
		return
	}
	httperrors.Write(c, httperrors.Internal(fallback))
}

func currentUserID(c *gin.Context) (uint64, bool) {
	userID, ok := middleware.AdminID(c)
	if !ok {
		httperrors.Write(c, httperrors.InvalidArgument("user_id is required"))
		return 0, false
	}
	return userID, true
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

func parseOptionalPositiveUintQuery(c *gin.Context, name string) (uint64, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, true
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		httperrors.Write(c, httperrors.InvalidArgument(name+" must be a positive integer"))
		return 0, false
	}
	return value, true
}

func parsePositiveIntQuery(c *gin.Context, name string, fallback int) (int, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		httperrors.Write(c, httperrors.InvalidArgument(name+" must be a positive integer"))
		return 0, false
	}
	return value, true
}

func mapCommentViews(views []videoapp.CommentView) []dto.CommentData {
	out := make([]dto.CommentData, 0, len(views))
	for _, view := range views {
		out = append(out, mapCommentView(view))
	}
	return out
}

func mapCommentView(view videoapp.CommentView) dto.CommentData {
	data := dto.CommentData{
		ID:               view.ID,
		UserID:           view.UserID,
		Username:         view.Username,
		Nickname:         view.Nickname,
		AvatarURL:        view.AvatarURL,
		ReplyToUsername:  view.ReplyToUsername,
		Content:          view.Content,
		LikeCount:        view.LikeCount,
		DoubleLikeCount:  view.DoubleLikeCount,
		UserReactionType: string(view.UserReactionType),
		CreatedAtUnix:    view.CreatedAt.Unix(),
		ReplyCount:       view.ReplyCount,
		HasMoreReplies:   view.HasMoreReplies,
	}
	if len(view.Mentions) > 0 {
		data.Mentions = make([]dto.MentionData, 0, len(view.Mentions))
		for _, m := range view.Mentions {
			data.Mentions = append(data.Mentions, dto.MentionData{Nickname: m.Nickname, UserID: m.UserID})
		}
	}
	if len(view.Replies) > 0 {
		data.Replies = mapCommentViews(view.Replies)
	}
	return data
}
