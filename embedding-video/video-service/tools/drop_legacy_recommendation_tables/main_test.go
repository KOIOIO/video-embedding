package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseOptionsDefaultsToDryRun(t *testing.T) {
	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.execute {
		t.Fatal("execute = true, want default dry-run")
	}
}

func TestParseOptionsRequiresConfirmForExecute(t *testing.T) {
	if _, err := parseOptions([]string{"--execute"}); err == nil {
		t.Fatal("execute without confirm returned nil error")
	}
	opts, err := parseOptions([]string{"--execute", "--confirm", "drop-legacy-recommendation-tables"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if !opts.execute {
		t.Fatal("execute = false, want true")
	}
}

func TestDropStatementsOnlyTargetLegacyRecommendationTables(t *testing.T) {
	got := dropStatements()
	want := []string{
		"DROP TABLE IF EXISTS public.edu_video_item_embedding",
		"DROP TABLE IF EXISTS public.edu_user_tower_embedding",
		"DROP TABLE IF EXISTS public.edu_recommend_model_version",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dropStatements = %#v, want %#v", got, want)
	}
	for _, statement := range got {
		if !strings.HasPrefix(statement, "DROP TABLE IF EXISTS public.edu_") {
			t.Fatalf("unexpected destructive statement: %s", statement)
		}
	}
}
