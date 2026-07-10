package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultHTTPAddr                    = ":8081"
	defaultHTTPShutdownTimeoutSec      = 30
	defaultHTTPLogDir                  = "logs"
	defaultHTTPSlowRequestMs           = 1000
	defaultCORSAllowOrigin             = "*"
	defaultCORSAllowMethods            = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	defaultCORSAllowHeaders            = "Origin, Content-Type, Accept, Authorization, X-Requested-With"
	defaultCORSExposeHeaders           = "Content-Length, Content-Type"
	defaultCORSMaxAge                  = "86400"
	defaultRawObjectPrefix             = "raw"
	defaultHLSObjectPrefix             = "hls"
	defaultMediaRoutePrefix            = "/videos"
	defaultRawURLPrefix                = "/videos/raw"
	defaultHLSURLPrefix                = "/videos/hls"
	defaultCoverURLPrefix              = "/videos"
	defaultHLSMasterName               = "master.m3u8"
	defaultTranscodeQueueKey           = "video:transcode:queue"
	defaultVectorizeQueueKey           = "video:vectorize:queue"
	defaultVectorPrepareQueueKey       = "video:vector:prepare"
	defaultVectorCoarseQueueKey        = "video:vector:coarse"
	defaultVectorRefineQueueKey        = "video:vector:refine"
	defaultVectorFinalizeQueueKey      = "video:vector:finalize"
	defaultVideoReactionQueueKey       = "video:reaction:queue"
	defaultVideoReactionCountsPrefix   = "video:reaction:counts:"
	defaultVideoReactionUserPrefix     = "video:reaction:user:"
	defaultSegmentReactionQueueKey     = "segment:reaction:queue"
	defaultSegmentReactionCountsPrefix = "segment:reaction:counts:"
	defaultSegmentReactionUserPrefix   = "segment:reaction:user:"
	defaultTranscodeStatusPrefix       = "video:transcode:status:"
	defaultRuntimeActiveCounterPrefix  = "video:runtime:active:"
	defaultRandomPlayRecentPrefix      = "video:random_play:recent:"
	defaultRandomPlayBucketPrefix      = "video:random_play:bucket:"
	defaultEmbeddingDim                = 1536
	defaultAIProvider                  = "legacy"
	defaultDashscopeCompatBaseURL      = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	defaultDashscopeWSURL              = "wss://dashscope.aliyuncs.com/api-ws/v1/inference/"
	defaultRecommendationEngine        = "knowledge_match"
	defaultRandomPlayDedupeWindowSec   = 1800
	defaultRandomPlayRecentMaxSize     = 200
	defaultGorseEndpoint               = "http://localhost:8087"
	defaultGorseTimeoutSeconds         = 2
	defaultGorseCandidateLimit         = 100
	defaultGorseSyncIntervalMins       = 60
	defaultGorseDataRetentionDays      = 30
)

func HTTPAddr(cfg Config) string {
	if v := strings.TrimSpace(cfg.HTTP.Addr); v != "" {
		return v
	}
	return defaultHTTPAddr
}

func HTTPShutdownTimeout(cfg Config) time.Duration {
	seconds := cfg.HTTP.ShutdownTimeoutSec
	if seconds <= 0 {
		seconds = defaultHTTPShutdownTimeoutSec
	}
	return time.Duration(seconds) * time.Second
}

func HTTPLogDir(cfg Config) string {
	return firstConfigValue(cfg.HTTP.LogDir, defaultHTTPLogDir)
}

func LogDir(cfg Config) string {
	return HTTPLogDir(cfg)
}

func HTTPSlowRequestThreshold(cfg Config) time.Duration {
	ms := cfg.HTTP.SlowRequestMs
	if ms <= 0 {
		ms = defaultHTTPSlowRequestMs
	}
	return time.Duration(ms) * time.Millisecond
}

func CORSAllowOrigin(cfg Config) string {
	return firstConfigValue(cfg.HTTP.CORS.AllowOrigin, defaultCORSAllowOrigin)
}

func CORSAllowMethods(cfg Config) string {
	return firstConfigValue(cfg.HTTP.CORS.AllowMethods, defaultCORSAllowMethods)
}

func CORSAllowHeaders(cfg Config) string {
	return firstConfigValue(cfg.HTTP.CORS.AllowHeaders, defaultCORSAllowHeaders)
}

func CORSExposeHeaders(cfg Config) string {
	return firstConfigValue(cfg.HTTP.CORS.ExposeHeaders, defaultCORSExposeHeaders)
}

func CORSMaxAge(cfg Config) string {
	return firstConfigValue(cfg.HTTP.CORS.MaxAge, defaultCORSMaxAge)
}

func RawPath(cfg Config) string {
	return firstConfigValue(cfg.Video.RawPath, filepath.Join(os.TempDir(), "embedding-video", "tmp", "raw"))
}

func HLSPath(cfg Config) string {
	return firstConfigValue(cfg.Video.HlsPath, filepath.Join(os.TempDir(), "embedding-video", "tmp", "hls"))
}

func VectorTempPath(cfg Config) string {
	return firstConfigValue(cfg.Storage.VectorTempPath, filepath.Join(os.TempDir(), "embedding-video", "tmp", "video_vectorize"))
}

func RawObjectPrefix(cfg Config) string {
	return cleanObjectPrefix(firstConfigValue(cfg.Storage.RawObjectPrefix, defaultRawObjectPrefix))
}

func HLSObjectPrefix(cfg Config) string {
	return cleanObjectPrefix(firstConfigValue(cfg.Storage.HLSObjectPrefix, defaultHLSObjectPrefix))
}

