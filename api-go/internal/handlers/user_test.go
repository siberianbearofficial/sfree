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

func TestCreateUser(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := repository.NewUserRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/api/v1/users", CreateUser(repo))
	body, _ := json.Marshal(map[string]string{"username": "user-test"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		ID        string    `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		Password  string    `json:"password"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.ID == "" || resp.Password == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	stored, err := repo.GetByUsername(context.Background(), "user-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(resp.Password)); err != nil {
		t.Fatalf("password hash mismatch: %v", err)
	}
}
