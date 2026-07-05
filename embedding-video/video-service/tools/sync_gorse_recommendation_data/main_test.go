package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseOptionsSupportsGorseSyncFlags(t *testing.T) {
	opts, err := parseOptions([]string{
		"--config", "configs/custom.yml",
		"--endpoint", "http://gorse:8087",
		"--api-key", "secret",
		"--batch-size", "123",
		"--limit", "456",
		"--dry-run",
		"--users=false",
		"--items=true",
		"--feedback=false",
		"--gate=false",
	})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.configFile != "configs/custom.yml" || opts.endpoint != "http://gorse:8087" || opts.apiKey != "secret" {
		t.Fatalf("opts identity = %+v", opts)
	}
	if opts.batchSize != 123 || opts.limit != 456 || !opts.dryRun {
		t.Fatalf("opts numeric = %+v", opts)
	}
	if opts.syncUsers || !opts.syncItems || opts.syncFeedback || opts.enableGate {
		t.Fatalf("opts booleans = %+v", opts)
	}
}

func TestPrintResultIncludesGateAndCounts(t *testing.T) {
	var out bytes.Buffer
	printResult(&out, syncResult{
		DryRun:     true,
		Users:      7,
		Items:      55,
		Feedback:   892,
		GatePassed: true,
	})
	got := out.String()
	for _, want := range []string{"dry_run=true", "users=7", "items=55", "feedback=892", "gate_passed=true"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}
