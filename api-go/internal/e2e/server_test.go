//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/example/sfree/api-go/internal/app"
	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"net/http/httptest"
)

func TestServerStartup(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	router := app.SetupRouter(mongoConn)
	ts := httptest.NewServer(router)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
