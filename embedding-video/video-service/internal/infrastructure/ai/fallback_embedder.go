package ai

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strings"
	"sync"
	"time"
)

// Embedder 是最小文本向量化接口。
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// FallbackEmbedder 先尝试主向量化器，失败后切换到本地兜底向量化器。
type FallbackEmbedder struct {
	Primary   Embedder
	Fallback  Embedder
	Breaker   *CircuitBreaker
	fallbacks int
}

// NewFallbackEmbedder 创建带熔断的 fallback embedder。
func NewFallbackEmbedder(primary Embedder, fallback Embedder) *FallbackEmbedder {
	return &FallbackEmbedder{
		Primary:  primary,
		Fallback: fallback,
		Breaker:  NewCircuitBreaker(2, 30*time.Second),
	}
}

// Embed 优先使用主 embedding，主服务不可用时回退到本地 deterministic embedding。
func (e *FallbackEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if e == nil {
		return nil, context.Canceled
	}
	if e.Breaker == nil {
		e.Breaker = NewCircuitBreaker(2, 30*time.Second)
	}
	if e.Primary != nil && e.Breaker.Allow(time.Now()) {
		vec, err := e.Primary.Embed(ctx, text)
		if err == nil {
			e.Breaker.Success()
			return vec, nil
		}
		e.Breaker.Failure(time.Now())
	}
	if e.Fallback != nil {
		e.fallbacks++
		return e.Fallback.Embed(ctx, text)
	}
	return nil, context.Canceled
}

// CircuitBreaker 是一个极小的本地熔断器，避免 provider 断开时持续打爆上游。
type CircuitBreaker struct {
	mu             sync.Mutex
	failureCount   int
	threshold      int
	cooldown       time.Duration
	openUntil      time.Time
	halfOpenInFlight bool
}

// NewCircuitBreaker 创建熔断器。
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 1
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitBreaker{threshold: threshold, cooldown: cooldown}
}

// Allow 判断当前是否允许尝试主 provider。
func (b *CircuitBreaker) Allow(now time.Time) bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openUntil.IsZero() {
		return true
	}
	if now.Before(b.openUntil) {
		return false
	}
	if b.halfOpenInFlight {
		return false
	}
	b.halfOpenInFlight = true
	return true
}

// Success 关闭熔断器并清空失败计数。
func (b *CircuitBreaker) Success() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failureCount = 0
	b.openUntil = time.Time{}
	b.halfOpenInFlight = false
}

// Failure 记录一次失败，达到阈值后打开熔断器。
func (b *CircuitBreaker) Failure(now time.Time) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failureCount++
	b.halfOpenInFlight = false
	if b.failureCount >= b.threshold {
		b.openUntil = now.Add(b.cooldown)
	}
}

// LocalEmbedder 是一个不依赖外部模型的确定性兜底向量化器。
type LocalEmbedder struct {
	Dim int
}

// NewLocalEmbedder 创建本地兜底 embedder。
func NewLocalEmbedder(dim int) *LocalEmbedder {
	if dim <= 0 {
		dim = 1536
	}
	return &LocalEmbedder{Dim: dim}
}

// Embed 生成稳定、可重复的本地兜底向量。
func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if e == nil || e.Dim <= 0 {
		return nil, context.Canceled
	}
	seed := sha256.Sum256([]byte(strings.TrimSpace(text)))
	vec := make([]float32, e.Dim)
	for i := 0; i < e.Dim; i++ {
		b := seed[i%len(seed)]
		v := float64((int(b)+i)%255) / 255.0
		vec[i] = float32(math.Round(v*1000) / 1000)
	}
	return vec, nil
}

// Uint64Seed 提供一个简单的稳定种子，便于测试扩展。
func Uint64Seed(text string) uint64 {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return binary.LittleEndian.Uint64(sum[:8])
}
