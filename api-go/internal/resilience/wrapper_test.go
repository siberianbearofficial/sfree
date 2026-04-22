package resilience

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// mockClient is a test double for the source client interface.
type mockClient struct {
	uploadErr   error
	downloadErr error
	deleteErr   error
	delay       time.Duration

	uploadCalls   atomic.Int32
	downloadCalls atomic.Int32
	deleteCalls   atomic.Int32
}

func (m *mockClient) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	m.uploadCalls.Add(1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "uploaded-" + name, m.uploadErr
}

func (m *mockClient) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	m.downloadCalls.Add(1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.downloadErr != nil {
		return nil, m.downloadErr
	}
	return io.NopCloser(strings.NewReader("data")), nil
}

func (m *mockClient) Delete(ctx context.Context, name string) error {
	m.deleteCalls.Add(1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.deleteErr
}

func TestWrapperPassesThrough(t *testing.T) {
	inner := &mockClient{}
	w := Wrap(inner, DefaultWrapperConfig())

	ctx := context.Background()
	name, err := w.Upload(ctx, "file.txt", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if name != "uploaded-file.txt" {
		t.Fatalf("expected uploaded-file.txt, got %s", name)
	}

	rc, err := w.Download(ctx, "file.txt")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	_ = rc.Close()

	if err := w.Delete(ctx, "file.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestWrapperTimeout(t *testing.T) {
	inner := &mockClient{delay: 200 * time.Millisecond}
	cfg := WrapperConfig{
		Timeout:          50 * time.Millisecond,
		FailureThreshold: 100,
		RecoveryTimeout:  time.Second,
		MaxRetries:       0, // no retries to isolate timeout behavior
	}
	w := Wrap(inner, cfg)

	_, err := w.Upload(context.Background(), "slow.txt", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestWrapperCircuitBreakerTrips(t *testing.T) {
	inner := &mockClient{uploadErr: errors.New("backend down")}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 3,
		RecoveryTimeout:  time.Second,
		MaxRetries:       0, // no retries
	}
	w := Wrap(inner, cfg)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := w.Upload(ctx, "fail.txt", strings.NewReader("data"))
		if err == nil {
			t.Fatalf("request %d: expected error", i+1)
		}
	}

	// 4th request should get circuit breaker error, not backend error.
	_, err := w.Upload(ctx, "fail.txt", strings.NewReader("data"))
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestWrapperCircuitBreakerRecovers(t *testing.T) {
	inner := &mockClient{uploadErr: errors.New("backend down")}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 2,
		RecoveryTimeout:  50 * time.Millisecond,
		MaxRetries:       0,
	}
	w := Wrap(inner, cfg)
	clock := newFakeClock()
	w.(*wrapper).cb = newCircuitBreakerWithClock(cfg.FailureThreshold, cfg.RecoveryTimeout, clock.Now)

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		_, err := w.Upload(ctx, "fail.txt", strings.NewReader("data"))
		if err == nil {
			t.Fatalf("request %d: expected error", i+1)
		}
	}

	// Circuit is open.
	_, err := w.Upload(ctx, "fail.txt", strings.NewReader("data"))
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	// Fix the backend.
	inner.uploadErr = nil
	clock.Advance(50 * time.Millisecond)

	// Should succeed (half-open probe).
	name, err := w.Upload(ctx, "ok.txt", strings.NewReader("data"))
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if name != "uploaded-ok.txt" {
		t.Fatalf("expected uploaded-ok.txt, got %s", name)
	}
}

// --- Retry-specific tests ---

// flakeyClient fails a configurable number of times then succeeds.
type flakeyClient struct {
	failCount int
	callCount atomic.Int32
}

func (f *flakeyClient) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	n := int(f.callCount.Add(1))
	if n <= f.failCount {
		return "", errors.New("transient failure")
	}
	return "uploaded-" + name, nil
}

func (f *flakeyClient) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	n := int(f.callCount.Add(1))
	if n <= f.failCount {
		return nil, errors.New("transient failure")
	}
	return io.NopCloser(strings.NewReader("data")), nil
}

func (f *flakeyClient) Delete(ctx context.Context, name string) error {
	n := int(f.callCount.Add(1))
	if n <= f.failCount {
		return errors.New("transient failure")
	}
	return nil
}

func TestWrapperRetryUploadSucceeds(t *testing.T) {
	inner := &flakeyClient{failCount: 2} // fails twice, then succeeds
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 10,
		RecoveryTimeout:  time.Second,
		MaxRetries:       3,
		RetryBaseDelay:   1 * time.Millisecond,
		RetryMaxDelay:    10 * time.Millisecond,
	}
	w := Wrap(inner, cfg)

	name, err := w.Upload(context.Background(), "file.txt", strings.NewReader("data"))
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if name != "uploaded-file.txt" {
		t.Fatalf("unexpected name: %s", name)
	}
	if got := int(inner.callCount.Load()); got != 3 {
		t.Fatalf("expected 3 calls (2 failures + 1 success), got %d", got)
	}
}

