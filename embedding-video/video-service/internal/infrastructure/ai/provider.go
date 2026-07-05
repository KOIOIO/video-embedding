package ai

import (
	"context"
	"strings"
)

const (
	ProviderLegacy = "legacy"
	ProviderEino   = "eino"
)

type ChatClient interface {
	ChatCompletionsWithTimeout(ctx context.Context, model string, prompt string, timeoutMinutes int) (string, error)
}

type BatchEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

func NormalizeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case ProviderEino:
		return ProviderEino
	default:
		return ProviderLegacy
	}
}
