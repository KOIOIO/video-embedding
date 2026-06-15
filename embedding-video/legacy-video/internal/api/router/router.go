package router

import (
	"legacy-video/middleware"

	"legacy-video/internal/api/handler"

	"github.com/gin-gonic/gin"
)

// SetupRouter 构建 HTTP 路由树，并把 API 层的调试/联调入口统一注册到 Gin。
func SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.AccessLogMiddleware())
	r.GET("/api/healthz", handler.Healthz)

	// 视频相关路由
	videoGroup := r.Group("/api/video")
	{
		// 问题检索推荐（按分段返回 Top3，可用于片段化播放）
		videoGroup.POST("/recommend_by_question", handler.RecommendByQuestion)

		// 上传视频
		videoGroup.POST("/upload", handler.UploadVideo)
		// 获取视频列表
		videoGroup.GET("/list", handler.ListVideos)
		// 修改视频标题和描述
		videoGroup.PUT("/:id", handler.UpdateVideoMetadata)
		// 删除视频
		videoGroup.DELETE("/:id", handler.DeleteVideo)
		// 播放视频
		videoGroup.GET("/play/:id", handler.PlayVideo)
		// 相近视频
		videoGroup.GET("/similar/:id", handler.GetSimilarVideos)
		// 观看次数
		videoGroup.GET("/view_count/:id", handler.GetViewCount)
		// 推荐池列表
		videoGroup.GET("/recommend_pool", handler.ListRecommendPoolVideos)
		// 加入/移出推荐池
		videoGroup.POST("/recommend/:id", handler.SetVideoRecommend)
		// 上传封面
		videoGroup.POST("/cover/:id", handler.UploadVideoCover)
		// 发布/取消发布
		videoGroup.POST("/publish/:id", handler.SetVideoPublished)
		// 获取转码状态
		videoGroup.GET("/status/:taskId", handler.GetTranscodeStatus)
		// 上报观看记录
		videoGroup.POST("/report_watch", handler.ReportWatch)
	}

	r.GET("/videos/*filepath", handler.ProxyVideo)

	// 题库相关路由
	questionGroup := r.Group("/api/question")
	{
		questionGroup.GET("/list", handler.ListQuestions)
		questionGroup.GET("/:id", handler.GetQuestion)
	}

	return r
}
