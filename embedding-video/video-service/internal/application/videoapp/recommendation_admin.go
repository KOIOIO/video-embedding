package videoapp

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
)

type RecommendationAdminOverview struct {
	Engine       string
	GeneratedAt  time.Time
	PreviewOnly  bool
	Gorse        RecommendationAdminGorseOverview
	RecBole      RecommendationAdminRecBoleOverview
	RedisRuntime RecommendationAdminRedisOverview
}

type RecommendationAdminGorseOverview struct {
	Configured        bool
	CandidateLimit    int
	MinRecommendItems int
	WriteBackEnabled  bool
	ShadowMode        bool
}

type RecommendationAdminRecBoleOverview struct {
	ActiveModelVersion string
	ActiveModelFound   bool
}

type RecommendationAdminRedisOverview struct {
	RecentTTLSeconds int64
	RecentMaxSize    int
	BucketEnabled    bool
	BucketTTLSeconds int64
}

type RecommendationDatasourceStats struct {
	VideoTotal             int64
	PublishedVideos        int64
	RecommendVideos        int64
	SegmentTotal           int64
	PlayableSegments       int64
	EmbeddedSegments       int64
	ExposureTotal          int64
	WatchedExposures       int64
	RecommendationRows     int64
	WatchedRecommendations int64
	RecBoleUsers           int64
	RecBoleItems           int64
	ReactionRows           int64
}

type RecommendationEffectMetrics struct {
	Daily      []RecommendationDailyEffectMetric
	Strategies []RecommendationStrategyEffectMetric
}

type RecommendationDailyEffectMetric struct {
	Day       string
	Exposures int64
	Watched   int64
	WatchRate float64
}

type RecommendationStrategyEffectMetric struct {
	Strategy     string
	ModelVersion string
	Exposures    int64
	Watched      int64
	WatchRate    float64
	AverageRank  float64
	AverageScore float64
}

type RecommendationEffectMetricsInput struct {
	Days int
}

var ErrInvalidGorsePerformanceMetric = errors.New("invalid gorse performance metric")

type RecommendationGorsePerformanceInput struct {
	Metric string
	Begin  time.Time
	End    time.Time
}

type RecommendationGorseMetric struct {
	Value string
	Label string
}

type RecommendationGorsePerformancePoint struct {
	Timestamp time.Time
	Value     float64
}

type RecommendationGorsePerformance struct {
	Metric           string
	Label            string
	AvailableMetrics []RecommendationGorseMetric
	Points           []RecommendationGorsePerformancePoint
}

type RecommendationDiagnosticsInput struct {
	Days  int
	Limit int
}

type RecommendationDiagnostics struct {
	GeneratedAt     time.Time
	Days            int
	RequestLimit    int
	Health          []RecommendationDiagnosticCheck
	Freshness       []RecommendationDataFreshness
	RecentRequests  []RecommendationRecentRequest
	StrategyEffects []RecommendationStrategyEffectMetric
	Tasks           []RecommendationTaskStatus
}

type RecommendationDiagnosticCheck struct {
	Key    string
	Name   string
	Status string
	Value  string
	Detail string
}

type RecommendationDataFreshness struct {
	Source            string
	Label             string
	Status            string
	LatestAt          time.Time
	HasData           bool
	AgeSeconds        int64
	StaleAfterSeconds int64
	Detail            string
}

type RecommendationRecentRequest struct {
	RequestID     string
	UserID        uint64
	QuestionID    uint64
	Exposures     int64
	Watched       int64
	WatchRate     float64
	Strategy      string
	ModelVersion  string
	LastEventTime time.Time
}

type RecommendationTaskStatus struct {
	Name       string
	Status     string
	Detail     string
	LastRunAt  time.Time
	HasRunTime bool
}

type RecommendationTrace struct {
	Mode         string
	Engine       string
	UserID       uint64
	QuestionID   uint64
	QuestionText string
	Limit        int
	GeneratedAt  time.Time
	PreviewOnly  bool
	Stages       []RecommendationTraceStage
	Items        []RecommendationTraceItem
}

type RecommendationTraceStage struct {
	Name   string
	Status string
	Detail string
}

