//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func TestUserRepository(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewUserRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	password := "secret"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := User{Username: "testuser", PasswordHash: string(hash), CreatedAt: time.Now()}
	created, err := repo.Create(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID.IsZero() {
		t.Fatalf("expected ID to be set")
	}
	u, err := repo.GetByUsername(context.Background(), "testuser")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		t.Fatalf("password hash mismatch: %v", err)
	}
}
