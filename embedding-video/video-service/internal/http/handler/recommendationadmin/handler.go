package recommendationadmin

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
	app recommendationAdminApp
}

type recommendationAdminApp interface {
	RecommendationAdminOverview(ctx context.Context) (videoapp.RecommendationAdminOverview, error)
	RecommendationDiagnostics(ctx context.Context, input videoapp.RecommendationDiagnosticsInput) (videoapp.RecommendationDiagnostics, error)
	RecommendationDatasourceStats(ctx context.Context) (videoapp.RecommendationDatasourceStats, error)
	RecommendationEffectMetrics(ctx context.Context, input videoapp.RecommendationEffectMetricsInput) (videoapp.RecommendationEffectMetrics, error)
	RecommendationTraceRandomPlay(ctx context.Context, input videoapp.RandomPlayVideoSegmentInput) (videoapp.RecommendationTrace, error)
	RecommendationTraceByQuestion(ctx context.Context, input videoapp.RecommendByQuestionInput) (videoapp.RecommendationTrace, error)
	RecommendationRedisState(ctx context.Context, input videoapp.RecommendationRedisStateInput) (videoapp.RecommendationRedisState, error)
	PreviewRandomPlayVideoSegments(ctx context.Context, input videoapp.RandomPlayVideoSegmentInput) ([]videoapp.RecommendResultItem, error)
	PreviewRecommendByQuestion(ctx context.Context, input videoapp.RecommendByQuestionInput) ([]videoapp.RecommendResultItem, error)
	ResolvePlaybackURL(ctx context.Context, video domainvideo.Video) string
}

func New(app any) *Handler {
	switch v := app.(type) {
	case recommendationAdminApp:
		return &Handler{app: v}
	default:
		panic("unsupported recommendation admin app")
	}
}

func (h *Handler) Overview(c *gin.Context) {
	overview, err := h.app.RecommendationAdminOverview(c.Request.Context())
	if err != nil {
		httperrors.Write(c, httperrors.Internal("recommendation overview failed"))
		return
	}
	writeSuccess(c, mapOverview(overview))
}

func (h *Handler) Diagnostics(c *gin.Context) {
	days, err := parseOptionalDaysQuery(c)
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("days must be a positive integer"))
		return
	}
	limit, err := parseOptionalLimitQuery(c)
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("limit must be a non-negative integer"))
		return
	}
	diagnostics, err := h.app.RecommendationDiagnostics(c.Request.Context(), videoapp.RecommendationDiagnosticsInput{
		Days:  days,
		Limit: limit,
	})
	if err != nil {
		httperrors.Write(c, httperrors.Internal("recommendation diagnostics failed"))
		return
	}
	writeSuccess(c, mapDiagnostics(diagnostics))
}

func (h *Handler) Datasources(c *gin.Context) {
	stats, err := h.app.RecommendationDatasourceStats(c.Request.Context())
	if err != nil {
		httperrors.Write(c, httperrors.Internal("recommendation datasource stats failed"))
		return
	}
	writeSuccess(c, mapDatasourceStats(stats))
}

func (h *Handler) Effects(c *gin.Context) {
	days, err := parseOptionalDaysQuery(c)
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("days must be a positive integer"))
		return
	}
	metrics, err := h.app.RecommendationEffectMetrics(c.Request.Context(), videoapp.RecommendationEffectMetricsInput{Days: days})
	if err != nil {
		httperrors.Write(c, httperrors.Internal("recommendation effect metrics failed"))
		return
	}
	writeSuccess(c, mapEffectMetrics(days, metrics))
}

func (h *Handler) PreviewRandomPlay(c *gin.Context) {
	userID, err := parseRequiredUintQuery(c, "user_id")
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("user_id must be a positive integer"))
		return
	}
	limit, err := parseOptionalLimitQuery(c)
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("limit must be a non-negative integer"))
		return
	}

	items, err := h.app.PreviewRandomPlayVideoSegments(c.Request.Context(), videoapp.RandomPlayVideoSegmentInput{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		writeRecommendationAdminError(c, err, "preview random-play failed")
		return
	}
	writeSuccess(c, dto.RecommendationAdminPreviewListData{
		Items:       h.mapPreviewItems(c.Request.Context(), items),
		Total:       len(items),
		PreviewOnly: true,
	})
}

func (h *Handler) TraceRandomPlay(c *gin.Context) {
	userID, err := parseRequiredUintQuery(c, "user_id")
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("user_id must be a positive integer"))
		return
	}
	limit, err := parseOptionalLimitQuery(c)
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("limit must be a non-negative integer"))
		return
	}

	trace, err := h.app.RecommendationTraceRandomPlay(c.Request.Context(), videoapp.RandomPlayVideoSegmentInput{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		writeRecommendationAdminError(c, err, "trace random-play failed")
		return
	}
	writeSuccess(c, mapTrace(trace))
}

