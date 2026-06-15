package recommendations

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
	app recommendApp
}

type recommendApp interface {
	RecommendByQuestion(ctx context.Context, input videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error)
	ListRecommendations(ctx context.Context, input videoapp.ListRecommendationsInput) ([]videoapp.RecommendResultItem, error)
	ReportWatch(ctx context.Context, input videoapp.ReportWatchInput) error
	ResolvePlaybackURL(ctx context.Context, video domainvideo.Video) string
}

func New(app any) *Handler {
	switch v := app.(type) {
	case recommendApp:
		return &Handler{app: v}
	default:
		panic("unsupported recommend app")
	}
}

func (h *Handler) RecommendByQuestion(c *gin.Context) {
	var req dto.RecommendByQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}

	questionText := strings.TrimSpace(req.QuestionText)
	if req.QuestionID == 0 && questionText == "" {
		httperrors.Write(c, httperrors.InvalidArgument("question_text is required when question_id is absent"))
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 3
	}

	items, err := h.app.RecommendByQuestion(c.Request.Context(), videoapp.RecommendByQuestionInput{
		QuestionID:   req.QuestionID,
		QuestionText: questionText,
		UserID:       req.UserID,
		Limit:        limit,
	})
	if err != nil {
		var degradedErr videoapp.DegradedError
		if errors.As(err, &degradedErr) {
			writeSuccess(c, dto.RecommendationListData{
				Items:    h.mapRecommendationItems(c.Request.Context(), degradedErr.Items),
				Total:    len(degradedErr.Items),
				Degraded: true,
				Message:  "AI provider temporarily unavailable; returned fallback result",
			})
			return
		}
		writeRecommendError(c, err, "recommend by question failed")
		return
	}

	writeSuccess(c, dto.RecommendationListData{Items: h.mapRecommendationItems(c.Request.Context(), items), Total: len(items)})
}

func (h *Handler) ListRecommendations(c *gin.Context) {
	questionID, err := parseOptionalUintQuery(c, "question_id")
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("question_id must be a positive integer"))
		return
	}
	if questionID == 0 {
		httperrors.Write(c, httperrors.InvalidArgument("question_id is required"))
		return
	}

	userID, err := parseOptionalUintQuery(c, "user_id")
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("user_id must be a positive integer"))
		return
	}
	limit, err := parseOptionalIntQuery(c, "limit")
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("limit must be an integer"))
		return
	}

	items, err := h.app.ListRecommendations(c.Request.Context(), videoapp.ListRecommendationsInput{
		QuestionID: questionID,
		UserID:     userID,
		Limit:      limit,
	})
	if err != nil {
		writeRecommendError(c, err, "list recommendations failed")
		return
	}

	writeSuccess(c, dto.RecommendationListData{Items: h.mapRecommendationItems(c.Request.Context(), items), Total: len(items)})
}

func (h *Handler) ReportWatch(c *gin.Context) {
	var req dto.ReportWatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("invalid request body"))
		return
	}
	if req.VideoSegmentID == 0 {
		httperrors.Write(c, httperrors.InvalidArgument("video_segment_id is required"))
		return
	}
	if req.WatchDuration < 0 {
		httperrors.Write(c, httperrors.InvalidArgument("watch_duration must be greater than or equal to 0"))
		return
	}

	err := h.app.ReportWatch(c.Request.Context(), videoapp.ReportWatchInput{
		QuestionID:     req.QuestionID,
		UserID:         req.UserID,
		VideoSegmentID: req.VideoSegmentID,
		IsWatched:      req.IsWatched,
		WatchDuration:  req.WatchDuration,
	})
	if err != nil {
		writeRecommendError(c, err, "report watch failed")
		return
	}

	writeSuccess(c, dto.WatchRecordData{Recorded: true})
}

func (h *Handler) mapRecommendationItems(ctx context.Context, items []videoapp.RecommendResultItem) []dto.RecommendationItem {
	playURLCache := make(map[uint64]string, len(items))
	out := make([]dto.RecommendationItem, 0, len(items))
	for _, item := range items {
		title := item.Video.Title
		if item.TitleOverride != "" {
			title = item.TitleOverride
		}
		playURL := playURLCache[item.VideoID]
		if playURL == "" {
			playURL = strings.TrimSpace(h.app.ResolvePlaybackURL(ctx, item.Video))
			playURLCache[item.VideoID] = playURL
		}
		out = append(out, dto.RecommendationItem{
			QuestionID:     item.QuestionID,
			VideoID:        item.VideoID,
			VideoSegmentID: item.VideoSegmentID,
			RecommendScore: item.RecommendScore,
			IsWatched:      item.IsWatched,
			WatchDuration:  item.WatchDuration,
			StartTimeSec:   item.StartTimeSec,
			EndTimeSec:     item.EndTimeSec,
			Title:          title,
			CoverURL:       item.Video.CoverURL,
			PlayURL:        playURL,
		})
	}
	return out
}

func parseOptionalUintQuery(c *gin.Context, name string) (uint64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid")
	}
	return value, nil
}

func parseOptionalIntQuery(c *gin.Context, name string) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func writeRecommendError(c *gin.Context, err error, fallback string) {
	if err == nil {
		httperrors.Write(c, httperrors.Internal(fallback))
		return
	}
	var validationErr videoapp.ValidationError
	if errors.As(err, &validationErr) {
		httperrors.Write(c, httperrors.InvalidArgument(err.Error()))
		return
	}
	if errors.Is(err, videoapp.ErrVideoSegmentNotFound) {
		httperrors.Write(c, httperrors.NotFound("video_segment_not_found", err.Error()))
		return
	}
	httperrors.Write(c, httperrors.Internal(fallback))
}

func writeSuccess[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, dto.SuccessResponse[T]{Success: true, Data: data})
}
