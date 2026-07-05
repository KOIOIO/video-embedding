package main

import (
	"testing"
	"time"
)

func TestParseOptionsDefaults(t *testing.T) {
	opts, err := parseOptions([]string{})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.modelVersion != "video_profile_v1" {
		t.Fatalf("modelVersion = %q", opts.modelVersion)
	}
	if opts.limitUsers != 1000 {
		t.Fatalf("limitUsers = %d", opts.limitUsers)
	}
	if opts.dryRun {
		t.Fatal("dryRun should default false")
	}
}

func TestParseOptionsAcceptsUserAndDryRun(t *testing.T) {
	opts, err := parseOptions([]string{"--user-id", "123", "--limit-users", "7", "--model-version", "custom", "--dry-run"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.userID != 123 || opts.limitUsers != 7 || opts.modelVersion != "custom" || !opts.dryRun {
		t.Fatalf("opts = %+v", opts)
	}
}

func TestChooseSysUserIDColumnPrefersUserID(t *testing.T) {
	got, err := chooseSysUserIDColumn([]string{"id", "user_id"})
	if err != nil {
		t.Fatalf("chooseSysUserIDColumn returned error: %v", err)
	}
	if got != "user_id" {
		t.Fatalf("column = %q, want user_id", got)
	}
}

func TestChooseSysUserIDColumnFallsBackToID(t *testing.T) {
	got, err := chooseSysUserIDColumn([]string{"id", "name"})
	if err != nil {
		t.Fatalf("chooseSysUserIDColumn returned error: %v", err)
	}
	if got != "id" {
		t.Fatalf("column = %q, want id", got)
	}
}

func TestBuildProfileRowMarksPositiveProfileActive(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	row := buildProfileRow(7, "video_profile_v1", []profileEvent{
		{
			sourceType:   "segment_reaction",
			reactionType: "like",
			vector:       []float32{1, 0},
			eventTime:    now,
		},
	}, now)

	if row.UserID != 7 || row.ModelVersion != "video_profile_v1" || row.Status != 1 {
		t.Fatalf("row = %+v", row)
	}
	if row.PositiveCount != 1 || row.SourceEventCount != 1 || len(row.ProfileVector) != 2 {
		t.Fatalf("row counts/vector = %+v", row)
	}
}

func TestBuildProfileRowMarksNegativeOnlyProfileInactive(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	row := buildProfileRow(7, "video_profile_v1", []profileEvent{
		{
			sourceType:   "segment_reaction",
			reactionType: "dislike",
			vector:       []float32{1, 0},
			eventTime:    now,
		},
	}, now)

	if row.Status != 0 {
		t.Fatalf("status = %d, want inactive", row.Status)
	}
	if row.PositiveCount != 0 || row.NegativeCount != 1 || row.SourceEventCount != 1 {
		t.Fatalf("row counts = %+v", row)
	}
	if len(row.ProfileVector) != profileVectorDim {
		t.Fatalf("inactive profile vector len = %d, want %d", len(row.ProfileVector), profileVectorDim)
	}
}
