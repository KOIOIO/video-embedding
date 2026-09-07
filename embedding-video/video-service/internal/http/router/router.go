package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"video-service/internal/http/app"
	"video-service/internal/http/handler"
	knowledgevideohandler "video-service/internal/http/handler/knowledgevideos"
	"video-service/middleware"
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
	commentHandler := handler.NewCommentHandler(httpApp.Service)
	uploadHandler := handler.NewUploadHandler(httpApp.Service)
	recommendHandler := handler.NewRecommendHandler(httpApp.Service)
	recommendationAdminHandler := handler.NewRecommendationAdminHandler(httpApp.Service)
	questionHandler := handler.NewQuestionHandler(httpApp.Service)
	objectProxyHandler := handler.NewObjectProxyHandler(httpApp.Store)
	systemHandler := handler.NewSystemHandler(httpApp.Service)
	authHandler := handler.NewAdminAuthHandler(httpApp.AdminAuth)
	userProfileHandler := handler.NewUserProfileHandler(httpApp.UserProfileService, httpApp.Store)
	public := r.Group("")
	adminRoutes := r.Group("")
	adminRoutes.Use(middleware.RequireAdmin(httpApp.AdminAuth))
	public.POST("/api/auth/login", authHandler.Login)
	adminRoutes.GET("/api/auth/me", authHandler.Me)
	if httpApp.KnowledgeVideoService != nil {
		knowledgeHandler := knowledgevideohandler.New(httpApp.KnowledgeVideoService, httpApp.KnowledgeVideoMaxRequestBytes)
		public.GET("/api/knowledge-videos/tree", knowledgeHandler.Tree)
		adminRoutes.POST("/api/admin/knowledge-videos/batches", knowledgeHandler.Import)
		adminRoutes.GET("/api/admin/knowledge-videos/batches/:batchId", knowledgeHandler.Progress)
		public.GET("/api/knowledge-points/:knowledgePointId/video", knowledgeHandler.Playback)
		public.GET("/api/knowledge-points/:knowledgePointId/videos", knowledgeHandler.Playback)
		public.POST("/api/knowledge-videos/:knowledgeVideoId/playbacks", knowledgeHandler.RecordPlayback)
	}
	if httpApp.KnowledgeVideoStore != nil && httpApp.KnowledgeVideoRepository != nil {
		mediaHandler := handler.NewKnowledgeVideoMediaHandler(httpApp.KnowledgeVideoRepository, httpApp.KnowledgeVideoStore)
		prefix := strings.TrimRight(strings.TrimSpace(httpApp.KnowledgeVideoMediaRoutePrefix), "/")
		if prefix == "" {
			prefix = "/knowledge-video-media"
		}
		public.GET(prefix+"/hls/:videoId/*filepath", mediaHandler.Proxy)
	}

	public.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	public.GET("/api/healthz", handler.Healthz)
	adminRoutes.GET("/api/system/metrics", systemHandler.GetSystemMetrics)
	public.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	registerObjectProxyRoutes(r, httpApp.MediaRoutePrefix, objectProxyHandler.ProxyVideo)
	r.GET("/api/question/list", questionHandler.ListQuestions)
	r.GET("/api/question/:id", questionHandler.GetQuestion)
	adminRoutes.POST("/api/video/upload", uploadHandler.UploadVideo)
	adminRoutes.POST("/api/video/upload_archive", uploadHandler.UploadVideoArchive)
	adminRoutes.POST("/api/videos", uploadHandler.UploadVideo)
	adminRoutes.POST("/api/videos/archive", uploadHandler.UploadVideoArchive)
	adminRoutes.POST("/api/videos/archive/uploads", uploadHandler.InitiateChunkedArchiveUpload)
	adminRoutes.POST("/api/videos/archive/uploads/:uploadId/complete", uploadHandler.CompleteChunkedArchiveUpload)
	adminRoutes.GET("/api/videos/archive/batches/:batchId/progress", uploadHandler.GetArchiveProcessingProgress)
	adminRoutes.POST("/api/videos/uploads", uploadHandler.InitiateChunkedUpload)
	adminRoutes.GET("/api/videos/uploads/:uploadId", uploadHandler.GetChunkedUploadStatus)
	adminRoutes.PUT("/api/videos/uploads/:uploadId/chunks/:chunkIndex", uploadHandler.UploadVideoChunk)
	adminRoutes.POST("/api/videos/uploads/:uploadId/complete", uploadHandler.CompleteChunkedUpload)
	r.POST("/api/recommendations/by-question", recommendHandler.RecommendByQuestion)
	r.POST("/api/video/recommend_by_question", recommendHandler.RecommendByQuestion)
	r.GET("/api/recommendations", recommendHandler.ListRecommendations)
	adminRoutes.GET("/api/admin/recommendation/overview", recommendationAdminHandler.Overview)
	adminRoutes.GET("/api/admin/recommendation/diagnostics", recommendationAdminHandler.Diagnostics)
	adminRoutes.GET("/api/admin/recommendation/datasources", recommendationAdminHandler.Datasources)
	adminRoutes.GET("/api/admin/recommendation/effects", recommendationAdminHandler.Effects)
	adminRoutes.GET("/api/admin/recommendation/recbole/performance", recommendationAdminHandler.RecBolePerformance)
	adminRoutes.GET("/api/admin/recommendation/trace/random-play", recommendationAdminHandler.TraceRandomPlay)
	adminRoutes.POST("/api/admin/recommendation/trace/by-question", recommendationAdminHandler.TraceByQuestion)
	adminRoutes.GET("/api/admin/recommendation/redis-state", recommendationAdminHandler.RedisState)
	adminRoutes.GET("/api/admin/recommendation/preview/random-play", recommendationAdminHandler.PreviewRandomPlay)
	adminRoutes.POST("/api/admin/recommendation/preview/by-question", recommendationAdminHandler.PreviewByQuestion)
	r.POST("/api/watch-records", recommendHandler.ReportWatch)
	r.POST("/api/video/report_watch", recommendHandler.ReportWatch)
	r.GET("/api/questions", questionHandler.ListQuestions)
	r.GET("/api/questions/:id", questionHandler.GetQuestion)
	r.GET("/api/videos", videoHandler.ListVideos)
	r.GET("/api/video/list", videoHandler.ListVideos)
	adminRoutes.PUT("/api/video/:id", videoHandler.UpdateVideoMetadata)
	adminRoutes.PATCH("/api/videos/:id", videoHandler.UpdateVideoMetadata)
	adminRoutes.DELETE("/api/video/:id", videoHandler.DeleteVideo)
	adminRoutes.DELETE("/api/videos/:id", videoHandler.DeleteVideo)
	adminRoutes.POST("/api/video/cover/:id", uploadHandler.UploadVideoCover)
	adminRoutes.POST("/api/videos/:id/cover", uploadHandler.UploadVideoCover)
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
	r.GET("/api/internal/recommendations/external/recbole", videoHandler.ExternalRecBoleRecommendations)
	r.POST("/api/video-segments/:id/reactions", videoHandler.SubmitSegmentReaction)
	r.GET("/api/video-segments/:id/reaction-counts", videoHandler.GetSegmentReactionCounts)
	public.GET("/api/video-segments/:id/comments", commentHandler.ListComments)
	public.GET("/api/video-segments/:id/comment-counts", commentHandler.GetCommentCounts)
	// 用户资料：公开查询
	public.GET("/api/users/:id/profile", userProfileHandler.GetProfile)
	// TODO: replace with real user authentication middleware — currently reads X-User-ID header inside handler
	public.PUT("/api/me/profile", userProfileHandler.UpdateProfile)
	public.POST("/api/me/avatar", userProfileHandler.UploadAvatar)
	adminRoutes.POST("/api/video-segments/:id/comments", commentHandler.CreateComment)
	public.GET("/api/comments/:id/replies", commentHandler.ListReplies)
	adminRoutes.POST("/api/comments/:id/replies", commentHandler.CreateReply)
	adminRoutes.POST("/api/comments/:id/reactions", commentHandler.ToggleCommentLike)
	adminRoutes.POST("/api/video/publish/:id", videoHandler.SetVideoPublished)
	adminRoutes.POST("/api/videos/:id/publish", videoHandler.SetVideoPublished)
	adminRoutes.POST("/api/video/recommend/:id", videoHandler.SetVideoRecommend)
	adminRoutes.POST("/api/videos/:id/recommend", videoHandler.SetVideoRecommend)
	adminRoutes.GET("/api/video/status/:taskId", videoHandler.GetTranscodeStatus)
	adminRoutes.GET("/api/transcode-tasks/:taskId", videoHandler.GetTranscodeStatus)
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
