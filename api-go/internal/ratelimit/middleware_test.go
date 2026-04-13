package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(cfg Config) *gin.Engine {
	r := gin.New()
	r.Use(Middleware(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func setupRouterWithAuth(cfg Config) *gin.Engine {
	r := gin.New()
	// Simulate auth middleware that sets userID before rate limiter.
	r.Use(func(c *gin.Context) {
		if c.GetHeader("Authorization") != "" {
			c.Set("userID", "test-user-123")
		}
		c.Next()
	})
	r.Use(Middleware(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func TestMiddlewareAllowsNormalTraffic(t *testing.T) {
	r := setupRouter(Config{PerIPReqsPerMin: 10, PerKeyReqsPerMin: 100})
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMiddlewareReturns429WhenExceeded(t *testing.T) {
	r := setupRouter(Config{PerIPReqsPerMin: 2, PerKeyReqsPerMin: 100})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestMiddlewareAuthenticatedUsesKeyLimiter(t *testing.T) {
	r := setupRouterWithAuth(Config{PerIPReqsPerMin: 1, PerKeyReqsPerMin: 5})

	// Authenticated requests should use the higher per-key limit.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.2:1234"
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("authenticated request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 6th request should be denied.
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for exceeded auth limit, got %d", w.Code)
	}
}

func TestMiddlewareFakeAuthHeaderDoesNotBypassIPLimit(t *testing.T) {
	r := setupRouter(Config{PerIPReqsPerMin: 2, PerKeyReqsPerMin: 100})

	// Sending fake Authorization headers should NOT bypass IP rate limit
	// since the middleware keys by IP, not header value.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		req.Header.Set("Authorization", "Bearer fake-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request with a different fake token should still be denied.
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("Authorization", "Bearer different-fake-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, fake auth header should not bypass IP limit, got %d", w.Code)
	}
}

func TestMiddlewareDifferentIPsIndependent(t *testing.T) {
	r := setupRouter(Config{PerIPReqsPerMin: 1, PerKeyReqsPerMin: 100})

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.3:1234"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("IP1 first request: expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.4:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("IP2 first request: expected 200, got %d", w2.Code)
	}
}
