package main

import (
	"strings"
	"testing"
)

func TestParseOptionsRequiresOutput(t *testing.T) {
	_, err := parseOptions([]string{"--output", ""})
	if err == nil {
		t.Fatal("parseOptions returned nil error, want output required")
	}
}

func TestParseOptionsDefaultsToRecBoleModel(t *testing.T) {
	opts, err := parseOptions([]string{"--output", "baseline.json"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.modelName != "recbole" {
		t.Fatalf("modelName = %q, want recbole", opts.modelName)
	}
}

func TestBuildActiveMetricsQueryFiltersActiveRecsysModel(t *testing.T) {
	query := buildActiveMetricsQuery()
	for _, fragment := range []string{
		"SELECT COALESCE(metrics_json::text, '{}')",
		"FROM recsys.recommend_model_version",
		"model_name = $1",
		"is_active = TRUE",
		"status = 1",
		"deleted = 0",
		"ORDER BY published_at DESC",
		"LIMIT 1",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q:\n%s", fragment, query)
		}
	}
	if strings.Contains(query, "public.edu_recommend_model_version") {
		t.Fatalf("query references legacy table:\n%s", query)
	}
}
