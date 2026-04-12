//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCreateSource(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewSourceRepository(mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	src := Source{
		UserID:    primitive.NewObjectID(),
		Type:      SourceTypeGDrive,
		Name:      "gdrive-test",
		Key:       "{}",
		CreatedAt: time.Now().UTC(),
	}
	created, err := repo.Create(context.Background(), src)
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}
	if created.ID.IsZero() {
		t.Fatalf("expected ID to be set")
	}
	stored, err := repo.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("failed to get source: %v", err)
	}
	if stored.Name != src.Name || stored.Key != src.Key || stored.Type != src.Type {
		t.Fatalf("unexpected stored source: %+v", stored)
	}
}
