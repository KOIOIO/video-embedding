package videoapp

import (
	"context"
	"errors"
	"strings"
	"time"
)

// DegradedError 表示 AI 能力降级但请求仍可返回的状态。
type DegradedError struct {
	Reason string
	Items  []RecommendResultItem
}

func (e DegradedError) Error() string {
	if strings.TrimSpace(e.Reason) == "" {
		return "ai degraded"
	}
	return e.Reason
}

// DecideAIRetry 统一分类 AI 上游错误，供 worker 和服务层复用。
func DecideAIRetry(err error, retries int) RetryDecision {
	if err == nil {
		return RetryDecision{}
	}
	if retries >= 3 {
		return RetryDecision{Retry: false, Reason: "retry_exhausted"}
	}
	msg := strings.ToLower(err.Error())
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "timeout") {
		return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * 2 * time.Second, Reason: "timeout"}
	}
	if strings.Contains(msg, "429") || strings.Contains(msg, "too many requests") {
		return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * 5 * time.Second, Reason: "throttled"}
	}
	if strings.Contains(msg, "connection reset") || strings.Contains(msg, "temporary") || strings.Contains(msg, "unavailable") {
		return RetryDecision{Retry: true, Delay: time.Duration(retries+1) * 3 * time.Second, Reason: "transient"}
	}
	return RetryDecision{Retry: false, Reason: "terminal"}
}

// IsAIProviderUnavailable 判断是否应该进入降级路径。
func IsAIProviderUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var degraded DegradedError
	if errors.As(err, &degraded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "unavailable") || strings.Contains(msg, "too many requests")
}
