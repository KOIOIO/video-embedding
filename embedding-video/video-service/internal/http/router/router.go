package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"nlp-video-analysis/internal/http/app"
	"nlp-video-analysis/internal/http/handler"
	"nlp-video-analysis/middleware"
)

func New(httpApp *app.App) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.CORSMiddlewareWithOptions(middleware.CORSOptions{
		AllowOrigin:   httpApp.HTTP.CORSAllowOrigin,
		AllowMethods:  httpApp.HTTP.CORSAllowMethods,
		AllowHeaders:  httpApp.HTTP.CORSAllowHeaders,
		ExposeHeaders: httpApp.HTTP.CORSExposeHeaders,
		MaxAge:        httpApp.HTTP.CORSMaxAge,
	}), middleware.AccessLogMiddlewareWithOptions(middleware.AccessLogOptions{
		LogDir:               httpApp.HTTP.LogDir,
		SlowRequestThreshold: httpApp.HTTP.SlowRequestThreshold,
	}))
	videoHandler := handler.NewVideoHandler(httpApp.Service)
	uploadHandler := handler.NewUploadHandler(httpApp.Service)
	recommendHandler := handler.NewRecommendHandler(httpApp.Service)
	questionHandler := handler.NewQuestionHandler(httpApp.Service)
	objectProxyHandler := handler.NewObjectProxyHandler(httpApp.Store)
	systemHandler := handler.NewSystemHandler(httpApp.Service)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/api/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/api/system/metrics", systemHandler.GetSystemMetrics)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	registerObjectProxyRoutes(r, httpApp.MediaRoutePrefix, objectProxyHandler.ProxyVideo)
	r.GET("/api/question/list", questionHandler.ListQuestions)
	r.GET("/api/question/:id", questionHandler.GetQuestion)
	r.POST("/api/video/upload", uploadHandler.UploadVideo)
	r.POST("/api/video/upload_archive", uploadHandler.UploadVideoArchive)
	r.POST("/api/videos", uploadHandler.UploadVideo)
	r.POST("/api/videos/archive", uploadHandler.UploadVideoArchive)
	r.POST("/api/videos/archive/uploads", uploadHandler.InitiateChunkedArchiveUpload)
	r.POST("/api/videos/archive/uploads/:uploadId/complete", uploadHandler.CompleteChunkedArchiveUpload)
	r.GET("/api/videos/archive/batches/:batchId/progress", uploadHandler.GetArchiveProcessingProgress)
	r.POST("/api/videos/uploads", uploadHandler.InitiateChunkedUpload)
	r.GET("/api/videos/uploads/:uploadId", uploadHandler.GetChunkedUploadStatus)
	r.PUT("/api/videos/uploads/:uploadId/chunks/:chunkIndex", uploadHandler.UploadVideoChunk)
	r.POST("/api/videos/uploads/:uploadId/complete", uploadHandler.CompleteChunkedUpload)
	r.POST("/api/recommendations/by-question", recommendHandler.RecommendByQuestion)
	r.POST("/api/video/recommend_by_question", recommendHandler.RecommendByQuestion)
	r.GET("/api/recommendations", recommendHandler.ListRecommendations)
	r.POST("/api/watch-records", recommendHandler.ReportWatch)
	r.POST("/api/video/report_watch", recommendHandler.ReportWatch)
	r.GET("/api/questions", questionHandler.ListQuestions)
	r.GET("/api/questions/:id", questionHandler.GetQuestion)
	r.GET("/api/videos", videoHandler.ListVideos)
	r.GET("/api/video/list", videoHandler.ListVideos)
	r.PUT("/api/video/:id", videoHandler.UpdateVideoMetadata)
	r.PATCH("/api/videos/:id", videoHandler.UpdateVideoMetadata)
	r.DELETE("/api/video/:id", videoHandler.DeleteVideo)
	r.DELETE("/api/videos/:id", videoHandler.DeleteVideo)
	r.POST("/api/video/cover/:id", uploadHandler.UploadVideoCover)
	r.POST("/api/videos/:id/cover", uploadHandler.UploadVideoCover)
	r.GET("/api/video/play/:id", videoHandler.PlayVideo)
	r.GET("/api/videos/:id/play", videoHandler.PlayVideo)
	r.GET("/api/video/similar/:id", videoHandler.GetSimilarVideos)
	r.GET("/api/videos/:id/similar", videoHandler.GetSimilarVideos)
	r.GET("/api/video/view_count/:id", videoHandler.GetViewCount)
	r.GET("/api/videos/:id/view-count", videoHandler.GetViewCount)
	r.POST("/api/video/reaction/:id", videoHandler.SubmitVideoReaction)
	r.POST("/api/videos/:id/reactions", videoHandler.SubmitVideoReaction)
	r.GET("/api/video/reaction_counts/:id", videoHandler.GetVideoReactionCounts)
	r.GET("/api/videos/:id/reaction-counts", videoHandler.GetVideoReactionCounts)
	r.GET("/api/video-segment/random-play", videoHandler.RandomPlayVideoSegment)
	r.GET("/api/video-segments/random-play", videoHandler.RandomPlayVideoSegment)
	r.GET("/api/internal/recommendations/external/two-tower", videoHandler.ExternalTwoTowerRecommendations)
	r.POST("/api/video-segments/:id/reactions", videoHandler.SubmitSegmentReaction)
	r.GET("/api/video-segments/:id/reaction-counts", videoHandler.GetSegmentReactionCounts)
	r.POST("/api/video/publish/:id", videoHandler.SetVideoPublished)
	r.POST("/api/videos/:id/publish", videoHandler.SetVideoPublished)
	r.POST("/api/video/recommend/:id", videoHandler.SetVideoRecommend)
	r.POST("/api/videos/:id/recommend", videoHandler.SetVideoRecommend)
	r.GET("/api/video/status/:taskId", videoHandler.GetTranscodeStatus)
	r.GET("/api/transcode-tasks/:taskId", videoHandler.GetTranscodeStatus)
	return r
}

func registerObjectProxyRoutes(r *gin.Engine, mediaRoutePrefix string, handler gin.HandlerFunc) {
	prefix := strings.TrimRight(strings.TrimSpace(mediaRoutePrefix), "/")
	if prefix == "" {
		prefix = "/videos"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	r.GET(prefix+"/*filepath", handler)
	if prefix != "/videos" {
		r.GET("/videos/*filepath", handler)
	}
}
