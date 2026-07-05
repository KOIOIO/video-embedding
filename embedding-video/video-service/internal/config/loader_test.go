package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfigPathUsesLocalConfigOnDarwin(t *testing.T) {
	originalGOOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() {
		runtimeGOOS = originalGOOS
	})

	t.Setenv("CONFIG_FILE", "")
	t.Setenv("VIDEO_CONFIG_FILE", "")

	if got := DefaultConfigPath(); got != defaultConfigPath {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, defaultConfigPath)
	}
}

func TestDefaultConfigPathPrefersExplicitEnv(t *testing.T) {
	originalGOOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() {
		runtimeGOOS = originalGOOS
	})

	t.Setenv("CONFIG_FILE", "configs/custom.yml")
	t.Setenv("VIDEO_CONFIG_FILE", "configs/ignored.yml")

	if got := DefaultConfigPath(); got != "configs/custom.yml" {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, "configs/custom.yml")
	}
}

func TestRuntimeConfigDefaultsPreserveExistingValues(t *testing.T) {
	cfg := Config{}

	if got := HTTPAddr(cfg); got != ":8081" {
		t.Fatalf("HTTPAddr() = %q, want %q", got, ":8081")
	}
	if got := HTTPShutdownTimeout(cfg); got != 30*time.Second {
		t.Fatalf("HTTPShutdownTimeout() = %s, want %s", got, 30*time.Second)
	}
	if got := HTTPLogDir(cfg); got != "logs" {
		t.Fatalf("HTTPLogDir() = %q, want %q", got, "logs")
	}
	if got := HTTPSlowRequestThreshold(cfg); got != time.Second {
		t.Fatalf("HTTPSlowRequestThreshold() = %s, want %s", got, time.Second)
	}
	if got := CORSAllowOrigin(cfg); got != "*" {
		t.Fatalf("CORSAllowOrigin() = %q, want %q", got, "*")
	}
	if got := RawObjectPrefix(cfg); got != "raw" {
		t.Fatalf("RawObjectPrefix() = %q, want %q", got, "raw")
	}
	if got := HLSObjectPrefix(cfg); got != "hls" {
		t.Fatalf("HLSObjectPrefix() = %q, want %q", got, "hls")
	}
	if got := MediaRoutePrefix(cfg); got != "/videos" {
		t.Fatalf("MediaRoutePrefix() = %q, want %q", got, "/videos")
	}
	if got := RawURLPrefix(cfg); got != "/videos/raw" {
		t.Fatalf("RawURLPrefix() = %q, want %q", got, "/videos/raw")
	}
	if got := HLSURLPrefix(cfg); got != "/videos/hls" {
		t.Fatalf("HLSURLPrefix() = %q, want %q", got, "/videos/hls")
	}
	if got := HLSMasterName(cfg); got != "master.m3u8" {
		t.Fatalf("HLSMasterName() = %q, want %q", got, "master.m3u8")
	}
	if got := TranscodeQueueKey(cfg); got != "video:transcode:queue" {
		t.Fatalf("TranscodeQueueKey() = %q, want %q", got, "video:transcode:queue")
	}
	if got := VideoReactionCountsPrefix(cfg); got != "video:reaction:counts:" {
		t.Fatalf("VideoReactionCountsPrefix() = %q, want %q", got, "video:reaction:counts:")
	}
	if got := VideoReactionUserPrefix(cfg); got != "video:reaction:user:" {
		t.Fatalf("VideoReactionUserPrefix() = %q, want %q", got, "video:reaction:user:")
	}
	if got := SegmentReactionQueueKey(cfg); got != "segment:reaction:queue" {
		t.Fatalf("SegmentReactionQueueKey() = %q, want %q", got, "segment:reaction:queue")
	}
	if got := SegmentReactionCountsPrefix(cfg); got != "segment:reaction:counts:" {
		t.Fatalf("SegmentReactionCountsPrefix() = %q, want %q", got, "segment:reaction:counts:")
	}
	if got := SegmentReactionUserPrefix(cfg); got != "segment:reaction:user:" {
		t.Fatalf("SegmentReactionUserPrefix() = %q, want %q", got, "segment:reaction:user:")
	}
	if got := RandomPlayRecentPrefix(cfg); got != "video:random_play:recent:" {
		t.Fatalf("RandomPlayRecentPrefix() = %q, want %q", got, "video:random_play:recent:")
	}
	if got := EmbeddingDim(cfg); got != 1536 {
		t.Fatalf("EmbeddingDim() = %d, want %d", got, 1536)
	}
	if got := AIProvider(cfg); got != "legacy" {
		t.Fatalf("AIProvider() = %q, want %q", got, "legacy")
	}
	if got := RecommendationEngine(cfg); got != "knowledge_match" {
		t.Fatalf("RecommendationEngine() = %q, want %q", got, "knowledge_match")
	}
	if got := RandomPlayDedupeWindow(cfg); got != 30*time.Minute {
		t.Fatalf("RandomPlayDedupeWindow() = %s, want %s", got, 30*time.Minute)
	}
	if got := GorseEndpoint(cfg); got != "http://localhost:8087" {
		t.Fatalf("GorseEndpoint() = %q, want %q", got, "http://localhost:8087")
	}
	if got := GorseTimeout(cfg); got != 2*time.Second {
		t.Fatalf("GorseTimeout() = %s, want %s", got, 2*time.Second)
	}
	if got := GorseCandidateLimit(cfg); got != 100 {
		t.Fatalf("GorseCandidateLimit() = %d, want 100", got)
	}
	if got := GorseSyncInterval(cfg); got != time.Hour {
		t.Fatalf("GorseSyncInterval() = %s, want %s", got, time.Hour)
	}
	if got := GorseDataTTL(cfg); got != 30*24*time.Hour {
		t.Fatalf("GorseDataTTL() = %s, want %s", got, 30*24*time.Hour)
	}
}

