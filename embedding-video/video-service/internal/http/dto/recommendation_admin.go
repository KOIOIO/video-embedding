package dto

import "time"

type RecommendationAdminOverviewData struct {
	Engine       string                               `json:"engine"`
	GeneratedAt  time.Time                            `json:"generated_at"`
	PreviewOnly  bool                                 `json:"preview_only"`
	Gorse        RecommendationAdminGorseOverviewData `json:"gorse"`
	RecBole      RecommendationAdminRecBoleData       `json:"recbole"`
	RedisRuntime RecommendationAdminRedisOverviewData `json:"redis"`
}

type RecommendationAdminGorseOverviewData struct {
	Configured        bool `json:"configured"`
	CandidateLimit    int  `json:"candidate_limit"`
	MinRecommendItems int  `json:"min_recommend_items"`
	WriteBackEnabled  bool `json:"write_back_enabled"`
	ShadowMode        bool `json:"shadow_mode"`
}

type RecommendationAdminRecBoleData struct {
	ActiveModelVersion string `json:"active_model_version"`
	ActiveModelFound   bool   `json:"active_model_found"`
}

type RecommendationAdminRedisOverviewData struct {
	RecentTTLSeconds int64 `json:"recent_ttl_seconds"`
	RecentMaxSize    int   `json:"recent_max_size"`
	BucketEnabled    bool  `json:"bucket_enabled"`
	BucketTTLSeconds int64 `json:"bucket_ttl_seconds"`
}

type RecommendationDatasourceStatsData struct {
	VideoTotal              int64   `json:"video_total"`
	PublishedVideos         int64   `json:"published_videos"`
	RecommendVideos         int64   `json:"recommend_videos"`
	SegmentTotal            int64   `json:"segment_total"`
	PlayableSegments        int64   `json:"playable_segments"`
	EmbeddedSegments        int64   `json:"embedded_segments"`
	SegmentEmbeddingRate    float64 `json:"segment_embedding_rate"`
	ExposureTotal           int64   `json:"exposure_total"`
	WatchedExposures        int64   `json:"watched_exposures"`
	ExposureWatchRate       float64 `json:"exposure_watch_rate"`
	RecommendationRows      int64   `json:"recommendation_rows"`
	WatchedRecommendations  int64   `json:"watched_recommendations"`
	RecommendationWatchRate float64 `json:"recommendation_watch_rate"`
	RecBoleUsers            int64   `json:"recbole_users"`
	RecBoleItems            int64   `json:"recbole_items"`
	ReactionRows            int64   `json:"reaction_rows"`
}

type RecommendationDiagnosticsData struct {
	GeneratedAt     time.Time                                `json:"generated_at"`
	Days            int                                      `json:"days"`
	RequestLimit    int                                      `json:"request_limit"`
	Health          []RecommendationDiagnosticCheckData      `json:"health"`
	Freshness       []RecommendationDataFreshnessData        `json:"freshness"`
	RecentRequests  []RecommendationRecentRequestData        `json:"recent_requests"`
	StrategyEffects []RecommendationStrategyEffectMetricData `json:"strategy_effects"`
	Tasks           []RecommendationTaskStatusData           `json:"tasks"`
}

type RecommendationDiagnosticCheckData struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

type RecommendationDataFreshnessData struct {
	Source            string    `json:"source"`
	Label             string    `json:"label"`
	Status            string    `json:"status"`
	LatestAt          time.Time `json:"latest_at,omitempty"`
	HasData           bool      `json:"has_data"`
	AgeSeconds        int64     `json:"age_seconds"`
	StaleAfterSeconds int64     `json:"stale_after_seconds"`
	Detail            string    `json:"detail"`
}

type RecommendationRecentRequestData struct {
	RequestID     string    `json:"request_id"`
	UserID        uint64    `json:"user_id"`
	QuestionID    uint64    `json:"question_id"`
	Exposures     int64     `json:"exposures"`
	Watched       int64     `json:"watched"`
	WatchRate     float64   `json:"watch_rate"`
	Strategy      string    `json:"strategy"`
	ModelVersion  string    `json:"model_version"`
	LastEventTime time.Time `json:"last_event_time"`
}

type RecommendationTaskStatusData struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Detail     string    `json:"detail"`
	LastRunAt  time.Time `json:"last_run_at,omitempty"`
	HasRunTime bool      `json:"has_run_time"`
}

type RecommendationEffectMetricsData struct {
	Days       int                                      `json:"days"`
	Daily      []RecommendationDailyEffectMetricData    `json:"daily"`
	Strategies []RecommendationStrategyEffectMetricData `json:"strategies"`
}

type RecommendationGorsePerformanceData struct {
	Metric           string                                    `json:"metric"`
	Label            string                                    `json:"label"`
	AvailableMetrics []RecommendationGorseMetricData           `json:"available_metrics"`
	Points           []RecommendationGorsePerformancePointData `json:"points"`
}

