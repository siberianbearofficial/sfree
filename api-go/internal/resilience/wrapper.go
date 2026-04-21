package resilience

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/example/sfree/api-go/internal/gdrive"
	"github.com/example/sfree/api-go/internal/s3compat"
)

var ErrUnsupportedOperation = errors.New("unsupported source client operation")

// Client mirrors the source client interface (Upload, Download, Delete).
type Client interface {
	Upload(ctx context.Context, name string, r io.Reader) (string, error)
	Download(ctx context.Context, name string) (io.ReadCloser, error)
	Delete(ctx context.Context, name string) error
}

// WrapperConfig holds timeout, circuit breaker, and retry settings.
type WrapperConfig struct {
	Timeout          time.Duration // per-request timeout (default 30s)
	FailureThreshold int           // consecutive failures before circuit opens (default 5)
	RecoveryTimeout  time.Duration // time before half-open probe (default 30s)
	MaxRetries       int           // retry attempts after first call (default 3)
	RetryBaseDelay   time.Duration // initial backoff delay (default 100ms)
	RetryMaxDelay    time.Duration // max backoff delay cap (default 5s)
}

// DefaultWrapperConfig returns sensible defaults.
func DefaultWrapperConfig() WrapperConfig {
	rc := DefaultRetryConfig()
	return WrapperConfig{
		Timeout:          30 * time.Second,
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		MaxRetries:       rc.MaxRetries,
		RetryBaseDelay:   rc.BaseDelay,
		RetryMaxDelay:    rc.MaxDelay,
	}
}

// Wrap wraps a source client with timeout, circuit breaker, and retry behavior.
func Wrap(c Client, cfg WrapperConfig) Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	rc := RetryConfig{
		MaxRetries: cfg.MaxRetries,
		BaseDelay:  cfg.RetryBaseDelay,
		MaxDelay:   cfg.RetryMaxDelay,
	}
	if rc.MaxRetries < 0 {
		rc.MaxRetries = 0
	}
	return &wrapper{
		inner:    c,
		cb:       NewCircuitBreaker(cfg.FailureThreshold, cfg.RecoveryTimeout),
		cfg:      cfg,
		retryCfg: rc,
	}
}

type wrapper struct {
	inner    Client
	cb       *CircuitBreaker
	cfg      WrapperConfig
	retryCfg RetryConfig
}

func (w *wrapper) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	if err := w.cb.Allow(); err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 0; attempt <= w.retryCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, w.retryCfg)
			slog.WarnContext(ctx, "retrying upload",
				slog.String("name", name),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", delay),
				slog.String("last_error", lastErr.Error()),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				w.cb.RecordFailure()
				return "", ctx.Err()
			}
			// Re-check circuit before retry.
			if err := w.cb.Allow(); err != nil {
				return "", err
			}
		}

		reqCtx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
		result, err := w.inner.Upload(reqCtx, name, r)
		cancel()

		if err == nil {
			w.cb.RecordSuccess()
			return result, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	w.cb.RecordFailure()
	return "", lastErr
}

func (w *wrapper) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := w.cb.Allow(); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= w.retryCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, w.retryCfg)
			slog.WarnContext(ctx, "retrying download",
				slog.String("name", name),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", delay),
				slog.String("last_error", lastErr.Error()),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				w.cb.RecordFailure()
				return nil, ctx.Err()
			}
			if err := w.cb.Allow(); err != nil {
				return nil, err
			}
		}

		reqCtx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
		rc, err := w.inner.Download(reqCtx, name)
		cancel()

		if err == nil {
			w.cb.RecordSuccess()
			return rc, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	w.cb.RecordFailure()
	return nil, lastErr
}

func (w *wrapper) DownloadStream(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := w.cb.Allow(); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= w.retryCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, w.retryCfg)
			slog.WarnContext(ctx, "retrying streamed download",
				slog.String("name", name),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", delay),
				slog.String("last_error", lastErr.Error()),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				w.cb.RecordFailure()
				return nil, ctx.Err()
			}
			if err := w.cb.Allow(); err != nil {
				return nil, err
			}
		}

		reqCtx, cancel := context.WithCancel(ctx)
		rc, err := w.inner.Download(reqCtx, name)
		if err == nil {
			w.cb.RecordSuccess()
			return &cancelOnCloseReadCloser{ReadCloser: rc, cancel: cancel}, nil
		}
		cancel()
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	w.cb.RecordFailure()
	return nil, lastErr
}

