package resilience

import (
	"errors"
	"math"
	"time"
)

// RetryConfig holds settings for retry with exponential backoff.
type RetryConfig struct {
	MaxRetries int           // max retry attempts after the first call (default 3)
	BaseDelay  time.Duration // initial backoff delay (default 100ms)
	MaxDelay   time.Duration // maximum backoff delay cap (default 5s)
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Second,
	}
}

// Backoff calculates the exponential backoff duration for the given attempt
// (0-indexed). The delay is BaseDelay * 2^attempt, capped at MaxDelay.
func Backoff(attempt int, cfg RetryConfig) time.Duration {
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 100 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 5 * time.Second
	}
	delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))
	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}
	return delay
}

// IsCircuitOpen returns true if the error is ErrCircuitOpen.
// Useful for callers deciding whether to failover to another source.
func IsCircuitOpen(err error) bool {
	return errors.Is(err, ErrCircuitOpen)
}

// isRetryable returns true for errors that should trigger a retry.
// Circuit-open errors are not retryable (they indicate the source is
// degraded and should be skipped for failover instead).
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, ErrCircuitOpen)
}