func (h *Handler) TraceByQuestion(c *gin.Context) {
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

	trace, err := h.app.RecommendationTraceByQuestion(c.Request.Context(), videoapp.RecommendByQuestionInput{
		QuestionID:   req.QuestionID,
		QuestionText: questionText,
		UserID:       req.UserID,
		Limit:        normalizeAdminPreviewLimit(req.Limit),
	})
	if err != nil {
		writeRecommendationAdminError(c, err, "trace by question failed")
		return
	}
	writeSuccess(c, mapTrace(trace))
}

func (h *Handler) RedisState(c *gin.Context) {
	userID, err := parseRequiredUintQuery(c, "user_id")
	if err != nil {
		httperrors.Write(c, httperrors.InvalidArgument("user_id must be a positive integer"))
		return
	}
	state, err := h.app.RecommendationRedisState(c.Request.Context(), videoapp.RecommendationRedisStateInput{UserID: userID})
	if err != nil {
		httperrors.Write(c, httperrors.Internal("recommendation redis state failed"))
		return
	}
	writeSuccess(c, mapRedisState(state))
}

func (h *Handler) PreviewByQuestion(c *gin.Context) {
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

	items, err := h.app.PreviewRecommendByQuestion(c.Request.Context(), videoapp.RecommendByQuestionInput{
		QuestionID:   req.QuestionID,
		QuestionText: questionText,
		UserID:       req.UserID,
		Limit:        normalizeAdminPreviewLimit(req.Limit),
	})
	if err != nil {
		writeRecommendationAdminError(c, err, "preview by question failed")
		return
	}
	writeSuccess(c, dto.RecommendationAdminPreviewListData{
		Items:       h.mapPreviewItems(c.Request.Context(), items),
		Total:       len(items),
		PreviewOnly: true,
	})
}

func mapDiagnostics(diagnostics videoapp.RecommendationDiagnostics) dto.RecommendationDiagnosticsData {
	health := make([]dto.RecommendationDiagnosticCheckData, 0, len(diagnostics.Health))
	for _, check := range diagnostics.Health {
		health = append(health, dto.RecommendationDiagnosticCheckData{
			Key:    check.Key,
			Name:   check.Name,
			Status: check.Status,
			Value:  check.Value,
			Detail: check.Detail,
		})
	}
	freshness := make([]dto.RecommendationDataFreshnessData, 0, len(diagnostics.Freshness))
	for _, row := range diagnostics.Freshness {
		freshness = append(freshness, dto.RecommendationDataFreshnessData{
			Source:            row.Source,
			Label:             row.Label,
			Status:            row.Status,
			LatestAt:          row.LatestAt,
			HasData:           row.HasData,
			AgeSeconds:        row.AgeSeconds,
			StaleAfterSeconds: row.StaleAfterSeconds,
			Detail:            row.Detail,
		})
	}
	recentRequests := make([]dto.RecommendationRecentRequestData, 0, len(diagnostics.RecentRequests))
	for _, row := range diagnostics.RecentRequests {
		recentRequests = append(recentRequests, dto.RecommendationRecentRequestData{
			RequestID:     row.RequestID,
			UserID:        row.UserID,
			QuestionID:    row.QuestionID,
			Exposures:     row.Exposures,
			Watched:       row.Watched,
			WatchRate:     row.WatchRate,
			Strategy:      row.Strategy,
			ModelVersion:  row.ModelVersion,
			LastEventTime: row.LastEventTime,
		})
	}
	tasks := make([]dto.RecommendationTaskStatusData, 0, len(diagnostics.Tasks))
	for _, task := range diagnostics.Tasks {
		tasks = append(tasks, dto.RecommendationTaskStatusData{
			Name:       task.Name,
			Status:     task.Status,
			Detail:     task.Detail,
			LastRunAt:  task.LastRunAt,
			HasRunTime: task.HasRunTime,
		})
	}
	return dto.RecommendationDiagnosticsData{
		GeneratedAt:     diagnostics.GeneratedAt,
		Days:            diagnostics.Days,
		RequestLimit:    diagnostics.RequestLimit,
		Health:          health,
		Freshness:       freshness,
		RecentRequests:  recentRequests,
		StrategyEffects: mapStrategyEffects(diagnostics.StrategyEffects),
		Tasks:           tasks,
	}
}