func TestRuntimeConfigUsesExplicitValues(t *testing.T) {
	cfg := Config{
		HTTP: HTTPConfig{
			Addr:               ":9092",
			ShutdownTimeoutSec: 45,
			LogDir:             "var/log/video",
			SlowRequestMs:      2500,
			CORS: CORSConfig{
				AllowOrigin:   "https://example.com",
				AllowMethods:  "GET, OPTIONS",
				AllowHeaders:  "Content-Type",
				ExposeHeaders: "Content-Length",
				MaxAge:        "60",
			},
		},
		Storage: StorageConfig{
			RawObjectPrefix:  " source ",
			HLSObjectPrefix:  "/stream/",
			MediaRoutePrefix: "/media",
			RawURLPrefix:     "media/source/",
			HLSURLPrefix:     "/media/stream/",
			CoverURLPrefix:   "/media",
			VectorTempPath:   "/var/tmp/vector",
		},
		FFmpeg: FFmpegConfig{HLS: FFmpegHLSConfig{MasterName: "playlist.m3u8"}},
		RedisKeys: RedisKeysConfig{
			TranscodeQueue:        "custom:transcode",
			VectorizeQueue:        "custom:vectorize",
			VideoReactionQueue:    "custom:reaction",
			VideoReactionCounts:   "custom:reaction:counts:",
			VideoReactionUser:     "custom:reaction:user:",
			SegmentReactionQueue:  "custom:segment:reaction",
			SegmentReactionCounts: "custom:segment:reaction:counts:",
			SegmentReactionUser:   "custom:segment:reaction:user:",
			TranscodeStatus:       "custom:status:",
			RuntimeActiveCounter:  "custom:active:",
			RandomPlayRecent:      "custom:random:recent:",
		},
		AI: AIConfig{
			EmbeddingDim: 768,
			Provider:     "eino",
		},
		Recommendation: RecommendationConfig{
			Engine:                    "two_tower",
			RandomPlayDedupeWindowSec: 600,
		},
		Gorse: GorseConfig{
			Endpoint:          " http://gorse:8087/ ",
			APIKey:            "secret",
			TimeoutSeconds:    5,
			ShadowMode:        true,
			SyncEnabled:       true,
			WriteBackEnabled:  true,
			CandidateLimit:    120,
			SyncIntervalMins:  15,
			EnableGate:        false,
			MinFeedbackCount:  80,
			MinRecommendItems: 10,
			CleanupEnabled:    false,
			DataRetentionDays: 7,
		},
	}

	if got := HTTPAddr(cfg); got != ":9092" {
		t.Fatalf("HTTPAddr() = %q, want %q", got, ":9092")
	}
	if got := HTTPShutdownTimeout(cfg); got != 45*time.Second {
		t.Fatalf("HTTPShutdownTimeout() = %s, want %s", got, 45*time.Second)
	}
	if got := HTTPLogDir(cfg); got != "var/log/video" {
		t.Fatalf("HTTPLogDir() = %q, want %q", got, "var/log/video")
	}
	if got := HTTPSlowRequestThreshold(cfg); got != 2500*time.Millisecond {
		t.Fatalf("HTTPSlowRequestThreshold() = %s, want %s", got, 2500*time.Millisecond)
	}
	if got := CORSAllowOrigin(cfg); got != "https://example.com" {
		t.Fatalf("CORSAllowOrigin() = %q, want %q", got, "https://example.com")
	}
	if got := CORSAllowMethods(cfg); got != "GET, OPTIONS" {
		t.Fatalf("CORSAllowMethods() = %q, want %q", got, "GET, OPTIONS")
	}
	if got := CORSAllowHeaders(cfg); got != "Content-Type" {
		t.Fatalf("CORSAllowHeaders() = %q, want %q", got, "Content-Type")
	}
	if got := CORSExposeHeaders(cfg); got != "Content-Length" {
		t.Fatalf("CORSExposeHeaders() = %q, want %q", got, "Content-Length")
	}
	if got := CORSMaxAge(cfg); got != "60" {
		t.Fatalf("CORSMaxAge() = %q, want %q", got, "60")
	}
	if got := RawObjectPrefix(cfg); got != "source" {
		t.Fatalf("RawObjectPrefix() = %q, want %q", got, "source")
	}
	if got := HLSObjectPrefix(cfg); got != "stream" {
		t.Fatalf("HLSObjectPrefix() = %q, want %q", got, "stream")
	}
	if got := MediaRoutePrefix(cfg); got != "/media" {
		t.Fatalf("MediaRoutePrefix() = %q, want %q", got, "/media")
	}
	if got := RawURLPrefix(cfg); got != "/media/source" {
		t.Fatalf("RawURLPrefix() = %q, want %q", got, "/media/source")
	}
	if got := HLSURLPrefix(cfg); got != "/media/stream" {
		t.Fatalf("HLSURLPrefix() = %q, want %q", got, "/media/stream")
	}
	if got := VectorTempPath(cfg); got != "/var/tmp/vector" {
		t.Fatalf("VectorTempPath() = %q, want %q", got, "/var/tmp/vector")
	}
	if got := HLSMasterName(cfg); got != "playlist.m3u8" {
		t.Fatalf("HLSMasterName() = %q, want %q", got, "playlist.m3u8")
	}
	if got := TranscodeQueueKey(cfg); got != "custom:transcode" {
		t.Fatalf("TranscodeQueueKey() = %q, want %q", got, "custom:transcode")
	}
	if got := VectorizeQueueKey(cfg); got != "custom:vectorize" {
		t.Fatalf("VectorizeQueueKey() = %q, want %q", got, "custom:vectorize")
	}
	if got := VideoReactionQueueKey(cfg); got != "custom:reaction" {
		t.Fatalf("VideoReactionQueueKey() = %q, want %q", got, "custom:reaction")
	}
	if got := VideoReactionCountsPrefix(cfg); got != "custom:reaction:counts:" {
		t.Fatalf("VideoReactionCountsPrefix() = %q, want %q", got, "custom:reaction:counts:")
	}
	if got := VideoReactionUserPrefix(cfg); got != "custom:reaction:user:" {
		t.Fatalf("VideoReactionUserPrefix() = %q, want %q", got, "custom:reaction:user:")
	}
	if got := SegmentReactionQueueKey(cfg); got != "custom:segment:reaction" {
		t.Fatalf("SegmentReactionQueueKey() = %q, want %q", got, "custom:segment:reaction")
	}
	if got := SegmentReactionCountsPrefix(cfg); got != "custom:segment:reaction:counts:" {
		t.Fatalf("SegmentReactionCountsPrefix() = %q, want %q", got, "custom:segment:reaction:counts:")
	}
	if got := SegmentReactionUserPrefix(cfg); got != "custom:segment:reaction:user:" {
		t.Fatalf("SegmentReactionUserPrefix() = %q, want %q", got, "custom:segment:reaction:user:")
	}
	if got := TranscodeStatusPrefix(cfg); got != "custom:status:" {
		t.Fatalf("TranscodeStatusPrefix() = %q, want %q", got, "custom:status:")
	}
	if got := RuntimeActiveCounterPrefix(cfg); got != "custom:active:" {
		t.Fatalf("RuntimeActiveCounterPrefix() = %q, want %q", got, "custom:active:")
	}
	if got := RandomPlayRecentPrefix(cfg); got != "custom:random:recent:" {
		t.Fatalf("RandomPlayRecentPrefix() = %q, want %q", got, "custom:random:recent:")
	}
	if got := EmbeddingDim(cfg); got != 768 {
		t.Fatalf("EmbeddingDim() = %d, want %d", got, 768)
	}
	if got := AIProvider(cfg); got != "eino" {
		t.Fatalf("AIProvider() = %q, want %q", got, "eino")
	}
	if got := RecommendationEngine(cfg); got != "two_tower" {
		t.Fatalf("RecommendationEngine() = %q, want %q", got, "two_tower")
	}
	if got := RandomPlayDedupeWindow(cfg); got != 10*time.Minute {
		t.Fatalf("RandomPlayDedupeWindow() = %s, want %s", got, 10*time.Minute)
	}
	if got := GorseEndpoint(cfg); got != "http://gorse:8087" {
		t.Fatalf("GorseEndpoint() = %q, want %q", got, "http://gorse:8087")
	}
	if got := cfg.Gorse.APIKey; got != "secret" {
		t.Fatalf("Gorse APIKey = %q, want %q", got, "secret")
	}
	if got := GorseTimeout(cfg); got != 5*time.Second {
		t.Fatalf("GorseTimeout() = %s, want %s", got, 5*time.Second)
	}
	if !cfg.Gorse.ShadowMode {
		t.Fatal("Gorse ShadowMode = false, want true")
	}
	if !cfg.Gorse.SyncEnabled {
		t.Fatal("Gorse SyncEnabled = false, want true")
	}
	if !cfg.Gorse.WriteBackEnabled {
		t.Fatal("Gorse WriteBackEnabled = false, want true")
	}
	if got := GorseCandidateLimit(cfg); got != 120 {
		t.Fatalf("GorseCandidateLimit() = %d, want 120", got)
	}
	if got := GorseSyncInterval(cfg); got != 15*time.Minute {
		t.Fatalf("GorseSyncInterval() = %s, want %s", got, 15*time.Minute)
	}
	if cfg.Gorse.EnableGate {
		t.Fatal("Gorse EnableGate = true, want false")
	}
	if got := cfg.Gorse.MinFeedbackCount; got != 80 {
		t.Fatalf("Gorse MinFeedbackCount = %d, want 80", got)
	}
	if got := cfg.Gorse.MinRecommendItems; got != 10 {
		t.Fatalf("Gorse MinRecommendItems = %d, want 10", got)
	}
	if cfg.Gorse.CleanupEnabled {
		t.Fatal("Gorse CleanupEnabled = true, want false")
	}
	if got := GorseDataTTL(cfg); got != 7*24*time.Hour {
		t.Fatalf("GorseDataTTL() = %s, want %s", got, 7*24*time.Hour)
	}
}

