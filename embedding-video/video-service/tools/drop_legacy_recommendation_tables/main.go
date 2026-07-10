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

const (
	defaultConfigPath = "configs/video.yml"
	confirmToken      = "drop-legacy-recommendation-tables"
)

var legacyTables = []string{
	"public.edu_video_item_embedding",
	"public.edu_user_tower_embedding",
	"public.edu_recommend_model_version",
}

type options struct {
	configFile string
	execute    bool
	confirm    string
}

type tablePlan struct {
	Name  string
	Count int64
}

func main() {
	config.EnsureProjectRoot()
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{configFile: defaultConfigPath}
	fs := flag.NewFlagSet("drop_legacy_recommendation_tables", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configFile, "config", opts.configFile, "config file used for PostgreSQL DSN")
	fs.BoolVar(&opts.execute, "execute", false, "drop legacy recommendation tables")
	fs.StringVar(&opts.confirm, "confirm", "", "must be drop-legacy-recommendation-tables when execute is true")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	opts.configFile = strings.TrimSpace(opts.configFile)
	opts.confirm = strings.TrimSpace(opts.confirm)
	if opts.configFile == "" {
		return options{}, errors.New("config is required")
	}
	if opts.execute && opts.confirm != confirmToken {
		return options{}, fmt.Errorf("--execute requires --confirm %s", confirmToken)
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
	plans, err := inspectLegacyTables(ctx, db)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "execute=%v\n", opts.execute)
	for _, plan := range plans {
		fmt.Fprintf(out, "legacy_table=%s rows=%d planned_drop=%v\n", plan.Name, plan.Count, opts.execute)
	}
	if !opts.execute {
		fmt.Fprintln(out, "dry_run=true")
		return nil
	}
	return dropLegacyTables(ctx, db)
}

func inspectLegacyTables(ctx context.Context, db *sql.DB) ([]tablePlan, error) {
	plans := make([]tablePlan, 0, len(legacyTables))
	for _, table := range legacyTables {
		count, err := countRows(ctx, db, table)
		if err != nil {
			return nil, err
		}
		plans = append(plans, tablePlan{Name: table, Count: count})
	}
	return plans, nil
}

func countRows(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var count int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return count, err
}

func dropLegacyTables(ctx context.Context, db *sql.DB) error {
	for _, statement := range dropStatements() {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func dropStatements() []string {
	statements := make([]string, 0, len(legacyTables))
	for _, table := range legacyTables {
		statements = append(statements, "DROP TABLE IF EXISTS "+table)
	}
	return statements
}
