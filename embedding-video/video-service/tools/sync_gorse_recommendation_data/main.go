package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
	"nlp-video-analysis/internal/application/videoapp/recommendation/gorsesync"
	"nlp-video-analysis/internal/config"
)

type options struct {
	configFile   string
	endpoint     string
	apiKey       string
	batchSize    int
	limit        int
	dryRun       bool
	syncUsers    bool
	syncItems    bool
	syncFeedback bool
	enableGate   bool
}

type syncResult = gorsesync.Result

func main() {
	config.EnsureProjectRoot()
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func parseOptions(args []string) (options, error) {
	opts := options{
		configFile:   "configs/video.yml",
		batchSize:    500,
		limit:        50000,
		syncUsers:    true,
		syncItems:    true,
		syncFeedback: true,
		enableGate:   true,
	}
	fs := flag.NewFlagSet("sync_gorse_recommendation_data", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.configFile, "config", opts.configFile, "config file")
	fs.StringVar(&opts.endpoint, "endpoint", "", "Gorse endpoint override")
	fs.StringVar(&opts.apiKey, "api-key", "", "Gorse API key override")
	fs.IntVar(&opts.batchSize, "batch-size", opts.batchSize, "batch size")
	fs.IntVar(&opts.limit, "limit", opts.limit, "maximum feedback events to load")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print counts without writing to Gorse")
	fs.BoolVar(&opts.syncUsers, "users", true, "sync users")
	fs.BoolVar(&opts.syncItems, "items", true, "sync items")
	fs.BoolVar(&opts.syncFeedback, "feedback", true, "sync feedback")
	fs.BoolVar(&opts.enableGate, "gate", true, "enforce publish gate")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if opts.batchSize <= 0 {
		return options{}, fmt.Errorf("batch-size must be > 0")
	}
	if opts.limit <= 0 {
		return options{}, fmt.Errorf("limit must be > 0")
	}
	return opts, nil
}

func run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	cfg := config.MustLoad(opts.configFile)
	if opts.endpoint != "" {
		cfg.Gorse.Endpoint = opts.endpoint
	}
	if opts.apiKey != "" {
		cfg.Gorse.APIKey = opts.apiKey
	}
	db, err := openDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	source := selectableSource{
		Source: gorsesync.PostgresSource{DB: db, UserIDColumn: "id", Limit: opts.limit},
		Users:  opts.syncUsers,
		Items:  opts.syncItems,
		Events: opts.syncFeedback,
	}
	client := recommendationapp.NewGorseHTTPClient(recommendationapp.GorseClientConfig{
		Endpoint: config.GorseEndpoint(cfg),
		APIKey:   cfg.Gorse.APIKey,
		Timeout:  config.GorseTimeout(cfg),
	})
	result, err := (gorsesync.Syncer{
		Source: source,
		Client: client,
		Options: gorsesync.Options{
			DryRun:            opts.dryRun,
			BatchSize:         opts.batchSize,
			EnableGate:        opts.enableGate && cfg.Gorse.EnableGate,
			MinFeedbackCount:  cfg.Gorse.MinFeedbackCount,
			MinRecommendItems: cfg.Gorse.MinRecommendItems,
		},
	}).Run(ctx)
	if err != nil {
		return err
	}
	printResult(out, result)
	return nil
}

func printResult(out io.Writer, result syncResult) {
	fmt.Fprintf(out, "dry_run=%v\n", result.DryRun)
	fmt.Fprintf(out, "users=%d\n", result.Users)
	fmt.Fprintf(out, "items=%d\n", result.Items)
	fmt.Fprintf(out, "feedback=%d\n", result.Feedback)
	fmt.Fprintf(out, "gate_passed=%v\n", result.GatePassed)
	if result.GateReason != "" {
		fmt.Fprintf(out, "gate_reason=%s\n", result.GateReason)
	}
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

type selectableSource struct {
	Source gorsesync.PostgresSource
	Users  bool
	Items  bool
	Events bool
}

func (s selectableSource) LoadUsers(ctx context.Context) ([]recommendationapp.GorseUser, error) {
	if !s.Users {
		return nil, nil
	}
	return s.Source.LoadUsers(ctx)
}

func (s selectableSource) LoadItems(ctx context.Context) ([]recommendationapp.GorseItem, error) {
	if !s.Items {
		return nil, nil
	}
	return s.Source.LoadItems(ctx)
}

func (s selectableSource) LoadFeedback(ctx context.Context) ([]recommendationapp.GorseFeedback, error) {
	if !s.Events {
		return nil, nil
	}
	return s.Source.LoadFeedback(ctx)
}
