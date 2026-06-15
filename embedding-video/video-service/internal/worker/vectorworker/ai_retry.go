package vectorworker

import (
	"context"
	"errors"
	"strings"
	"time"
)

type AIRetryDecision struct {
	Retry  bool
	Delay  time.Duration
	Reason string
}

func DecideVectorAIRetry(err error, retries int) AIRetryDecision {
	if err == nil {
		return AIRetryDecision{}
	}
	if retries >= 3 {
		return AIRetryDecision{Retry: false, Reason: "retry_exhausted"}
	}
	msg := strings.ToLower(err.Error())
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "timeout") {
		return AIRetryDecision{Retry: true, Delay: time.Duration(retries+1) * 5 * time.Second, Reason: "timeout"}
	}
	if strings.Contains(msg, "429") || strings.Contains(msg, "too many requests") {
		return AIRetryDecision{Retry: true, Delay: time.Duration(retries+1) * 10 * time.Second, Reason: "throttled"}
	}
	if strings.Contains(msg, "connection reset") || strings.Contains(msg, "temporary") || strings.Contains(msg, "unavailable") {
		return AIRetryDecision{Retry: true, Delay: time.Duration(retries+1) * 7 * time.Second, Reason: "transient"}
	}
	return AIRetryDecision{Retry: false, Reason: "terminal"}
}