type RecommendationTraceItem struct {
	Rank           int
	QuestionID     uint64
	VideoID        uint64
	VideoSegmentID uint64
	RecommendScore float64
	Strategy       string
	ModelVersion   string
	Status         string
	Reasons        []string
	StartTimeSec   int
	EndTimeSec     int
	Title          string
	IsWatched      bool
	WatchDuration  int
}

type RecommendationRedisStateInput struct {
	UserID uint64
}

type RecommendationRedisState struct {
	UserID      uint64
	GeneratedAt time.Time
	Bucket      RecommendationRedisBucketState
	Recent      RecommendationRedisRecentState
}

type RecommendationRedisBucketState struct {
	Enabled    bool
	Exists     bool
	TTLSeconds int64
	Count      int64
	MaxSize    int
	MinSize    int
	Items      []RecommendationRedisBucketItem
}

type RecommendationRedisBucketItem struct {
	Rank           int
	VideoID        uint64
	VideoSegmentID uint64
	Strategy       string
	ModelVersion   string
	Score          float64
}

type RecommendationRedisRecentState struct {
	Enabled    bool
	Exists     bool
	TTLSeconds int64
	Count      int64
	MaxSize    int
	OverLimit  bool
	SegmentIDs []uint64
}

type RecommendationAdminStatsRepository interface {
	GetRecommendationDatasourceStats(ctx context.Context) (RecommendationDatasourceStats, error)
	ListRecommendationEffectMetrics(ctx context.Context, days int) (RecommendationEffectMetrics, error)
}

type RecommendationAdminDiagnosticsRepository interface {
	ListRecommendationRecentRequests(ctx context.Context, limit int) ([]RecommendationRecentRequest, error)
	ListRecommendationDataFreshness(ctx context.Context) ([]RecommendationDataFreshness, error)
}

type randomPlayBucketTTLStore interface {
	TTL(ctx context.Context, userID uint64) (time.Duration, error)
}

type recentSegmentTTLStore interface {
	TTL(ctx context.Context, userID uint64) (time.Duration, error)
}

type recentSegmentMaxSizer interface {
	MaxSize() int
}

func (s *Service) RecommendationAdminOverview(ctx context.Context) (RecommendationAdminOverview, error) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	ttl := s.randomPlayBucketTTL()
	overview := RecommendationAdminOverview{
		Engine:      normalizeAdminRecommendationEngine(s.RecommendationEngine),
		GeneratedAt: now,
		PreviewOnly: true,
		Gorse: RecommendationAdminGorseOverview{
			Configured:        s.GorseClient != nil || s.GorseDashboardClient != nil,
			CandidateLimit:    s.GorseOptions.CandidateLimit,
			MinRecommendItems: s.GorseOptions.MinRecommendItems,
			WriteBackEnabled:  s.GorseOptions.WriteBackEnabled,
			ShadowMode:        s.GorseOptions.ShadowMode,
		},
		RecBole: RecommendationAdminRecBoleOverview{
			ActiveModelVersion: recommendationapp.DefaultRecBoleModelVersion,
		},
		RedisRuntime: RecommendationAdminRedisOverview{
			RecentTTLSeconds: int64(ttl.Seconds()),
			RecentMaxSize:    s.RecentSegmentMaxSize,
			BucketEnabled:    s.RandomPlayBucket != nil,
			BucketTTLSeconds: int64(ttl.Seconds()),
		},
	}

	if s.Repo == nil {
		return overview, nil
	}
	repo, ok := s.Repo.(RecBoleModelVersionRepository)
	if !ok {
		return overview, nil
	}
	version, found, err := repo.GetActiveRecBoleModelVersion(ctx)
	if err != nil {
		return RecommendationAdminOverview{}, err
	}
	version = strings.TrimSpace(version)
	if found && version != "" {
		overview.RecBole.ActiveModelVersion = version
		overview.RecBole.ActiveModelFound = true
	}
	return overview, nil
}

func (s *Service) RecommendationDatasourceStats(ctx context.Context) (RecommendationDatasourceStats, error) {
	repo, ok := s.Repo.(RecommendationAdminStatsRepository)
	if !ok {
		return RecommendationDatasourceStats{}, nil
	}
	return repo.GetRecommendationDatasourceStats(ctx)
}

