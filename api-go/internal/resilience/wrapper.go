package resilience

import (
	"bytes"
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

		reqCtx, cancel, timeoutErr := downloadAttemptContext(ctx, w.cfg.Timeout)
		rc, err := w.inner.Download(reqCtx, name)

		if err == nil {
			if err := timeoutErr(); err != nil {
				cancel()
				_ = rc.Close()
				lastErr = err
				break
			}
			w.cb.RecordSuccess()
			return &cancelOnCloseReadCloser{ReadCloser: rc, cancel: cancel}, nil
		}
		cancel()
		if timeoutErr() != nil && errors.Is(err, context.Canceled) {
			err = context.DeadlineExceeded
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

func downloadAttemptContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc, func() error) {
	ctx, cancel := context.WithCancel(parent)
	timer := time.AfterFunc(timeout, func() {
		cancel()
	})
	timeoutErr := func() error {
		if timer.Stop() {
			return nil
		}
		cancel()
		return context.DeadlineExceeded
	}
	return ctx, cancel, timeoutErr
}

func (w *wrapper) ListFiles(ctx context.Context) ([]gdrive.File, error) {
	inner, ok := w.inner.(interface {
		ListFiles(context.Context) ([]gdrive.File, error)
	})
	if !ok {
		return nil, ErrUnsupportedOperation
	}

	var files []gdrive.File
	err := w.withRetry(ctx, "retrying list files", nil, timeoutRetryContext(w.cfg.Timeout), true, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		var err error
		files, err = inner.ListFiles(reqCtx)
		return err
	})
	return files, err
}

func (w *wrapper) StorageInfo(ctx context.Context) (int64, int64, int64, error) {
	inner, ok := w.inner.(interface {
		StorageInfo(context.Context) (int64, int64, int64, error)
	})
	if !ok {
		return 0, 0, 0, ErrUnsupportedOperation
	}

	var total, used, free int64
	err := w.withRetry(ctx, "retrying storage info", nil, timeoutRetryContext(w.cfg.Timeout), true, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		var err error
		total, used, free, err = inner.StorageInfo(reqCtx)
		return err
	})
	return total, used, free, err
}

func (w *wrapper) ListObjects(ctx context.Context) ([]s3compat.ObjectInfo, int64, error) {
	inner, ok := w.inner.(interface {
		ListObjects(context.Context) ([]s3compat.ObjectInfo, int64, error)
	})
	if !ok {
		return nil, 0, ErrUnsupportedOperation
	}

	var objects []s3compat.ObjectInfo
	var used int64
	err := w.withRetry(ctx, "retrying list objects", nil, timeoutRetryContext(w.cfg.Timeout), true, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		var err error
		objects, used, err = inner.ListObjects(reqCtx)
		return err
	})
	return objects, used, err
}

func (w *wrapper) HeadBucket(ctx context.Context) error {
	inner, ok := w.inner.(interface {
		HeadBucket(context.Context) error
	})
	if !ok {
		return ErrUnsupportedOperation
	}

	return w.withRetry(ctx, "retrying head bucket", nil, timeoutRetryContext(w.cfg.Timeout), true, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		return inner.HeadBucket(reqCtx)
	})
}

func (w *wrapper) CheckChat(ctx context.Context) error {
	inner, ok := w.inner.(interface {
		CheckChat(context.Context) error
	})
	if !ok {
		return ErrUnsupportedOperation
	}

	return w.withRetry(ctx, "retrying telegram chat check", nil, timeoutRetryContext(w.cfg.Timeout), true, nil, func(reqCtx context.Context, cancel context.CancelFunc) error {
		return inner.CheckChat(reqCtx)
	})
}
