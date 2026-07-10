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
	configFile string
	modelName  string
	outputFile string
}

func main() {
	config.EnsureProjectRoot()
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{configFile: defaultConfigPath, modelName: "recbole"}
	fs := flag.NewFlagSet("export_active_recsys_model_metrics", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configFile, "config", opts.configFile, "config file used for PostgreSQL DSN")
	fs.StringVar(&opts.modelName, "model", opts.modelName, "recommendation model name")
	fs.StringVar(&opts.outputFile, "output", "", "JSON output path")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.configFile = strings.TrimSpace(opts.configFile)
	opts.modelName = strings.TrimSpace(opts.modelName)
	opts.outputFile = strings.TrimSpace(opts.outputFile)
	if opts.configFile == "" {
		return options{}, errors.New("config is required")
	}
	if opts.modelName == "" {
		return options{}, errors.New("model is required")
	}
	if opts.outputFile == "" {
		return options{}, errors.New("output is required")
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
	db.SetConnMaxLifetime(30 * time.Second)
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	metricsJSON, err := loadActiveMetricsJSON(ctx, db, opts.modelName)
	if err != nil {
		return err
	}
	if err := os.WriteFile(opts.outputFile, []byte(metricsJSON+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "active_recsys_metrics_output=%s\n", opts.outputFile)
	return nil
}

func loadActiveMetricsJSON(ctx context.Context, db *sql.DB, modelName string) (string, error) {
	var metricsJSON string
	err := db.QueryRowContext(ctx, buildActiveMetricsQuery(), modelName).Scan(&metricsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return "{}", nil
	}
	if err != nil {
		return "", err
	}
	metricsJSON = strings.TrimSpace(metricsJSON)
	if metricsJSON == "" {
		return "{}", nil
	}
	return metricsJSON, nil
}

func buildActiveMetricsQuery() string {
	return `
SELECT COALESCE(metrics_json::text, '{}')
FROM recsys.recommend_model_version
WHERE model_name = $1
  AND is_active = TRUE
  AND status = 1
  AND deleted = 0
ORDER BY published_at DESC
LIMIT 1`
}
