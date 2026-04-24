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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

func TestCreateGDriveSource(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	userRepo, err := repository.NewUserRepository(context.Background(), mongoConn.DB)
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
	repo, err := repository.NewSourceRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	auth := BasicAuth(userRepo)
	router.POST("/api/v1/sources/gdrive", auth, CreateGDriveSource(repo))
	body, _ := json.Marshal(map[string]string{"name": "src-test", "key": "{}"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sources/gdrive", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("tester", "pass")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		Type      string    `json:"type"`
		Key       string    `json:"key"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Name != "src-test" || resp.Type != string(repository.SourceTypeGDrive) || resp.Key != "{}" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	oid, err := primitive.ObjectIDFromHex(resp.ID)
	if err != nil {
		t.Fatalf("invalid id: %v", err)
	}
	stored, err := repo.GetByID(context.Background(), oid)
	if err != nil {
		t.Fatalf("failed to get stored source: %v", err)
	}
	if stored.UserID != user.ID {
		t.Fatalf("unexpected user id: %s != %s", stored.UserID.Hex(), user.ID.Hex())
	}
}

func TestCreateTelegramSource(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	userRepo, err := repository.NewUserRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	user, err := userRepo.Create(context.Background(), repository.User{
		Username:     "tester-tg",
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	repo, err := repository.NewSourceRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	auth := BasicAuth(userRepo)
	router.POST("/api/v1/sources/telegram", auth, CreateTelegramSource(repo))
	body, _ := json.Marshal(map[string]string{"name": "src-tg", "token": "token", "chat_id": "123"})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/sources/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("tester-tg", "pass")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		ID        string    `json:"id"`
		Name      string    `json:"name"`
		Type      string    `json:"type"`
		Key       string    `json:"key"`
		CreatedAt time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Name != "src-tg" || resp.Type != string(repository.SourceTypeTelegram) || resp.Key == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	oid, err := primitive.ObjectIDFromHex(resp.ID)
	if err != nil {
		t.Fatalf("invalid id: %v", err)
	}
	stored, err := repo.GetByID(context.Background(), oid)
	if err != nil {
		t.Fatalf("failed to get stored source: %v", err)
	}
	if stored.UserID != user.ID {
		t.Fatalf("unexpected user id: %s != %s", stored.UserID.Hex(), user.ID.Hex())
	}
}

func TestDeleteSourceBlockedWhenBucketUsesIt(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	userRepo, err := repository.NewUserRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	user, err := userRepo.Create(context.Background(), repository.User{
		Username:     "tester-delete-src",
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceRepo, err := repository.NewSourceRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	bucketRepo, err := repository.NewBucketRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	source, err := sourceRepo.Create(context.Background(), repository.Source{
		UserID:    user.ID,
		Type:      repository.SourceTypeGDrive,
		Name:      "src-delete-block",
		Key:       "{}",
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = bucketRepo.Create(context.Background(), repository.Bucket{
		UserID:          user.ID,
		Key:             "bucket-delete-block",
		AccessKey:       "bucket-delete-block",
		AccessSecretEnc: "enc",
		SourceIDs:       []primitive.ObjectID{source.ID},
		CreatedAt:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	auth := BasicAuth(userRepo)
	router.DELETE("/api/v1/sources/:id", auth, DeleteSource(sourceRepo, bucketRepo))
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/sources/"+source.ID.Hex(), nil)
	req.SetBasicAuth("tester-delete-src", "pass")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
	if _, err := sourceRepo.GetByID(context.Background(), source.ID); err != nil {
		t.Fatalf("expected source to remain after conflict: %v", err)
	}
}
