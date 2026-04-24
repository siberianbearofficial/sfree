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
		BucketID:     bucketID,
		Name:         name,
		CreatedAt:    time.Now(),
		Chunks:       firstChunks,
		ContentType:  "text/plain",
		UserMetadata: map[string]string{"owner": "alice"},
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
		BucketID:     bucketID,
		Name:         name,
		CreatedAt:    time.Now(),
		Chunks:       secondChunks,
		ContentType:  "application/json",
		UserMetadata: map[string]string{"owner": "bob"},
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
	if previous.ContentType != "text/plain" || previous.UserMetadata["owner"] != "alice" {
		t.Fatalf("expected previous metadata, got content_type=%q metadata=%#v", previous.ContentType, previous.UserMetadata)
	}

	got, err := repo.GetByName(ctx, bucketID, name)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || len(got.Chunks) != 1 || got.Chunks[0].Name != "chunk-b" {
		t.Fatalf("unexpected authoritative file: %+v", got)
	}
	if got.ContentType != "application/json" || got.UserMetadata["owner"] != "bob" {
		t.Fatalf("unexpected authoritative metadata: content_type=%q metadata=%#v", got.ContentType, got.UserMetadata)
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

	repo, err := NewFileRepository(ctx, testDB)
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

func TestFileRepositoryCreatesChunkReferenceIndex(t *testing.T) {
	testDB, _ := newFileRepositoryTestDB(t)
	assertMongoIndex(t, testDB.Collection("files"), fileChunkReferenceIndex, bson.D{
		{Key: "chunks.source_id", Value: 1},
		{Key: "chunks.name", Value: 1},
	})
}

func TestFileRepositoryListByBucketByNameQuery(t *testing.T) {
	_, repo := newFileRepositoryTestDB(t)
	ctx := context.Background()
	bucketID := primitive.NewObjectID()
	otherBucketID := primitive.NewObjectID()
	now := time.Now().UTC()

	for _, file := range []File{
		{BucketID: bucketID, Name: "alpha.txt", CreatedAt: now},
		{BucketID: bucketID, Name: "Quarterly Report.pdf", CreatedAt: now},
		{BucketID: bucketID, Name: "report[1].txt", CreatedAt: now},
		{BucketID: bucketID, Name: "report1.txt", CreatedAt: now},
		{BucketID: otherBucketID, Name: "other-report.txt", CreatedAt: now},
	} {
		if _, err := repo.Create(ctx, file); err != nil {
			t.Fatal(err)
		}
	}

	matches, err := repo.ListByBucketByNameQuery(ctx, bucketID, "report")
	if err != nil {
		t.Fatal(err)
	}
	assertFileNames(t, matches, []string{"Quarterly Report.pdf", "report1.txt", "report[1].txt"})

	literalMatches, err := repo.ListByBucketByNameQuery(ctx, bucketID, "report[1]")
	if err != nil {
		t.Fatal(err)
	}
	assertFileNames(t, literalMatches, []string{"report[1].txt"})

	blankMatches, err := repo.ListByBucketByNameQuery(ctx, bucketID, "   ")
	if err != nil {
		t.Fatal(err)
	}
	unfiltered, err := repo.ListByBucket(ctx, bucketID)
	if err != nil {
		t.Fatal(err)
	}
	assertFileNames(t, blankMatches, fileNames(unfiltered))
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
	repo, err := NewFileRepository(ctx, testDB)
	if err != nil {
		t.Fatal(err)
	}
	return testDB, repo
}

func assertMongoIndex(t *testing.T, coll *mongo.Collection, name string, want bson.D) {
	t.Helper()
	cursor, err := coll.Indexes().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cursor.Close(context.Background()) }()

	for cursor.Next(context.Background()) {
		var idx indexInfo
		if err := cursor.Decode(&idx); err != nil {
			t.Fatal(err)
		}
		if idx.Name != name {
			continue
		}
		if !indexKeysEqual(idx.Key, want) {
			t.Fatalf("index %q keys: got %v, want %v", name, idx.Key, want)
		}
		return
	}
	if err := cursor.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatalf("missing index %q", name)
}

func indexKeysEqual(got, want bson.D) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i].Key != want[i].Key {
			return false
		}
		if !indexValueIsOne(got[i].Value) || !indexValueIsOne(want[i].Value) {
			return false
		}
	}
	return true
}

func assertFileNames(t *testing.T, files []File, want []string) {
	t.Helper()
	got := fileNames(files)
	if len(got) != len(want) {
		t.Fatalf("expected file names %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected file names %v, got %v", want, got)
		}
	}
}

func fileNames(files []File) []string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Name)
	}
	return names
}
