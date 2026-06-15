package config

import (
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
	if got := EmbeddingDim(cfg); got != 1536 {
		t.Fatalf("EmbeddingDim() = %d, want %d", got, 1536)
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
		},
		AI: AIConfig{EmbeddingDim: 768},
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
	if got := EmbeddingDim(cfg); got != 768 {
		t.Fatalf("EmbeddingDim() = %d, want %d", got, 768)
	}
}
