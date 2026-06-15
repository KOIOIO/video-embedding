package transcodeworker

import (
	"context"
	"errors"
	"os"
	"time"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/config"
	"nlp-video-analysis/internal/infrastructure/fs"
	"nlp-video-analysis/internal/infrastructure/objectstorage"
	"nlp-video-analysis/internal/infrastructure/persistence"
	infraredis "nlp-video-analysis/internal/infrastructure/redis"
	"nlp-video-analysis/internal/infrastructure/transcode"
	"nlp-video-analysis/internal/lifecycle"
	"nlp-video-analysis/internal/model"

	goredis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// WorkerCountFromConfig 返回转码 worker 数量，配置缺失或非法时至少启动 1 个。
func WorkerCountFromConfig(cfg config.Config) int {
	if cfg.Transcode.WorkerCount <= 0 {
		return 1
	}
	return cfg.Transcode.WorkerCount
}

// Register 向生命周期容器注册转码 worker。
// 它负责连接 Redis、PostgreSQL、对象存储，并启动多个消费循环。
func Register(app *lifecycle.App, cfg config.Config) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	app.AddCloser(func(ctx context.Context) error { return rdb.Close() })

	if err := rdb.Ping(app.Context()).Err(); err != nil {
		zap.L().Fatal("redis_connect_failed", zap.String("worker", "transcode"), zap.String("err", err.Error()))
	}

	if cfg.Postgres.DSN == "" {
		zap.L().Fatal("postgres_dsn_empty", zap.String("worker", "transcode"))
	}
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		zap.L().Fatal("db_connect_failed", zap.String("worker", "transcode"), zap.String("err", err.Error()))
	}
	if sqlDB, err := db.DB(); err == nil {
		app.AddCloser(func(ctx context.Context) error { return sqlDB.Close() })
	}
	_ = db.Exec("CREATE EXTENSION IF NOT EXISTS vector;").Error
	if err := db.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoUserReaction{}, &model.EduUserReaction{}, &model.EduVideoSegment{}, &model.EduVideoVectorStage{}, &model.EduUserVideoRecommend{}); err != nil {
		zap.L().Fatal("db_migrate_failed", zap.String("worker", "transcode"), zap.String("err", err.Error()))
	}
	if err := persistence.EnsureIntegrity(db); err != nil {
		zap.L().Fatal("db_integrity_failed", zap.String("worker", "transcode"), zap.String("err", err.Error()))
	}

	rawDir := config.RawPath(cfg)
	hlsDir := config.HLSPath(cfg)

	accessKey := cfg.RustFS.AccessKey
	if accessKey == "" {
		accessKey = os.Getenv("RUSTFS_ACCESS_KEY")
	}
	secretKey := cfg.RustFS.SecretKey
	if secretKey == "" {
		secretKey = os.Getenv("RUSTFS_SECRET_KEY")
	}
	store, err := objectstorage.NewRustFS(objectstorage.Config{
		Endpoint:  cfg.RustFS.Endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    cfg.RustFS.Bucket,
		UseSSL:    cfg.RustFS.UseSSL,
	})
	if err != nil {
		zap.L().Fatal("init_rustfs_failed", zap.String("worker", "transcode"), zap.String("err", err.Error()))
	}
	if err := store.EnsureBucket(app.Context()); err != nil {
		zap.L().Fatal("ensure_bucket_failed", zap.String("worker", "transcode"), zap.String("err", err.Error()))
	}
	uploader := objectstorage.NewDirUploader(store)

	repo := persistence.NewGormVideoRepository(db)
	fileStorage := fs.NewLocalFileStorage()
	queueKey := config.TranscodeQueueKey(cfg)
	queue := infraredis.NewTranscodeQueue(rdb, queueKey)
	reactionQueue := infraredis.NewVideoReactionBufferWithOptions(rdb, infraredis.VideoReactionBufferOptions{
		StreamKey:    config.VideoReactionQueueKey(cfg),
		CountsPrefix: config.VideoReactionCountsPrefix(cfg),
		UserPrefix:   config.VideoReactionUserPrefix(cfg),
	})
	segmentReactionQueue := infraredis.NewVideoReactionBufferWithOptions(rdb, infraredis.VideoReactionBufferOptions{
		StreamKey:    config.SegmentReactionQueueKey(cfg),
		CountsPrefix: config.SegmentReactionCountsPrefix(cfg),
		UserPrefix:   config.SegmentReactionUserPrefix(cfg),
	})
	statusStore := infraredis.NewTranscodeStatusStore(rdb, config.TranscodeStatusPrefix(cfg))
	videoapp.SetRuntimeCounters(infraredis.NewRuntimeCounterStore(rdb, config.RuntimeActiveCounterPrefix(cfg)))
	transcoder := transcode.NewFFmpegTranscoder(cfg.FFmpeg, cfg.Transcode.Mode)
	taskTimeout := time.Duration(cfg.Transcode.TaskTimeoutMinutes) * time.Minute
	if taskTimeout <= 0 {
		taskTimeout = 6 * time.Hour
	}
	worker := videoapp.NewWorker(queue, statusStore, repo, transcoder, store, store, uploader, fileStorage, rawDir, hlsDir, taskTimeout)
	worker.CoverURLPrefix = config.CoverURLPrefix(cfg)
	reactionWorker := videoapp.NewVideoReactionWorker(reactionQueue, repo)
	segmentReactionWorker := videoapp.NewSegmentReactionWorker(segmentReactionQueue, repo)

	zap.L().Info("transcode_worker_start",
		zap.String("queue_key", queueKey),
		zap.String("cover", "on"),
	)

	for i := 0; i < WorkerCountFromConfig(cfg); i++ {
		id := i
		app.Go(func(ctx context.Context) error {
			for {
				if err := worker.RunOnce(ctx); err != nil {
					if errors.Is(err, context.Canceled) {
						return nil
					}
					zap.L().Error("transcode_run_once_failed",
						zap.Int("worker_id", id),
						zap.String("err", err.Error()),
					)
					select {
					case <-time.After(time.Second):
					case <-ctx.Done():
						return nil
					}
				}
			}
		})
	}

	app.Go(func(ctx context.Context) error {
		for {
			if err := reactionWorker.RunOnce(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				zap.L().Error("video_reaction_run_once_failed", zap.String("err", err.Error()))
				select {
				case <-time.After(time.Second):
				case <-ctx.Done():
					return nil
				}
			}
		}
	})

	app.Go(func(ctx context.Context) error {
		for {
			if err := segmentReactionWorker.RunOnce(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				zap.L().Error("segment_reaction_run_once_failed", zap.String("err", err.Error()))
				select {
				case <-time.After(time.Second):
				case <-ctx.Done():
					return nil
				}
			}
		}
	})
}