func mapTrace(trace videoapp.RecommendationTrace) dto.RecommendationTraceData {
	stages := make([]dto.RecommendationTraceStageData, 0, len(trace.Stages))
	for _, stage := range trace.Stages {
		stages = append(stages, dto.RecommendationTraceStageData{
			Name:   stage.Name,
			Status: stage.Status,
			Detail: stage.Detail,
		})
	}
	items := make([]dto.RecommendationTraceItemData, 0, len(trace.Items))
	for _, item := range trace.Items {
		items = append(items, dto.RecommendationTraceItemData{
			Rank:           item.Rank,
			QuestionID:     item.QuestionID,
			VideoID:        item.VideoID,
			VideoSegmentID: item.VideoSegmentID,
			RecommendScore: item.RecommendScore,
			Strategy:       item.Strategy,
			ModelVersion:   item.ModelVersion,
			Status:         item.Status,
			Reasons:        item.Reasons,
			StartTimeSec:   item.StartTimeSec,
			EndTimeSec:     item.EndTimeSec,
			Title:          item.Title,
			IsWatched:      item.IsWatched,
			WatchDuration:  item.WatchDuration,
		})
	}
	return dto.RecommendationTraceData{
		Mode:         trace.Mode,
		Engine:       trace.Engine,
		UserID:       trace.UserID,
		QuestionID:   trace.QuestionID,
		QuestionText: trace.QuestionText,
		Limit:        trace.Limit,
		GeneratedAt:  trace.GeneratedAt,
		PreviewOnly:  trace.PreviewOnly,
		Stages:       stages,
		Items:        items,
	}
}

func mapRedisState(state videoapp.RecommendationRedisState) dto.RecommendationRedisStateData {
	items := make([]dto.RecommendationRedisBucketItemData, 0, len(state.Bucket.Items))
	for _, item := range state.Bucket.Items {
		items = append(items, dto.RecommendationRedisBucketItemData{
			Rank:           item.Rank,
			VideoID:        item.VideoID,
			VideoSegmentID: item.VideoSegmentID,
			Strategy:       item.Strategy,
			ModelVersion:   item.ModelVersion,
			Score:          item.Score,
		})
	}
	return dto.RecommendationRedisStateData{
		UserID:      state.UserID,
		GeneratedAt: state.GeneratedAt,
		Bucket: dto.RecommendationRedisBucketStateData{
			Enabled:    state.Bucket.Enabled,
			Exists:     state.Bucket.Exists,
			TTLSeconds: state.Bucket.TTLSeconds,
			Count:      state.Bucket.Count,
			MaxSize:    state.Bucket.MaxSize,
			MinSize:    state.Bucket.MinSize,
			Items:      items,
		},
		Recent: dto.RecommendationRedisRecentStateData{
			Enabled:    state.Recent.Enabled,
			Exists:     state.Recent.Exists,
			TTLSeconds: state.Recent.TTLSeconds,
			Count:      state.Recent.Count,
			MaxSize:    state.Recent.MaxSize,
			OverLimit:  state.Recent.OverLimit,
			SegmentIDs: state.Recent.SegmentIDs,
		},
	}
}

func mapDatasourceStats(stats videoapp.RecommendationDatasourceStats) dto.RecommendationDatasourceStatsData {
	return dto.RecommendationDatasourceStatsData{
		VideoTotal:              stats.VideoTotal,
		PublishedVideos:         stats.PublishedVideos,
		RecommendVideos:         stats.RecommendVideos,
		SegmentTotal:            stats.SegmentTotal,
		PlayableSegments:        stats.PlayableSegments,
		EmbeddedSegments:        stats.EmbeddedSegments,
		SegmentEmbeddingRate:    ratio(stats.EmbeddedSegments, stats.SegmentTotal),
		ExposureTotal:           stats.ExposureTotal,
		WatchedExposures:        stats.WatchedExposures,
		ExposureWatchRate:       ratio(stats.WatchedExposures, stats.ExposureTotal),
		RecommendationRows:      stats.RecommendationRows,
		WatchedRecommendations:  stats.WatchedRecommendations,
		RecommendationWatchRate: ratio(stats.WatchedRecommendations, stats.RecommendationRows),
		RecBoleUsers:            stats.RecBoleUsers,
		RecBoleItems:            stats.RecBoleItems,
		ReactionRows:            stats.ReactionRows,
	}
}

func mapEffectMetrics(days int, metrics videoapp.RecommendationEffectMetrics) dto.RecommendationEffectMetricsData {
	daily := make([]dto.RecommendationDailyEffectMetricData, 0, len(metrics.Daily))
	for _, row := range metrics.Daily {
		daily = append(daily, dto.RecommendationDailyEffectMetricData{
			Day:       row.Day,
			Exposures: row.Exposures,
			Watched:   row.Watched,
			WatchRate: row.WatchRate,
		})
	}
	return dto.RecommendationEffectMetricsData{
		Days:       days,
		Daily:      daily,
		Strategies: mapStrategyEffects(metrics.Strategies),
	}
}

