package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSessionLoginSetsSecureCookieForHTTPSFrontend(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		JWTSecret:   "jwt-secret",
		FrontendURL: "https://frontend.example",
	}

	router := gin.New()
	router.POST("/api/v1/auth/session", setUserID(primitive.NewObjectID().Hex()), SessionLogin(cfg))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/session", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	authCookie := findCookie(t, w.Result(), authCookieName)
	if authCookie.Value == "" {
		t.Fatal("auth cookie should have a value")
	}
	if !authCookie.HttpOnly {
		t.Fatal("auth cookie should be HttpOnly")
	}
	if !authCookie.Secure {
		t.Fatal("auth cookie should be Secure")
	}
	if authCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("auth cookie SameSite = %v, want %v", authCookie.SameSite, http.SameSiteLaxMode)
	}
	if authCookie.MaxAge != authSessionMaxAge() {
		t.Fatalf("auth cookie MaxAge = %d, want %d", authCookie.MaxAge, authSessionMaxAge())
	}
}

func TestSessionLoginAllowsLocalHTTPFrontend(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		JWTSecret:   "jwt-secret",
		FrontendURL: "http://localhost:3000",
	}

	router := gin.New()
	router.POST("/api/v1/auth/session", setUserID(primitive.NewObjectID().Hex()), SessionLogin(cfg))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/session", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	authCookie := findCookie(t, w.Result(), authCookieName)
	if authCookie.Secure {
		t.Fatal("auth cookie should not be Secure for local HTTP frontend")
	}
}

func TestSessionLogoutClearsCookie(t *testing.T) {
	t.Parallel()

	router := gin.New()
	router.DELETE("/api/v1/auth/session", SessionLogout(&config.Config{FrontendURL: "https://frontend.example"}))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/session", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	authCookie := findCookie(t, w.Result(), authCookieName)
	if authCookie.Value != "" {
		t.Fatalf("auth cookie value = %q, want empty", authCookie.Value)
	}
	if authCookie.MaxAge >= 0 {
		t.Fatalf("auth cookie MaxAge = %d, want negative", authCookie.MaxAge)
	}
}