type RecommendationGorseMetricData struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type RecommendationGorsePerformancePointData struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type RecommendationDailyEffectMetricData struct {
	Day       string  `json:"day"`
	Exposures int64   `json:"exposures"`
	Watched   int64   `json:"watched"`
	WatchRate float64 `json:"watch_rate"`
}

type RecommendationStrategyEffectMetricData struct {
	Strategy     string  `json:"strategy"`
	ModelVersion string  `json:"model_version"`
	Exposures    int64   `json:"exposures"`
	Watched      int64   `json:"watched"`
	WatchRate    float64 `json:"watch_rate"`
	AverageRank  float64 `json:"average_rank"`
	AverageScore float64 `json:"average_score"`
}

type RecommendationTraceData struct {
	Mode         string                         `json:"mode"`
	Engine       string                         `json:"engine"`
	UserID       uint64                         `json:"user_id"`
	QuestionID   uint64                         `json:"question_id"`
	QuestionText string                         `json:"question_text"`
	Limit        int                            `json:"limit"`
	GeneratedAt  time.Time                      `json:"generated_at"`
	PreviewOnly  bool                           `json:"preview_only"`
	Stages       []RecommendationTraceStageData `json:"stages"`
	Items        []RecommendationTraceItemData  `json:"items"`
}

type RecommendationTraceStageData struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type RecommendationTraceItemData struct {
	Rank           int      `json:"rank"`
	QuestionID     uint64   `json:"question_id"`
	VideoID        uint64   `json:"video_id"`
	VideoSegmentID uint64   `json:"video_segment_id"`
	RecommendScore float64  `json:"recommend_score"`
	Strategy       string   `json:"strategy"`
	ModelVersion   string   `json:"model_version"`
	Status         string   `json:"status"`
	Reasons        []string `json:"reasons"`
	StartTimeSec   int      `json:"start_time_sec"`
	EndTimeSec     int      `json:"end_time_sec"`
	Title          string   `json:"title"`
	IsWatched      bool     `json:"is_watched"`
	WatchDuration  int      `json:"watch_duration"`
}

type RecommendationRedisStateData struct {
	UserID      uint64                             `json:"user_id"`
	GeneratedAt time.Time                          `json:"generated_at"`
	Bucket      RecommendationRedisBucketStateData `json:"bucket"`
	Recent      RecommendationRedisRecentStateData `json:"recent"`
}

type RecommendationRedisBucketStateData struct {
	Enabled    bool                                `json:"enabled"`
	Exists     bool                                `json:"exists"`
	TTLSeconds int64                               `json:"ttl_seconds"`
	Count      int64                               `json:"count"`
	MaxSize    int                                 `json:"max_size"`
	MinSize    int                                 `json:"min_size"`
	Items      []RecommendationRedisBucketItemData `json:"items"`
}

type RecommendationRedisBucketItemData struct {
	Rank           int     `json:"rank"`
	VideoID        uint64  `json:"video_id"`
	VideoSegmentID uint64  `json:"video_segment_id"`
	Strategy       string  `json:"strategy"`
	ModelVersion   string  `json:"model_version"`
	Score          float64 `json:"score"`
}

type RecommendationRedisRecentStateData struct {
	Enabled    bool     `json:"enabled"`
	Exists     bool     `json:"exists"`
	TTLSeconds int64    `json:"ttl_seconds"`
	Count      int64    `json:"count"`
	MaxSize    int      `json:"max_size"`
	OverLimit  bool     `json:"over_limit"`
	SegmentIDs []uint64 `json:"segment_ids"`
}

type RecommendationAdminPreviewListData struct {
	Items       []RecommendationAdminPreviewItem `json:"items"`
	Total       int                              `json:"total"`
	PreviewOnly bool                             `json:"preview_only"`
}

type RecommendationAdminPreviewItem struct {
	Rank             int     `json:"rank"`
	QuestionID       uint64  `json:"question_id"`
	VideoID          uint64  `json:"video_id"`
	VideoSegmentID   uint64  `json:"video_segment_id"`
	RecommendScore   float64 `json:"recommend_score"`
	Strategy         string  `json:"strategy"`
	ModelVersion     string  `json:"model_version"`
	IsWatched        bool    `json:"is_watched"`
	WatchDuration    int     `json:"watch_duration"`
	StartTimeSec     int     `json:"start_time_sec"`
	EndTimeSec       int     `json:"end_time_sec"`
	Title            string  `json:"title"`
	CoverURL         string  `json:"cover_url"`
	PlayURL          string  `json:"play_url"`
	UserReacted      bool    `json:"user_reacted"`
	UserReactionType string  `json:"user_reaction_type"`
}
