package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestPlanTablesExcludesVideoAndRecommendationTables(t *testing.T) {
	sourceTables := []string{
		"edu_recommend_exposure",
		"edu_recommend_model_version",
		"edu_user_reaction",
		"edu_user_tower_embedding",
		"edu_user_video_profile",
		"edu_user_video_recommend",
		"edu_video_item_embedding",
		"edu_video_resource",
		"edu_video_segment",
		"edu_video_segement",
		"edu_video_user_reaction",
		"edu_video_vector_stage",
		"sys_admin",
		"sys_role",
	}
	targetTables := []string{"edu_video_resource", "sys_admin"}

	plan := planTables(sourceTables, targetTables, true)

	wantCopyTables := []string{"sys_admin", "sys_role"}
	if !reflect.DeepEqual(plan.copyTables, wantCopyTables) {
		t.Fatalf("copyTables = %v, want %v", plan.copyTables, wantCopyTables)
	}

	wantMissingToCreate := []string{"sys_role"}
	if !reflect.DeepEqual(plan.missingToCreate, wantMissingToCreate) {
		t.Fatalf("missingToCreate = %v, want %v", plan.missingToCreate, wantMissingToCreate)
	}

	wantExcluded := []string{
		"edu_recommend_exposure",
		"edu_recommend_model_version",
		"edu_user_reaction",
		"edu_user_tower_embedding",
		"edu_user_video_profile",
		"edu_user_video_recommend",
		"edu_video_item_embedding",
		"edu_video_resource",
		"edu_video_segement",
		"edu_video_segment",
		"edu_video_user_reaction",
		"edu_video_vector_stage",
	}
	if !reflect.DeepEqual(plan.excludedTables, wantExcluded) {
		t.Fatalf("excludedTables = %v, want %v", plan.excludedTables, wantExcluded)
	}
}

func TestPlanTablesSkipsMissingTargetTablesWhenCreateMissingDisabled(t *testing.T) {
	sourceTables := []string{"edu_video_resource", "sys_admin", "sys_role"}
	targetTables := []string{"sys_admin"}

	plan := planTables(sourceTables, targetTables, false)

	wantCopyTables := []string{"sys_admin"}
	if !reflect.DeepEqual(plan.copyTables, wantCopyTables) {
		t.Fatalf("copyTables = %v, want %v", plan.copyTables, wantCopyTables)
	}

	wantSkippedMissing := []string{"sys_role"}
	if !reflect.DeepEqual(plan.skippedMissing, wantSkippedMissing) {
		t.Fatalf("skippedMissing = %v, want %v", plan.skippedMissing, wantSkippedMissing)
	}

	wantExcluded := []string{"edu_video_resource"}
	if !reflect.DeepEqual(plan.excludedTables, wantExcluded) {
		t.Fatalf("excludedTables = %v, want %v", plan.excludedTables, wantExcluded)
	}
}

func TestResolveDSNsLoadsDefaultsFromConfigPaths(t *testing.T) {
	sourceDSN, targetDSN, err := resolveDSNs(dsnOptions{
		sourceConfigPath: "configs/video_prod.yml",
		targetConfigPath: "configs/video.yml",
	}, func(path string) (string, error) {
		switch path {
		case "configs/video_prod.yml":
			return "prod-dsn", nil
		case "configs/video.yml":
			return "local-dsn", nil
		default:
			t.Fatalf("unexpected config path: %s", path)
			return "", nil
		}
	})
	if err != nil {
		t.Fatalf("resolveDSNs returned error: %v", err)
	}
	if sourceDSN != "prod-dsn" {
		t.Fatalf("sourceDSN = %q, want %q", sourceDSN, "prod-dsn")
	}
	if targetDSN != "local-dsn" {
		t.Fatalf("targetDSN = %q, want %q", targetDSN, "local-dsn")
	}
}

func TestResolveDSNsExplicitValuesOverrideConfigPaths(t *testing.T) {
	sourceDSN, targetDSN, err := resolveDSNs(dsnOptions{
		sourceDSN:        "explicit-source",
		targetDSN:        "explicit-target",
		sourceConfigPath: "configs/video_prod.yml",
		targetConfigPath: "configs/video.yml",
	}, func(path string) (string, error) {
		t.Fatalf("config loader should not be called when DSNs are explicit: %s", path)
		return "", nil
	})
	if err != nil {
		t.Fatalf("resolveDSNs returned error: %v", err)
	}
	if sourceDSN != "explicit-source" {
		t.Fatalf("sourceDSN = %q, want %q", sourceDSN, "explicit-source")
	}
	if targetDSN != "explicit-target" {
		t.Fatalf("targetDSN = %q, want %q", targetDSN, "explicit-target")
	}
}

func TestResolveDSNsRequiresSourceWhenConfigIsEmpty(t *testing.T) {
	_, _, err := resolveDSNs(dsnOptions{
		sourceConfigPath: "configs/video_prod.yml",
		targetConfigPath: "configs/video.yml",
	}, func(path string) (string, error) {
		if path == "configs/video.yml" {
			return "local-dsn", nil
		}
		return "", nil
	})
	if err == nil {
		t.Fatal("resolveDSNs returned nil error")
	}
	if !strings.Contains(err.Error(), "source DSN is required") {
		t.Fatalf("error = %q, want source DSN message", err.Error())
	}
}

func TestCreateEnumTypeSQLQuotesNameAndLabels(t *testing.T) {
	ddl := createEnumTypeSQL(enumType{
		Name:   `literature"type`,
		Labels: []string{"article", "teacher's pick"},
	})

	wantParts := []string{
		`CREATE TYPE public."literature""type" AS ENUM ('article', 'teacher''s pick')`,
		`EXCEPTION WHEN duplicate_object THEN NULL`,
	}
	for _, part := range wantParts {
		if !strings.Contains(ddl, part) {
			t.Fatalf("ddl = %q, want to contain %q", ddl, part)
		}
	}
}

func TestColumnDefinitionSQLPreservesSourceDefinition(t *testing.T) {
	def := columnDefinitionSQL(columnDescription{
		Name:    "original_image_storage_type",
		Type:    "text",
		Default: "'local'::text",
		NotNull: true,
	})

	want := `"original_image_storage_type" text DEFAULT 'local'::text NOT NULL`
	if def != want {
		t.Fatalf("column definition = %q, want %q", def, want)
	}
}

func TestColumnDefinitionSQLUsesIdentityForSerialDefaults(t *testing.T) {
	def := columnDefinitionSQL(columnDescription{
		Name:    "id",
		Type:    "bigint",
		Default: `nextval('some_sequence'::regclass)`,
		NotNull: true,
	})

	want := `"id" bigint GENERATED BY DEFAULT AS IDENTITY NOT NULL`
	if def != want {
		t.Fatalf("column definition = %q, want %q", def, want)
	}
}
