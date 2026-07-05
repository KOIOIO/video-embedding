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
	recommendationapp "nlp-video-analysis/internal/application/videoapp/recommendation"
	"nlp-video-analysis/internal/config"
	aiinfra "nlp-video-analysis/internal/infrastructure/ai"
	einoai "nlp-video-analysis/internal/infrastructure/ai/eino"
	"nlp-video-analysis/internal/infrastructure/embedding"
	"nlp-video-analysis/internal/infrastructure/fs"
	"nlp-video-analysis/internal/infrastructure/objectstorage"
	"nlp-video-analysis/internal/infrastructure/persistence"
	infraredis "nlp-video-analysis/internal/infrastructure/redis"
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
	primaryEmbedder := newRecommendationEmbedder(ctx, cfg)
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
	gorseClient, recommendationEngine, gorseOptions := recommendationRuntimeFromConfig(cfg)
	service.RecommendationEngine = recommendationEngine
	service.GorseClient = gorseClient
	service.GorseOptions = gorseOptions
	service.RecentSegments = infraredis.NewRecentSegmentStore(rdb, config.RandomPlayRecentPrefix(cfg))
	service.RecentSegmentTTL = config.RandomPlayDedupeWindow(cfg)
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

func recommendationRuntimeFromConfig(cfg config.Config) (recommendationapp.GorseClient, string, recommendationapp.GorseOptions) {
	engine := config.RecommendationEngine(cfg)
	options := recommendationapp.GorseOptions{
		CandidateLimit:    config.GorseCandidateLimit(cfg),
		MinRecommendItems: cfg.Gorse.MinRecommendItems,
		WriteBackEnabled:  cfg.Gorse.WriteBackEnabled,
		ShadowMode:        cfg.Gorse.ShadowMode,
	}
	if engine != recommendationapp.EngineGorse {
		return nil, engine, options
	}
	return recommendationapp.NewGorseHTTPClient(recommendationapp.GorseClientConfig{
		Endpoint: config.GorseEndpoint(cfg),
		APIKey:   cfg.Gorse.APIKey,
		Timeout:  config.GorseTimeout(cfg),
	}), engine, options
}

func newRecommendationEmbedder(ctx context.Context, cfg config.Config) aiinfra.Embedder {
	if aiinfra.NormalizeProvider(config.AIProvider(cfg)) == aiinfra.ProviderEino {
		client, err := einoai.NewEmbeddingClient(ctx, einoai.EmbeddingConfig{
			BaseURL: cfg.Embedding.BaseURL,
			APIKey: firstNonEmpty(
				os.Getenv("DASHSCOPE_API_KEY"),
				os.Getenv("OPENAI_API_KEY"),
				os.Getenv("EMBEDDING_API_KEY"),
				cfg.Embedding.APIKey,
			),
			Model: cfg.Embedding.Options.Model,
		})
		if err == nil {
			return textEmbedderAdapter{client: client}
		}
		zap.L().Warn("eino_embedding_init_failed_fallback_legacy", zap.Error(err))
	}
	return embedding.NewClient(cfg.Embedding)
}

type textEmbedderAdapter struct {
	client interface {
		EmbedText(context.Context, string) ([]float32, error)
	}
}

func (a textEmbedderAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	return a.client.EmbedText(ctx, text)
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
	return persistence.EnsureSchema(db)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}

func newObjectStore(cfg config.Config) (*objectstorage.RustFS, error) {
	return objectstorage.NewRustFS(config.ObjectStorageConfig(cfg))
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
