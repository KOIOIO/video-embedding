package app

import (
	"testing"
	"time"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
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

func TestRecommendationRuntimeFromConfigDefaultsToKnowledgeMatchWithoutGorseClient(t *testing.T) {
	client, engine, options := recommendationRuntimeFromConfig(config.Config{})

	if client != nil {
		t.Fatalf("client = %T, want nil for default knowledge_match", client)
	}
	if engine != recommendationapp.EngineKnowledgeMatch {
		t.Fatalf("engine = %q, want knowledge_match", engine)
	}
	if options.CandidateLimit == 0 {
		t.Fatal("expected default options to be populated")
	}
}

func TestRecommendationRuntimeFromConfigCreatesGorseClientWhenExplicitlyEnabled(t *testing.T) {
	client, engine, options := recommendationRuntimeFromConfig(config.Config{
		Recommendation: config.RecommendationConfig{Engine: "gorse"},
		Gorse: config.GorseConfig{
			Endpoint:          " http://gorse:8087/ ",
			APIKey:            "secret",
			TimeoutSeconds:    5,
			CandidateLimit:    120,
			ShadowMode:        true,
			WriteBackEnabled:  true,
			MinRecommendItems: 2,
		},
	})

	if client == nil {
		t.Fatal("client is nil")
	}
	if engine != recommendationapp.EngineGorse {
		t.Fatalf("engine = %q, want gorse", engine)
	}
	if options.CandidateLimit != 120 || !options.ShadowMode || !options.WriteBackEnabled || options.MinRecommendItems != 2 {
		t.Fatalf("options = %+v", options)
	}
	httpClient, ok := client.(*recommendationapp.GorseHTTPClient)
	if !ok {
		t.Fatalf("client type = %T, want *GorseHTTPClient", client)
	}
	if httpClient.Timeout() != 5*time.Second {
		t.Fatalf("client timeout = %s, want 5s", httpClient.Timeout())
	}
}

func TestRecommendationRuntimeFromConfigLeavesClientNilForRecBoleEngine(t *testing.T) {
	client, engine, options := recommendationRuntimeFromConfig(config.Config{
		Recommendation: config.RecommendationConfig{Engine: "recbole"},
	})

	if client != nil {
		t.Fatalf("client = %T, want nil", client)
	}
	if engine != recommendationapp.EngineRecBole {
		t.Fatalf("engine = %q, want recbole", engine)
	}
	if options.CandidateLimit == 0 {
		t.Fatal("expected default options to be populated")
	}
}
