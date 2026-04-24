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
	"go.mongodb.org/mongo-driver/mongo"
)

func TestUpdateGrantRejectsCrossBucketGrantID(t *testing.T) {
	ctx := context.Background()
	bucketRepo, grantRepo := newBucketGrantHandlerTestRepos(t)
	ownerA := primitive.NewObjectID()
	ownerB := primitive.NewObjectID()
	bucketA := createGrantTestBucket(t, ctx, bucketRepo, ownerA)
	bucketB := createGrantTestBucket(t, ctx, bucketRepo, ownerB)
	crossGrant := createGrantTestGrant(t, ctx, grantRepo, bucketB.ID, primitive.NewObjectID(), ownerB, repository.RoleViewer)

	router := gin.New()
	router.PATCH("/buckets/:id/grants/:grant_id", setUserID(ownerA.Hex()), UpdateGrant(bucketRepo, grantRepo))
	body, _ := json.Marshal(map[string]string{"role": string(repository.RoleEditor)})
	req, _ := http.NewRequest(
		http.MethodPatch,
		"/buckets/"+bucketA.ID.Hex()+"/grants/"+crossGrant.ID.Hex(),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-bucket update, got %d", w.Code)
	}
	unchanged, err := grantRepo.GetByBucketAndUser(ctx, bucketB.ID, crossGrant.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Role != repository.RoleViewer {
		t.Fatalf("expected cross-bucket grant role to remain viewer, got %q", unchanged.Role)
	}

	sameBucketGrant := createGrantTestGrant(t, ctx, grantRepo, bucketA.ID, primitive.NewObjectID(), ownerA, repository.RoleViewer)
	req, _ = http.NewRequest(
		http.MethodPatch,
		"/buckets/"+bucketA.ID.Hex()+"/grants/"+sameBucketGrant.ID.Hex(),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for same-bucket update, got %d", w.Code)
	}
	updated, err := grantRepo.GetByBucketAndUser(ctx, bucketA.ID, sameBucketGrant.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Role != repository.RoleEditor {
		t.Fatalf("expected same-bucket grant role editor, got %q", updated.Role)
	}
}

func TestDeleteGrantRejectsCrossBucketGrantID(t *testing.T) {
	ctx := context.Background()
	bucketRepo, grantRepo := newBucketGrantHandlerTestRepos(t)
	ownerA := primitive.NewObjectID()
	ownerB := primitive.NewObjectID()
	bucketA := createGrantTestBucket(t, ctx, bucketRepo, ownerA)
	bucketB := createGrantTestBucket(t, ctx, bucketRepo, ownerB)
	crossGrant := createGrantTestGrant(t, ctx, grantRepo, bucketB.ID, primitive.NewObjectID(), ownerB, repository.RoleViewer)

	router := gin.New()
	router.DELETE("/buckets/:id/grants/:grant_id", setUserID(ownerA.Hex()), DeleteGrant(bucketRepo, grantRepo))
	req, _ := http.NewRequest(
		http.MethodDelete,
		"/buckets/"+bucketA.ID.Hex()+"/grants/"+crossGrant.ID.Hex(),
		nil,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-bucket delete, got %d", w.Code)
	}
	if _, err := grantRepo.GetByBucketAndUser(ctx, bucketB.ID, crossGrant.UserID); err != nil {
		t.Fatal(err)
	}

	sameBucketGrant := createGrantTestGrant(t, ctx, grantRepo, bucketA.ID, primitive.NewObjectID(), ownerA, repository.RoleViewer)
	req, _ = http.NewRequest(
		http.MethodDelete,
		"/buckets/"+bucketA.ID.Hex()+"/grants/"+sameBucketGrant.ID.Hex(),
		nil,
	)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for same-bucket delete, got %d", w.Code)
	}
	if _, err := grantRepo.GetByBucketAndUser(ctx, bucketA.ID, sameBucketGrant.UserID); err != mongo.ErrNoDocuments {
		t.Fatalf("expected same-bucket grant to be deleted, got %v", err)
	}
}

func newBucketGrantHandlerTestRepos(t *testing.T) (*repository.BucketRepository, *repository.BucketGrantRepository) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	testDB := mongoConn.Client.Database("sfree_bucket_grant_handlers_" + primitive.NewObjectID().Hex())
	t.Cleanup(func() {
		_ = testDB.Drop(context.Background())
		_ = mongoConn.Close(context.Background())
	})
	bucketRepo, err := repository.NewBucketRepository(context.Background(), testDB)
	if err != nil {
		t.Fatal(err)
	}
	grantRepo, err := repository.NewBucketGrantRepository(context.Background(), testDB)
	if err != nil {
		t.Fatal(err)
	}
	return bucketRepo, grantRepo
}

func createGrantTestBucket(t *testing.T, ctx context.Context, repo *repository.BucketRepository, ownerID primitive.ObjectID) *repository.Bucket {
	t.Helper()
	suffix := primitive.NewObjectID().Hex()
	bucket, err := repo.Create(ctx, repository.Bucket{
		UserID:          ownerID,
		Key:             "bucket-" + suffix,
		AccessKey:       "access-" + suffix,
		AccessSecretEnc: "secret-" + suffix,
		CreatedAt:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return bucket
}

func createGrantTestGrant(
	t *testing.T,
	ctx context.Context,
	repo *repository.BucketGrantRepository,
	bucketID, userID, grantedBy primitive.ObjectID,
	role repository.BucketRole,
) *repository.BucketGrant {
	t.Helper()
	grant, err := repo.Create(ctx, repository.BucketGrant{
		BucketID:  bucketID,
		UserID:    userID,
		Role:      role,
		GrantedBy: grantedBy,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return grant
}
