package knowledgevideoworker

import (
	"testing"

	"video-service/internal/config"
)

func TestKnowledgeVideoWorkerCountFromConfigDefaultsToOne(t *testing.T) {
	if got := WorkerCountFromConfig(config.Config{}); got != 1 {
		t.Fatalf("got %d", got)
	}
	if got := WorkerCountFromConfig(config.Config{KnowledgeVideoWorker: config.KnowledgeVideoWorkerConfig{WorkerCount: 3}}); got != 3 {
		t.Fatalf("got %d", got)
	}
}
