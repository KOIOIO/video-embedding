package main

import (
	"strings"
	"testing"

	"video-service/internal/application/videoapp/profile"
)

func TestParseOptionsDefaults(t *testing.T) {
	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.configFile != defaultConfigPath {
		t.Fatalf("configFile = %q, want %q", opts.configFile, defaultConfigPath)
	}
	if opts.userID != 0 {
		t.Fatalf("userID = %d, want 0", opts.userID)
	}
	if opts.modelVersion != defaultModelVersion {
		t.Fatalf("modelVersion = %q, want %q", opts.modelVersion, defaultModelVersion)
	}
	if opts.dryRun {
		t.Fatal("dryRun = true, want false")
	}
}

func TestParseOptionsWithUserID(t *testing.T) {
	opts, err := parseOptions([]string{"--user-id", "42", "--dry-run"})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.userID != 42 {
		t.Fatalf("userID = %d, want 42", opts.userID)
	}
	if !opts.dryRun {
		t.Fatal("dryRun = false, want true")
	}
}

func TestParseOptionsRejectsEmptyModelVersion(t *testing.T) {
	if _, err := parseOptions([]string{"--model-version", "  "}); err == nil {
		t.Fatal("expected error for empty model-version")
	}
}

func TestSourceTypeFromString(t *testing.T) {
	cases := []struct {
		input string
		want  profile.SourceType
		ok    bool
	}{
		{"comment", profile.SourceComment, true},
		{"comment_like", profile.SourceCommentLike, true},
		{"user_publish", profile.SourceUserPublish, true},
		{"profile_visit", "", false},
		{"unknown", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := sourceTypeFromString(c.input)
		if ok != c.ok {
			t.Fatalf("sourceTypeFromString(%q) ok = %v, want %v", c.input, ok, c.ok)
		}
		if ok && got != c.want {
			t.Fatalf("sourceTypeFromString(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestVectorToLiteral(t *testing.T) {
	if got := vectorToLiteral(nil); got != "[]" {
		t.Fatalf("vectorToLiteral(nil) = %q, want []", got)
	}
	got := vectorToLiteral([]float32{0.5, -0.25, 1.0})
	if !strings.HasPrefix(got, "[") || !strings.HasSuffix(got, "]") {
		t.Fatalf("vectorToLiteral missing brackets: %q", got)
	}
	for _, token := range []string{"0.5", "-0.25", "1"} {
		if !strings.Contains(got, token) {
			t.Fatalf("vectorToLiteral = %q, missing %q", got, token)
		}
	}
}

func TestParseVectorText(t *testing.T) {
	vec, err := parseVectorText("[0.1,0.2,0.3]")
	if err != nil {
		t.Fatalf("parseVectorText error: %v", err)
	}
	if len(vec) != 3 {
		t.Fatalf("len(vec) = %d, want 3", len(vec))
	}
	if vec[0] != 0.1 || vec[1] != 0.2 || vec[2] != 0.3 {
		t.Fatalf("vec = %v, want [0.1 0.2 0.3]", vec)
	}

	if vec, err := parseVectorText("[]"); err != nil || len(vec) != 0 {
		t.Fatalf("parseVectorText([]) = %v, %v, want nil, nil", vec, err)
	}

	if _, err := parseVectorText("[0.1,bad]"); err == nil {
		t.Fatal("expected error for invalid vector text")
	}
}

func TestBuildSocialEventsQueryFiltersDeletedAndStatus(t *testing.T) {
	q := buildSocialEventsQuery()
	required := []string{
		"c.deleted = 0",
		"cl.deleted = 0",
		"r.deleted = 0",
		"s.deleted = 0",
		"s.status = 1",
		"r.source_type = 'user_publish'",
		"r.is_published = true",
		"UNION ALL",
	}
	for _, fragment := range required {
		if !strings.Contains(q, fragment) {
			t.Fatalf("query missing %q\n--- query ---\n%s", fragment, q)
		}
	}
}

func TestGroupEventsByUser(t *testing.T) {
	events := []socialEventRow{
		{UserID: 1, SourceType: "comment"},
		{UserID: 2, SourceType: "comment_like"},
		{UserID: 1, SourceType: "user_publish"},
	}
	grouped := groupEventsByUser(events)
	if len(grouped) != 2 {
		t.Fatalf("len(grouped) = %d, want 2", len(grouped))
	}
	if len(grouped[1]) != 2 {
		t.Fatalf("grouped[1] = %d events, want 2", len(grouped[1]))
	}
	if len(grouped[2]) != 1 {
		t.Fatalf("grouped[2] = %d events, want 1", len(grouped[2]))
	}
}
