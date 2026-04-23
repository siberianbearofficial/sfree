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

func TestMultipartUploadRepositorySetPartReplacesWithoutDroppingParts(t *testing.T) {
	_, repo := newMultipartRepositoryTestDB(t)

	ctx := context.Background()
	bucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	uploadID := "upload-" + primitive.NewObjectID().Hex()
	created, err := repo.Create(ctx, MultipartUpload{
		BucketID:  bucketID,
		ObjectKey: "object.bin",
		UploadID:  uploadID,
		Parts: []UploadPart{
			{
				PartNumber: 1,
				ETag:       `"old"`,
				Size:       5,
				Chunks:     []FileChunk{{SourceID: sourceID, Name: "old-part-1", Order: 0, Size: 5}},
			},
			{
				PartNumber: 1,
				ETag:       `"duplicate-old"`,
				Size:       6,
				Chunks:     []FileChunk{{SourceID: sourceID, Name: "duplicate-old-part-1", Order: 1, Size: 6}},
			},
			{
				PartNumber: 2,
				ETag:       `"kept"`,
				Size:       7,
				Chunks:     []FileChunk{{SourceID: sourceID, Name: "kept-part-2", Order: 2, Size: 7}},
			},
		},
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	replacement := UploadPart{
		PartNumber: 1,
		ETag:       `"new"`,
		Size:       9,
		Chunks:     []FileChunk{{SourceID: sourceID, Name: "new-part-1", Order: 0, Size: 9}},
	}
	previous, err := repo.SetPart(ctx, uploadID, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if previous == nil || previous.ETag != `"old"` {
		t.Fatalf("expected previous part to be returned, got %+v", previous)
	}

	got, err := repo.GetByUploadID(ctx, uploadID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected upload id %s, got %s", created.ID.Hex(), got.ID.Hex())
	}
	if len(got.Parts) != 2 {
		t.Fatalf("expected exactly 2 parts after replacement, got %+v", got.Parts)
	}
	assertMultipartPart(t, got.Parts, 1, `"new"`, "new-part-1")
	assertMultipartPart(t, got.Parts, 2, `"kept"`, "kept-part-2")

	added := UploadPart{
		PartNumber: 3,
		ETag:       `"added"`,
		Size:       11,
		Chunks:     []FileChunk{{SourceID: sourceID, Name: "added-part-3", Order: 2, Size: 11}},
	}
	previous, err = repo.SetPart(ctx, uploadID, added)
	if err != nil {
		t.Fatal(err)
	}
	if previous != nil {
		t.Fatalf("expected no previous part for insert, got %+v", previous)
	}

	got, err = repo.GetByUploadID(ctx, uploadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Parts) != 3 {
		t.Fatalf("expected 3 parts after insert, got %+v", got.Parts)
	}
	assertMultipartPart(t, got.Parts, 1, `"new"`, "new-part-1")
	assertMultipartPart(t, got.Parts, 2, `"kept"`, "kept-part-2")
	assertMultipartPart(t, got.Parts, 3, `"added"`, "added-part-3")

	_, err = repo.SetPart(ctx, "missing-upload", added)
	if err != mongo.ErrNoDocuments {
		t.Fatalf("expected ErrNoDocuments for missing upload, got %v", err)
	}
}

func TestMultipartUploadRepositoryCreatesPartChunkReferenceIndex(t *testing.T) {
	testDB, _ := newMultipartRepositoryTestDB(t)
	assertMongoIndex(t, testDB.Collection("multipart_uploads"), multipartPartChunkNameBucketIndex, bson.D{
		{Key: "parts.chunks.name", Value: 1},
		{Key: "bucket_id", Value: 1},
	})
}

func assertMultipartPart(t *testing.T, parts []UploadPart, partNumber int, etag string, chunkName string) {
	t.Helper()
	for _, part := range parts {
		if part.PartNumber != partNumber {
			continue
		}
		if part.ETag != etag {
			t.Fatalf("part %d etag: got %q, want %q", partNumber, part.ETag, etag)
		}
		if len(part.Chunks) != 1 || part.Chunks[0].Name != chunkName {
			t.Fatalf("part %d chunks: got %+v, want %q", partNumber, part.Chunks, chunkName)
		}
		return
	}
	t.Fatalf("missing part %d in %+v", partNumber, parts)
}

func newMultipartRepositoryTestDB(t *testing.T) (*mongo.Database, *MultipartUploadRepository) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	testDB := mongoConn.Client.Database("sfree_multipart_repository_" + primitive.NewObjectID().Hex())
	t.Cleanup(func() {
		_ = testDB.Drop(context.Background())
		_ = mongoConn.Close(context.Background())
	})
	repo, err := NewMultipartUploadRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	return testDB, repo
}