func TestWrapperUploadErrorDropsReturnedName(t *testing.T) {
	inner := &mockClient{uploadErr: errors.New("upload failed")}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 10,
		RecoveryTimeout:  time.Second,
		MaxRetries:       0,
	}
	w := Wrap(inner, cfg)

	name, err := w.Upload(context.Background(), "file.txt", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected upload error")
	}
	if name != "" {
		t.Fatalf("expected empty name on upload error, got %q", name)
	}
}

type bodyDrainingUploadClient struct {
	want      string
	callCount atomic.Int32
}

func (c *bodyDrainingUploadClient) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	got, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	if string(got) != c.want {
		return "", errors.New("upload retry body did not match original payload")
	}
	if c.callCount.Add(1) == 1 {
		return "", errors.New("transient failure after reading body")
	}
	return "uploaded-" + name, nil
}

func (c *bodyDrainingUploadClient) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (c *bodyDrainingUploadClient) Delete(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

func TestWrapperRetryUploadReplaysBody(t *testing.T) {
	inner := &bodyDrainingUploadClient{want: "chunk-payload"}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 10,
		RecoveryTimeout:  time.Second,
		MaxRetries:       1,
		RetryBaseDelay:   1 * time.Millisecond,
		RetryMaxDelay:    10 * time.Millisecond,
	}
	w := Wrap(inner, cfg)

	name, err := w.Upload(context.Background(), "file.txt", strings.NewReader("chunk-payload"))
	if err != nil {
		t.Fatalf("expected retry success with replayed body, got %v", err)
	}
	if name != "uploaded-file.txt" {
		t.Fatalf("unexpected name: %s", name)
	}
	if got := int(inner.callCount.Load()); got != 2 {
		t.Fatalf("expected 2 upload calls, got %d", got)
	}
}

type countingReadSeeker struct {
	*strings.Reader
	readCalls atomic.Int32
}

func (r *countingReadSeeker) Read(p []byte) (int, error) {
	r.readCalls.Add(1)
	return r.Reader.Read(p)
}

type firstAttemptUploadClient struct {
	body *countingReadSeeker
}

func (c *firstAttemptUploadClient) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	if c.body.readCalls.Load() != 0 {
		return "", errors.New("upload body was read before first attempt")
	}
	if _, err := io.ReadAll(r); err != nil {
		return "", err
	}
	return "uploaded-" + name, nil
}

func (c *firstAttemptUploadClient) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (c *firstAttemptUploadClient) Delete(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

func TestWrapperRetryUploadDoesNotPreReadSeekableBody(t *testing.T) {
	body := &countingReadSeeker{Reader: strings.NewReader("chunk-payload")}
	inner := &firstAttemptUploadClient{body: body}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 10,
		RecoveryTimeout:  time.Second,
		MaxRetries:       1,
		RetryBaseDelay:   1 * time.Millisecond,
		RetryMaxDelay:    10 * time.Millisecond,
	}
	w := Wrap(inner, cfg)

	name, err := w.Upload(context.Background(), "file.txt", body)
	if err != nil {
		t.Fatalf("expected upload without pre-reading seekable body, got %v", err)
	}
	if name != "uploaded-file.txt" {
		t.Fatalf("unexpected name: %s", name)
	}
}

func TestWrapperRetryDownloadSucceeds(t *testing.T) {
	inner := &flakeyClient{failCount: 1}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 10,
		RecoveryTimeout:  time.Second,
		MaxRetries:       2,
		RetryBaseDelay:   1 * time.Millisecond,
		RetryMaxDelay:    10 * time.Millisecond,
	}
	w := Wrap(inner, cfg)

	rc, err := w.Download(context.Background(), "file.txt")
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	_ = rc.Close()
	if got := int(inner.callCount.Load()); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

type contextBoundDownloadClient struct {
	ctx context.Context
}

func (c *contextBoundDownloadClient) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	return "", errors.New("not implemented")
}

func (c *contextBoundDownloadClient) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	c.ctx = ctx
	return &contextBoundBody{ctx: ctx, r: strings.NewReader("data")}, nil
}

func (c *contextBoundDownloadClient) Delete(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

type contextBoundBody struct {
	ctx context.Context
	r   io.Reader
}

func (b *contextBoundBody) Read(p []byte) (int, error) {
	select {
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	default:
		return b.r.Read(p)
	}
}

func (b *contextBoundBody) Close() error {
	return nil
}

func TestWrapperDownloadKeepsContextAliveUntilBodyClose(t *testing.T) {
	inner := &contextBoundDownloadClient{}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 10,
		RecoveryTimeout:  time.Second,
		MaxRetries:       0,
	}
	w := Wrap(inner, cfg)

	rc, err := w.Download(context.Background(), "file.txt")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	select {
	case <-inner.ctx.Done():
		t.Fatal("download context canceled before body close")
	default:
	}

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(got) != "data" {
		t.Fatalf("expected data, got %q", string(got))
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}
	select {
	case <-inner.ctx.Done():
	default:
		t.Fatal("download context was not canceled on body close")
	}
}

