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

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
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
	userRepo, err := repository.NewUserRepository(mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	user, err := userRepo.Create(context.Background(), repository.User{
		Username:     "tester",
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	repo, err := repository.NewBucketRepository(mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	sourceRepo, err := repository.NewSourceRepository(mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	source, err := sourceRepo.Create(context.Background(), repository.Source{
		UserID:    user.ID,
		Type:      repository.SourceTypeGDrive,
		Name:      "src-test",
		Key:       "{}",
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	auth := BasicAuth(userRepo)
	router.POST("/api/v1/buckets", auth, CreateBucket(repo, sourceRepo, "testkey"))
	body, _ := json.Marshal(map[string]any{"key": "bucket-test", "source_ids": []string{source.ID.Hex()}})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/buckets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("tester", "pass")
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
	badReq, _ := http.NewRequest(http.MethodPost, "/api/v1/buckets", bytes.NewReader(body))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.SetBasicAuth("tester", "wrong")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, badReq)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w2.Code)
	}
}
