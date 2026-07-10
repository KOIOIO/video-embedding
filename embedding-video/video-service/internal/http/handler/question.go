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
// @Summary 查询题目列表
// @Tags 视频服务
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} dto.QuestionListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/questions [get]
func (h *QuestionHandler) ListQuestions(c *gin.Context) {
	h.inner.ListQuestions(c)
}

// GetQuestion godoc
// @Summary 查询题目详情
// @Tags 视频服务
// @Produce json
// @Param id path int true "题目ID"
// @Success 200 {object} dto.QuestionDetailResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/questions/{id} [get]
func (h *QuestionHandler) GetQuestion(c *gin.Context) {
	h.inner.GetQuestion(c)
}