type blockingDownloadClient struct{}

func (c *blockingDownloadClient) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	return "", errors.New("not implemented")
}

func (c *blockingDownloadClient) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (c *blockingDownloadClient) Delete(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

func TestWrapperDownloadTimeoutBeforeBodyReturned(t *testing.T) {
	inner := &blockingDownloadClient{}
	cfg := WrapperConfig{
		Timeout:          10 * time.Millisecond,
		FailureThreshold: 10,
		RecoveryTimeout:  time.Second,
		MaxRetries:       0,
	}
	w := Wrap(inner, cfg)

	_, err := w.Download(context.Background(), "slow.txt")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

type streamContextClient struct {
	ctxCh chan context.Context
}

func (c *streamContextClient) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
	return "", errors.New("not implemented")
}

func (c *streamContextClient) Download(ctx context.Context, name string) (io.ReadCloser, error) {
	c.ctxCh <- ctx
	return io.NopCloser(strings.NewReader("data")), nil
}

func (c *streamContextClient) Delete(ctx context.Context, name string) error {
	return errors.New("not implemented")
}

func TestWrapperDownloadStreamCancelsOnClose(t *testing.T) {
	inner := &streamContextClient{ctxCh: make(chan context.Context, 1)}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 10,
		RecoveryTimeout:  time.Second,
		MaxRetries:       0,
	}
	w := Wrap(inner, cfg).(interface {
		DownloadStream(context.Context, string) (io.ReadCloser, error)
	})

	rc, err := w.DownloadStream(context.Background(), "file.txt")
	if err != nil {
		t.Fatalf("download stream: %v", err)
	}

	reqCtx := <-inner.ctxCh
	select {
	case <-reqCtx.Done():
		t.Fatal("stream context was canceled before close")
	default:
	}

	if err := rc.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case <-reqCtx.Done():
	case <-time.After(50 * time.Millisecond):
		t.Fatal("stream context was not canceled after close")
	}
}

func TestWrapperRetryDeleteSucceeds(t *testing.T) {
	inner := &flakeyClient{failCount: 1}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 10,
		RecoveryTimeout:  time.Second,
		MaxRetries:       2,
		RetryBaseDelay:   1 * time.Millisecond,
		RetryMaxDelay:    10 * time.Millisecond,
	}
	w := Wrap(inner, cfg)

	err := w.Delete(context.Background(), "file.txt")
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if got := int(inner.callCount.Load()); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestWrapperRetryExhausted(t *testing.T) {
	inner := &mockClient{uploadErr: errors.New("permanent failure")}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 100,
		RecoveryTimeout:  time.Second,
		MaxRetries:       2,
		RetryBaseDelay:   1 * time.Millisecond,
		RetryMaxDelay:    10 * time.Millisecond,
	}
	w := Wrap(inner, cfg)

	_, err := w.Upload(context.Background(), "fail.txt", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	// 1 initial + 2 retries = 3 total calls
	if got := int(inner.uploadCalls.Load()); got != 3 {
		t.Fatalf("expected 3 upload calls, got %d", got)
	}
}

func TestWrapperNoRetryOnCircuitOpen(t *testing.T) {
	inner := &mockClient{uploadErr: errors.New("backend down")}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 1, // opens after 1 recorded failure
		RecoveryTimeout:  10 * time.Second,
		MaxRetries:       2,
		RetryBaseDelay:   1 * time.Millisecond,
		RetryMaxDelay:    10 * time.Millisecond,
	}
	w := Wrap(inner, cfg)

	// First call: retries exhaust (1 initial + 2 retries = 3 backend calls),
	// then records one failure to circuit breaker, which opens it.
	_, err := w.Upload(context.Background(), "fail.txt", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected error")
	}
	if got := int(inner.uploadCalls.Load()); got != 3 {
		t.Fatalf("expected 3 backend calls (1 + 2 retries), got %d", got)
	}

	// Second call should get circuit open immediately with zero backend calls.
	_, err = w.Upload(context.Background(), "fail2.txt", strings.NewReader("data"))
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	// No additional backend calls.
	if got := int(inner.uploadCalls.Load()); got != 3 {
		t.Fatalf("expected still 3 backend calls (no new ones), got %d", got)
	}
}

func TestWrapperRetryZeroMeansNoRetry(t *testing.T) {
	inner := &mockClient{uploadErr: errors.New("fail")}
	cfg := WrapperConfig{
		Timeout:          time.Second,
		FailureThreshold: 100,
		RecoveryTimeout:  time.Second,
		MaxRetries:       0,
	}
	w := Wrap(inner, cfg)

	_, err := w.Upload(context.Background(), "file.txt", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected error")
	}
	if got := int(inner.uploadCalls.Load()); got != 1 {
		t.Fatalf("expected exactly 1 call with 0 retries, got %d", got)
	}
}
