package vectorworker

import (
	"errors"
	"testing"
	"time"
)

func TestStageRetryDecisionRetriesBeforeLimit(t *testing.T) {
	decision := decideStageRetry(VectorStageTask{RetryCount: 0}, errors.New("temporary"))
	if !decision.Retry {
		t.Fatal("expected retry")
	}
	if decision.Delay <= 0 {
		t.Fatalf("delay = %v, want positive", decision.Delay)
	}
}

func TestStageRetryDecisionStopsAfterLimit(t *testing.T) {
	decision := decideStageRetry(VectorStageTask{RetryCount: 3}, errors.New("temporary"))
	if decision.Retry {
		t.Fatal("did not expect retry after max retries")
	}
}

func TestNextRetryTaskIncrementsRetryCount(t *testing.T) {
	task := VectorStageTask{TaskID: "task-1", Stage: VectorStageCoarse, RetryCount: 1}
	got := nextRetryTask(task)
	if got.RetryCount != 2 {
		t.Fatalf("RetryCount = %d, want 2", got.RetryCount)
	}
	if got.TaskID != task.TaskID || got.Stage != task.Stage {
		t.Fatalf("identity changed: %+v", got)
	}
}

func TestStageRetryBackoffIncreases(t *testing.T) {
	first := decideStageRetry(VectorStageTask{RetryCount: 0}, errors.New("temporary"))
	second := decideStageRetry(VectorStageTask{RetryCount: 1}, errors.New("temporary"))
	if second.Delay <= first.Delay {
		t.Fatalf("second delay = %v, first = %v", second.Delay, first.Delay)
	}
	if second.Delay > 30*time.Second {
		t.Fatalf("delay too large: %v", second.Delay)
	}
}
