package handler

import (
	"github.com/gin-gonic/gin"

	questionhandler "nlp-video-analysis/internal/http/handler/questions"
)

type QuestionHandler struct {
	inner *questionhandler.Handler
}

func NewQuestionHandler(app any) *QuestionHandler {
	return &QuestionHandler{inner: questionhandler.New(app)}
}

// ListQuestions godoc
// @Summary List questions
// @Tags questions
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} dto.QuestionListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/questions [get]
func (h *QuestionHandler) ListQuestions(c *gin.Context) {
	h.inner.ListQuestions(c)
}

// GetQuestion godoc
// @Summary Get question detail
// @Tags questions
// @Produce json
// @Param id path int true "Question ID"
// @Success 200 {object} dto.QuestionDetailResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/questions/{id} [get]
func (h *QuestionHandler) GetQuestion(c *gin.Context) {
	h.inner.GetQuestion(c)
}
