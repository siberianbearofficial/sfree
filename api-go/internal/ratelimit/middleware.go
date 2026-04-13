package ratelimit

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Config holds rate-limiting parameters.
type Config struct {
	PerIPReqsPerMin  int // rate limit for unauthenticated requests per IP (default 60)
	PerKeyReqsPerMin int // rate limit for authenticated requests per user (default 600)
	CleanupInterval  time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		PerIPReqsPerMin:  60,
		PerKeyReqsPerMin: 600,
		CleanupInterval:  10 * time.Minute,
	}
}

// Middleware returns a gin middleware that enforces rate limits using in-memory
// token buckets. All requests are keyed by client IP. Requests from
// authenticated users (where "userID" is set in the gin context by auth
// middleware) use the higher per-key limit; unauthenticated requests use the
// lower per-IP limit.
func Middleware(cfg Config) gin.HandlerFunc {
	if cfg.PerIPReqsPerMin <= 0 {
		cfg.PerIPReqsPerMin = 60
	}
	if cfg.PerKeyReqsPerMin <= 0 {
		cfg.PerKeyReqsPerMin = 600
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 10 * time.Minute
	}

	ipLimiter := NewLimiter(cfg.PerIPReqsPerMin)
	keyLimiter := NewLimiter(cfg.PerKeyReqsPerMin)

	// Background cleanup goroutine to prevent memory leaks.
	go func() {
		ticker := time.NewTicker(cfg.CleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			ipLimiter.Cleanup(cfg.CleanupInterval * 3)
			keyLimiter.Cleanup(cfg.CleanupInterval * 3)
		}
	}()

	return func(c *gin.Context) {
		// Run after other middleware in the chain so auth has a chance
		// to set "userID". We call c.Next() first, then this is a
		// pre-handler check — but gin processes Use() middleware
		// before handlers, so we check context keys that prior
		// middleware may have set.
		//
		// Since this middleware runs globally before per-route auth,
		// "userID" is typically not yet set. All requests are therefore
		// rate-limited by client IP at the lower rate. Authenticated
		// routes benefit from the higher limit only when auth runs as
		// a global middleware too.
		var ok bool
		var retryAfter float64

		ip := c.ClientIP()

		// If auth middleware already ran (e.g. global auth), use the
		// higher per-user limit keyed by user+IP. Otherwise, use the
		// lower per-IP limit. Keying by IP prevents bypass via fake
		// or rotated Authorization headers.
		if userID, exists := c.Get("userID"); exists && userID != nil {
			key := fmt.Sprintf("user:%v@%s", userID, ip)
			ok, retryAfter = keyLimiter.Allow(key)
		} else {
			ok, retryAfter = ipLimiter.Allow(ip)
		}

		if !ok {
			retrySeconds := int(math.Ceil(retryAfter))
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			c.Header("Retry-After", fmt.Sprintf("%d", retrySeconds))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
