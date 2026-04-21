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
	"go.mongodb.org/mongo-driver/mongo"
)

func TestFileRepositoryUniqueBucketNameAndReplace(t *testing.T) {
	_, repo := newFileRepositoryTestDB(t)

	ctx := context.Background()
	bucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	name := "object-" + primitive.NewObjectID().Hex()
	firstChunks := []FileChunk{{SourceID: sourceID, Name: "chunk-a", Order: 0, Size: 5}}

	created, previous, err := repo.ReplaceByName(ctx, File{
		BucketID:  bucketID,
		Name:      name,
		CreatedAt: time.Now(),
		Chunks:    firstChunks,
	})
	if err != nil {
		t.Fatal(err)
	}
	if previous != nil {
		t.Fatalf("expected no previous file, got %+v", previous)
	}
	if created.ID.IsZero() {
		t.Fatal("expected generated file id")
	}

	_, err = repo.Create(ctx, File{
		BucketID:  bucketID,
		Name:      name,
		CreatedAt: time.Now(),
		Chunks:    []FileChunk{{SourceID: sourceID, Name: "duplicate", Order: 0, Size: 9}},
	})
	if !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("expected duplicate key error, got %v", err)
	}

	secondChunks := []FileChunk{{SourceID: sourceID, Name: "chunk-b", Order: 0, Size: 7}}
	updated, previous, err := repo.ReplaceByName(ctx, File{
		BucketID:  bucketID,
		Name:      name,
		CreatedAt: time.Now(),
		Chunks:    secondChunks,
	})
	if err != nil {
		t.Fatal(err)
	}
	if previous == nil {
		t.Fatal("expected previous file on overwrite")
	}
	if previous.ID != created.ID || updated.ID != created.ID {
		t.Fatalf("expected overwrite to preserve id: created=%s previous=%s updated=%s", created.ID.Hex(), previous.ID.Hex(), updated.ID.Hex())
	}
	if len(previous.Chunks) != 1 || previous.Chunks[0].Name != "chunk-a" {
		t.Fatalf("expected previous chunks, got %+v", previous.Chunks)
	}

	got, err := repo.GetByName(ctx, bucketID, name)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || len(got.Chunks) != 1 || got.Chunks[0].Name != "chunk-b" {
		t.Fatalf("unexpected authoritative file: %+v", got)
	}

	files, err := repo.ListByBucket(ctx, bucketID)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one file document, got %d", len(files))
	}
}

func TestFileRepositoryMigratesLegacyBucketNameIndex(t *testing.T) {
	testDB, _ := newFileRepositoryTestDB(t)
	ctx := context.Background()
	coll := testDB.Collection("files")
	_, err := coll.Indexes().DropOne(ctx, fileBucketNameUniqueIndex)
	if err != nil {
		t.Fatal(err)
	}
	_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "bucket_id", Value: 1}, {Key: "name", Value: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	repo, err := NewFileRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	bucketID := primitive.NewObjectID()
	name := "legacy-" + primitive.NewObjectID().Hex()
	_, err = repo.Create(ctx, File{BucketID: bucketID, Name: name, CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Create(ctx, File{BucketID: bucketID, Name: name, CreatedAt: time.Now()})
	if !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("expected duplicate key error after legacy index migration, got %v", err)
	}
}

func newFileRepositoryTestDB(t *testing.T) (*mongo.Database, *FileRepository) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	testDB := mongoConn.Client.Database("sfree_file_repository_" + primitive.NewObjectID().Hex())
	t.Cleanup(func() {
		_ = testDB.Drop(context.Background())
		_ = mongoConn.Close(context.Background())
	})
	repo, err := NewFileRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	return testDB, repo
}
