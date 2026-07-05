package gorsesync

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
	appsync "nlp-video-analysis/internal/application/videoapp/recommendation/gorsesync"
	"nlp-video-analysis/internal/config"
	"nlp-video-analysis/internal/lifecycle"
)

type Scheduler struct {
	Interval time.Duration
	Sync     func(context.Context) (appsync.Result, error)

	mu      sync.Mutex
	running bool
}

func Register(app *lifecycle.App, cfg config.Config) {
	if !cfg.Gorse.SyncEnabled {
		zap.L().Info("gorse_sync_disabled")
		return
	}
	db, err := openDB(cfg)
	if err != nil {
		zap.L().Error("gorse_sync_db_open_failed", zap.Error(err))
		return
	}
	app.AddCloser(func(context.Context) error { return db.Close() })
	client := recommendationapp.NewGorseHTTPClient(recommendationapp.GorseClientConfig{
		Endpoint: config.GorseEndpoint(cfg),
		APIKey:   cfg.Gorse.APIKey,
		Timeout:  config.GorseTimeout(cfg),
	})
	syncer := appsync.Syncer{
		Source: appsync.PostgresSource{DB: db, UserIDColumn: "id"},
		Client: client,
		Options: appsync.Options{
			BatchSize:         500,
			EnableGate:        cfg.Gorse.EnableGate,
			MinFeedbackCount:  cfg.Gorse.MinFeedbackCount,
			MinRecommendItems: cfg.Gorse.MinRecommendItems,
		},
	}
	scheduler := &Scheduler{
		Interval: config.GorseSyncInterval(cfg),
		Sync:    syncer.Run,
	}
	app.Go(scheduler.Run)
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("gorse sync scheduler is nil")
	}
	if s.Sync == nil {
		return fmt.Errorf("gorse sync function is nil")
	}
	ticker := time.NewTicker(s.interval())
	defer ticker.Stop()
	if err := s.runOnce(ctx); err != nil {
		zap.L().Warn("gorse_sync_run_failed", zap.Error(err))
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.runOnce(ctx); err != nil {
				zap.L().Warn("gorse_sync_run_failed", zap.Error(err))
			}
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) error {
	if !s.tryStart() {
		zap.L().Warn("gorse_sync_skip_overlapping_run")
		return nil
	}
	defer s.finish()
	result, err := s.Sync(ctx)
	if err != nil {
		return err
	}
	zap.L().Info("gorse_sync_run_finished",
		zap.Int("users", result.Users),
		zap.Int("items", result.Items),
		zap.Int("feedback", result.Feedback),
		zap.Bool("gate_passed", result.GatePassed),
		zap.String("gate_reason", result.GateReason),
	)
	return nil
}

func (s *Scheduler) interval() time.Duration {
	if s.Interval <= 0 {
		return time.Hour
	}
	return s.Interval
}

func (s *Scheduler) tryStart() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return false
	}
	s.running = true
	return true
}

func (s *Scheduler) finish() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
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
		return nil, err
	}
	return db, nil
}
