//go:build integration
// +build integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/s3aas/api-go/internal/config"
	"github.com/example/s3aas/api-go/internal/db"
	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-gonic/gin"
)

func TestCreateBucket(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := repository.NewBucketRepository(mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/api/v1/buckets", CreateBucket(repo))
	body, _ := json.Marshal(map[string]string{"key": "bucket-test"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/buckets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Key          string    `json:"key"`
		AccessKey    string    `json:"access_key"`
		AccessSecret string    `json:"access_secret"`
		CreatedAt    time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Key != "bucket-test" || resp.AccessKey == "" || resp.AccessSecret == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
