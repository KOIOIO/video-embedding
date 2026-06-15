package vectorworker

import (
	"context"
	"errors"
	"testing"
)

func TestDecideVectorAIRetry_RetryableOutage(t *testing.T) {
	got := DecideVectorAIRetry(context.DeadlineExceeded, 0)
	if !got.Retry || got.Reason != "timeout" {
		t.Fatalf("got %#v", got)
	}
	got = DecideVectorAIRetry(errors.New("model not found"), 0)
	if got.Retry || got.Reason != "terminal" {
		t.Fatalf("got %#v", got)
	}
}
