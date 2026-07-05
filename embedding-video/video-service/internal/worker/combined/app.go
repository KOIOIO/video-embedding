package combined

import (
	"context"
	"time"

	"nlp-video-analysis/internal/config"
	"nlp-video-analysis/internal/lifecycle"
	gorsesyncworker "nlp-video-analysis/internal/worker/gorsesync"
	transcodeworker "nlp-video-analysis/internal/worker/transcodeworker"
	"nlp-video-analysis/internal/worker/twotowertrainer"
	vectorworker "nlp-video-analysis/internal/worker/vectorworker"
	"nlp-video-analysis/middleware"

	"go.uber.org/zap"
)

// Run 启动统一 worker 进程，并把转码与向量化 worker 注册到同一个生命周期容器中。
func Run() {
	config.EnsureProjectRoot()
	cfg := config.MustLoadDefault()
	f, err := middleware.InitFileLoggerWithOptions("worker", middleware.FileLoggerOptions{LogDir: config.LogDir(cfg)})
	if err != nil {
		zap.L().Fatal("init_logger_failed", zap.String("err", err.Error()))
	}

	app := lifecycle.New("worker", maxShutdownTimeout(cfg))
	app.AddCloser(func(ctx context.Context) error { return f.Close() })

	transcodeworker.Register(app, cfg)
	vectorworker.Register(app, cfg)
	gorsesyncworker.Register(app, cfg)
	twotowertrainer.Register(app, cfg)

	zap.L().Info("worker_start",
		zap.String("mode", "combined"),
		zap.Int("transcode_workers", normalizedTranscodeWorkerCount(cfg)),
		zap.Int("vector_workers", normalizedVectorWorkerCount(cfg)),
		zap.Bool("gorse_sync_enabled", cfg.Gorse.SyncEnabled),
		zap.Bool("two_tower_trainer_enabled", twotowertrainer.EnabledFromEnv()),
	)

	if err := app.Run(nil); err != nil {
		zap.L().Error("worker_exit", zap.String("err", err.Error()))
	}
}

// maxShutdownTimeout 取两个 worker 的较大退出超时，保证统一进程关闭时不会过早强停。
func maxShutdownTimeout(cfg config.Config) time.Duration {
	transcodeTimeout := time.Duration(cfg.Transcode.ShutdownTimeoutSec) * time.Second
	vectorTimeout := time.Duration(cfg.VectorWorker.ShutdownTimeoutSec) * time.Second
	if transcodeTimeout <= 0 {
		transcodeTimeout = 10 * time.Minute
	}
	if vectorTimeout <= 0 {
		vectorTimeout = 10 * time.Minute
	}
	if vectorTimeout > transcodeTimeout {
		return vectorTimeout
	}
	return transcodeTimeout
}

// normalizedTranscodeWorkerCount 返回日志展示用的转码 worker 数量。
func normalizedTranscodeWorkerCount(cfg config.Config) int {
	return transcodeworker.WorkerCountFromConfig(cfg)
}

// normalizedVectorWorkerCount 返回日志展示用的向量 worker 数量。
func normalizedVectorWorkerCount(cfg config.Config) int {
	if cfg.VectorWorker.CoarseWorkers <= 0 {
		return 1
	}
	return cfg.VectorWorker.CoarseWorkers
}
