//go:build integration
// +build integration

package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/s3aas/api-go/internal/config"
	"github.com/example/s3aas/api-go/internal/db"
)

func TestRouterWithDB(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	r := SetupRouter(mongoConn, cfg)
	req, _ := http.NewRequest(http.MethodGet, "/dbz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
