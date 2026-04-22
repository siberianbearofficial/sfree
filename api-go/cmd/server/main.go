// Package main implements the HTTP server.
//
// @title SFree API
// @version 1.0
// @BasePath /
// @securityDefinitions.basic BasicAuth
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

const (
	serverAddr      = ":8080"
	shutdownTimeout = 10 * time.Second
)

type gracefulServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func main() {
	observability.SetupLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("server exited with error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	shutdownTracer, err := telemetry.Init(ctx, "sfree-api")
	if err != nil {
		return fmt.Errorf("failed to init telemetry: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := shutdownTracer(shutdownCtx); err != nil {
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
		return fmt.Errorf("failed to connect mongo: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := mongoConn.Close(shutdownCtx); err != nil {
			slog.Error("failed to close mongo", slog.String("error", err.Error()))
		}
	}()

	router, err := app.SetupRouter(mongoConn, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize router: %w", err)
	}

	server := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	slog.Info("starting server", slog.String("addr", server.Addr))
	if err := runServer(ctx, server, shutdownTimeout); err != nil {
		return fmt.Errorf("failed to run server: %w", err)
	}
	slog.Info("server stopped")
	return nil
}

func runServer(ctx context.Context, server gracefulServer, timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	err := <-errCh
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
