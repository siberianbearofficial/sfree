// Package main implements the HTTP server.
//
// @title SFree API
// @version 1.0
// @BasePath /
// @securityDefinitions.basic BasicAuth
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/example/sfree/api-go/internal/app"
	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	_ "github.com/example/sfree/api-go/internal/docs"
	"github.com/example/sfree/api-go/internal/observability"
	"github.com/example/sfree/api-go/internal/telemetry"
)

func main() {
	observability.SetupLogger()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}
	ctx := context.Background()

	shutdownTracer, err := telemetry.Init(ctx, "sfree-api")
	if err != nil {
		slog.Error("failed to init telemetry", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracer(ctx); err != nil {
			slog.Error("failed to shutdown tracer", slog.String("error", err.Error()))
		}
	}()

	mongoConn, err := db.Connect(ctx, cfg.Mongo)
	if err != nil {
		slog.Error("failed to connect mongo", slog.String("error", err.Error()))
		os.Exit(1)
	}
	router, err := app.SetupRouter(mongoConn, cfg)
	if err != nil {
		slog.Error("failed to initialize router", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("starting server")
	if err := router.Run(); err != nil {
		slog.Error("failed to run server", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
