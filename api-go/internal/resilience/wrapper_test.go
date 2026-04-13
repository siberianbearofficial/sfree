package resilience

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// mockClient is a test double for the source client interface.
type mockClient struct {
	uploadErr   error
	downloadErr error
	deleteErr   error
	delay       time.Duration
}

func (m *mockClient) Upload(ctx context.Context, name string, r io.Reader) (string, error) {
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
	cfg := WrapperConfig{Timeout: 50 * time.Millisecond, FailureThreshold: 100, RecoveryTimeout: time.Second}
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
	cfg := WrapperConfig{Timeout: time.Second, FailureThreshold: 3, RecoveryTimeout: time.Second}
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
	cfg := WrapperConfig{Timeout: time.Second, FailureThreshold: 2, RecoveryTimeout: 50 * time.Millisecond}
	w := Wrap(inner, cfg)

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		w.Upload(ctx, "fail.txt", strings.NewReader("data"))
	}

	// Circuit is open.
	_, err := w.Upload(ctx, "fail.txt", strings.NewReader("data"))
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	// Fix the backend.
	inner.uploadErr = nil
	time.Sleep(60 * time.Millisecond)

	// Should succeed (half-open probe).
	name, err := w.Upload(ctx, "ok.txt", strings.NewReader("data"))
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if name != "uploaded-ok.txt" {
		t.Fatalf("expected uploaded-ok.txt, got %s", name)
	}
}
