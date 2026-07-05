package gorsesync

import (
	"context"
	"testing"
	"time"

	appsync "nlp-video-analysis/internal/application/videoapp/recommendation/gorsesync"
)

func TestSchedulerRunOnceSkipsOverlappingRun(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	scheduler := &Scheduler{
		Sync: func(ctx context.Context) (appsync.Result, error) {
			close(started)
			<-release
			return appsync.Result{GatePassed: true}, nil
		},
	}

	go func() {
		done <- scheduler.runOnce(context.Background())
	}()
	<-started

	if err := scheduler.runOnce(context.Background()); err != nil {
		t.Fatalf("overlapping run returned error: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first run returned error: %v", err)
	}
}

func TestSchedulerIntervalDefaults(t *testing.T) {
	scheduler := &Scheduler{}
	if got := scheduler.interval(); got != time.Hour {
		t.Fatalf("interval = %s, want 1h", got)
	}
	scheduler.Interval = 15 * time.Minute
	if got := scheduler.interval(); got != 15*time.Minute {
		t.Fatalf("interval = %s, want 15m", got)
	}
}
