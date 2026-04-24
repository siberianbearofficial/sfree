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
	"go.mongodb.org/mongo-driver/mongo"
)

func TestShareLinkRepositoryDeleteByFile(t *testing.T) {
	_, repo := newShareLinkRepositoryTestDB(t)
	ctx := context.Background()
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	otherFileID := primitive.NewObjectID()
	userID := primitive.NewObjectID()

	for _, link := range []ShareLink{
		{BucketID: bucketID, FileID: fileID, UserID: userID, Token: "file-a", CreatedAt: time.Now().UTC()},
		{BucketID: bucketID, FileID: fileID, UserID: userID, Token: "file-b", CreatedAt: time.Now().UTC()},
		{BucketID: bucketID, FileID: otherFileID, UserID: userID, Token: "other-file", CreatedAt: time.Now().UTC()},
	} {
		if _, err := repo.Create(ctx, link); err != nil {
			t.Fatal(err)
		}
	}

	if err := repo.DeleteByFile(ctx, fileID); err != nil {
		t.Fatal(err)
	}
	links, err := repo.ListByFile(ctx, fileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 0 {
		t.Fatalf("expected deleted file links, got %d", len(links))
	}
	links, err = repo.ListByFile(ctx, otherFileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Token != "other-file" {
		t.Fatalf("expected unrelated file link to remain, got %+v", links)
	}
}

func TestShareLinkRepositoryDeleteByBucket(t *testing.T) {
	_, repo := newShareLinkRepositoryTestDB(t)
	ctx := context.Background()
	bucketID := primitive.NewObjectID()
	otherBucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	otherFileID := primitive.NewObjectID()
	userID := primitive.NewObjectID()

	for _, link := range []ShareLink{
		{BucketID: bucketID, FileID: fileID, UserID: userID, Token: "bucket-a", CreatedAt: time.Now().UTC()},
		{BucketID: bucketID, FileID: primitive.NewObjectID(), UserID: userID, Token: "bucket-b", CreatedAt: time.Now().UTC()},
		{BucketID: otherBucketID, FileID: otherFileID, UserID: userID, Token: "other-bucket", CreatedAt: time.Now().UTC()},
	} {
		if _, err := repo.Create(ctx, link); err != nil {
			t.Fatal(err)
		}
	}

	if err := repo.DeleteByBucket(ctx, bucketID); err != nil {
		t.Fatal(err)
	}
	links, err := repo.ListByFile(ctx, fileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 0 {
		t.Fatalf("expected deleted bucket link, got %d", len(links))
	}
	links, err = repo.ListByFile(ctx, otherFileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Token != "other-bucket" {
		t.Fatalf("expected unrelated bucket link to remain, got %+v", links)
	}
}

func newShareLinkRepositoryTestDB(t *testing.T) (*mongo.Database, *ShareLinkRepository) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	testDB := mongoConn.Client.Database("sfree_share_link_repository_" + primitive.NewObjectID().Hex())
	t.Cleanup(func() {
		_ = testDB.Drop(context.Background())
		_ = mongoConn.Close(context.Background())
	})
	repo, err := NewShareLinkRepository(context.Background(), testDB)
	if err != nil {
		t.Fatal(err)
	}
	return testDB, repo
}
