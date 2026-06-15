package main

import (
	"context"
	"legacy-video/internal/application/videoapp"
	"legacy-video/internal/config"
	"legacy-video/internal/infrastructure/embedding"
	"legacy-video/internal/infrastructure/fs"
	"legacy-video/internal/infrastructure/objectstorage"
	"legacy-video/internal/infrastructure/persistence"
	infraredis "legacy-video/internal/infrastructure/redis"
	"legacy-video/internal/lifecycle"
	"legacy-video/internal/model"
	"legacy-video/internal/rpc/service"
	"legacy-video/middleware"
	"legacy-video/video"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// main 启动 gRPC 服务，并装配仓储、对象存储、队列、Embedding 客户端与拦截器。
func main() {
	config.EnsureProjectRoot()
	f, err := middleware.InitFileLogger("rpc")
	if err != nil {
		log.Fatalf("init rpc logger failed: %v", err)
	}
	defer f.Close()

	cfg := config.MustLoadDefault()
	lc := lifecycle.New("rpc", 30*time.Second)

	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		zap.L().Fatal("server_start_fail", zap.String("err", err.Error()))
	}
	lc.AddCloser(func(ctx context.Context) error { return lis.Close() })

	rawDir := filepath.Join(os.TempDir(), "legacy-video", "tmp", "raw")
	hlsDir := filepath.Join(os.TempDir(), "legacy-video", "tmp", "hls")

	if cfg.Postgres.DSN == "" {
		zap.L().Fatal("postgres_dsn_empty", zap.String("err", "Postgres DSN 不能为空，请配置 configs/video.yml 的 Postgres.DSN"))
	}
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
	if err != nil {
		zap.L().Fatal("Fail to Connect postgreSql on")
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
		lc.AddCloser(func(ctx context.Context) error { return sqlDB.Close() })
	}
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS vector;").Error; err != nil {
		log.Fatalf("启用 pgvector 扩展失败: %v\n", err)
		zap.L().Fatal("Fail to open postgreSql", zap.String("err", err.Error()))
	}
	if err := db.AutoMigrate(&model.EduVideoResource{}, &model.EduVideoSegment{}, &model.EduUserVideoRecommend{}); err != nil {
		zap.L().Fatal("db_migrate_failed", zap.String("err", err.Error()))
	}
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_video ON edu_video_segment(video_id);`).Error
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_video_segment_embedding ON edu_video_segment USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);`).Error
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_video_recommend_user ON edu_user_video_recommend(user_id);`).Error
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_video_recommend_video ON edu_user_video_recommend(video_id);`).Error
	if err := persistence.EnsureIntegrity(db); err != nil {
		zap.L().Fatal("db_integrity_failed", zap.String("err", err.Error()))
	}

	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	lc.AddCloser(func(ctx context.Context) error { return rdb.Close() })
	if err := rdb.Ping(lc.Context()).Err(); err != nil {
		zap.L().Fatal("Fail to ping Redis", zap.String("err", err.Error()))
	}

	repo := persistence.NewGormVideoRepository(db)
	fileStorage := fs.NewLocalFileStorage()
	queue := infraredis.NewTranscodeQueue(rdb, "video:transcode:queue")
	vectorQueue := infraredis.NewVectorizeQueue(rdb, "video:vectorize:queue")
	statusStore := infraredis.NewTranscodeStatusStore(rdb, "video:transcode:status:")
	embedder := embedding.NewClient(cfg.Embedding)

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
		zap.L().Fatal("rustfs_init_failed", zap.String("err", err.Error()))
	}
	if err := store.EnsureBucket(lc.Context()); err != nil {
		zap.L().Fatal("rustfs_bucket_failed", zap.String("err", err.Error()))
	}

	appSvc := videoapp.NewService(repo, queue, vectorQueue, statusStore, store, fileStorage, embedder, videoapp.Paths{
		RawDir:       rawDir,
		HLSDir:       hlsDir,
		RawURLPrefix: "/videos/raw",
		HLSURLPrefix: "/videos/hls",
	})

	grpcOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			middleware.UnaryIdempotencyInterceptor(rdb),
			middleware.UnaryAccessLogInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			middleware.StreamIdempotencyInterceptor(rdb),
			middleware.StreamAccessLogInterceptor(),
		),
	}
	if cfg.GRPC.MaxMsgSize > 0 {
		grpcOpts = append(grpcOpts, grpc.MaxRecvMsgSize(cfg.GRPC.MaxMsgSize))
		grpcOpts = append(grpcOpts, grpc.MaxSendMsgSize(cfg.GRPC.MaxMsgSize))
	}
	if cfg.GRPC.KeepaliveTime > 0 && cfg.GRPC.KeepaliveTimeout > 0 {
		grpcOpts = append(grpcOpts, grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    time.Duration(cfg.GRPC.KeepaliveTime) * time.Second,
			Timeout: time.Duration(cfg.GRPC.KeepaliveTimeout) * time.Second,
		}))
	}
	if cfg.GRPC.MaxConnectionAge > 0 {
		grpcOpts = append(grpcOpts, grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge:      time.Duration(cfg.GRPC.MaxConnectionAge) * time.Second,
			MaxConnectionAgeGrace: time.Duration(cfg.GRPC.MaxConnectionAgeGrace) * time.Second,
		}))
	}

	s := grpc.NewServer(grpcOpts...)
	lc.AddCloser(func(ctx context.Context) error {
		done := make(chan struct{})
		go func() {
			s.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			s.Stop()
		}
		return nil
	})
	video.RegisterVideoServiceServer(s, service.NewVideoService(appSvc, db, &cfg))
	zap.L().Info("rpc running in ", zap.String("addr", lis.Addr().String()))
	err = lc.Run(func(ctx context.Context) error { return s.Serve(lis) })
	if err != nil {
		zap.L().Error("RPC 退出", zap.Error(err))
	}
}
