package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"

	"nlp-video-analysis/internal/config"
)

func newTestRunner(t *testing.T) (*runner, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	cfg := config.Config{
		RedisKeys: config.RedisKeysConfig{
			TranscodeQueue:      "test:transcode",
			VectorCoarseQueue:   "test:vector:coarse",
			VectorFinalizeQueue: "test:vector:finalize",
		},
	}
	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}
	return &runner{cfg: cfg, rdb: rdb}, cleanup
}

func TestQueueSpecsIncludeConfiguredQueues(t *testing.T) {
	specs := queueSpecs(config.Config{
		RedisKeys: config.RedisKeysConfig{
			TranscodeQueue:    "custom:transcode",
			VectorCoarseQueue: "custom:coarse",
		},
	})

	transcode, ok := specs["transcode"]
	if !ok {
		t.Fatal("missing transcode queue")
	}
	if transcode.StreamKey != "custom:transcode" {
		t.Fatalf("transcode key = %q, want custom:transcode", transcode.StreamKey)
	}
	coarse, ok := specs["vector-coarse"]
	if !ok {
		t.Fatal("missing vector-coarse queue")
	}
	if coarse.StreamKey != "custom:coarse" {
		t.Fatalf("vector-coarse key = %q, want custom:coarse", coarse.StreamKey)
	}
}

func TestRunListShowsDeadLetters(t *testing.T) {
	ctx := context.Background()
	r, cleanup := newTestRunner(t)
	defer cleanup()
	if _, err := r.rdb.XAdd(ctx, &goredis.XAddArgs{
		Stream: "test:transcode:dlq",
		Values: map[string]interface{}{"payload": `{"video_id":10}`, "reason": "terminal"},
	}).Result(); err != nil {
		t.Fatalf("seed dlq: %v", err)
	}

	var out bytes.Buffer
	if err := r.run(ctx, []string{"list", "--queue", "transcode"}, &out); err != nil {
		t.Fatalf("run list: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "test:transcode") || !strings.Contains(got, "terminal") || !strings.Contains(got, "video_id") {
		t.Fatalf("unexpected list output:\n%s", got)
	}
}

func TestRunReplayMovesDeadLetterBackToMainQueue(t *testing.T) {
	ctx := context.Background()
	r, cleanup := newTestRunner(t)
	defer cleanup()
	id, err := r.rdb.XAdd(ctx, &goredis.XAddArgs{
		Stream: "test:transcode:dlq",
		Values: map[string]interface{}{"payload": `{"video_id":10}`, "reason": "terminal"},
	}).Result()
	if err != nil {
		t.Fatalf("seed dlq: %v", err)
	}

	var out bytes.Buffer
	if err := r.run(ctx, []string{"replay", "--queue", "transcode", "--id", id}, &out); err != nil {
		t.Fatalf("run replay: %v", err)
	}
	if got := r.rdb.XLen(ctx, "test:transcode").Val(); got != 1 {
		t.Fatalf("main queue len = %d, want 1", got)
	}
	if got := r.rdb.XLen(ctx, "test:transcode:dlq").Val(); got != 0 {
		t.Fatalf("dlq len = %d, want 0", got)
	}
	if !strings.Contains(out.String(), "replayed") {
		t.Fatalf("unexpected replay output: %s", out.String())
	}
}

func TestRunReplayDryRunDoesNotMoveMessage(t *testing.T) {
	ctx := context.Background()
	r, cleanup := newTestRunner(t)
	defer cleanup()
	id, err := r.rdb.XAdd(ctx, &goredis.XAddArgs{
		Stream: "test:transcode:dlq",
		Values: map[string]interface{}{"payload": `{"video_id":10}`, "reason": "terminal"},
	}).Result()
	if err != nil {
		t.Fatalf("seed dlq: %v", err)
	}

	var out bytes.Buffer
	if err := r.run(ctx, []string{"replay", "--queue", "transcode", "--id", id, "--dry-run"}, &out); err != nil {
		t.Fatalf("run replay dry-run: %v", err)
	}
	if got := r.rdb.XLen(ctx, "test:transcode").Val(); got != 0 {
		t.Fatalf("main queue len = %d, want 0", got)
	}
	if got := r.rdb.XLen(ctx, "test:transcode:dlq").Val(); got != 1 {
		t.Fatalf("dlq len = %d, want 1", got)
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Fatalf("unexpected dry-run output: %s", out.String())
	}
}

func TestRunRejectsUnknownQueue(t *testing.T) {
	ctx := context.Background()
	r, cleanup := newTestRunner(t)
	defer cleanup()

	var out bytes.Buffer
	err := r.run(ctx, []string{"list", "--queue", "missing"}, &out)
	if err == nil {
		t.Fatal("expected unknown queue error")
	}
	if !strings.Contains(err.Error(), "unknown queue") {
		t.Fatalf("error = %v, want unknown queue", err)
	}
}

func TestNewRedisClientRequiresAddr(t *testing.T) {
	_, err := newRedisClient(config.Config{})
	if err == nil {
		t.Fatal("expected missing redis addr error")
	}
	if !strings.Contains(err.Error(), "redis addr is required") {
		t.Fatalf("error = %v, want redis addr is required", err)
	}
}
