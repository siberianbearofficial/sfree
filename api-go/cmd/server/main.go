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

	"time"

	"github.com/example/sfree/api-go/internal/app"
	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	_ "github.com/example/sfree/api-go/internal/docs"
	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/observability"
	"github.com/example/sfree/api-go/internal/resilience"
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

	// Apply source client resilience settings from config.
	rcfg := resilience.DefaultWrapperConfig()
	if cfg.SourceClient.TimeoutSeconds > 0 {
		rcfg.Timeout = time.Duration(cfg.SourceClient.TimeoutSeconds) * time.Second
	}
	if cfg.SourceClient.FailureThreshold > 0 {
		rcfg.FailureThreshold = cfg.SourceClient.FailureThreshold
	}
	if cfg.SourceClient.RecoverySeconds > 0 {
		rcfg.RecoveryTimeout = time.Duration(cfg.SourceClient.RecoverySeconds) * time.Second
	}
	if cfg.SourceClient.MaxRetries > 0 {
		rcfg.MaxRetries = cfg.SourceClient.MaxRetries
	}
	if cfg.SourceClient.RetryBaseDelayMs > 0 {
		rcfg.RetryBaseDelay = time.Duration(cfg.SourceClient.RetryBaseDelayMs) * time.Millisecond
	}
	if cfg.SourceClient.RetryMaxDelayMs > 0 {
		rcfg.RetryMaxDelay = time.Duration(cfg.SourceClient.RetryMaxDelayMs) * time.Millisecond
	}
	manager.ResilienceConfig = rcfg

	mongoConn, err := db.Connect(ctx, cfg.Mongo)
	if err != nil {
		slog.Error("failed to connect mongo", slog.String("error", err.Error()))
		os.Exit(1)
	}
	router := app.SetupRouter(mongoConn, cfg)
	slog.Info("starting server")
	if err := router.Run(); err != nil {
		slog.Error("failed to run server", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