func TestMustLoadParsesCOSObjectStorageFields(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "video.yml")
	data := []byte(`
RustFS:
  Endpoint: "cos.ap-beijing.myqcloud.com"
  AccessKey: "ak"
  SecretKey: "sk"
  Bucket: "video-embedding-storage"
  UseSSL: true
  Region: "ap-beijing"
  BucketLookup: "dns"
`)
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := MustLoad(cfgPath)

	if cfg.RustFS.Region != "ap-beijing" {
		t.Fatalf("Region = %q, want ap-beijing", cfg.RustFS.Region)
	}
	if cfg.RustFS.BucketLookup != "dns" {
		t.Fatalf("BucketLookup = %q, want dns", cfg.RustFS.BucketLookup)
	}
}

func TestMustLoadAppliesSensitiveEnvOverrides(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "video.yml")
	data := []byte(`
Redis:
  Password: "file-redis"
Postgres:
  DSN: "file-postgres"
RustFS:
  AccessKey: "file-ak"
  SecretKey: "file-sk"
Gorse:
  APIKey: "file-gorse"
`)
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("POSTGRES_DSN", "env-postgres")
	t.Setenv("REDIS_PASSWORD", "env-redis")
	t.Setenv("COS_SECRET_ID", "env-ak")
	t.Setenv("COS_SECRET_KEY", "env-sk")
	t.Setenv("GORSE_API_KEY", "env-gorse")

	cfg := MustLoad(cfgPath)

	if cfg.Postgres.DSN != "env-postgres" {
		t.Fatalf("Postgres.DSN = %q, want env override", cfg.Postgres.DSN)
	}
	if cfg.Redis.Password != "env-redis" {
		t.Fatalf("Redis.Password = %q, want env override", cfg.Redis.Password)
	}
	if cfg.RustFS.AccessKey != "env-ak" || cfg.RustFS.SecretKey != "env-sk" {
		t.Fatalf("RustFS credentials = %q/%q, want env overrides", cfg.RustFS.AccessKey, cfg.RustFS.SecretKey)
	}
	if cfg.Gorse.APIKey != "env-gorse" {
		t.Fatalf("Gorse.APIKey = %q, want env override", cfg.Gorse.APIKey)
	}
}

func TestMustLoadAppliesDotEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "video.yml")
	data := []byte(`
Postgres:
  DSN: ""
RustFS:
  AccessKey: ""
  SecretKey: ""
Gorse:
  APIKey: ""
`)
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("HSTV_ENV_FILE=.env.local\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".env.local"), []byte(`
POSTGRES_DSN=dotenv-postgres
COS_SECRET_ID=dotenv-ak
COS_SECRET_KEY=dotenv-sk
GORSE_API_KEY=dotenv-gorse
`), 0o600); err != nil {
		t.Fatalf("write .env.local: %v", err)
	}

	cleanupEnv := cleanEnv(t, "HSTV_ENV_FILE", "POSTGRES_DSN", "COS_SECRET_ID", "COS_SECRET_KEY", "GORSE_API_KEY")
	defer cleanupEnv()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	cfg := MustLoad(cfgPath)

	if cfg.Postgres.DSN != "dotenv-postgres" {
		t.Fatalf("Postgres.DSN = %q, want dotenv override", cfg.Postgres.DSN)
	}
	if cfg.RustFS.AccessKey != "dotenv-ak" || cfg.RustFS.SecretKey != "dotenv-sk" {
		t.Fatalf("RustFS credentials = %q/%q, want dotenv overrides", cfg.RustFS.AccessKey, cfg.RustFS.SecretKey)
	}
	if cfg.Gorse.APIKey != "dotenv-gorse" {
		t.Fatalf("Gorse.APIKey = %q, want dotenv override", cfg.Gorse.APIKey)
	}
}

func TestObjectStorageConfigPassesCOSFieldsAndEnvFallback(t *testing.T) {
	t.Setenv("COS_SECRET_ID", "env-ak")
	t.Setenv("COS_SECRET_KEY", "env-sk")

	got := ObjectStorageConfig(Config{
		RustFS: RustFSConfig{
			Endpoint:     "cos.ap-beijing.myqcloud.com",
			Bucket:       "video-embedding-storage",
			UseSSL:       true,
			Region:       "ap-beijing",
			BucketLookup: "dns",
		},
	})

	if got.AccessKey != "env-ak" || got.SecretKey != "env-sk" {
		t.Fatalf("credentials = %q/%q, want env fallback", got.AccessKey, got.SecretKey)
	}
	if got.Region != "ap-beijing" || got.BucketLookup != "dns" {
		t.Fatalf("COS fields = region %q lookup %q, want ap-beijing/dns", got.Region, got.BucketLookup)
	}
}

func cleanEnv(t *testing.T, names ...string) func() {
	t.Helper()
	originals := make(map[string]string, len(names))
	present := make(map[string]bool, len(names))
	for _, name := range names {
		value, ok := os.LookupEnv(name)
		originals[name] = value
		present[name] = ok
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
	return func() {
		for _, name := range names {
			if present[name] {
				_ = os.Setenv(name, originals[name])
			} else {
				_ = os.Unsetenv(name)
			}
		}
	}
}
