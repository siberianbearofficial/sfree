package resilience

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/example/sfree/api-go/internal/sourcecap"
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

type retryContextFactory func(context.Context) (context.Context, context.CancelFunc)

type abortRetryError struct {
	err error
}

func (e abortRetryError) Error() string {
	return e.err.Error()
}

func (e abortRetryError) Unwrap() error {
	return e.err
}

func timeoutRetryContext(timeout time.Duration) retryContextFactory {
	return func(ctx context.Context) (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, timeout)
	}
}

func cancelRetryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(ctx)
}

func (w *wrapper) withRetry(ctx context.Context, logMessage string, logAttrs []any, newContext retryContextFactory, cancelOnSuccess bool, prepare func() error, fn func(context.Context, context.CancelFunc) error) error {
	if err := w.cb.Allow(); err != nil {
		return err
	}
	if prepare != nil {
		if err := prepare(); err != nil {
			w.cb.RecordFailure()
			return err
		}
	}

	var lastErr error
	for attempt := 0; attempt <= w.retryCfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := Backoff(attempt-1, w.retryCfg)
			attrs := append([]any{}, logAttrs...)
			attrs = append(attrs,
				slog.Int("attempt", attempt),
				slog.Duration("backoff", delay),
				slog.String("last_error", lastErr.Error()),
			)
			slog.WarnContext(ctx, logMessage, attrs...)
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

		reqCtx, cancel := newContext(ctx)
		err := fn(reqCtx, cancel)
		if err == nil {
			if cancelOnSuccess {
				cancel()
			}
			w.cb.RecordSuccess()
			return nil
		}
		cancel()
		var abortErr abortRetryError
		if errors.As(err, &abortErr) {
			lastErr = abortErr.err
			break
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
	}

	w.cb.RecordFailure()
	return lastErr
}

type uploadBodyReplay interface {
	Reader() (io.Reader, error)
}

type seekableUploadBody struct {
	r     io.ReadSeeker
	start int64
}

func (b seekableUploadBody) Reader() (io.Reader, error) {
	if _, err := b.r.Seek(b.start, io.SeekStart); err != nil {
		return nil, err
	}
	return b.r, nil
}

type bufferedUploadBody []byte

func (b bufferedUploadBody) Reader() (io.Reader, error) {
	return bytes.NewReader(b), nil
}

func newUploadBodyReplay(r io.Reader) (uploadBodyReplay, error) {
	if rs, ok := r.(io.ReadSeeker); ok {
		start, err := rs.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		return seekableUploadBody{r: rs, start: start}, nil
	}

	payload, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return bufferedUploadBody(payload), nil
}

func (w *wrapper) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	var replay uploadBodyReplay
	prepare := func() error {
		if w.retryCfg.MaxRetries > 0 {
			var err error
			replay, err = newUploadBodyReplay(r)
			if err != nil {
				return err
			}
		}
		return nil
	}

	var result string
	err := w.withRetry(ctx, "retrying upload", []any{slog.String("name", name)}, timeoutRetryContext(w.cfg.Timeout), true, prepare, func(reqCtx context.Context, cancel context.CancelFunc) error {
		body := r
		if replay != nil {
			var err error
			body, err = replay.Reader()
			if err != nil {
				return abortRetryError{err: err}
			}
		}
		var err error
		result, err = w.inner.Upload(reqCtx, name, body)
		return err
	})
	return result, err
}

func (w *wrapper) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	var rc io.ReadCloser
	err := w.withRetry(ctx, "retrying download", []any{slog.String("name", name)}, timeoutRetryContext(w.cfg.Timeout), true, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		var err error
		rc, err = w.inner.Download(reqCtx, name)
		return err
	})
	return rc, err
}

func (w *wrapper) DownloadStream(ctx context.Context, name string) (io.ReadCloser, error) {
	var rc io.ReadCloser
	err := w.withRetry(ctx, "retrying streamed download", []any{slog.String("name", name)}, cancelRetryContext, false, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		stream, err := w.inner.Download(reqCtx, name)
		if err == nil {
			rc = &cancelOnCloseReadCloser{ReadCloser: stream, cancel: cancel}
		}
		return err
	})
	return rc, err
}

func (w *wrapper) Delete(ctx context.Context, name string) error {
	return w.withRetry(ctx, "retrying delete", []any{slog.String("name", name)}, timeoutRetryContext(w.cfg.Timeout), true, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		return w.inner.Delete(reqCtx, name)
	})
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

func (w *wrapper) SourceInfo(ctx context.Context) (sourcecap.Info, error) {
	inner, ok := w.inner.(sourcecap.InfoProvider)
	if !ok {
		return sourcecap.Info{}, sourcecap.ErrUnsupportedCapability
	}

	var info sourcecap.Info
	err := w.withRetry(ctx, "retrying source info", nil, timeoutRetryContext(w.cfg.Timeout), true, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		var err error
		info, err = inner.SourceInfo(reqCtx)
		return err
	})
	return info, err
}

func (w *wrapper) ProbeSourceHealth(ctx context.Context) (sourcecap.Health, error) {
	inner, ok := w.inner.(sourcecap.HealthProber)
	if !ok {
		return sourcecap.Health{}, sourcecap.ErrUnsupportedCapability
	}

	var health sourcecap.Health
	err := w.withRetry(ctx, "retrying source health probe", nil, timeoutRetryContext(w.cfg.Timeout), true, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		var err error
		health, err = inner.ProbeSourceHealth(reqCtx)
		return err
	})
	return health, err
}
