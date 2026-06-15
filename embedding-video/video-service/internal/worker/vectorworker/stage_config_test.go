package vectorworker

import (
	"testing"

	"nlp-video-analysis/internal/config"
)

func TestCoarseStageQueueKeyFromConfigUsesDefaults(t *testing.T) {
	cfg := config.Config{}
	cases := map[string]string{
		VectorStagePrepare:  "video:vector:prepare",
		VectorStageCoarse:   "video:vector:coarse",
		VectorStageRefine:   "video:vector:refine",
		VectorStageFinalize: "video:vector:finalize",
	}
	for stage, want := range cases {
		if got := vectorStageQueueKeyFromConfig(cfg, stage); got != want {
			t.Fatalf("vectorStageQueueKeyFromConfig(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestCoarseStageQueueKeyFromConfigUsesOverrides(t *testing.T) {
	cfg := config.Config{
		RedisKeys: config.RedisKeysConfig{
			VectorPrepareQueue:  "custom:prepare",
			VectorCoarseQueue:   "custom:coarse",
			VectorRefineQueue:   "custom:refine",
			VectorFinalizeQueue: "custom:finalize",
		},
	}
	cases := map[string]string{
		VectorStagePrepare:  "custom:prepare",
		VectorStageCoarse:   "custom:coarse",
		VectorStageRefine:   "custom:refine",
		VectorStageFinalize: "custom:finalize",
	}
	for stage, want := range cases {
		if got := vectorStageQueueKeyFromConfig(cfg, stage); got != want {
			t.Fatalf("vectorStageQueueKeyFromConfig(%q) = %q, want %q", stage, got, want)
		}
	}
}

func TestCoarseStageWorkerCountFromConfig(t *testing.T) {
	cfg := config.Config{
		VectorStageWorkers: config.VectorStageWorkersConfig{
			Prepare:  1,
			Coarse:   2,
			Refine:   3,
			Finalize: 4,
		},
	}
	cases := map[string]int{
		VectorStagePrepare:  1,
		VectorStageCoarse:   2,
		VectorStageRefine:   3,
		VectorStageFinalize: 4,
	}
	for stage, want := range cases {
		if got := vectorStageWorkerCountFromConfig(cfg, stage); got != want {
			t.Fatalf("vectorStageWorkerCountFromConfig(%q) = %d, want %d", stage, got, want)
		}
	}
}
