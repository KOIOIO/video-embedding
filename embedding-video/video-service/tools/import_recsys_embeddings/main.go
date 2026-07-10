package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pgvector/pgvector-go"

	"nlp-video-analysis/internal/config"
	"nlp-video-analysis/internal/infrastructure/persistence"
)

const (
	defaultConfigPath  = "configs/video.yml"
	defaultArtifactDir = "../recbole-training/artifacts/recbole_v1"
	defaultDim         = 64
	defaultModelName   = "recbole"
	retainedVersions   = 2
)

type options struct {
	configFile  string
	artifactDir string
	modelName   string
	dim         int
	dryRun      bool
	publish     bool
}

type itemEmbeddingRow struct {
	VideoSegmentID uint64
	VideoID        uint64
	Embedding      []float32
	ModelVersion   string
}

type userEmbeddingRow struct {
	UserID       uint64
	Embedding    []float32
	ModelVersion string
}

func main() {
	config.EnsureProjectRoot()
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{
		configFile:  defaultConfigPath,
		artifactDir: defaultArtifactDir,
		modelName:   defaultModelName,
		dim:         defaultDim,
	}
	fs := flag.NewFlagSet("import_recsys_embeddings", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configFile, "config", opts.configFile, "config file used for PostgreSQL DSN")
	fs.StringVar(&opts.artifactDir, "artifact-dir", opts.artifactDir, "RecBole artifact directory")
	fs.StringVar(&opts.modelName, "model", opts.modelName, "recommendation model name")
	fs.IntVar(&opts.dim, "dim", opts.dim, "expected embedding dimension")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "load and validate artifacts without writing database")
	fs.BoolVar(&opts.publish, "publish", false, "mark imported model_version active after import")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.configFile = strings.TrimSpace(opts.configFile)
	opts.artifactDir = strings.TrimSpace(opts.artifactDir)
	opts.modelName = strings.TrimSpace(opts.modelName)
	if opts.configFile == "" {
		return options{}, errors.New("config is required")
	}
	if opts.artifactDir == "" {
		return options{}, errors.New("artifact-dir is required")
	}
	if opts.modelName == "" {
		return options{}, errors.New("model is required")
	}
	if opts.dim <= 0 {
		return options{}, errors.New("dim must be greater than 0")
	}
	return opts, nil
}

func run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	items, err := loadItemRows(filepath.Join(opts.artifactDir, "item_embeddings.csv"), opts.dim)
	if err != nil {
		return err
	}
	users, err := loadUserRows(filepath.Join(opts.artifactDir, "user_embeddings.csv"), opts.dim)
	if err != nil {
		return err
	}
	modelVersion, err := inferArtifactModelVersion(items, users)
	if err != nil {
		return err
	}
	metricsJSON, err := loadMetricsJSON(opts.artifactDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "artifact_dir=%s item_embeddings=%d user_embeddings=%d dim=%d model=%s model_version=%s dry_run=%v publish=%v\n", opts.artifactDir, len(items), len(users), opts.dim, opts.modelName, modelVersion, opts.dryRun, opts.publish)
	if opts.dryRun {
		return nil
	}
	cfg := config.MustLoad(opts.configFile)
	if strings.TrimSpace(cfg.Postgres.DSN) == "" {
		return errors.New("postgres dsn is required")
	}
	db, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := ensureRecSysSchema(ctx, db); err != nil {
		return err
	}
	now := time.Now()
	if err := upsertEmbeddings(ctx, db, opts.modelName, items, users, now); err != nil {
		return err
	}
	fmt.Fprintf(out, "imported_item_embeddings=%d imported_user_embeddings=%d\n", len(items), len(users))
	if opts.publish {
		if err := publishModelVersion(ctx, db, opts.modelName, modelVersion, opts.artifactDir, metricsJSON, now); err != nil {
			return err
		}
		deletedItems, deletedUsers, err := cleanupOldEmbeddingVersions(ctx, db, opts.modelName, retainedVersions)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "published_model_version=%s cleaned_old_item_embeddings=%d cleaned_old_user_embeddings=%d retained_versions=%d\n", modelVersion, deletedItems, deletedUsers, retainedVersions)
	}
	return nil
}

