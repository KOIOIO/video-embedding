package main

import "testing"

func TestParseOptionsDefaultsToTwoTower(t *testing.T) {
	opts, err := parseOptions([]string{"--version", "two_tower_v2"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.configFile != defaultConfigPath {
		t.Fatalf("config = %q, want %q", opts.configFile, defaultConfigPath)
	}
	if opts.modelName != "two_tower" {
		t.Fatalf("model name = %q, want two_tower", opts.modelName)
	}
	if opts.modelVersion != "two_tower_v2" {
		t.Fatalf("version = %q, want two_tower_v2", opts.modelVersion)
	}
}

func TestParseOptionsRequiresVersion(t *testing.T) {
	if _, err := parseOptions(nil); err == nil {
		t.Fatal("expected missing version to fail")
	}
}
