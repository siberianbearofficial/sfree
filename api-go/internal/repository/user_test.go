//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"

	"github.com/example/s3aas/api-go/internal/config"
	"github.com/example/s3aas/api-go/internal/db"
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
	repo, err := NewUserRepository(mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	password := "secret"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := User{Username: "testuser", PasswordHash: string(hash)}
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	u, err := repo.GetByUsername(context.Background(), "testuser")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		t.Fatalf("password hash mismatch: %v", err)
	}
}