func openDB(cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		return nil, err
	}
	if cfg.Postgres.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.Postgres.MaxOpenConns)
	}
	if cfg.Postgres.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.Postgres.MaxIdleConns)
	}
	if cfg.Postgres.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.Postgres.ConnMaxLifetime) * time.Second)
	}
	if cfg.Postgres.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(cfg.Postgres.ConnMaxIdleTime) * time.Second)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return db, nil
}

func ensureRecSysSchema(ctx context.Context, db *sql.DB) error {
	for _, statement := range persistence.RecSysSchemaStatements() {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func loadItemRows(path string, dim int) ([]itemEmbeddingRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	rows := make([]itemEmbeddingRow, 0, len(records))
	for _, record := range records {
		videoSegmentID, err := parseUintField(record, "video_segment_id")
		if err != nil {
			return nil, err
		}
		videoID, err := parseUintField(record, "video_id")
		if err != nil {
			return nil, err
		}
		embedding, err := parseVector(record["embedding"], dim)
		if err != nil {
			return nil, fmt.Errorf("video_segment_id=%d: %w", videoSegmentID, err)
		}
		modelVersion := strings.TrimSpace(record["model_version"])
		if modelVersion == "" {
			return nil, fmt.Errorf("video_segment_id=%d: model_version is required", videoSegmentID)
		}
		rows = append(rows, itemEmbeddingRow{VideoSegmentID: videoSegmentID, VideoID: videoID, Embedding: embedding, ModelVersion: modelVersion})
	}
	return rows, nil
}

func loadUserRows(path string, dim int) ([]userEmbeddingRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	rows := make([]userEmbeddingRow, 0, len(records))
	for _, record := range records {
		userID, err := parseUintField(record, "user_id")
		if err != nil {
			return nil, err
		}
		embedding, err := parseVector(record["embedding"], dim)
		if err != nil {
			return nil, fmt.Errorf("user_id=%d: %w", userID, err)
		}
		modelVersion := strings.TrimSpace(record["model_version"])
		if modelVersion == "" {
			return nil, fmt.Errorf("user_id=%d: model_version is required", userID)
		}
		rows = append(rows, userEmbeddingRow{UserID: userID, Embedding: embedding, ModelVersion: modelVersion})
	}
	return rows, nil
}

func readCSV(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}
	header := rows[0]
	out := make([]map[string]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		record := make(map[string]string, len(header))
		for i, name := range header {
			if i < len(row) {
				record[name] = row[i]
			}
		}
		out = append(out, record)
	}
	return out, nil
}

func parseUintField(record map[string]string, field string) (uint64, error) {
	value, err := strconv.ParseUint(strings.TrimSpace(record[field]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is invalid: %w", field, err)
	}
	if value == 0 {
		return 0, fmt.Errorf("%s must be greater than 0", field)
	}
	return value, nil
}

func parseVector(raw string, dim int) ([]float32, error) {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "[")
	text = strings.TrimSuffix(text, "]")
	if text == "" {
		return nil, errors.New("embedding is empty")
	}
	parts := strings.Split(text, ",")
	if len(parts) != dim {
		return nil, fmt.Errorf("embedding dim = %d, want %d", len(parts), dim)
	}
	values := make([]float32, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 32)
		if err != nil {
			return nil, err
		}
		values = append(values, float32(value))
	}
	return values, nil
}

func inferArtifactModelVersion(items []itemEmbeddingRow, users []userEmbeddingRow) (string, error) {
	versions := make(map[string]struct{})
	for _, item := range items {
		version := strings.TrimSpace(item.ModelVersion)
		if version != "" {
			versions[version] = struct{}{}
		}
	}
	for _, user := range users {
		version := strings.TrimSpace(user.ModelVersion)
		if version != "" {
			versions[version] = struct{}{}
		}
	}
	if len(versions) == 0 {
		return "", errors.New("artifact model_version is required")
	}
	if len(versions) > 1 {
		return "", errors.New("artifact contains multiple model_version values")
	}
	for version := range versions {
		return version, nil
	}
	return "", errors.New("artifact model_version is required")
}

func loadMetricsJSON(artifactDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(artifactDir, "metrics.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "{}", nil
		}
		return "", err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "{}", nil
	}
	return text, nil
}