func mapStrategyEffects(rows []videoapp.RecommendationStrategyEffectMetric) []dto.RecommendationStrategyEffectMetricData {
	strategies := make([]dto.RecommendationStrategyEffectMetricData, 0, len(rows))
	for _, row := range rows {
		strategies = append(strategies, dto.RecommendationStrategyEffectMetricData{
			Strategy:     row.Strategy,
			ModelVersion: row.ModelVersion,
			Exposures:    row.Exposures,
			Watched:      row.Watched,
			WatchRate:    row.WatchRate,
			AverageRank:  row.AverageRank,
			AverageScore: row.AverageScore,
		})
	}
	return strategies
}

func mapOverview(overview videoapp.RecommendationAdminOverview) dto.RecommendationAdminOverviewData {
	return dto.RecommendationAdminOverviewData{
		Engine:      overview.Engine,
		GeneratedAt: overview.GeneratedAt,
		PreviewOnly: overview.PreviewOnly,
		Gorse: dto.RecommendationAdminGorseOverviewData{
			Configured:        overview.Gorse.Configured,
			CandidateLimit:    overview.Gorse.CandidateLimit,
			MinRecommendItems: overview.Gorse.MinRecommendItems,
			WriteBackEnabled:  overview.Gorse.WriteBackEnabled,
			ShadowMode:        overview.Gorse.ShadowMode,
		},
		RecBole: dto.RecommendationAdminRecBoleData{
			ActiveModelVersion: overview.RecBole.ActiveModelVersion,
			ActiveModelFound:   overview.RecBole.ActiveModelFound,
		},
		RedisRuntime: dto.RecommendationAdminRedisOverviewData{
			RecentTTLSeconds: overview.RedisRuntime.RecentTTLSeconds,
			RecentMaxSize:    overview.RedisRuntime.RecentMaxSize,
			BucketEnabled:    overview.RedisRuntime.BucketEnabled,
			BucketTTLSeconds: overview.RedisRuntime.BucketTTLSeconds,
		},
	}
}

func (h *Handler) mapPreviewItems(ctx context.Context, items []videoapp.RecommendResultItem) []dto.RecommendationAdminPreviewItem {
	out := make([]dto.RecommendationAdminPreviewItem, 0, len(items))
	playURLCache := make(map[uint64]string, len(items))
	for i, item := range items {
		title := item.Video.Title
		if item.TitleOverride != "" {
			title = item.TitleOverride
		}
		playURL := playURLCache[item.VideoID]
		if playURL == "" {
			playURL = strings.TrimSpace(h.app.ResolvePlaybackURL(ctx, item.Video))
			playURLCache[item.VideoID] = playURL
		}
		out = append(out, dto.RecommendationAdminPreviewItem{
			Rank:             i + 1,
			QuestionID:       item.QuestionID,
			VideoID:          item.VideoID,
			VideoSegmentID:   item.VideoSegmentID,
			RecommendScore:   item.RecommendScore,
			Strategy:         item.RecommendStrategy,
			ModelVersion:     item.RecommendModelVersion,
			IsWatched:        item.IsWatched,
			WatchDuration:    item.WatchDuration,
			StartTimeSec:     item.StartTimeSec,
			EndTimeSec:       item.EndTimeSec,
			Title:            title,
			CoverURL:         item.Video.CoverURL,
			PlayURL:          playURL,
			UserReacted:      item.UserReacted,
			UserReactionType: string(item.UserReactionType),
		})
	}
	return out
}

func parseOptionalDaysQuery(c *gin.Context) (int, error) {
	raw := strings.TrimSpace(c.Query("days"))
	if raw == "" {
		return 7, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid")
	}
	if value > 90 {
		return 90, nil
	}
	return value, nil
}

func parseRequiredUintQuery(c *gin.Context, name string) (uint64, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, errors.New("missing")
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid")
	}
	return value, nil
}

func parseOptionalLimitQuery(c *gin.Context) (int, error) {
	raw := strings.TrimSpace(c.Query("limit"))
	if raw == "" {
		return normalizeAdminPreviewLimit(0), nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, errors.New("invalid")
	}
	return normalizeAdminPreviewLimit(value), nil
}

func normalizeAdminPreviewLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func ratio(numerator int64, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func writeRecommendationAdminError(c *gin.Context, err error, fallback string) {
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
