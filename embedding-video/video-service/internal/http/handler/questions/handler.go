package questions

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/http/dto"
	httperrors "nlp-video-analysis/internal/http/errors"
)

type Handler struct {
	app questionApp
}

type questionApp interface {
	ListQuestions(ctx context.Context, input videoapp.ListQuestionsInput) (videoapp.QuestionPage, error)
	GetQuestion(ctx context.Context, id uint64) (videoapp.QuestionItem, bool, error)
}

func New(app any) *Handler {
	switch v := app.(type) {
	case questionApp:
		return &Handler{app: v}
	default:
		panic("unsupported question app")
	}
}

func (h *Handler) ListQuestions(c *gin.Context) {
	page, ok := parsePositiveIntQuery(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := parsePositiveIntQuery(c, "page_size", 20)
	if !ok {
		return
	}

	result, err := h.app.ListQuestions(c.Request.Context(), videoapp.ListQuestionsInput{Page: page, PageSize: pageSize})
	if err != nil {
		writeAppError(c, err, "list questions failed")
		return
	}

	items := make([]dto.QuestionItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, dto.QuestionItem{
			ID:               item.ID,
			Source:           item.Source,
			SourceQuestionID: item.SourceQuestionID,
			Content:          item.Content,
			Answer:           item.Answer,
			Analysis:         item.Analysis,
			Knowledge:        item.Knowledge,
			Subject:          item.Subject,
			Type:             item.Type,
			Status:           item.Status,
			CreateTime:       item.CreateTime.Unix(),
			UpdateTime:       item.UpdateTime.Unix(),
		})
	}

	writeSuccess(c, dto.QuestionListData{Total: result.Total, Page: result.Page, PageSize: result.PageSize, List: items})
}

func (h *Handler) GetQuestion(c *gin.Context) {
	id, ok := parsePositiveUintParam(c, "id")
	if !ok {
		return
	}

	question, found, err := h.app.GetQuestion(c.Request.Context(), id)
	if err != nil {
		writeAppError(c, err, "get question failed")
		return
	}
	if !found {
		httperrors.Write(c, httperrors.NotFound("question_not_found", "question does not exist"))
		return
	}

	writeSuccess(c, dto.QuestionDetailData{Question: dto.QuestionItem{
		ID:               question.ID,
		Source:           question.Source,
		SourceQuestionID: question.SourceQuestionID,
		Content:          question.Content,
		Answer:           question.Answer,
		Analysis:         question.Analysis,
		Knowledge:        question.Knowledge,
		Subject:          question.Subject,
		Type:             question.Type,
		Status:           question.Status,
		CreateTime:       question.CreateTime.Unix(),
		UpdateTime:       question.UpdateTime.Unix(),
	}})
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
