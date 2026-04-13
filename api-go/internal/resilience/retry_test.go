package resilience

import (
	"errors"
	"testing"
	"time"
)

func TestBackoffExponential(t *testing.T) {
	t.Parallel()
	cfg := RetryConfig{BaseDelay: 100 * time.Millisecond, MaxDelay: 5 * time.Second}

	cases := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1600 * time.Millisecond},
		{5, 3200 * time.Millisecond},
		{6, 5 * time.Second}, // capped
		{10, 5 * time.Second},
	}
	for _, tc := range cases {
		got := Backoff(tc.attempt, cfg)
		if got != tc.expected {
			t.Errorf("Backoff(%d): got %v, want %v", tc.attempt, got, tc.expected)
		}
	}
}

func TestBackoffDefaultValues(t *testing.T) {
	t.Parallel()
	cfg := RetryConfig{} // zero values
	d := Backoff(0, cfg)
	if d != 100*time.Millisecond {
		t.Errorf("expected default base delay 100ms, got %v", d)
	}
}

func TestIsCircuitOpen(t *testing.T) {
	t.Parallel()
	if !IsCircuitOpen(ErrCircuitOpen) {
		t.Fatal("expected true for ErrCircuitOpen")
	}
	if IsCircuitOpen(errors.New("other error")) {
		t.Fatal("expected false for other error")
	}
	if IsCircuitOpen(nil) {
		t.Fatal("expected false for nil")
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	if isRetryable(nil) {
		t.Fatal("nil should not be retryable")
	}
	if isRetryable(ErrCircuitOpen) {
		t.Fatal("ErrCircuitOpen should not be retryable")
	}
	if !isRetryable(errors.New("timeout")) {
		t.Fatal("transient error should be retryable")
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.BaseDelay != 100*time.Millisecond {
		t.Errorf("expected BaseDelay 100ms, got %v", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 5*time.Second {
		t.Errorf("expected MaxDelay 5s, got %v", cfg.MaxDelay)
	}
}
