package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"nlp-video-analysis/internal/application/videoapp"
	"nlp-video-analysis/internal/config"
	aiinfra "nlp-video-analysis/internal/infrastructure/ai"
	"nlp-video-analysis/internal/infrastructure/embedding"
	"nlp-video-analysis/internal/infrastructure/fs"
	"nlp-video-analysis/internal/infrastructure/objectstorage"
	"nlp-video-analysis/internal/infrastructure/persistence"
	infraredis "nlp-video-analysis/internal/infrastructure/redis"
	"nlp-video-analysis/internal/model"
)

type App struct {
	DB               *gorm.DB
	Redis            *redis.Client
	Store            *objectstorage.RustFS
	Service          *videoapp.Service
	MediaRoutePrefix string
	HTTP             HTTPRuntimeConfig
}

type HTTPRuntimeConfig struct {
	LogDir               string
	SlowRequestThreshold time.Duration
	CORSAllowOrigin      string
	CORSAllowMethods     string
	CORSAllowHeaders     string
	CORSExposeHeaders    string
	CORSMaxAge           string
}

func ResolveHTTPAddr(cfg config.Config) string {
	if addr := strings.TrimSpace(os.Getenv("HTTP_ADDR")); addr != "" {
		return addr
	}
	return config.HTTPAddr(cfg)
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	rawDir := config.RawPath(cfg)
	hlsDir := config.HLSPath(cfg)

	if cfg.Postgres.DSN == "" {
		return nil, errConfig("postgres_dsn_empty", "Postgres DSN is required")
	}
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := configureDB(db, cfg); err != nil {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}

	store, err := newObjectStore(cfg)
	if err != nil {
		_ = rdb.Close()
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	if err := store.EnsureBucket(ctx); err != nil {
		_ = rdb.Close()
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}

	repo := persistence.NewGormVideoRepository(db)
	queue := infraredis.NewTranscodeQueue(rdb, config.TranscodeQueueKey(cfg))
	vectorQueue := infraredis.NewVectorizeQueue(rdb, config.VectorizeQueueKey(cfg))
	reactionBuffer := infraredis.NewVideoReactionBufferWithOptions(rdb, infraredis.VideoReactionBufferOptions{
		StreamKey:    config.VideoReactionQueueKey(cfg),
		CountsPrefix: config.VideoReactionCountsPrefix(cfg),
		UserPrefix:   config.VideoReactionUserPrefix(cfg),
	})
	segmentReactionBuffer := infraredis.NewVideoReactionBufferWithOptions(rdb, infraredis.VideoReactionBufferOptions{
		StreamKey:    config.SegmentReactionQueueKey(cfg),
		CountsPrefix: config.SegmentReactionCountsPrefix(cfg),
		UserPrefix:   config.SegmentReactionUserPrefix(cfg),
	})
	statusStore := infraredis.NewTranscodeStatusStore(rdb, config.TranscodeStatusPrefix(cfg))
	videoapp.SetRuntimeCounters(infraredis.NewRuntimeCounterStore(rdb, config.RuntimeActiveCounterPrefix(cfg)))
	primaryEmbedder := embedding.NewClient(cfg.Embedding)
	fallbackEmbedder := aiinfra.NewFallbackEmbedder(primaryEmbedder, aiinfra.NewLocalEmbedder(config.EmbeddingDim(cfg)))
	service := videoapp.NewService(repo, queue, vectorQueue, statusStore, store, fs.NewLocalFileStorage(), fallbackEmbedder, videoapp.Paths{
		RawDir:          rawDir,
		HLSDir:          hlsDir,
		RawObjectPrefix: config.RawObjectPrefix(cfg),
		HLSObjectPrefix: config.HLSObjectPrefix(cfg),
		RawURLPrefix:    config.RawURLPrefix(cfg),
		HLSURLPrefix:    config.HLSURLPrefix(cfg),
		CoverURLPrefix:  config.CoverURLPrefix(cfg),
		HLSMasterName:   config.HLSMasterName(cfg),
	})
	service.ReactionStore = reactionBuffer
	service.SegmentReactionStore = segmentReactionBuffer

	return &App{
		DB:               db,
		Redis:            rdb,
		Store:            store,
		Service:          service,
		MediaRoutePrefix: config.MediaRoutePrefix(cfg),
		HTTP: HTTPRuntimeConfig{
			LogDir:               config.HTTPLogDir(cfg),
			SlowRequestThreshold: config.HTTPSlowRequestThreshold(cfg),
			CORSAllowOrigin:      config.CORSAllowOrigin(cfg),
			CORSAllowMethods:     config.CORSAllowMethods(cfg),
			CORSAllowHeaders:     config.CORSAllowHeaders(cfg),
			CORSExposeHeaders:    config.CORSExposeHeaders(cfg),
			CORSMaxAge:           config.CORSMaxAge(cfg),
		},
	}, nil
}

func configureDB(db *gorm.DB, cfg config.Config) error {
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
	}
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector;").Error; err != nil {
		return err
	}
	if err := db.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoUserReaction{}, &model.EduUserReaction{}, &model.EduVideoSegment{}, &model.EduVideoVectorStage{}, &model.EduUserVideoRecommend{}); err != nil {
		return err
	}
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_video ON edu_video_segment(video_id);`).Error
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_embedding ON edu_video_segment USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);`).Error
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_video_recommend_user ON edu_user_video_recommend(user_id);`).Error
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_video_recommend_video ON edu_user_video_recommend(video_id);`).Error
	return persistence.EnsureIntegrity(db)
}

func newObjectStore(cfg config.Config) (*objectstorage.RustFS, error) {
	accessKey := cfg.RustFS.AccessKey
	if accessKey == "" {
		accessKey = os.Getenv("RUSTFS_ACCESS_KEY")
	}
	secretKey := cfg.RustFS.SecretKey
	if secretKey == "" {
		secretKey = os.Getenv("RUSTFS_SECRET_KEY")
	}
	return objectstorage.NewRustFS(objectstorage.Config{
		Endpoint:  cfg.RustFS.Endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    cfg.RustFS.Bucket,
		UseSSL:    cfg.RustFS.UseSSL,
	})
}

func (a *App) Close(ctx context.Context) error {
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
	if a.DB != nil {
		if sqlDB, err := a.DB.DB(); err == nil {
			return sqlDB.Close()
		}
	}
	return nil
}

func errConfig(msg string, detail string) error {
	zap.L().Error(msg, zap.String("err", detail))
	return errors.New(detail)
}
