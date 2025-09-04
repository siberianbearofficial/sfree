//go:build integration
// +build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/example/s3aas/api-go/internal/config"
	"github.com/example/s3aas/api-go/internal/db"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBucketRepository(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewBucketRepository(mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	b := Bucket{UserID: primitive.NewObjectID(), Key: "testbucket", AccessKey: "ak", AccessSecretEnc: "secret", CreatedAt: time.Now()}
	created, err := repo.Create(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByKey(context.Background(), "testbucket")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Key != "testbucket" {
		t.Fatalf("retrieved bucket mismatch")
	}
}
