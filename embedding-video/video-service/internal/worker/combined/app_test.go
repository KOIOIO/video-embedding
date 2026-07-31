package combined

import (
	"testing"
	"time"

	"video-service/internal/config"
)

func TestRunSymbolExists(t *testing.T) {
	_ = Run
}

func TestKnowledgeVideoShutdownTimeoutParticipatesInMaximum(t *testing.T) {
	cfg := config.Config{KnowledgeVideoWorker: config.KnowledgeVideoWorkerConfig{ShutdownTimeoutSec: 900}, Transcode: config.TransConfig{ShutdownTimeoutSec: 30}, VectorWorker: config.VectorWorkerConfig{ShutdownTimeoutSec: 30}}
	if got := maxShutdownTimeout(cfg); got != 15*time.Minute {
		t.Fatalf("got %s", got)
	}
}