func (w *wrapper) Delete(ctx context.Context, name string) error {
	if err := w.cb.Allow(); err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt <= w.retryCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, w.retryCfg)
			slog.WarnContext(ctx, "retrying delete",
				slog.String("name", name),
				slog.Int("attempt", attempt),
				slog.Duration("backoff", delay),
				slog.String("last_error", lastErr.Error()),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				w.cb.RecordFailure()
				return ctx.Err()
			}
			if err := w.cb.Allow(); err != nil {
				return err
			}
		}

		reqCtx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
		err := w.inner.Delete(reqCtx, name)
		cancel()

		if err == nil {
			w.cb.RecordSuccess()
			return nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	w.cb.RecordFailure()
	return lastErr
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelOnCloseReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}

func (w *wrapper) ListFiles(ctx context.Context) ([]gdrive.File, error) {
	inner, ok := w.inner.(interface {
		ListFiles(context.Context) ([]gdrive.File, error)
	})
	if !ok {
		return nil, ErrUnsupportedOperation
	}
	if err := w.cb.Allow(); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= w.retryCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, w.retryCfg)
			slog.WarnContext(ctx, "retrying list files",
				slog.Int("attempt", attempt),
				slog.Duration("backoff", delay),
				slog.String("last_error", lastErr.Error()),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				w.cb.RecordFailure()
				return nil, ctx.Err()
			}
			if err := w.cb.Allow(); err != nil {
				return nil, err
			}
		}

		reqCtx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
		files, err := inner.ListFiles(reqCtx)
		cancel()
		if err == nil {
			w.cb.RecordSuccess()
			return files, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	w.cb.RecordFailure()
	return nil, lastErr
}

func (w *wrapper) StorageInfo(ctx context.Context) (int64, int64, int64, error) {
	inner, ok := w.inner.(interface {
		StorageInfo(context.Context) (int64, int64, int64, error)
	})
	if !ok {
		return 0, 0, 0, ErrUnsupportedOperation
	}
	if err := w.cb.Allow(); err != nil {
		return 0, 0, 0, err
	}

	var lastErr error
	for attempt := 0; attempt <= w.retryCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, w.retryCfg)
			slog.WarnContext(ctx, "retrying storage info",
				slog.Int("attempt", attempt),
				slog.Duration("backoff", delay),
				slog.String("last_error", lastErr.Error()),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				w.cb.RecordFailure()
				return 0, 0, 0, ctx.Err()
			}
			if err := w.cb.Allow(); err != nil {
				return 0, 0, 0, err
			}
		}

		reqCtx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
		total, used, free, err := inner.StorageInfo(reqCtx)
		cancel()
		if err == nil {
			w.cb.RecordSuccess()
			return total, used, free, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	w.cb.RecordFailure()
	return 0, 0, 0, lastErr
}

func (w *wrapper) ListObjects(ctx context.Context) ([]s3compat.ObjectInfo, int64, error) {
	inner, ok := w.inner.(interface {
		ListObjects(context.Context) ([]s3compat.ObjectInfo, int64, error)
	})
	if !ok {
		return nil, 0, ErrUnsupportedOperation
	}
	if err := w.cb.Allow(); err != nil {
		return nil, 0, err
	}

	var lastErr error
	for attempt := 0; attempt <= w.retryCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, w.retryCfg)
			slog.WarnContext(ctx, "retrying list objects",
				slog.Int("attempt", attempt),
				slog.Duration("backoff", delay),
				slog.String("last_error", lastErr.Error()),
			)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				w.cb.RecordFailure()
				return nil, 0, ctx.Err()
			}
			if err := w.cb.Allow(); err != nil {
				return nil, 0, err
			}
		}

		reqCtx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
		objects, used, err := inner.ListObjects(reqCtx)
		cancel()
		if err == nil {
			w.cb.RecordSuccess()
			return objects, used, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	w.cb.RecordFailure()
	return nil, 0, lastErr
}
