package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseVectorRequiresExpectedDimension(t *testing.T) {
	values, err := parseVector("[0.100000,-0.200000]", 2)
	if err != nil {
		t.Fatalf("parseVector returned error: %v", err)
	}
	if len(values) != 2 || values[0] != 0.1 || values[1] != -0.2 {
		t.Fatalf("values = %#v", values)
	}
	if _, err := parseVector("[0.1]", 2); err == nil {
		t.Fatal("expected dimension mismatch to fail")
	}
}

func TestLoadArtifactRows(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "item_embeddings.csv"), []byte(`video_segment_id,video_id,embedding,model_version
101,11,"[0.1,0.2]",recbole_v1
`), 0o644); err != nil {
		t.Fatalf("write item csv: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "user_embeddings.csv"), []byte(`user_id,embedding,model_version
7,"[0.3,0.4]",recbole_v1
`), 0o644); err != nil {
		t.Fatalf("write user csv: %v", err)
	}

	items, err := loadItemRows(filepath.Join(dir, "item_embeddings.csv"), 2)
	if err != nil {
		t.Fatalf("loadItemRows returned error: %v", err)
	}
	users, err := loadUserRows(filepath.Join(dir, "user_embeddings.csv"), 2)
	if err != nil {
		t.Fatalf("loadUserRows returned error: %v", err)
	}
	if len(items) != 1 || items[0].VideoSegmentID != 101 || items[0].VideoID != 11 || items[0].ModelVersion != "recbole_v1" {
		t.Fatalf("items = %+v", items)
	}
	if len(users) != 1 || users[0].UserID != 7 || users[0].ModelVersion != "recbole_v1" {
		t.Fatalf("users = %+v", users)
	}
}

func TestInferArtifactModelVersionRequiresOneConsistentVersion(t *testing.T) {
	version, err := inferArtifactModelVersion(
		[]itemEmbeddingRow{{ModelVersion: "recbole_v2"}},
		[]userEmbeddingRow{{ModelVersion: "recbole_v2"}},
	)
	if err != nil {
		t.Fatalf("inferArtifactModelVersion returned error: %v", err)
	}
	if version != "recbole_v2" {
		t.Fatalf("version = %q, want recbole_v2", version)
	}
	if _, err := inferArtifactModelVersion(
		[]itemEmbeddingRow{{ModelVersion: "recbole_v2"}},
		[]userEmbeddingRow{{ModelVersion: "recbole_v3"}},
	); err == nil {
		t.Fatal("expected mismatched artifact versions to fail")
	}
}

func TestLoadMetricsJSONDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	metrics, err := loadMetricsJSON(dir)
	if err != nil {
		t.Fatalf("loadMetricsJSON missing returned error: %v", err)
	}
	if metrics != "{}" {
		t.Fatalf("metrics = %q, want {}", metrics)
	}
	if err := os.WriteFile(filepath.Join(dir, "metrics.json"), []byte(`{"algorithm":"BPR"}`), 0o644); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	metrics, err = loadMetricsJSON(dir)
	if err != nil {
		t.Fatalf("loadMetricsJSON returned error: %v", err)
	}
	if metrics != `{"algorithm":"BPR"}` {
		t.Fatalf("metrics = %q", metrics)
	}
}

func TestRecSysSQLDoesNotReferenceLegacyPublicTables(t *testing.T) {
	sqlText := strings.Join([]string{
		upsertItemEmbeddingSQL(),
		upsertUserEmbeddingSQL(),
		publishModelVersionSQL(),
	}, "\n")
	for _, fragment := range []string{
		"recsys.recommend_item_embedding",
		"recsys.recommend_user_embedding",
		"recsys.recommend_model_version",
		"ON CONFLICT (video_segment_id, model_name, model_version)",
		"ON CONFLICT (user_id, model_name, model_version)",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("sql missing %q:\n%s", fragment, sqlText)
		}
	}
	for _, legacy := range []string{
		"public.edu_video_item_embedding",
		"public.edu_user_tower_embedding",
		"public.edu_recommend_model_version",
	} {
		if strings.Contains(sqlText, legacy) {
			t.Fatalf("sql references legacy table %q:\n%s", legacy, sqlText)
		}
	}
}

func TestCleanupOldEmbeddingVersionsKeepsLatestRecsysVersions(t *testing.T) {
	exec := &captureExec{affected: 3}
	itemRows, userRows, err := cleanupOldEmbeddingVersions(context.Background(), exec, "recbole", 2)
	if err != nil {
		t.Fatalf("cleanupOldEmbeddingVersions returned error: %v", err)
	}
	if itemRows != 3 || userRows != 3 {
		t.Fatalf("deleted rows item=%d user=%d, want 3 and 3", itemRows, userRows)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("exec calls = %d, want 2", len(exec.calls))
	}
	if !strings.Contains(exec.calls[0].query, "DELETE FROM recsys.recommend_item_embedding") {
		t.Fatalf("item cleanup query = %s", exec.calls[0].query)
	}
	if !strings.Contains(exec.calls[1].query, "DELETE FROM recsys.recommend_user_embedding") {
		t.Fatalf("user cleanup query = %s", exec.calls[1].query)
	}
	for _, call := range exec.calls {
		for _, fragment := range []string{
			"FROM recsys.recommend_model_version",
			"model_name = $1",
			"ORDER BY published_at DESC, id DESC",
			"OFFSET $2",
		} {
			if !strings.Contains(call.query, fragment) {
				t.Fatalf("cleanup query missing %q: %s", fragment, call.query)
			}
		}
		if len(call.args) != 2 || call.args[0] != "recbole" || call.args[1] != 2 {
			t.Fatalf("cleanup args = %#v, want [recbole 2]", call.args)
		}
	}
}

type captureExec struct {
	affected int64
	calls    []execCall
}

type execCall struct {
	query string
	args  []any
}

func (e *captureExec) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	e.calls = append(e.calls, execCall{query: query, args: args})
	return fakeResult(e.affected), nil
}

type fakeResult int64

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return int64(r), nil }
