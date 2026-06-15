package vectorworker

import (
	"testing"

	"nlp-video-analysis/internal/config"
)

func TestNormalizeASRWorkersDefaultsToFour(t *testing.T) {
	if got := normalizeASRWorkers(0); got != 4 {
		t.Fatalf("normalizeASRWorkers(0) = %d, want 4", got)
	}
}

func TestNormalizeASRWorkersCapsAtTwenty(t *testing.T) {
	if got := normalizeASRWorkers(60); got != 20 {
		t.Fatalf("normalizeASRWorkers(60) = %d, want 20", got)
	}
}

func TestNormalizeASRWorkersKeepsSmallExplicitValue(t *testing.T) {
	if got := normalizeASRWorkers(8); got != 8 {
		t.Fatalf("normalizeASRWorkers(8) = %d, want 8", got)
	}
}

func TestResolvePoolSizeUsesNamedPoolOverride(t *testing.T) {
	pools := config.WorkerPoolsConfig{
		"vector.refine_asr": {Size: 6},
	}
	if got := resolvePoolSize(pools, "vector.refine_asr", 4); got != 6 {
		t.Fatalf("resolvePoolSize() = %d, want 6", got)
	}
}

func TestResolvePoolSizeFallsBackWhenMissing(t *testing.T) {
	if got := resolvePoolSize(nil, "vector.refine_asr", 4); got != 4 {
		t.Fatalf("resolvePoolSize() = %d, want 4", got)
	}
}

func TestShouldSkipVectorWorkerForMissingAPIKey(t *testing.T) {
	if !shouldSkipVectorWorker(errMissingOpenAICompatAPIKey) {
		t.Fatal("expected vector worker to be skipped when API key is missing")
	}
}

func TestShouldSkipVectorWorkerOnlyForMissingAPIKey(t *testing.T) {
	if shouldSkipVectorWorker(assertAnError{}) {
		t.Fatal("did not expect vector worker to be skipped for unrelated errors")
	}
}

type assertAnError struct{}

func (assertAnError) Error() string {
	return "unrelated error"
}
