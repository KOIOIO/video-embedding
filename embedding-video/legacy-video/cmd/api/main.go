package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"legacy-video/internal/api/client"
	"legacy-video/internal/api/handler"
	"legacy-video/internal/api/handler/impl"
	"legacy-video/internal/api/router"
	"legacy-video/internal/config"
	"legacy-video/internal/infrastructure/objectstorage"
	"legacy-video/internal/lifecycle"
	"legacy-video/middleware"

	redis "github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// main 启动 HTTP API 进程，并完成对象存储、题库数据库与 gRPC 客户端初始化。
func main() {
	config.EnsureProjectRoot()
	cfg := config.MustLoadDefault()

	// 尽早初始化结构化日志，后续所有日志均使用 zap.L()
	middleware.InitFileLogger("api")

	lc := lifecycle.New("api", 30*time.Second)

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
		zap.L().Fatal("初始化 RustFS 失败", zap.Error(err))
	}
	if err := store.EnsureBucket(lc.Context()); err != nil {
		zap.L().Fatal("检查 RustFS bucket 失败", zap.Error(err))
	}
	handler.InitRustFS(store)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	lc.AddCloser(func(ctx context.Context) error { return rdb.Close() })

	// 初始化 DB（用于题库查询）
	if cfg.Postgres.DSN != "" {
		db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{})
		if err != nil {
			zap.L().Warn("API 层连接数据库失败，题库功能不可用", zap.Error(err))
		} else {
			impl.QuestionDB = db
			if sqlDB, err := db.DB(); err == nil {
				lc.AddCloser(func(ctx context.Context) error { return sqlDB.Close() })
			}
			zap.L().Info("API 层数据库连接成功")
		}
	}

	// 初始化gRPC客户端
	rpcAddr := os.Getenv("RPC_ADDR")
	if rpcAddr == "" {
		if cfg.Host != "" && cfg.Port != 0 {
			rpcAddr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		} else {
			rpcAddr = "localhost:9090"
		}
	}

	if err := client.InitVideoClient(rpcAddr); err != nil {
		zap.L().Fatal("初始化gRPC客户端失败", zap.Error(err))
	}
	lc.AddCloser(func(ctx context.Context) error { return client.Close() })

	// 测试连接
	if err := client.Ping(); err != nil {
		zap.L().Warn("连接RPC服务失败", zap.Error(err))
	}

	// 设置路由
	r := router.SetupRouter()

	// 启动服务器
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	lc.AddCloser(func(ctx context.Context) error { return srv.Shutdown(ctx) })
	zap.L().Info("API服务器启动", zap.String("addr", addr))
	if err := lc.Run(func(ctx context.Context) error {
		err := srv.ListenAndServe()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}); err != nil {
		zap.L().Info("API 退出", zap.Error(err))
	}
}
