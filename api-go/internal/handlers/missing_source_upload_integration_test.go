//go:build integration
// +build integration

package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type missingSourceUploadFactory struct {
	calls int
}

func (f *missingSourceUploadFactory) Factory(context.Context, *repository.Source) (manager.SourceClient, error) {
	f.calls++
	return missingSourceUploadClient{}, nil
}

type missingSourceUploadClient struct{}

func (missingSourceUploadClient) Upload(context.Context, string, io.Reader) (string, error) {
	return "", errors.New("unexpected upload")
}

func (missingSourceUploadClient) Download(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("unexpected download")
}

func (missingSourceUploadClient) Delete(context.Context, string) error {
	return errors.New("unexpected delete")
}

func TestUploadFileMissingBucketSourceReturnsBadRequest(t *testing.T) {
	bucketRepo, sourceRepo, fileRepo, _, factory := newMissingSourceUploadHandlerFixture(t)
	ctx := context.Background()
	ownerID := primitive.NewObjectID()
	bucket := createMissingSourceUploadBucket(t, ctx, bucketRepo, ownerID)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", "object.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fileWriter.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.POST("/buckets/:id/upload", setUserID(ownerID.Hex()), UploadFileWithFactory(bucketRepo, sourceRepo, fileRepo, nil, 4, factory.Factory))
	req := httptest.NewRequest(http.MethodPost, "/buckets/"+bucket.ID.Hex()+"/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%q", w.Code, w.Body.String())
	}
	if factory.calls != 0 {
		t.Fatalf("expected no source client factory calls, got %d", factory.calls)
	}
	if _, err := fileRepo.GetByName(ctx, bucket.ID, "object.txt"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected no stored file, got err=%v", err)
	}
}

func TestPutObjectMissingBucketSourceReturnsS3InvalidRequest(t *testing.T) {
	bucketRepo, sourceRepo, fileRepo, _, factory := newMissingSourceUploadHandlerFixture(t)
	bucket := createMissingSourceUploadBucket(t, context.Background(), bucketRepo, primitive.NewObjectID())

	router := gin.New()
	router.PUT("/api/s3/:bucket/*object", setS3AccessKey(bucket.AccessKey), PutObjectWithFactory(bucketRepo, sourceRepo, fileRepo, 4, factory.Factory))
	req := httptest.NewRequest(http.MethodPut, "/api/s3/"+bucket.Key+"/object.txt", strings.NewReader("payload"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%q", w.Code, w.Body.String())
	}
	assertS3InvalidRequest(t, w.Body.String())
	if factory.calls != 0 {
		t.Fatalf("expected no source client factory calls, got %d", factory.calls)
	}
}

func TestUploadPartMissingBucketSourceReturnsS3InvalidRequest(t *testing.T) {
	bucketRepo, sourceRepo, fileRepo, mpRepo, factory := newMissingSourceUploadHandlerFixture(t)
	ctx := context.Background()
	bucket := createMissingSourceUploadBucket(t, ctx, bucketRepo, primitive.NewObjectID())
	uploadID := primitive.NewObjectID().Hex()
	if _, err := mpRepo.Create(ctx, repository.MultipartUpload{
		BucketID:  bucket.ID,
		ObjectKey: "object.txt",
		UploadID:  uploadID,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.PUT("/api/s3/:bucket/*object", setS3AccessKey(bucket.AccessKey), PutObjectOrPartWithFactory(bucketRepo, sourceRepo, fileRepo, mpRepo, 4, factory.Factory))
	req := httptest.NewRequest(http.MethodPut, "/api/s3/"+bucket.Key+"/object.txt?uploadId="+uploadID+"&partNumber=1", strings.NewReader("payload"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%q", w.Code, w.Body.String())
	}
	assertS3InvalidRequest(t, w.Body.String())
	if factory.calls != 0 {
		t.Fatalf("expected no source client factory calls, got %d", factory.calls)
	}
	stored, err := mpRepo.GetByUploadID(ctx, uploadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Parts) != 0 {
		t.Fatalf("expected no stored upload parts, got %+v", stored.Parts)
	}
}

func newMissingSourceUploadHandlerFixture(t *testing.T) (*repository.BucketRepository, *repository.SourceRepository, *repository.FileRepository, *repository.MultipartUploadRepository, *missingSourceUploadFactory) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	testDB := mongoConn.Client.Database("sfree_missing_source_upload_handlers_" + primitive.NewObjectID().Hex())
	t.Cleanup(func() {
		_ = testDB.Drop(context.Background())
		_ = mongoConn.Close(context.Background())
	})
	bucketRepo, err := repository.NewBucketRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	sourceRepo, err := repository.NewSourceRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	fileRepo, err := repository.NewFileRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	mpRepo, err := repository.NewMultipartUploadRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	return bucketRepo, sourceRepo, fileRepo, mpRepo, &missingSourceUploadFactory{}
}

func createMissingSourceUploadBucket(t *testing.T, ctx context.Context, repo *repository.BucketRepository, ownerID primitive.ObjectID) *repository.Bucket {
	t.Helper()
	suffix := primitive.NewObjectID().Hex()
	bucket, err := repo.Create(ctx, repository.Bucket{
		UserID:          ownerID,
		Key:             "missing-source-" + suffix,
		AccessKey:       "access-" + suffix,
		AccessSecretEnc: "secret-" + suffix,
		SourceIDs:       []primitive.ObjectID{primitive.NewObjectID()},
		CreatedAt:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return bucket
}

func setS3AccessKey(accessKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("accessKey", accessKey)
		c.Next()
	}
}

func assertS3InvalidRequest(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, "<Code>InvalidRequest</Code>") {
		t.Fatalf("expected InvalidRequest body, got %q", body)
	}
	if strings.Contains(body, "<Code>InternalError</Code>") {
		t.Fatalf("expected non-InternalError body, got %q", body)
	}
}
