package app

import (
	"testing"

	"nlp-video-analysis/internal/config"
)

func TestResolveHTTPAddrUsesConfigWhenEnvMissing(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")

	got := ResolveHTTPAddr(config.Config{
		HTTP: config.HTTPConfig{Addr: ":9092"},
	})

	if got != ":9092" {
		t.Fatalf("ResolveHTTPAddr() = %q, want %q", got, ":9092")
	}
}

func TestResolveHTTPAddrPrefersEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", " :9093 ")

	got := ResolveHTTPAddr(config.Config{
		HTTP: config.HTTPConfig{Addr: ":9092"},
	})

	if got != ":9093" {
		t.Fatalf("ResolveHTTPAddr() = %q, want %q", got, ":9093")
	}
}