func (s *Service) RecommendationEffectMetrics(ctx context.Context, input RecommendationEffectMetricsInput) (RecommendationEffectMetrics, error) {
	days := input.Days
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	repo, ok := s.Repo.(RecommendationAdminStatsRepository)
	if !ok {
		return RecommendationEffectMetrics{}, nil
	}
	return repo.ListRecommendationEffectMetrics(ctx, days)
}

func (s *Service) RecommendationGorsePerformance(ctx context.Context, input RecommendationGorsePerformanceInput) (RecommendationGorsePerformance, error) {
	if s.GorseDashboardClient == nil {
		return RecommendationGorsePerformance{}, errors.New("gorse dashboard client is not configured")
	}
	feedbackTypes, err := s.GorseDashboardClient.PositiveFeedbackTypes(ctx)
	if err != nil {
		return RecommendationGorsePerformance{}, err
	}
	metrics := buildGorsePerformanceMetrics(feedbackTypes)
	metric := strings.TrimSpace(input.Metric)
	if metric == "" {
		metric = "positive_feedback_ratio"
	}
	label := ""
	for _, candidate := range metrics {
		if candidate.Value == metric {
			label = candidate.Label
			break
		}
	}
	if label == "" {
		return RecommendationGorsePerformance{}, ErrInvalidGorsePerformanceMetric
	}
	points, err := s.GorseDashboardClient.Timeseries(ctx, metric, input.Begin, input.End)
	if err != nil {
		return RecommendationGorsePerformance{}, err
	}
	out := make([]RecommendationGorsePerformancePoint, 0, len(points))
	for _, point := range points {
		out = append(out, RecommendationGorsePerformancePoint{Timestamp: point.Timestamp, Value: point.Value})
	}
	return RecommendationGorsePerformance{
		Metric:           metric,
		Label:            label,
		AvailableMetrics: metrics,
		Points:           out,
	}, nil
}

func buildGorsePerformanceMetrics(feedbackTypes []string) []RecommendationGorseMetric {
	metrics := []RecommendationGorseMetric{{Value: "positive_feedback_ratio", Label: "正向反馈率（全部）"}}
	seen := map[string]struct{}{"positive_feedback_ratio": {}}
	for _, feedbackType := range feedbackTypes {
		feedbackType = strings.TrimSpace(feedbackType)
		if !safeGorseFeedbackType(feedbackType) {
			continue
		}
		value := "positive_feedback_ratio_" + feedbackType
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		metrics = append(metrics, RecommendationGorseMetric{
			Value: value,
			Label: "正向反馈率（" + formatGorseFeedbackLabel(feedbackType) + "）",
		})
	}
	metrics = append(metrics,
		RecommendationGorseMetric{Value: "cf_ndcg", Label: "协同过滤 · NDCG"},
		RecommendationGorseMetric{Value: "cf_precision", Label: "协同过滤 · Precision"},
		RecommendationGorseMetric{Value: "cf_recall", Label: "协同过滤 · Recall"},
		RecommendationGorseMetric{Value: "ctr_auc", Label: "点击率模型 · AUC"},
		RecommendationGorseMetric{Value: "ctr_precision", Label: "点击率模型 · Precision"},
		RecommendationGorseMetric{Value: "ctr_recall", Label: "点击率模型 · Recall"},
	)
	return metrics
}

