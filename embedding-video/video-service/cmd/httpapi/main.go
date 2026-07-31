package main

import (
	"context"
	"log"
	"net/http"

	"go.uber.org/zap"

	_ "video-service/docs/swagger"
	"video-service/internal/config"
	appbuilder "video-service/internal/http/app"
	"video-service/internal/http/router"
	"video-service/internal/lifecycle"
	"video-service/middleware"
)

// @title 视频平板视频服务接口
// @version 1.0
// @description 提供视频上传、播放、推荐、题目查询等 HTTP 接口。
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @tag.name 管理员认证
// @tag.description 管理员登录与当前会话
// @tag.name 系统与健康
// @tag.description 服务健康状态与运行指标
// @tag.name 视频资源
// @tag.description 视频资源查询、维护与状态管理
// @tag.name 视频上传
// @tag.description 单文件、压缩包及分片上传
// @tag.name 视频播放与转码
// @tag.description 视频播放地址与转码任务状态
// @tag.name 视频互动
// @tag.description 视频互动及观看进度
// @tag.name 视频片段
// @tag.description 视频片段播放与互动
// @tag.name 题目
// @tag.description 题目查询
// @tag.name 推荐
// @tag.description 面向业务调用的推荐接口
// @tag.name 推荐管理
// @tag.description 推荐系统诊断、追踪、预览与运行状态
// @tag.name 知识点视频
// @tag.description 知识点视频导入、进度、列表与播放
// @tag.name 内部接口
// @tag.description 服务间调用接口，不建议客户端直接依赖
// @tag.name 媒体访问
// @tag.description HLS 清单、视频分片及对象媒体代理

type appFactory func(context.Context, config.Config) (*appbuilder.App, error)

type routerFactory func(*appbuilder.App) http.Handler

func prepareServer(ctx context.Context, cfg config.Config, buildApp appFactory, buildRouter routerFactory) (*appbuilder.App, *http.Server, error) {
	httpApp, err := buildApp(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return httpApp, &http.Server{
		Addr:    appbuilder.ResolveHTTPAddr(cfg),
		Handler: buildRouter(httpApp),
	}, nil
}

func main() {
	config.EnsureProjectRoot()
	cfg := config.MustLoadDefault()
	f, err := middleware.InitFileLoggerWithOptions("httpapi", middleware.FileLoggerOptions{LogDir: config.HTTPLogDir(cfg)})
	if err != nil {
		log.Fatalf("init http logger failed: %v", err)
	}
	defer f.Close()

	lc := lifecycle.New("httpapi", config.HTTPShutdownTimeout(cfg))

	httpApp, srv, err := prepareServer(lc.Context(), cfg, appbuilder.New, func(httpApp *appbuilder.App) http.Handler {
		return router.New(httpApp)
	})
	if err != nil {
		zap.L().Fatal("http_app_init_failed", zap.Error(err))
	}
	lc.AddCloser(httpApp.Close)
	lc.AddCloser(func(ctx context.Context) error { return srv.Shutdown(ctx) })
	zap.L().Info("http_server_start", zap.String("addr", srv.Addr))
	if err := lc.Run(func(ctx context.Context) error {
		_ = ctx
		err := srv.ListenAndServe()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}); err != nil {
		zap.L().Error("http_server_exit", zap.Error(err))
	}
}
