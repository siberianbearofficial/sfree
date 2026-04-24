//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"go.mongodb.org/mongo-driver/bson"
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
	repo, err := NewSourceRepository(context.Background(), mongoConn.DB)
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

func TestListMetadataByUserDoesNotDecryptKeys(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewSourceRepository(context.Background(), mongoConn.DB, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	userID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	_, err = repo.coll.InsertOne(context.Background(), Source{
		ID:        sourceID,
		UserID:    userID,
		Type:      SourceTypeS3,
		Name:      "metadata-only",
		Key:       "not-valid-ciphertext",
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = repo.coll.DeleteMany(context.Background(), bson.M{"user_id": userID})
	})

	sources, err := repo.ListMetadataByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("metadata listing should not decrypt keys: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected one source, got %d", len(sources))
	}
	if sources[0].ID != sourceID || sources[0].Name != "metadata-only" || sources[0].Type != SourceTypeS3 {
		t.Fatalf("unexpected source metadata: %+v", sources[0])
	}
	if sources[0].Key != "" {
		t.Fatalf("expected projected key to be empty, got %q", sources[0].Key)
	}
	if _, err := repo.GetByID(context.Background(), sourceID); err == nil {
		t.Fatalf("expected GetByID to decrypt key material")
	}
	if _, err := repo.ListByIDs(context.Background(), []primitive.ObjectID{sourceID}); err == nil {
		t.Fatalf("expected ListByIDs to decrypt key material")
	}
}
