package handler

import (
	"legacy-video/internal/api/handler/impl"

	"github.com/gin-gonic/gin"
)

// RustfsStore 描述 handler 层依赖的最小对象存储能力。
// 这里保持接口很窄，避免 handler 直接依赖完整对象存储实现。
var RustfsStore interface {
	Put(ctx *gin.Context, key string, r interface{}, size int64, contentType string) error
}

// init 把包级依赖透传给 impl 子包。
func init() {
	impl.RustfsStore = RustfsStore
}

// UploadVideo 转发上传视频请求到具体实现。
func UploadVideo(c *gin.Context) {
	impl.UploadVideo(c)
}

// Healthz 返回 API 层健康检查结果，供前端和网关统一探活。
func Healthz(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// ListVideos 转发视频列表请求到具体实现。
func ListVideos(c *gin.Context) {
	impl.ListVideos(c)
}

// UpdateVideoMetadata 转发视频元数据更新请求到具体实现。
func UpdateVideoMetadata(c *gin.Context) {
	impl.UpdateVideoMetadata(c)
}

// GetTranscodeStatus 转发转码状态查询请求到具体实现。
func GetTranscodeStatus(c *gin.Context) {
	impl.GetTranscodeStatus(c)
}

// DeleteVideo 转发删除视频请求到具体实现。
func DeleteVideo(c *gin.Context) {
	impl.DeleteVideo(c)
}

// PlayVideo 转发播放请求到具体实现。
func PlayVideo(c *gin.Context) {
	impl.PlayVideo(c)
}

// RecommendByQuestion 转发题目检索推荐请求到具体实现。
func RecommendByQuestion(c *gin.Context) {
	impl.RecommendByQuestion(c)
}

// GetSimilarVideos 转发相似视频查询请求到具体实现。
func GetSimilarVideos(c *gin.Context) {
	impl.GetSimilarVideos(c)
}

// GetViewCount 转发观看次数查询请求到具体实现。
func GetViewCount(c *gin.Context) {
	impl.GetViewCount(c)
}

// ListRecommendPoolVideos 转发推荐池列表请求到具体实现。
func ListRecommendPoolVideos(c *gin.Context) {
	impl.ListRecommendPoolVideos(c)
}

// SetVideoPublished 转发发布状态修改请求到具体实现。
func SetVideoPublished(c *gin.Context) {
	impl.SetVideoPublished(c)
}

// SetVideoRecommend 转发推荐状态修改请求到具体实现。
func SetVideoRecommend(c *gin.Context) {
	impl.SetVideoRecommend(c)
}

// UploadVideoCover 转发封面上传请求到具体实现。
func UploadVideoCover(c *gin.Context) {
	impl.UploadVideoCover(c)
}

// ReportWatch 转发观看上报请求到具体实现。
func ReportWatch(c *gin.Context) {
	impl.ReportWatch(c)
}

// ListQuestions 转发题库列表请求到具体实现。
func ListQuestions(c *gin.Context) {
	impl.ListQuestions(c)
}

// GetQuestion 转发题目详情请求到具体实现。
func GetQuestion(c *gin.Context) {
	impl.GetQuestion(c)
}
