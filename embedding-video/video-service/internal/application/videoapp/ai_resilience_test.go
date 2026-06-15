package videoapp

import (
	"context"
	"errors"
	"testing"
)

func TestDecideAIRetry_RetryableAndTerminal(t *testing.T) {
	got := DecideAIRetry(context.DeadlineExceeded, 1)
	if !got.Retry || got.Reason != "timeout" {
		t.Fatalf("timeout decision = %#v", got)
	}

	got = DecideAIRetry(errors.New("model not found"), 1)
	if got.Retry || got.Reason != "terminal" {
		t.Fatalf("terminal decision = %#v", got)
	}
}

func TestIsAIProviderUnavailable(t *testing.T) {
	if !IsAIProviderUnavailable(DegradedError{Reason: "provider_unavailable"}) {
		t.Fatal("expected degraded error to be unavailable")
	}
	if !IsAIProviderUnavailable(errors.New("upstream timeout")) {
		t.Fatal("expected timeout to be unavailable")
	}
	if IsAIProviderUnavailable(nil) {
		t.Fatal("nil should not be unavailable")
	}
}

func TestRetryDecisionDelaysIncrease(t *testing.T) {
	first := DecideAIRetry(errors.New("timeout"), 0)
	second := DecideAIRetry(errors.New("timeout"), 1)
	if first.Delay <= 0 || second.Delay <= first.Delay {
		t.Fatalf("unexpected delays: first=%s second=%s", first.Delay, second.Delay)
	}
}