func MediaRoutePrefix(cfg Config) string {
	return cleanURLPrefix(firstConfigValue(cfg.Storage.MediaRoutePrefix, defaultMediaRoutePrefix))
}

func RawURLPrefix(cfg Config) string {
	return cleanURLPrefix(firstConfigValue(cfg.Storage.RawURLPrefix, defaultRawURLPrefix))
}

func HLSURLPrefix(cfg Config) string {
	return cleanURLPrefix(firstConfigValue(cfg.Storage.HLSURLPrefix, defaultHLSURLPrefix))
}

func CoverURLPrefix(cfg Config) string {
	return cleanURLPrefix(firstConfigValue(cfg.Storage.CoverURLPrefix, defaultCoverURLPrefix))
}

func HLSMasterName(cfg Config) string {
	return firstConfigValue(cfg.FFmpeg.HLS.MasterName, defaultHLSMasterName)
}

func TranscodeQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.TranscodeQueue, defaultTranscodeQueueKey)
}

func VectorizeQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VectorizeQueue, defaultVectorizeQueueKey)
}

func VectorPrepareQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VectorPrepareQueue, defaultVectorPrepareQueueKey)
}

func VectorCoarseQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VectorCoarseQueue, defaultVectorCoarseQueueKey)
}

func VectorRefineQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VectorRefineQueue, defaultVectorRefineQueueKey)
}

func VectorFinalizeQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VectorFinalizeQueue, defaultVectorFinalizeQueueKey)
}

func VideoReactionQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VideoReactionQueue, defaultVideoReactionQueueKey)
}

func VideoReactionCountsPrefix(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VideoReactionCounts, defaultVideoReactionCountsPrefix)
}

func VideoReactionUserPrefix(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.VideoReactionUser, defaultVideoReactionUserPrefix)
}

func SegmentReactionQueueKey(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.SegmentReactionQueue, defaultSegmentReactionQueueKey)
}

func SegmentReactionCountsPrefix(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.SegmentReactionCounts, defaultSegmentReactionCountsPrefix)
}

func SegmentReactionUserPrefix(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.SegmentReactionUser, defaultSegmentReactionUserPrefix)
}

func TranscodeStatusPrefix(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.TranscodeStatus, defaultTranscodeStatusPrefix)
}

func RuntimeActiveCounterPrefix(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.RuntimeActiveCounter, defaultRuntimeActiveCounterPrefix)
}

func RandomPlayRecentPrefix(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.RandomPlayRecent, defaultRandomPlayRecentPrefix)
}

func RandomPlayBucketPrefix(cfg Config) string {
	return firstConfigValue(cfg.RedisKeys.RandomPlayBucket, defaultRandomPlayBucketPrefix)
}

func EmbeddingDim(cfg Config) int {
	if cfg.AI.EmbeddingDim > 0 {
		return cfg.AI.EmbeddingDim
	}
	return defaultEmbeddingDim
}

func AIProvider(cfg Config) string {
	provider := strings.ToLower(strings.TrimSpace(cfg.AI.Provider))
	if provider == "" {
		return defaultAIProvider
	}
	return provider
}

func RecommendationEngine(cfg Config) string {
	engine := strings.ToLower(strings.TrimSpace(cfg.Recommendation.Engine))
	if engine == "" {
		return defaultRecommendationEngine
	}
	return engine
}

func RandomPlayDedupeWindow(cfg Config) time.Duration {
	seconds := cfg.Recommendation.RandomPlayDedupeWindowSec
	if seconds <= 0 {
		seconds = defaultRandomPlayDedupeWindowSec
	}
	return time.Duration(seconds) * time.Second
}

func RandomPlayRecentMaxSize(cfg Config) int {
	if cfg.Recommendation.RandomPlayRecentMaxSize > 0 {
		return cfg.Recommendation.RandomPlayRecentMaxSize
	}
	return defaultRandomPlayRecentMaxSize
}

func GorseEndpoint(cfg Config) string {
	return strings.TrimRight(firstConfigValue(cfg.Gorse.Endpoint, defaultGorseEndpoint), "/")
}

func GorseTimeout(cfg Config) time.Duration {
	seconds := cfg.Gorse.TimeoutSeconds
	if seconds <= 0 {
		seconds = defaultGorseTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func GorseCandidateLimit(cfg Config) int {
	if cfg.Gorse.CandidateLimit > 0 {
		return cfg.Gorse.CandidateLimit
	}
	return defaultGorseCandidateLimit
}

func GorseSyncInterval(cfg Config) time.Duration {
	minutes := cfg.Gorse.SyncIntervalMins
	if minutes <= 0 {
		minutes = defaultGorseSyncIntervalMins
	}
	return time.Duration(minutes) * time.Minute
}

func GorseDataTTL(cfg Config) time.Duration {
	days := cfg.Gorse.DataRetentionDays
	if days <= 0 {
		days = defaultGorseDataRetentionDays
	}
	return time.Duration(days) * 24 * time.Hour
}

func DashscopeCompatBaseURL() string {
	return defaultDashscopeCompatBaseURL
}

func ASRWSURL(cfg Config) string {
	return firstConfigValue(os.Getenv("ASR_WS_URL"), cfg.ASR.WSURL, defaultDashscopeWSURL)
}

func firstConfigValue(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

func cleanObjectPrefix(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	return value
}

func cleanURLPrefix(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "/") {
		return value
	}
	return "/" + value
}
