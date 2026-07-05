package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"nlp-video-analysis/internal/config"
)

const defaultConfigPath = "configs/video.yml"

type options struct {
	configFile   string
	modelName    string
	modelVersion string
	artifactPath string
	metricsJSON  string
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
		modelName:   "two_tower",
		metricsJSON: "{}",
	}
	fs := flag.NewFlagSet("publish_recommend_model", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configFile, "config", opts.configFile, "config file used for PostgreSQL DSN")
	fs.StringVar(&opts.modelName, "model", opts.modelName, "recommend model name")
	fs.StringVar(&opts.modelVersion, "version", "", "model version to publish")
	fs.StringVar(&opts.artifactPath, "artifact-path", "", "optional model artifact path")
	fs.StringVar(&opts.metricsJSON, "metrics-json", opts.metricsJSON, "optional metrics JSON payload")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.configFile = strings.TrimSpace(opts.configFile)
	opts.modelName = strings.TrimSpace(opts.modelName)
	opts.modelVersion = strings.TrimSpace(opts.modelVersion)
	opts.artifactPath = strings.TrimSpace(opts.artifactPath)
	opts.metricsJSON = strings.TrimSpace(opts.metricsJSON)
	if opts.configFile == "" {
		return options{}, errors.New("config is required")
	}
	if opts.modelName == "" {
		return options{}, errors.New("model is required")
	}
	if opts.modelVersion == "" {
		return options{}, errors.New("version is required")
	}
	if opts.metricsJSON == "" {
		opts.metricsJSON = "{}"
	}
	return opts, nil
}

func run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	cfg := config.MustLoad(opts.configFile)
	if strings.TrimSpace(cfg.Postgres.DSN) == "" {
		return errors.New("postgres dsn is required")
	}
	db, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	if err := ensureSchema(ctx, db); err != nil {
		return err
	}
	now := time.Now()
	if err := publishModelVersion(ctx, db, opts.modelName, opts.modelVersion, opts.artifactPath, opts.metricsJSON, now); err != nil {
		return err
	}
	fmt.Fprintf(out, "published model=%s version=%s\n", opts.modelName, opts.modelVersion)
	return nil
}

func ensureSchema(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS public.edu_recommend_model_version (
  id BIGSERIAL PRIMARY KEY,
  model_name TEXT NOT NULL,
  model_version TEXT NOT NULL,
  artifact_path TEXT,
  metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  is_active BOOLEAN DEFAULT FALSE,
  status SMALLINT DEFAULT 1,
  published_at TIMESTAMP,
  create_time TIMESTAMP,
  update_time TIMESTAMP,
  deleted SMALLINT DEFAULT 0
)`,
		`DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uk_recommend_model_version_name_version') THEN
    ALTER TABLE public.edu_recommend_model_version
      ADD CONSTRAINT uk_recommend_model_version_name_version UNIQUE (model_name, model_version);
  END IF;
END$$`,
		`CREATE INDEX IF NOT EXISTS idx_recommend_model_version_active ON public.edu_recommend_model_version(model_name, is_active, status, deleted, published_at DESC)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
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
UPDATE public.edu_recommend_model_version
SET is_active = FALSE, update_time = $1
WHERE model_name = $2 AND deleted = 0`, now, modelName); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO public.edu_recommend_model_version
  (model_name, model_version, artifact_path, metrics_json, is_active, status, published_at, deleted, create_time, update_time)
VALUES
  ($1, $2, $3, $4::jsonb, TRUE, 1, $5, 0, $5, $5)
ON CONFLICT (model_name, model_version)
DO UPDATE SET
  artifact_path = COALESCE(NULLIF(EXCLUDED.artifact_path, ''), edu_recommend_model_version.artifact_path),
  metrics_json = EXCLUDED.metrics_json,
  is_active = TRUE,
  status = 1,
  published_at = EXCLUDED.published_at,
  deleted = 0,
  update_time = EXCLUDED.update_time`,
		modelName,
		modelVersion,
		artifactPath,
		metricsJSON,
		now,
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
