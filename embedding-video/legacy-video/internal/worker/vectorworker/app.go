package vectorworker

import (
	"context"
	"errors"
	"legacy-video/internal/config"
	"legacy-video/internal/infrastructure/objectstorage"
	"legacy-video/internal/infrastructure/persistence"
	infraredis "legacy-video/internal/infrastructure/redis"
	"legacy-video/internal/infrastructure/transcode"
	"legacy-video/internal/lifecycle"
	"legacy-video/internal/model"
	"legacy-video/internal/worker/antspool"
	"legacy-video/internal/worker/vectorworker/tasks"
	"os"
	"path/filepath"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const maxASRWorkers = 20

const (
	poolVectorCoarse    = "vector.coarse"
	poolVectorSampleASR = "vector.sample_asr"
	poolVectorRefineASR = "vector.refine_asr"
)

func normalizeASRWorkers(workers int) int {
	if workers <= 0 {
		workers = 4
	}
	if workers > maxASRWorkers {
		workers = maxASRWorkers
	}
	return workers
}

func resolvePoolSize(pools config.WorkerPoolsConfig, name string, fallback int) int {
	if pools != nil {
		if cfg, ok := pools[name]; ok && cfg.Size > 0 {
			return cfg.Size
		}
	}
	return fallback
}

// Register 向生命周期容器注册向量化 worker。
// 它负责装配 ASR、LLM、Embedding、对象存储和队列消费主循环。
func Register(app *lifecycle.App, cfg config.Config) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	app.AddCloser(func(ctx context.Context) error { return rdb.Close() })
	if err := rdb.Ping(app.Context()).Err(); err != nil {
		zap.L().Fatal("redis_connect_failed", zap.String("worker", "vector"), zap.String("err", err.Error()))
	}

	if cfg.Postgres.DSN == "" {
		zap.L().Fatal("postgres_dsn_empty", zap.String("worker", "vector"))
	}
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		zap.L().Fatal("db_connect_failed", zap.String("worker", "vector"), zap.String("err", err.Error()))
	}
	if sqlDB, err := db.DB(); err == nil {
		if cfg.Postgres.MaxOpenConns > 0 {
			sqlDB.SetMaxOpenConns(cfg.Postgres.MaxOpenConns)
		}
		if cfg.Postgres.MaxIdleConns > 0 {
			sqlDB.SetMaxIdleConns(cfg.Postgres.MaxIdleConns)
		}
		if cfg.Postgres.ConnMaxLifetime > 0 {
			sqlDB.SetConnMaxLifetime(time.Duration(cfg.Postgres.ConnMaxLifetime) * time.Second)
		}
		if cfg.Postgres.ConnMaxIdleTime > 0 {
			sqlDB.SetConnMaxIdleTime(time.Duration(cfg.Postgres.ConnMaxIdleTime) * time.Second)
		}
		app.AddCloser(func(ctx context.Context) error { return sqlDB.Close() })
	}
	_ = db.Exec("CREATE EXTENSION IF NOT EXISTS vector;").Error
	if err := db.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoSegment{}, &model.EduUserVideoRecommend{}); err != nil {
		zap.L().Fatal("db_migrate_failed", zap.String("worker", "vector"), zap.String("err", err.Error()))
	}
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_video ON edu_video_segment(video_id);`).Error
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_embedding ON edu_video_segment USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);`).Error
	if err := persistence.EnsureIntegrity(db); err != nil {
		zap.L().Fatal("db_integrity_failed", zap.String("worker", "vector"), zap.String("err", err.Error()))
	}

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
		zap.L().Fatal("rustfs_init_failed", zap.String("worker", "vector"), zap.String("err", err.Error()))
	}
	if err := store.EnsureBucket(app.Context()); err != nil {
		zap.L().Fatal("rustfs_bucket_failed", zap.String("worker", "vector"), zap.String("err", err.Error()))
	}

	client, err := newOpenAICompatClient(cfg)
	if err != nil {
		zap.L().Fatal("openai_client_init_failed", zap.String("worker", "vector"), zap.String("err", err.Error()))
	}

	windowSec := cfg.VectorWorker.SegmentWindowSec
	stepSec := cfg.VectorWorker.SegmentStepSec
	asrWorkers := cfg.VectorWorker.ASRWorkers
	coarseWorkers := cfg.VectorWorker.CoarseWorkers
	embedBatch := cfg.VectorWorker.EmbedBatch
	mode := cfg.VectorWorker.Mode
	sampleCount := cfg.VectorWorker.SampleCount
	sampleDurSec := cfg.VectorWorker.SampleDurSec
	coarseSegmentSec := cfg.VectorWorker.CoarseSegmentSec
	refineMinSegmentSec := cfg.VectorWorker.RefineMinSegmentSec
	refineMaxSegmentSec := cfg.VectorWorker.RefineMaxSegmentSec
	llmModel := cfg.VectorWorker.LLMModel
	llmTimeoutMinutes := cfg.VectorWorker.LLMTimeoutMinutes
	tailCfg := tasks.NormalizeTailAlignmentConfig(tasks.TailAlignmentConfig{
		Enabled:       cfg.VectorWorker.TailAlignmentEnabled,
		MaxExtendSec:  cfg.VectorWorker.TailAlignmentMaxExtendSec,
		ProbeStepSec:  cfg.VectorWorker.TailAlignmentProbeStepSec,
		MaxOverlapSec: cfg.VectorWorker.TailAlignmentMaxOverlapSec,
	})

	if windowSec <= 0 {
		windowSec = 60
	}
	if stepSec <= 0 {
		stepSec = windowSec
	}
	asrWorkers = normalizeASRWorkers(asrWorkers)
	if coarseWorkers <= 0 {
		coarseWorkers = asrWorkers
	}
	asrWorkers = normalizeASRWorkers(resolvePoolSize(cfg.WorkerPools, poolVectorSampleASR, asrWorkers))
	coarseWorkers = resolvePoolSize(cfg.WorkerPools, poolVectorCoarse, coarseWorkers)
	if embedBatch <= 0 {
		embedBatch = 64
	}
	if sampleCount <= 0 {
		sampleCount = 3
	}
	if sampleDurSec <= 0 {
		sampleDurSec = 10
	}
	if coarseSegmentSec <= 0 {
		coarseSegmentSec = 15
	}
	if refineMinSegmentSec <= 0 {
		refineMinSegmentSec = 20
	}
	if refineMaxSegmentSec <= 0 {
		refineMaxSegmentSec = 180
	}
	if strings.TrimSpace(llmModel) == "" {
		llmModel = "qwen-plus"
	}
	if llmTimeoutMinutes <= 0 {
		llmTimeoutMinutes = 3
	}
	if !cfg.VectorWorker.TailAlignmentConfigured {
		tailCfg.Enabled = true
	}

	tmpRoot := filepath.Join(os.TempDir(), "legacy-video", "tmp", "video_vectorize")
	_ = os.MkdirAll(tmpRoot, 0755)

	queueKey := "video:vectorize:queue"
	queue := infraredis.NewVectorizeQueue(rdb, queueKey)
	ff := transcode.NewFFmpegTranscoder(cfg.FFmpeg, cfg.Transcode.Mode)
	poolRecorder := antspool.NewMemoryRecorder()
	antspool.SetDefaultRecorder(poolRecorder)
	coarsePoolSize := resolvePoolSize(cfg.WorkerPools, poolVectorCoarse, coarseWorkers)
	sampleASRPoolSize := normalizeASRWorkers(resolvePoolSize(cfg.WorkerPools, poolVectorSampleASR, asrWorkers))
	refineASRPoolSize := normalizeASRWorkers(resolvePoolSize(cfg.WorkerPools, poolVectorRefineASR, asrWorkers))

	zap.L().Info("vector_worker_start",
		zap.String("queue_key", queueKey),
		zap.String("redis", cfg.Redis.Addr),
		zap.Int("redis_db", cfg.Redis.DB),
		zap.String("mode", mode),
		zap.Int("window_sec", windowSec),
		zap.Int("step_sec", stepSec),
		zap.Int("asr_workers", asrWorkers),
		zap.Int("coarse_workers", coarseWorkers),
		zap.Int("pool_vector_sample_asr", sampleASRPoolSize),
		zap.Int("pool_vector_coarse", coarsePoolSize),
		zap.Int("pool_vector_refine_asr", refineASRPoolSize),
		zap.Int("embed_batch", embedBatch),
		zap.Int("sample_count", sampleCount),
		zap.Int("sample_dur_sec", sampleDurSec),
		zap.Int("coarse_segment_sec", coarseSegmentSec),
		zap.Int("refine_min_sec", refineMinSegmentSec),
		zap.Int("refine_max_sec", refineMaxSegmentSec),
		zap.String("llm_model", llmModel),
		zap.Int("llm_timeout_min", llmTimeoutMinutes),
		zap.Bool("tail_alignment_enabled", tailCfg.Enabled),
		zap.Int("tail_alignment_max_extend_sec", tailCfg.MaxExtendSec),
		zap.Int("tail_alignment_probe_step_sec", tailCfg.ProbeStepSec),
		zap.Int("tail_alignment_max_overlap_sec", tailCfg.MaxOverlapSec),
	)

	app.Go(func(ctx context.Context) error {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				_, err := rdb.XLen(ctx, queueKey).Result()
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return nil
					}
					zap.L().Error("queue_len_failed", zap.String("queue_key", queueKey), zap.String("err", err.Error()))
					continue
				}
			case <-ctx.Done():
				return nil
			}
		}
	})

	workerCount := cfg.VectorWorker.CoarseWorkers
	if workerCount <= 0 {
		workerCount = 1
	}
	taskTimeout := time.Duration(cfg.VectorWorker.TaskTimeoutMinutes) * time.Minute
	if taskTimeout <= 0 {
		taskTimeout = 3 * time.Hour
	}
	for i := 0; i < workerCount; i++ {
		id := i
		const maxRetryTimes = 3

		app.Go(func(ctx context.Context) error {
			for {
				task, err := queue.Dequeue(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return nil
					}
					zap.L().Error("dequeue_failed", zap.Int("worker_id", id), zap.String("err", err.Error()))
					select {
					case <-time.After(time.Second):
					case <-ctx.Done():
						return nil
					}
					continue
				}
				zap.L().Info("dequeue",
					zap.Int("worker_id", id),
					zap.Uint64("video_id", task.VideoID),
					zap.String("task_id", task.TaskID),
					zap.String("raw_key", task.RawKey),
				)

				var lastErr error
				retryDelay := time.Second * 5
				taskStart := time.Now()

				for retry := 0; retry < maxRetryTimes; retry++ {
					taskCtx := ctx
					var cancel context.CancelFunc
					if taskTimeout > 0 {
						taskCtx, cancel = context.WithTimeout(ctx, taskTimeout)
					}

					zap.L().Info("start",
						zap.Int("worker_id", id),
						zap.Uint64("video_id", task.VideoID),
						zap.String("task_id", task.TaskID),
						zap.Int("retry", retry),
					)

					lastErr = handleVectorizeTask(taskCtx, db, store, ff, client, tmpRoot, mode, windowSec, stepSec, sampleASRPoolSize, coarsePoolSize, embedBatch, sampleCount, sampleDurSec, coarseSegmentSec, refineMinSegmentSec, refineMaxSegmentSec, llmModel, llmTimeoutMinutes, tailCfg, task.VideoID, task.TaskID, task.RawKey)
					if cancel != nil {
						cancel()
					}

					if lastErr == nil {
						zap.L().Info("done",
							zap.Int("worker_id", id),
							zap.Uint64("video_id", task.VideoID),
							zap.String("task_id", task.TaskID),
							zap.Int("retry", retry),
							zap.Int64("total_cost_ms", time.Since(taskStart).Milliseconds()),
						)
						break
					}

					if errors.Is(lastErr, context.Canceled) {
						return nil
					}

					if retry < maxRetryTimes-1 {
						zap.L().Warn("retry",
							zap.Int("worker_id", id),
							zap.Uint64("video_id", task.VideoID),
							zap.String("task_id", task.TaskID),
							zap.Int("retry", retry),
							zap.Int("max_retries", maxRetryTimes),
							zap.String("err", lastErr.Error()),
						)
						select {
						case <-time.After(retryDelay):
							retryDelay *= 2
						case <-ctx.Done():
							return nil
						}
					} else {
						zap.L().Error("failed",
							zap.Int("worker_id", id),
							zap.Uint64("video_id", task.VideoID),
							zap.String("task_id", task.TaskID),
							zap.Int("retries", maxRetryTimes),
							zap.Int64("total_cost_ms", time.Since(taskStart).Milliseconds()),
							zap.String("err", lastErr.Error()),
						)
					}
				}

				select {
				case <-time.After(time.Second):
				case <-ctx.Done():
					return nil
				}
			}
		})
	}

	app.Go(func(ctx context.Context) error {
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				for _, name := range []string{poolVectorCoarse, poolVectorSampleASR, poolVectorRefineASR} {
					s := poolRecorder.Snapshot(name)
					zap.L().Info("ants_pool_metrics",
						zap.String("pool_name", s.Name),
						zap.Int("size", s.Size),
						zap.Int("submitted_total", s.SubmittedTotal),
						zap.Int("submit_errors", s.SubmitErrors),
						zap.Int("completed_total", s.CompletedTotal),
						zap.Int("task_errors", s.TaskErrors),
						zap.Duration("last_task_cost", s.LastTaskCost),
					)
				}
			case <-ctx.Done():
				return nil
			}
		}
	})
}
