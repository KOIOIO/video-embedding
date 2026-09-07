package handler

import (
	"github.com/gin-gonic/gin"

	commenthandler "video-service/internal/http/handler/comments"
)

type CommentHandler struct {
	inner *commenthandler.Handler
}

func NewCommentHandler(app any) *CommentHandler {
	return &CommentHandler{inner: commenthandler.New(app)}
}

// ListComments godoc
// @Summary 查询视频片段评论列表
// @Tags 视频评论
// @Produce json
// @Param id path int true "视频片段ID"
// @Param user_id query int false "查看者用户ID，用于返回其点赞状态"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} dto.CommentListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/video-segments/{id}/comments [get]
func (h *CommentHandler) ListComments(c *gin.Context) {
	h.inner.ListComments(c)
}

// CreateComment godoc
// @Summary 发布视频片段评论
// @Tags 视频评论
// @Accept json
// @Produce json
// @Param id path int true "视频片段ID"
// @Param request body dto.CreateCommentRequest true "评论内容"
// @Success 200 {object} dto.CommentCreateResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/video-segments/{id}/comments [post]
func (h *CommentHandler) CreateComment(c *gin.Context) {
	h.inner.CreateComment(c)
}

// GetCommentCounts godoc
// @Summary 查询视频片段评论总数
// @Tags 视频评论
// @Produce json
// @Param id path int true "视频片段ID"
// @Success 200 {object} dto.SegmentCommentCountResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/video-segments/{id}/comment-counts [get]
func (h *CommentHandler) GetCommentCounts(c *gin.Context) {
	h.inner.GetCommentCounts(c)
}

// ListReplies godoc
// @Summary 查询评论的二级回复列表
// @Tags 视频评论
// @Produce json
// @Param id path int true "评论ID"
// @Param user_id query int false "查看者用户ID，用于返回其点赞状态"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(3)
// @Success 200 {object} dto.CommentListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/comments/{id}/replies [get]
func (h *CommentHandler) ListReplies(c *gin.Context) {
	h.inner.ListReplies(c)
}

// CreateReply godoc
// @Summary 回复一条评论（二级回复）
// @Tags 视频评论
// @Accept json
// @Produce json
// @Param id path int true "被回复的评论ID"
// @Param request body dto.CreateCommentRequest true "回复内容"
// @Success 200 {object} dto.CommentCreateResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/comments/{id}/replies [post]
func (h *CommentHandler) CreateReply(c *gin.Context) {
	h.inner.CreateReply(c)
}

// ToggleCommentLike godoc
// @Summary 提交或取消评论互动（点赞/双赞/踩）
// @Tags 视频评论
// @Accept json
// @Produce json
// @Param id path int true "评论ID"
// @Param request body dto.CommentReactionRequest true "互动类型"
// @Success 200 {object} dto.CommentLikeResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/comments/{id}/reactions [post]
func (h *CommentHandler) ToggleCommentLike(c *gin.Context) {
	h.inner.ToggleCommentLike(c)
}
