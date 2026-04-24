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
	"golang.org/x/crypto/bcrypt"
)

func TestCreateBucket(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	userRepo, err := repository.NewUserRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	user, err := userRepo.Create(context.Background(), repository.User{
		Username:     "tester",
		PasswordHash: string(hash),
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	repo, err := repository.NewBucketRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	sourceRepo, err := repository.NewSourceRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatal(err)
	}
	source, err := sourceRepo.Create(context.Background(), repository.Source{
		UserID:    user.ID,
		Type:      repository.SourceTypeGDrive,
		Name:      "src-test",
		Key:       "{}",
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	auth := BasicAuth(userRepo)
	router.POST("/api/v1/buckets", auth, CreateBucket(repo, sourceRepo, "testkey"))
	body, _ := json.Marshal(map[string]any{"key": "bucket-test", "source_ids": []string{source.ID.Hex()}})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/buckets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("tester", "pass")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Key          string    `json:"key"`
		AccessKey    string    `json:"access_key"`
		AccessSecret string    `json:"access_secret"`
		CreatedAt    time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Key != "bucket-test" || resp.AccessKey == "" || resp.AccessSecret == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	badReq, _ := http.NewRequest(http.MethodPost, "/api/v1/buckets", bytes.NewReader(body))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.SetBasicAuth("tester", "wrong")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, badReq)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w2.Code)
	}
}

func TestListFilesSearchQueryWithGrantAccess(t *testing.T) {
	bucketRepo, fileRepo, grantRepo := newBucketFileHandlerTestRepos(t)
	ctx := context.Background()
	ownerID := primitive.NewObjectID()
	viewerID := primitive.NewObjectID()
	bucket := createBucketFileTestBucket(t, ctx, bucketRepo, ownerID)
	otherBucket := createBucketFileTestBucket(t, ctx, bucketRepo, primitive.NewObjectID())

	if _, err := grantRepo.Create(ctx, repository.BucketGrant{
		BucketID:  bucket.ID,
		UserID:    viewerID,
		Role:      repository.RoleViewer,
		GrantedBy: ownerID,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	for _, file := range []repository.File{
		{BucketID: bucket.ID, Name: "annual-report.pdf", CreatedAt: now},
		{BucketID: bucket.ID, Name: "notes.txt", CreatedAt: now},
		{BucketID: otherBucket.ID, Name: "other-report.pdf", CreatedAt: now},
	} {
		if _, err := fileRepo.Create(ctx, file); err != nil {
			t.Fatal(err)
		}
	}

	router := gin.New()
	router.GET("/buckets/:id/files", setUserID(viewerID.Hex()), ListFiles(bucketRepo, fileRepo, grantRepo))
	req, _ := http.NewRequest(http.MethodGet, "/buckets/"+bucket.ID.Hex()+"/files?q=REPORT", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp []fileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp) != 1 || resp[0].Name != "annual-report.pdf" {
		t.Fatalf("unexpected files: %+v", resp)
	}

	noAccessRouter := gin.New()
	noAccessRouter.GET("/buckets/:id/files", setUserID(primitive.NewObjectID().Hex()), ListFiles(bucketRepo, fileRepo, grantRepo))
	noAccessReq, _ := http.NewRequest(http.MethodGet, "/buckets/"+bucket.ID.Hex()+"/files?q=report", nil)
	noAccessW := httptest.NewRecorder()
	noAccessRouter.ServeHTTP(noAccessW, noAccessReq)
	if noAccessW.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", noAccessW.Code)
	}
}

func newBucketFileHandlerTestRepos(t *testing.T) (*repository.BucketRepository, *repository.FileRepository, *repository.BucketGrantRepository) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	testDB := mongoConn.Client.Database("sfree_bucket_file_handlers_" + primitive.NewObjectID().Hex())
	t.Cleanup(func() {
		_ = testDB.Drop(context.Background())
		_ = mongoConn.Close(context.Background())
	})
	bucketRepo, err := repository.NewBucketRepository(context.Background(), testDB)
	if err != nil {
		t.Fatal(err)
	}
	fileRepo, err := repository.NewFileRepository(context.Background(), testDB)
	if err != nil {
		t.Fatal(err)
	}
	grantRepo, err := repository.NewBucketGrantRepository(context.Background(), testDB)
	if err != nil {
		t.Fatal(err)
	}
	return bucketRepo, fileRepo, grantRepo
}

func createBucketFileTestBucket(t *testing.T, ctx context.Context, repo *repository.BucketRepository, ownerID primitive.ObjectID) *repository.Bucket {
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
