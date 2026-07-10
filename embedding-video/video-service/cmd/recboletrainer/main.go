package main

import (
	"context"
	"time"

	"nlp-video-analysis/internal/config"
	"nlp-video-analysis/internal/lifecycle"
	"nlp-video-analysis/internal/worker/recboletrainer"
	"nlp-video-analysis/middleware"

	"go.uber.org/zap"
)

func main() {
	config.EnsureProjectRoot()
	cfg := config.MustLoadDefault()
	f, err := middleware.InitFileLoggerWithOptions("recbole_trainer", middleware.FileLoggerOptions{LogDir: config.LogDir(cfg)})
	if err != nil {
		zap.L().Fatal("init_logger_failed", zap.String("err", err.Error()))
	}

	app := lifecycle.New("recbole_trainer", 30*time.Second)
	app.AddCloser(func(ctx context.Context) error { return f.Close() })
	recboletrainer.Register(app, cfg)

	zap.L().Info("recbole_trainer_process_start", zap.Bool("enabled", recboletrainer.EnabledFromEnv()))
	if err := app.Run(nil); err != nil {
		zap.L().Error("recbole_trainer_process_exit", zap.String("err", err.Error()))
	}
}
