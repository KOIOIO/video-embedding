package main

import (
	"context"
	"net/http"
	"testing"

	"nlp-video-analysis/internal/config"
	appbuilder "nlp-video-analysis/internal/http/app"
)

func TestPrepareServerSucceedsWithoutRPCAddr(t *testing.T) {
	t.Setenv("RPC_ADDR", "127.0.0.1:9999")
	t.Setenv("HTTP_ADDR", "")

	builderCalled := false
	routerCalled := false

	httpApp, srv, err := prepareServer(context.Background(), config.Config{}, func(ctx context.Context, cfg config.Config) (*appbuilder.App, error) {
		builderCalled = true
		if got := cfg; got.Name != "" || got.Host != "" || got.Port != 0 || got.WorkerPools != nil {
			t.Fatalf("unexpected config passed to builder: %#v", got)
		}
		return &appbuilder.App{}, nil
	}, func(app *appbuilder.App) http.Handler {
		routerCalled = true
		if app == nil {
			t.Fatal("router received nil app")
		}
		return http.NewServeMux()
	})
	if err != nil {
		t.Fatalf("prepareServer() error = %v", err)
	}
	if !builderCalled {
		t.Fatal("prepareServer() did not call app builder")
	}
	if !routerCalled {
		t.Fatal("prepareServer() did not call router factory")
	}
	if httpApp == nil {
		t.Fatal("prepareServer() returned nil app")
	}
	if srv == nil {
		t.Fatal("prepareServer() returned nil server")
	}
	if srv.Addr != ":8081" {
		t.Fatalf("server addr = %q, want %q", srv.Addr, ":8081")
	}
	if srv.Handler == nil {
		t.Fatal("prepareServer() returned nil handler")
	}
	if srv.Addr == "127.0.0.1:9999" {
		t.Fatalf("prepareServer() unexpectedly used RPC_ADDR: %q", srv.Addr)
	}
}

func TestPrepareServerPrefersHTTPAddr(t *testing.T) {
	t.Setenv("RPC_ADDR", "127.0.0.1:9999")
	t.Setenv("HTTP_ADDR", " :9091 ")

	_, srv, err := prepareServer(context.Background(), config.Config{}, func(ctx context.Context, cfg config.Config) (*appbuilder.App, error) {
		return &appbuilder.App{}, nil
	}, func(app *appbuilder.App) http.Handler {
		return http.NewServeMux()
	})
	if err != nil {
		t.Fatalf("prepareServer() error = %v", err)
	}
	if srv == nil {
		t.Fatal("prepareServer() returned nil server")
	}
	if srv.Addr != ":9091" {
		t.Fatalf("server addr = %q, want %q", srv.Addr, ":9091")
	}
}

func TestPrepareServerUsesConfiguredHTTPAddr(t *testing.T) {
	t.Setenv("RPC_ADDR", "127.0.0.1:9999")
	t.Setenv("HTTP_ADDR", "")

	_, srv, err := prepareServer(context.Background(), config.Config{
		HTTP: config.HTTPConfig{Addr: ":9092"},
	}, func(ctx context.Context, cfg config.Config) (*appbuilder.App, error) {
		return &appbuilder.App{}, nil
	}, func(app *appbuilder.App) http.Handler {
		return http.NewServeMux()
	})
	if err != nil {
		t.Fatalf("prepareServer() error = %v", err)
	}
	if srv == nil {
		t.Fatal("prepareServer() returned nil server")
	}
	if srv.Addr != ":9092" {
		t.Fatalf("server addr = %q, want %q", srv.Addr, ":9092")
	}
}
