package main

import (
	"context"
	"time"

	"nlp-video-analysis/internal/config"
	"nlp-video-analysis/internal/lifecycle"
	"nlp-video-analysis/internal/worker/twotowertrainer"
	"nlp-video-analysis/middleware"

	"go.uber.org/zap"
)

func main() {
	config.EnsureProjectRoot()
	cfg := config.MustLoadDefault()
	f, err := middleware.InitFileLoggerWithOptions("two_tower_trainer", middleware.FileLoggerOptions{LogDir: config.LogDir(cfg)})
	if err != nil {
		zap.L().Fatal("init_logger_failed", zap.String("err", err.Error()))
	}

	app := lifecycle.New("two_tower_trainer", 30*time.Second)
	app.AddCloser(func(ctx context.Context) error { return f.Close() })
	twotowertrainer.RegisterScheduler(app, cfg)

	zap.L().Info("two_tower_trainer_process_start", zap.Bool("enabled", twotowertrainer.EnabledFromEnv()))
	if err := app.Run(nil); err != nil {
		zap.L().Error("two_tower_trainer_process_exit", zap.String("err", err.Error()))
	}
}
