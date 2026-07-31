package knowledgevideoworker

import (
	"context"
	"errors"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"video-service/internal/application/knowledgevideo"
	"video-service/internal/config"
	"video-service/internal/infrastructure/objectstorage"
	"video-service/internal/infrastructure/persistence"
	infraredis "video-service/internal/infrastructure/redis"
	"video-service/internal/infrastructure/transcode"
	"video-service/internal/lifecycle"
)

func WorkerCountFromConfig(cfg config.Config) int {
	if cfg.KnowledgeVideoWorker.WorkerCount <= 0 {
		return 1
	}
	return cfg.KnowledgeVideoWorker.WorkerCount
}

func Register(app *lifecycle.App, cfg config.Config) {
	rdb := goredis.NewClient(&goredis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	app.AddCloser(func(context.Context) error { return rdb.Close() })
	if err := rdb.Ping(app.Context()).Err(); err != nil {
		zap.L().Fatal("redis_connect_failed", zap.String("worker", "knowledge_video"), zap.Error(err))
	}
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		zap.L().Fatal("db_connect_failed", zap.String("worker", "knowledge_video"), zap.Error(err))
	}
	if sqlDB, err := db.DB(); err == nil {
		app.AddCloser(func(context.Context) error { return sqlDB.Close() })
	}
	if err := persistence.EnsureSchema(db); err != nil {
		zap.L().Fatal("db_integrity_failed", zap.String("worker", "knowledge_video"), zap.Error(err))
	}
	store, err := objectstorage.NewRustFS(config.KnowledgeVideoObjectStorageConfig(cfg))
	if err != nil {
		zap.L().Fatal("init_rustfs_failed", zap.String("worker", "knowledge_video"), zap.Error(err))
	}
	if err := store.EnsureBucket(app.Context()); err != nil {
		zap.L().Fatal("ensure_bucket_failed", zap.String("worker", "knowledge_video"), zap.Error(err))
	}
	repo := persistence.NewGormKnowledgeVideoRepository(db)
	queue := infraredis.NewKnowledgeVideoTranscodeQueue(rdb, cfg.RedisKeys.KnowledgeVideoTranscodeQueue)
	taskTimeout := time.Duration(cfg.KnowledgeVideoWorker.TaskTimeoutMinutes) * time.Minute
	if taskTimeout <= 0 {
		taskTimeout = 30 * time.Minute
	}
	queue.SetPendingMinIdle(taskTimeout + time.Minute)
	worker := knowledgevideo.Worker{
		Repository: repo, Queue: queue, Downloader: store,
		Transcoder: transcode.NewFFmpegTranscoder(cfg.FFmpeg, cfg.Transcode.Mode),
		Uploader:   objectstorage.NewDirUploader(store), TempRoot: cfg.KnowledgeVideoStorage.TempPath,
		MasterPlaylist: "master.m3u8", TaskTimeout: taskTimeout,
	}
	for index := 0; index < WorkerCountFromConfig(cfg); index++ {
		workerID := index
		app.Go(func(ctx context.Context) error {
			for {
				if err := worker.RunOnce(ctx); err != nil {
					if errors.Is(err, context.Canceled) {
						return nil
					}
					zap.L().Error("knowledge_video_worker_failed", zap.Int("worker_id", workerID), zap.Error(err))
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(time.Second):
				}
			}
		})
	}
	reconciler := knowledgevideo.Reconciler{Repository: repo, Queue: queue}
	app.Go(func(ctx context.Context) error {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if err := reconciler.RunOnce(ctx); err != nil {
					zap.L().Error("knowledge_video_reconcile_failed", zap.Error(err))
				}
			}
		}
	})
}
