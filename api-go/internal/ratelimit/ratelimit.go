package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a per-key token bucket rate limiter.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   int     // max tokens (bucket capacity)
	now     func() time.Time
}

type bucket struct {
	tokens   float64
	lastTime time.Time
}

// NewLimiter creates a rate limiter that allows reqsPerMin requests per minute
// with a burst size equal to reqsPerMin.
func NewLimiter(reqsPerMin int) *Limiter {
	return newLimiterWithClock(reqsPerMin, time.Now)
}

func newLimiterWithClock(reqsPerMin int, now func() time.Time) *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    float64(reqsPerMin) / 60.0,
		burst:   reqsPerMin,
		now:     now,
	}
}

// Allow checks whether the given key is allowed to proceed. It returns true if
// a token is available, false otherwise. When false, retryAfter indicates how
// many seconds until a token becomes available.
func (l *Limiter) Allow(key string) (ok bool, retryAfter float64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, exists := l.buckets[key]
	if !exists {
		b = &bucket{tokens: float64(l.burst), lastTime: now}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > float64(l.burst) {
		b.tokens = float64(l.burst)
	}
	b.lastTime = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}

	wait := (1 - b.tokens) / l.rate
	return false, wait
}

// Cleanup removes stale entries that have been idle for longer than maxAge.
// Call this periodically to prevent unbounded memory growth.
func (l *Limiter) Cleanup(maxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := l.now().Add(-maxAge)
	for key, b := range l.buckets {
		if b.lastTime.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}
