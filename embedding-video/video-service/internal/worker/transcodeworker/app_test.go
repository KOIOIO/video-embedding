package transcodeworker

import (
	"testing"

	"nlp-video-analysis/internal/config"
)

func TestWorkerCountFromConfigNormalizesNonPositiveValues(t *testing.T) {
	for _, value := range []int{0, -1} {
		cfg := config.Config{}
		cfg.Transcode.WorkerCount = value

		if got := WorkerCountFromConfig(cfg); got != 1 {
			t.Fatalf("WorkerCountFromConfig(%d) = %d, want 1", value, got)
		}
	}
}

func TestWorkerCountFromConfigKeepsPositiveValues(t *testing.T) {
	cfg := config.Config{}
	cfg.Transcode.WorkerCount = 3

	if got := WorkerCountFromConfig(cfg); got != 3 {
		t.Fatalf("WorkerCountFromConfig(3) = %d, want 3", got)
	}
}
