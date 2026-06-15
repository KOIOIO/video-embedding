package main

import (
	"context"
	"log"
	"net/http"

	"go.uber.org/zap"

	_ "nlp-video-analysis/docs/swagger"
	"nlp-video-analysis/internal/config"
	appbuilder "nlp-video-analysis/internal/http/app"
	"nlp-video-analysis/internal/http/router"
	"nlp-video-analysis/internal/lifecycle"
	"nlp-video-analysis/middleware"
)

// @title Intelligent Teaching Video Analysis and Recommendation API
// @version 1.0
// @description Standalone REST API for video upload, playback, recommendation, and question lookup.
// @BasePath /

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
