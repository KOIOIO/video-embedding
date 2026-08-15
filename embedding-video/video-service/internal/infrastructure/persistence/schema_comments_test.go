package persistence

import (
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestSchemaCommentDefinitionsCoverAllMigratedModels(t *testing.T) {
	definitions := commentDefinitionsByTable(t)

	for _, value := range autoMigrateModels {
		parsed, err := schema.Parse(value, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parse model %T: %v", value, err)
		}

		key := "public." + parsed.Table
		definition, ok := definitions[key]
		if !ok {
			t.Errorf("missing table comment definition for %s", key)
			continue
		}
		assertModelColumnsMatchComments(t, key, parsed, definition)
	}
}

func TestSchemaCommentDefinitionsCoverRecSysTables(t *testing.T) {
	definitions := commentDefinitionsByTable(t)
	want := map[string][]string{
		"recsys.recommend_model_version": {
			"id", "model_name", "model_version", "framework", "algorithm", "artifact_path", "metrics_json",
			"is_active", "status", "published_at", "create_time", "update_time", "deleted",
		},
		"recsys.recommend_user_embedding": {
			"id", "user_id", "embedding", "model_name", "model_version", "status", "deleted", "create_time", "update_time",
		},
		"recsys.recommend_item_embedding": {
			"id", "video_segment_id", "video_id", "embedding", "model_name", "model_version", "status", "deleted", "create_time", "update_time",
		},
	}

	for key, columns := range want {
		definition, ok := definitions[key]
		if !ok {
			t.Errorf("missing table comment definition for %s", key)
			continue
		}
		comments := columnCommentsByName(t, key, definition)
		if len(comments) != len(columns) {
			t.Errorf("%s has %d column comments, want %d", key, len(comments), len(columns))
		}
		for _, column := range columns {
			if strings.TrimSpace(comments[column]) == "" {
				t.Errorf("missing column comment for %s.%s", key, column)
			}
		}
	}
}

func TestSchemaCommentStatementsIncludeEveryDefinition(t *testing.T) {
	expected := 0
	for _, definition := range schemaCommentDefinitions {
		expected += 1 + len(definition.Columns)
	}

	statements := SchemaCommentStatements()
	if len(statements) != expected {
		t.Fatalf("SchemaCommentStatements() returned %d statements, want %d", len(statements), expected)
	}
	if expected != 185 {
		t.Fatalf("schema comment registry covers %d objects, want 14 tables and 171 columns", expected)
	}
	for _, statement := range statements {
		if !strings.HasPrefix(statement, "COMMENT ON ") {
			t.Errorf("unexpected schema comment statement: %q", statement)
		}
	}
}

func TestQuotePostgresLiteralEscapesApostrophes(t *testing.T) {
	if got, want := quotePostgresLiteral("teacher's video"), "'teacher''s video'"; got != want {
		t.Fatalf("quotePostgresLiteral() = %q, want %q", got, want)
	}
}

func commentDefinitionsByTable(t *testing.T) map[string]tableCommentDefinition {
	t.Helper()
	definitions := make(map[string]tableCommentDefinition, len(schemaCommentDefinitions))
	for _, definition := range schemaCommentDefinitions {
		key := definition.Schema + "." + definition.Table
		if strings.TrimSpace(definition.Comment) == "" {
			t.Errorf("missing table comment for %s", key)
		}
		if _, exists := definitions[key]; exists {
			t.Errorf("duplicate table comment definition for %s", key)
		}
		definitions[key] = definition
	}
	return definitions
}

func assertModelColumnsMatchComments(t *testing.T, key string, parsed *schema.Schema, definition tableCommentDefinition) {
	t.Helper()
	comments := columnCommentsByName(t, key, definition)
	want := make(map[string]struct{}, len(parsed.Fields))
	for _, field := range parsed.Fields {
		if field.DBName == "" {
			continue
		}
		want[field.DBName] = struct{}{}
		if strings.TrimSpace(comments[field.DBName]) == "" {
			t.Errorf("missing column comment for %s.%s", key, field.DBName)
		}
	}
	if len(comments) != len(want) {
		t.Errorf("%s has %d column comments, model has %d columns", key, len(comments), len(want))
	}
	for column := range comments {
		if _, ok := want[column]; !ok {
			t.Errorf("comment definition contains unknown column %s.%s", key, column)
		}
	}
}

func columnCommentsByName(t *testing.T, key string, definition tableCommentDefinition) map[string]string {
	t.Helper()
	comments := make(map[string]string, len(definition.Columns))
	for _, column := range definition.Columns {
		if _, exists := comments[column.Name]; exists {
			t.Errorf("duplicate column comment definition for %s.%s", key, column.Name)
		}
		comments[column.Name] = column.Comment
	}
	return comments
}
