package ratelimit

import (
	"testing"
	"time"
)

type fakeClock struct {
	t time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.t = c.t.Add(d)
}

func TestLimiterAllow(t *testing.T) {
	l := NewLimiter(60) // 60 req/min = 1 req/sec

	// First request should always pass.
	ok, _ := l.Allow("test")
	if !ok {
		t.Fatal("first request should be allowed")
	}
}

func TestLimiterBurst(t *testing.T) {
	l := NewLimiter(5) // 5 req/min burst of 5

	for i := 0; i < 5; i++ {
		ok, _ := l.Allow("burst")
		if !ok {
			t.Fatalf("request %d within burst should be allowed", i+1)
		}
	}

	ok, retryAfter := l.Allow("burst")
	if ok {
		t.Fatal("request beyond burst should be denied")
	}
	if retryAfter <= 0 {
		t.Fatal("retryAfter should be positive when denied")
	}
}

func TestLimiterSeparateKeys(t *testing.T) {
	l := NewLimiter(1) // 1 req/min, burst of 1

	ok, _ := l.Allow("a")
	if !ok {
		t.Fatal("key 'a' first request should be allowed")
	}

	ok, _ = l.Allow("b")
	if !ok {
		t.Fatal("key 'b' first request should be allowed (separate bucket)")
	}
}

func TestLimiterRefill(t *testing.T) {
	clock := newFakeClock()
	l := newLimiterWithClock(600, clock.Now) // 600 req/min = 10 req/sec

	// Exhaust all tokens.
	for i := 0; i < 600; i++ {
		l.Allow("refill")
	}

	ok, _ := l.Allow("refill")
	if ok {
		t.Fatal("should be denied after exhausting tokens")
	}

	clock.Advance(100 * time.Millisecond)

	ok, _ = l.Allow("refill")
	if !ok {
		t.Fatal("should be allowed after token refill")
	}
}

func TestCleanup(t *testing.T) {
	l := NewLimiter(60)
	l.Allow("stale")
	l.Allow("fresh")

	// Manually backdate the "stale" entry.
	l.mu.Lock()
	l.buckets["stale"].lastTime = time.Now().Add(-1 * time.Hour)
	l.mu.Unlock()

	l.Cleanup(30 * time.Minute)

	l.mu.Lock()
	_, staleExists := l.buckets["stale"]
	_, freshExists := l.buckets["fresh"]
	l.mu.Unlock()

	if staleExists {
		t.Fatal("stale entry should have been cleaned up")
	}
	if !freshExists {
		t.Fatal("fresh entry should still exist")
	}
}
