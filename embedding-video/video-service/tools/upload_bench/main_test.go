package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectInputFilesFromDirectory(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.mp4", "b.mov", "c.txt", "sub/d.mkv"}
	for _, name := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	got, err := collectInputFiles(dir)
	if err != nil {
		t.Fatalf("collectInputFiles error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len(files) = %d, want 3", len(got))
	}
}

func TestCollectInputFilesFromSingleFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "single.mp4")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := collectInputFiles(filePath)
	if err != nil {
		t.Fatalf("collectInputFiles error = %v", err)
	}
	if len(got) != 1 || got[0] != filePath {
		t.Fatalf("files = %#v, want [%q]", got, filePath)
	}
}

func TestBuildSummary(t *testing.T) {
	results := []requestResult{
		{Duration: 100 * time.Millisecond, StatusCode: 200, Success: true},
		{Duration: 200 * time.Millisecond, StatusCode: 200, Success: true},
		{Duration: 300 * time.Millisecond, StatusCode: 500, ErrorCode: "upload_failed", Success: false},
		{Duration: 400 * time.Millisecond, StatusCode: 200, Success: true},
	}

	s := buildSummary(results, 2*time.Second)
	if s.TotalRequests != 4 {
		t.Fatalf("TotalRequests = %d, want 4", s.TotalRequests)
	}
	if s.SuccessRequests != 3 {
		t.Fatalf("SuccessRequests = %d, want 3", s.SuccessRequests)
	}
	if s.FailedRequests != 1 {
		t.Fatalf("FailedRequests = %d, want 1", s.FailedRequests)
	}
	if s.P50Ms != 200 {
		t.Fatalf("P50Ms = %d, want 200", s.P50Ms)
	}
	if s.P95Ms != 400 {
		t.Fatalf("P95Ms = %d, want 400", s.P95Ms)
	}
	if s.StatusCodes[200] != 3 || s.StatusCodes[500] != 1 {
		t.Fatalf("unexpected status distribution: %#v", s.StatusCodes)
	}
	if s.ErrorCodes["upload_failed"] != 1 {
		t.Fatalf("unexpected error code distribution: %#v", s.ErrorCodes)
	}
}
