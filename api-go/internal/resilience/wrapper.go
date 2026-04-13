package resilience

import (
	"context"
	"io"
	"time"
)

// Client mirrors the source client interface (Upload, Download, Delete).
type Client interface {
	Upload(ctx context.Context, name string, r io.Reader) (string, error)
	Download(ctx context.Context, name string) (io.ReadCloser, error)
	Delete(ctx context.Context, name string) error
}

// WrapperConfig holds timeout and circuit breaker settings.
type WrapperConfig struct {
	Timeout          time.Duration // per-request timeout (default 30s)
	FailureThreshold int           // consecutive failures before circuit opens (default 5)
	RecoveryTimeout  time.Duration // time before half-open probe (default 30s)
}

// DefaultWrapperConfig returns sensible defaults.
func DefaultWrapperConfig() WrapperConfig {
	return WrapperConfig{
		Timeout:          30 * time.Second,
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
	}
}

// Wrap wraps a source client with timeout and circuit breaker behavior.
func Wrap(c Client, cfg WrapperConfig) Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &wrapper{
		inner: c,
		cb:    NewCircuitBreaker(cfg.FailureThreshold, cfg.RecoveryTimeout),
		cfg:   cfg,
	}
}

type wrapper struct {
	inner Client
	cb    *CircuitBreaker
	cfg   WrapperConfig
}

func (w *wrapper) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	if err := w.cb.Allow(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
	defer cancel()
	result, err := w.inner.Upload(ctx, name, r)
	if err != nil {
		w.cb.RecordFailure()
		return "", err
	}
	w.cb.RecordSuccess()
	return result, nil
}

func (w *wrapper) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := w.cb.Allow(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
	defer cancel()
	rc, err := w.inner.Download(ctx, name)
	if err != nil {
		w.cb.RecordFailure()
		return nil, err
	}
	w.cb.RecordSuccess()
	return rc, nil
}

func (w *wrapper) Delete(ctx context.Context, name string) error {
	if err := w.cb.Allow(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
	defer cancel()
	if err := w.inner.Delete(ctx, name); err != nil {
		w.cb.RecordFailure()
		return err
	}
	w.cb.RecordSuccess()
	return nil
}