func upsertEmbeddings(ctx context.Context, db *sql.DB, modelName string, items []itemEmbeddingRow, users []userEmbeddingRow, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, upsertItemEmbeddingSQL(),
			item.VideoSegmentID,
			item.VideoID,
			pgvector.NewVector(item.Embedding),
			modelName,
			item.ModelVersion,
			now,
		); err != nil {
			return fmt.Errorf("upsert item embedding segment=%d: %w", item.VideoSegmentID, err)
		}
	}
	for _, user := range users {
		if _, err := tx.ExecContext(ctx, upsertUserEmbeddingSQL(),
			user.UserID,
			pgvector.NewVector(user.Embedding),
			modelName,
			user.ModelVersion,
			now,
		); err != nil {
			return fmt.Errorf("upsert user embedding user=%d: %w", user.UserID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func upsertItemEmbeddingSQL() string {
	return `
INSERT INTO recsys.recommend_item_embedding
  (video_segment_id, video_id, embedding, model_name, model_version, status, deleted, create_time, update_time)
VALUES
  ($1, $2, $3, $4, $5, 1, 0, $6, $6)
ON CONFLICT (video_segment_id, model_name, model_version)
DO UPDATE SET
  video_id = EXCLUDED.video_id,
  embedding = EXCLUDED.embedding,
  status = 1,
  deleted = 0,
  update_time = EXCLUDED.update_time`
}

func upsertUserEmbeddingSQL() string {
	return `
INSERT INTO recsys.recommend_user_embedding
  (user_id, embedding, model_name, model_version, status, deleted, create_time, update_time)
VALUES
  ($1, $2, $3, $4, 1, 0, $5, $5)
ON CONFLICT (user_id, model_name, model_version)
DO UPDATE SET
  embedding = EXCLUDED.embedding,
  status = 1,
  deleted = 0,
  update_time = EXCLUDED.update_time`
}

func publishModelVersion(ctx context.Context, db *sql.DB, modelName string, modelVersion string, artifactPath string, metricsJSON string, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `
UPDATE recsys.recommend_model_version
SET is_active = FALSE, update_time = $1
WHERE model_name = $2 AND deleted = 0`, now, modelName); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, publishModelVersionSQL(), modelName, modelVersion, artifactPath, metricsJSON, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func publishModelVersionSQL() string {
	return `
INSERT INTO recsys.recommend_model_version
  (model_name, model_version, framework, algorithm, artifact_path, metrics_json, is_active, status, published_at, deleted, create_time, update_time)
VALUES
  ($1, $2, 'recbole', COALESCE(($4::jsonb->>'algorithm'), ''), $3, $4::jsonb, TRUE, 1, $5, 0, $5, $5)
ON CONFLICT (model_name, model_version)
DO UPDATE SET
  framework = EXCLUDED.framework,
  algorithm = EXCLUDED.algorithm,
  artifact_path = EXCLUDED.artifact_path,
  metrics_json = EXCLUDED.metrics_json,
  is_active = TRUE,
  status = 1,
  published_at = EXCLUDED.published_at,
  deleted = 0,
  update_time = EXCLUDED.update_time`
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func cleanupOldEmbeddingVersions(ctx context.Context, exec sqlExecutor, modelName string, keepVersions int) (int64, int64, error) {
	if keepVersions <= 0 {
		return 0, 0, errors.New("keepVersions must be greater than 0")
	}
	deletedItems, err := deleteOldEmbeddingVersions(ctx, exec, "recsys.recommend_item_embedding", modelName, keepVersions)
	if err != nil {
		return 0, 0, err
	}
	deletedUsers, err := deleteOldEmbeddingVersions(ctx, exec, "recsys.recommend_user_embedding", modelName, keepVersions)
	if err != nil {
		return 0, 0, err
	}
	return deletedItems, deletedUsers, nil
}

func deleteOldEmbeddingVersions(ctx context.Context, exec sqlExecutor, tableName string, modelName string, keepVersions int) (int64, error) {
	result, err := exec.ExecContext(ctx, fmt.Sprintf(`
DELETE FROM %s
WHERE model_name = $1
  AND model_version IN (
    SELECT model_version
    FROM recsys.recommend_model_version
    WHERE model_name = $1
      AND status = 1
      AND deleted = 0
    ORDER BY published_at DESC, id DESC
    OFFSET $2
  )`, tableName), modelName, keepVersions)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}
