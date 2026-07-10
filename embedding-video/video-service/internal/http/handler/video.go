package handler

import (
	"github.com/gin-gonic/gin"

	videohandler "nlp-video-analysis/internal/http/handler/videos"
)

type VideoHandler struct {
	inner *videohandler.Handler
}

func NewVideoHandler(app any) *VideoHandler {
	return &VideoHandler{inner: videohandler.New(app)}
}

// ListVideos godoc
// @Summary 查询视频列表
// @Tags 视频服务
// @Produce json
// @Param type query string false "视频类型筛选" Enums(ALL,RAW,HLS) default(ALL)
// @Success 200 {object} dto.VideoListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos [get]
func (h *VideoHandler) ListVideos(c *gin.Context) {
	h.inner.ListVideos(c)
}

// UpdateVideoMetadata godoc
// @Summary 修改视频基础信息
// @Tags 视频服务
// @Accept json
// @Produce json
// @Param id path int true "视频ID"
// @Param request body dto.UpdateVideoRequest true "视频基础信息"
// @Success 200 {object} dto.UpdateVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id} [patch]
func (h *VideoHandler) UpdateVideoMetadata(c *gin.Context) {
	h.inner.UpdateVideoMetadata(c)
}

// DeleteVideo godoc
// @Summary 删除视频
// @Tags 视频服务
// @Produce json
// @Param id path int true "视频ID"
// @Success 200 {object} dto.DeleteVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id} [delete]
func (h *VideoHandler) DeleteVideo(c *gin.Context) {
	h.inner.DeleteVideo(c)
}

// PlayVideo godoc
// @Summary 获取视频播放地址
// @Tags 视频服务
// @Produce json
// @Param id path int true "视频ID"
// @Success 200 {object} dto.PlayVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/play [get]
func (h *VideoHandler) PlayVideo(c *gin.Context) {
	h.inner.PlayVideo(c)
}

// GetSimilarVideos godoc
// @Summary 查询相似视频
// @Tags 视频服务
// @Produce json
// @Param id path int true "视频ID"
// @Param limit query int false "返回数量" default(6)
// @Success 200 {object} dto.SimilarVideosResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/similar [get]
func (h *VideoHandler) GetSimilarVideos(c *gin.Context) {
	h.inner.GetSimilarVideos(c)
}

// GetViewCount godoc
// @Summary 查询视频播放次数
// @Tags 视频服务
// @Produce json
// @Param id path int true "视频ID"
// @Success 200 {object} dto.ViewCountResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/view-count [get]
func (h *VideoHandler) GetViewCount(c *gin.Context) {
	h.inner.GetViewCount(c)
}

// SetVideoPublished godoc
// @Summary 设置视频发布状态
// @Tags 视频服务
// @Accept json
// @Produce json
// @Param id path int true "视频ID"
// @Param request body dto.PublishVideoRequest true "发布状态参数"
// @Success 200 {object} dto.PublishVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/publish [post]
func (h *VideoHandler) SetVideoPublished(c *gin.Context) {
	h.inner.SetVideoPublished(c)
}

// SetVideoRecommend godoc
// @Summary 设置视频推荐状态
// @Tags 视频服务
// @Accept json
// @Produce json
// @Param id path int true "视频ID"
// @Param request body dto.RecommendVideoRequest true "推荐状态参数"
// @Success 200 {object} dto.RecommendVideoResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/recommend [post]
func (h *VideoHandler) SetVideoRecommend(c *gin.Context) {
	h.inner.SetVideoRecommend(c)
}

// SubmitVideoReaction godoc
// @Summary 提交或取消视频互动
// @Tags 视频服务
// @Accept json
// @Produce json
// @Param id path int true "视频ID"
// @Param request body dto.VideoReactionRequest true "互动参数"
// @Success 200 {object} dto.VideoReactionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/reactions [post]
func (h *VideoHandler) SubmitVideoReaction(c *gin.Context) {
	h.inner.SubmitVideoReaction(c)
}

// GetVideoReactionCounts godoc
// @Summary 查询视频点赞和双击数量
// @Tags 视频服务
// @Produce json
// @Param id path int true "视频ID"
// @Success 200 {object} dto.VideoReactionCountsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/videos/{id}/reaction-counts [get]
func (h *VideoHandler) GetVideoReactionCounts(c *gin.Context) {
	h.inner.GetVideoReactionCounts(c)
}

// SubmitSegmentReaction godoc
// @Summary 提交或取消视频片段互动
// @Tags 视频服务
// @Accept json
// @Produce json
// @Param id path int true "视频片段ID"
// @Param request body dto.VideoReactionRequest true "互动参数"
// @Success 200 {object} dto.SegmentReactionResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/video-segments/{id}/reactions [post]
func (h *VideoHandler) SubmitSegmentReaction(c *gin.Context) {
	h.inner.SubmitSegmentReaction(c)
}

// GetSegmentReactionCounts godoc
// @Summary 查询视频片段点赞和双击数量
// @Tags 视频服务
// @Produce json
// @Param id path int true "视频片段ID"
// @Success 200 {object} dto.SegmentReactionCountsResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/video-segments/{id}/reaction-counts [get]
func (h *VideoHandler) GetSegmentReactionCounts(c *gin.Context) {
	h.inner.GetSegmentReactionCounts(c)
}

// RandomPlayVideoSegment godoc
// @Summary 随机播放视频片段
// @Tags 视频服务
// @Produce json
// @Param user_id query int false "用户ID，用于个性化推荐和最近播放去重"
// @Success 200 {object} dto.RandomVideoSegmentResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/video-segments/random-play [get]
func (h *VideoHandler) RandomPlayVideoSegment(c *gin.Context) {
	h.inner.RandomPlayVideoSegment(c)
}

// ExternalRecBoleRecommendations godoc
// @Summary 获取 RecBole 召回视频片段ID
// @Tags 视频服务
// @Produce json
// @Param user_id query int true "用户ID"
// @Param n query int false "返回的视频片段ID数量，最多500个"
// @Success 200 {array} string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/internal/recommendations/external/recbole [get]
func (h *VideoHandler) ExternalRecBoleRecommendations(c *gin.Context) {
	h.inner.ExternalRecBoleRecommendations(c)
}

// GetTranscodeStatus godoc
// @Summary 查询转码任务状态
// @Tags 视频服务
// @Produce json
// @Param taskId path string true "转码任务ID"
// @Success 200 {object} dto.TranscodeStatusResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/transcode-tasks/{taskId} [get]
func (h *VideoHandler) GetTranscodeStatus(c *gin.Context) {
	h.inner.GetTranscodeStatus(c)
}