func safeGorseFeedbackType(value string) bool {
	if value == "" || strings.ContainsAny(value, "/\\") {
		return false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func formatGorseFeedbackLabel(value string) string {
	words := strings.Split(value, "_")
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func (s *Service) RecommendationDiagnostics(ctx context.Context, input RecommendationDiagnosticsInput) (RecommendationDiagnostics, error) {
	days := normalizeDiagnosticsDays(input.Days)
	limit := normalizeDiagnosticsLimit(input.Limit)
	now := s.now()
	overview, err := s.RecommendationAdminOverview(ctx)
	if err != nil {
		return RecommendationDiagnostics{}, err
	}
	stats, err := s.RecommendationDatasourceStats(ctx)
	if err != nil {
		return RecommendationDiagnostics{}, err
	}
	effects, err := s.RecommendationEffectMetrics(ctx, RecommendationEffectMetricsInput{Days: days})
	if err != nil {
		return RecommendationDiagnostics{}, err
	}
	diagnostics := RecommendationDiagnostics{
		GeneratedAt:     now,
		Days:            days,
		RequestLimit:    limit,
		Health:          buildRecommendationHealthChecks(overview, stats),
		StrategyEffects: effects.Strategies,
		Tasks:           buildRecommendationTaskStatuses(overview, stats, nil),
	}
	repo, ok := s.Repo.(RecommendationAdminDiagnosticsRepository)
	if !ok {
		return diagnostics, nil
	}
	recentRequests, err := repo.ListRecommendationRecentRequests(ctx, limit)
	if err != nil {
		return RecommendationDiagnostics{}, err
	}
	freshness, err := repo.ListRecommendationDataFreshness(ctx)
	if err != nil {
		return RecommendationDiagnostics{}, err
	}
	diagnostics.RecentRequests = recentRequests
	diagnostics.Freshness = enrichRecommendationFreshness(freshness, now)
	diagnostics.Tasks = buildRecommendationTaskStatuses(overview, stats, diagnostics.Freshness)
	return diagnostics, nil
}

func (s *Service) RecommendationTraceRandomPlay(ctx context.Context, input RandomPlayVideoSegmentInput) (RecommendationTrace, error) {
	limit := normalizeAdminTraceLimit(input.Limit)
	now := s.now()
	trace := RecommendationTrace{
		Mode:        "random_play",
		Engine:      normalizeAdminRecommendationEngine(s.RecommendationEngine),
		UserID:      input.UserID,
		Limit:       limit,
		GeneratedAt: now,
		PreviewOnly: true,
		Stages: []RecommendationTraceStage{
			{Name: "input", Status: "ok", Detail: "random-play trace request accepted"},
			{Name: "engine", Status: "ok", Detail: "using " + normalizeAdminRecommendationEngine(s.RecommendationEngine) + " preview path"},
		},
	}
	items, err := s.PreviewRandomPlayVideoSegments(ctx, RandomPlayVideoSegmentInput{
		UserID: input.UserID,
		Limit:  limit,
	})
	if err != nil {
		trace.Stages = append(trace.Stages, RecommendationTraceStage{Name: "preview", Status: "error", Detail: err.Error()})
		return trace, err
	}
	trace.Items = mapTraceItems(items)
	trace.Stages = append(trace.Stages, RecommendationTraceStage{
		Name:   "preview",
		Status: "ok",
		Detail: tracePreviewDetail(len(items)),
	})
	return trace, nil
}

func (s *Service) RecommendationTraceByQuestion(ctx context.Context, input RecommendByQuestionInput) (RecommendationTrace, error) {
	limit := normalizeAdminTraceLimit(input.Limit)
	now := s.now()
	trace := RecommendationTrace{
		Mode:         "by_question",
		Engine:       RecommendStrategyQuestionVector,
		UserID:       input.UserID,
		QuestionID:   input.QuestionID,
		QuestionText: strings.TrimSpace(input.QuestionText),
		Limit:        limit,
		GeneratedAt:  now,
		PreviewOnly:  true,
		Stages: []RecommendationTraceStage{
			{Name: "input", Status: "ok", Detail: "by-question trace request accepted"},
			{Name: "engine", Status: "ok", Detail: "using question_vector preview path"},
		},
	}
	items, err := s.PreviewRecommendByQuestion(ctx, RecommendByQuestionInput{
		QuestionID:   input.QuestionID,
		QuestionText: input.QuestionText,
		UserID:       input.UserID,
		Limit:        limit,
	})
	if err != nil {
		trace.Stages = append(trace.Stages, RecommendationTraceStage{Name: "preview", Status: "error", Detail: err.Error()})
		return trace, err
	}
	trace.Items = mapTraceItems(items)
	trace.Stages = append(trace.Stages, RecommendationTraceStage{
		Name:   "preview",
		Status: "ok",
		Detail: tracePreviewDetail(len(items)),
	})
	return trace, nil
}

func (s *Service) RecommendationRedisState(ctx context.Context, input RecommendationRedisStateInput) (RecommendationRedisState, error) {
	state := RecommendationRedisState{
		UserID:      input.UserID,
		GeneratedAt: s.now(),
		Bucket: RecommendationRedisBucketState{
			Enabled: s.RandomPlayBucket != nil,
			MaxSize: randomPlayBucketMaxSize,
			MinSize: randomPlayBucketMinSize,
		},
		Recent: RecommendationRedisRecentState{
			Enabled: s.RecentSegments != nil,
			MaxSize: s.recentSegmentMaxSize(),
		},
	}
	if input.UserID == 0 {
		return state, nil
	}
	if s.RandomPlayBucket != nil {
		count, err := s.RandomPlayBucket.Len(ctx, input.UserID)
		if err != nil {
			return RecommendationRedisState{}, err
		}
		state.Bucket.Count = count
		items, err := s.RandomPlayBucket.List(ctx, input.UserID)
		if err != nil {
			return RecommendationRedisState{}, err
		}
		state.Bucket.Items = mapRedisBucketItems(items)
		if ttlStore, ok := s.RandomPlayBucket.(randomPlayBucketTTLStore); ok {
			ttl, err := ttlStore.TTL(ctx, input.UserID)
			if err != nil {
				return RecommendationRedisState{}, err
			}
			state.Bucket.TTLSeconds = durationSeconds(ttl)
		}
		state.Bucket.Exists = state.Bucket.Count > 0 || state.Bucket.TTLSeconds > 0
	}
	if s.RecentSegments != nil {
		segmentIDs, err := s.RecentSegments.ListRecent(ctx, input.UserID)
		if err != nil {
			return RecommendationRedisState{}, err
		}
		state.Recent.SegmentIDs = segmentIDs
		state.Recent.Count = int64(len(segmentIDs))
		if ttlStore, ok := s.RecentSegments.(recentSegmentTTLStore); ok {
			ttl, err := ttlStore.TTL(ctx, input.UserID)
			if err != nil {
				return RecommendationRedisState{}, err
			}
			state.Recent.TTLSeconds = durationSeconds(ttl)
		}
		state.Recent.Exists = state.Recent.Count > 0 || state.Recent.TTLSeconds > 0
		state.Recent.OverLimit = state.Recent.MaxSize > 0 && state.Recent.Count > int64(state.Recent.MaxSize)
	}
	return state, nil
}

func (s *Service) PreviewRandomPlayVideoSegments(ctx context.Context, input RandomPlayVideoSegmentInput) ([]RecommendResultItem, error) {
	items, err := newRecommendationService(s).PreviewRandomPlay(ctx, recommendationapp.RandomPlayInput{
		UserID: input.UserID,
		Limit:  input.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := mapRecommendItemsFromApp(items)
	for i := range out {
		item, err := s.withRandomPlayUserReaction(ctx, input.UserID, out[i])
		if err != nil {
			return nil, err
		}
		out[i] = item
	}
	return out, nil
}

func (s *Service) PreviewRecommendByQuestion(ctx context.Context, input RecommendByQuestionInput) ([]RecommendResultItem, error) {
	items, err := newRecommendationService(s).PreviewRecommendByQuestion(ctx, recommendationapp.RecommendByQuestionInput{
		QuestionID:   input.QuestionID,
		QuestionText: input.QuestionText,
		UserID:       input.UserID,
		Limit:        input.Limit,
	})
	if err != nil {
		return nil, err
	}
	return mapRecommendItemsFromApp(items), nil
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) recentSegmentMaxSize() int {
	if maxSizer, ok := s.RecentSegments.(recentSegmentMaxSizer); ok {
		if maxSize := maxSizer.MaxSize(); maxSize > 0 {
			return maxSize
		}
	}
	return s.RecentSegmentMaxSize
}

func normalizeAdminTraceLimit(limit int) int {
	if limit <= 0 {
		return 5
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func tracePreviewDetail(count int) string {
	if count == 0 {
		return "0 candidates returned without persistence"
	}
	return strconv.Itoa(count) + " candidates returned without persistence"
}

func mapTraceItems(items []RecommendResultItem) []RecommendationTraceItem {
	out := make([]RecommendationTraceItem, 0, len(items))
	for i, item := range items {
		title := item.Video.Title
		if item.TitleOverride != "" {
			title = item.TitleOverride
		}
		out = append(out, RecommendationTraceItem{
			Rank:           i + 1,
			QuestionID:     item.QuestionID,
			VideoID:        item.VideoID,
			VideoSegmentID: item.VideoSegmentID,
			RecommendScore: item.RecommendScore,
			Strategy:       item.RecommendStrategy,
			ModelVersion:   item.RecommendModelVersion,
			Status:         "returned",
			Reasons:        []string{"preview_only", "not_persisted"},
			StartTimeSec:   item.StartTimeSec,
			EndTimeSec:     item.EndTimeSec,
			Title:          title,
			IsWatched:      item.IsWatched,
			WatchDuration:  item.WatchDuration,
		})
	}
	return out
}

func mapRedisBucketItems(items []RecommendResultItem) []RecommendationRedisBucketItem {
	out := make([]RecommendationRedisBucketItem, 0, len(items))
	for i, item := range items {
		out = append(out, RecommendationRedisBucketItem{
			Rank:           i + 1,
			VideoID:        item.VideoID,
			VideoSegmentID: item.VideoSegmentID,
			Strategy:       item.RecommendStrategy,
			ModelVersion:   item.RecommendModelVersion,
			Score:          item.RecommendScore,
		})
	}
	return out
}

func durationSeconds(ttl time.Duration) int64 {
	return int64(ttl.Seconds())
}

func normalizeDiagnosticsDays(days int) int {
	if days <= 0 {
		return 14
	}
	if days > 90 {
		return 90
	}
	return days
}

func normalizeDiagnosticsLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func buildRecommendationHealthChecks(overview RecommendationAdminOverview, stats RecommendationDatasourceStats) []RecommendationDiagnosticCheck {
	recboleModelStatus := "warn"
	recboleModelDetail := "no active RecBole model version"
	recboleModelValue := overview.RecBole.ActiveModelVersion
	if overview.RecBole.ActiveModelFound {
		recboleModelStatus = "ok"
		recboleModelDetail = "active model " + overview.RecBole.ActiveModelVersion
	}

	embeddingStatus := "warn"
	embeddingDetail := "RecBole user or item embedding is missing"
	if stats.RecBoleUsers > 0 && stats.RecBoleItems > 0 {
		embeddingStatus = "ok"
		embeddingDetail = "user/item embeddings available"
	}

	segmentStatus := "ok"
	segmentDetail := "playable and embedded segments are available"
	if stats.PlayableSegments == 0 {
		segmentStatus = "error"
		segmentDetail = "no playable segment can be recommended"
	} else if stats.EmbeddedSegments == 0 {
		segmentStatus = "warn"
		segmentDetail = "playable segments exist but embeddings are empty"
	}

	gorseStatus := "warn"
	gorseValue := "not configured"
	gorseDetail := "Gorse client is not configured"
	if overview.Gorse.Configured {
		gorseStatus = "ok"
		gorseValue = "configured"
		gorseDetail = "candidate_limit=" + strconv.Itoa(overview.Gorse.CandidateLimit)
	}

	redisStatus := "warn"
	redisValue := "recent only"
	redisDetail := "random-play bucket is not enabled"
	if overview.RedisRuntime.BucketEnabled {
		redisStatus = "ok"
		redisValue = "bucket enabled"
		redisDetail = "bucket ttl=" + strconv.FormatInt(overview.RedisRuntime.BucketTTLSeconds, 10) + "s"
	}

	return []RecommendationDiagnosticCheck{
		{Key: "recbole_model", Name: "RecBole 模型", Status: recboleModelStatus, Value: recboleModelValue, Detail: recboleModelDetail},
		{Key: "recbole_embeddings", Name: "RecBole Embedding", Status: embeddingStatus, Value: strconv.FormatInt(stats.RecBoleUsers, 10) + " users / " + strconv.FormatInt(stats.RecBoleItems, 10) + " items", Detail: embeddingDetail},
		{Key: "segment_pool", Name: "候选片段池", Status: segmentStatus, Value: strconv.FormatInt(stats.PlayableSegments, 10) + " playable", Detail: segmentDetail},
		{Key: "gorse", Name: "Gorse", Status: gorseStatus, Value: gorseValue, Detail: gorseDetail},
		{Key: "redis_runtime", Name: "Redis Runtime", Status: redisStatus, Value: redisValue, Detail: redisDetail},
	}
}

func enrichRecommendationFreshness(rows []RecommendationDataFreshness, now time.Time) []RecommendationDataFreshness {
	out := make([]RecommendationDataFreshness, 0, len(rows))
	for _, row := range rows {
		row.StaleAfterSeconds = freshnessStaleAfterSeconds(row.Source)
		if !row.HasData {
			row.Status = "warn"
			row.Detail = "no data"
			out = append(out, row)
			continue
		}
		age := now.Sub(row.LatestAt)
		if age < 0 {
			age = 0
		}
		row.AgeSeconds = int64(age.Seconds())
		row.Status = "ok"
		if row.StaleAfterSeconds > 0 && row.AgeSeconds > row.StaleAfterSeconds {
			row.Status = "warn"
		}
		row.Detail = formatAgeDetail(age)
		out = append(out, row)
	}
	return out
}

func freshnessStaleAfterSeconds(source string) int64 {
	switch source {
	case "recbole_user_embedding", "recbole_item_embedding":
		return int64((48 * time.Hour).Seconds())
	default:
		return int64((24 * time.Hour).Seconds())
	}
}

func formatAgeDetail(age time.Duration) string {
	if age < time.Minute {
		return strconv.Itoa(int(age.Seconds())) + "s ago"
	}
	if age < time.Hour {
		return strconv.Itoa(int(age.Minutes())) + "m ago"
	}
	return strconv.Itoa(int(age.Hours())) + "h ago"
}

func buildRecommendationTaskStatuses(overview RecommendationAdminOverview, stats RecommendationDatasourceStats, freshness []RecommendationDataFreshness) []RecommendationTaskStatus {
	recboleStatus := "warn"
	recboleDetail := "active model not found"
	if overview.RecBole.ActiveModelFound {
		recboleStatus = "ok"
		recboleDetail = "active model " + overview.RecBole.ActiveModelVersion
	}
	embeddingStatus := "warn"
	embeddingDetail := "missing user/item embeddings"
	if stats.RecBoleUsers > 0 && stats.RecBoleItems > 0 {
		embeddingStatus = "ok"
		embeddingDetail = "user/item embeddings available"
	}
	gorseStatus := "warn"
	gorseDetail := "Gorse sync status table is not connected"
	if overview.Gorse.Configured {
		gorseStatus = "ok"
		gorseDetail = "Gorse client configured; sync history is read-only pending"
	}
	randomStatus := "warn"
	randomDetail := "no playable random-play candidates"
	if stats.PlayableSegments > 0 {
		randomStatus = "ok"
		randomDetail = "playable random-play candidates available"
	}
	embeddingLatestAt, embeddingHasTime := latestFreshnessTime(freshness, "recbole_item_embedding", "recbole_user_embedding")
	return []RecommendationTaskStatus{
		{Name: "RecBole 模型发布", Status: recboleStatus, Detail: recboleDetail},
		{Name: "RecBole Embedding 导入", Status: embeddingStatus, Detail: embeddingDetail, LastRunAt: embeddingLatestAt, HasRunTime: embeddingHasTime},
		{Name: "Gorse 同步", Status: gorseStatus, Detail: gorseDetail},
		{Name: "Random-play Runtime", Status: randomStatus, Detail: randomDetail},
	}
}

func latestFreshnessTime(rows []RecommendationDataFreshness, sources ...string) (time.Time, bool) {
	sourceSet := make(map[string]bool, len(sources))
	for _, source := range sources {
		sourceSet[source] = true
	}
	var latest time.Time
	var found bool
	for _, row := range rows {
		if !sourceSet[row.Source] || !row.HasData {
			continue
		}
		if !found || row.LatestAt.After(latest) {
			latest = row.LatestAt
			found = true
		}
	}
	return latest, found
}

func normalizeAdminRecommendationEngine(engine string) string {
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine == "" {
		return recommendationapp.EngineKnowledgeMatch
	}
	return engine
}
