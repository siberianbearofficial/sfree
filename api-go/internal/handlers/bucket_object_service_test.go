package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeBucketObjectMutationService struct {
	putObject        func(context.Context, *repository.Bucket, string, io.Reader, int) (manager.PutObjectResult, error)
	deleteObjectByID func(context.Context, primitive.ObjectID, primitive.ObjectID) (manager.DeleteObjectResult, error)
}

func (f *fakeBucketObjectMutationService) PutObject(ctx context.Context, bucket *repository.Bucket, name string, body io.Reader, chunkSize int) (manager.PutObjectResult, error) {
	return f.putObject(ctx, bucket, name, body, chunkSize)
}

func (f *fakeBucketObjectMutationService) DeleteObjectByID(ctx context.Context, bucketID primitive.ObjectID, fileID primitive.ObjectID) (manager.DeleteObjectResult, error) {
	return f.deleteObjectByID(ctx, bucketID, fileID)
}

func stubBucketFileMutation(t *testing.T, bucket *repository.Bucket, svc bucketObjectMutationService) {
	t.Helper()
	originalNewObjectService := newBucketObjectMutationService
	originalRequireAccess := requireBucketFileAccess
	t.Cleanup(func() {
		newBucketObjectMutationService = originalNewObjectService
		requireBucketFileAccess = originalRequireAccess
	})
	newBucketObjectMutationService = func(*repository.SourceRepository, *repository.FileRepository) bucketObjectMutationService {
		return svc
	}
	requireBucketFileAccess = func(c *gin.Context, _ *repository.BucketRepository, _ *repository.BucketGrantRepository, role repository.BucketRole) *bucketAccess {
		if role != repository.RoleEditor {
			t.Fatalf("expected editor access check, got %s", role)
		}
		return &bucketAccess{Bucket: bucket, Role: repository.RoleOwner}
	}
}

func multipartFileBody(t *testing.T, fieldName, fileName, content string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func TestUploadFileRoutesOverwriteThroughObjectService(t *testing.T) {
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	createdAt := time.Date(2026, 4, 21, 13, 0, 0, 0, time.UTC)
	bucket := &repository.Bucket{ID: bucketID}
	var called bool
	svc := &fakeBucketObjectMutationService{
		putObject: func(_ context.Context, gotBucket *repository.Bucket, name string, body io.Reader, chunkSize int) (manager.PutObjectResult, error) {
			called = true
			if gotBucket.ID != bucketID {
				t.Fatalf("expected bucket %s, got %s", bucketID.Hex(), gotBucket.ID.Hex())
			}
			if name != "object.txt" {
				t.Fatalf("expected object.txt, got %q", name)
			}
			if chunkSize != 16 {
				t.Fatalf("expected chunk size 16, got %d", chunkSize)
			}
			data, err := io.ReadAll(body)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "replacement" {
				t.Fatalf("unexpected body %q", string(data))
			}
			return manager.PutObjectResult{
				File:       repository.File{ID: fileID, BucketID: bucketID, Name: name, CreatedAt: createdAt},
				CleanupErr: errors.New("old chunk cleanup failed"),
			}, nil
		},
	}
	stubBucketFileMutation(t, bucket, svc)

	body, contentType := multipartFileBody(t, "file", "object.txt", "replacement")
	r := gin.New()
	r.POST("/buckets/:id/upload", UploadFile(&repository.BucketRepository{}, &repository.SourceRepository{}, &repository.FileRepository{}, nil, 16))
	req := httptest.NewRequest(http.MethodPost, "/buckets/"+bucketID.Hex()+"/upload", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !called {
		t.Fatal("expected object service PutObject call")
	}
	var resp uploadFileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != fileID.Hex() || resp.Name != "object.txt" || !resp.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestDeleteFileRoutesCleanupFailureFromObjectService(t *testing.T) {
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	bucket := &repository.Bucket{ID: bucketID}
	var called bool
	svc := &fakeBucketObjectMutationService{
		deleteObjectByID: func(_ context.Context, gotBucketID primitive.ObjectID, gotFileID primitive.ObjectID) (manager.DeleteObjectResult, error) {
			called = true
			if gotBucketID != bucketID {
				t.Fatalf("expected bucket %s, got %s", bucketID.Hex(), gotBucketID.Hex())
			}
			if gotFileID != fileID {
				t.Fatalf("expected file %s, got %s", fileID.Hex(), gotFileID.Hex())
			}
			return manager.DeleteObjectResult{Deleted: true, CleanupErr: errors.New("chunk cleanup failed")}, nil
		},
	}
	stubBucketFileMutation(t, bucket, svc)

	r := gin.New()
	r.DELETE("/buckets/:id/files/:file_id", DeleteFile(&repository.BucketRepository{}, &repository.SourceRepository{}, &repository.FileRepository{}, nil))
	req := httptest.NewRequest(http.MethodDelete, "/buckets/"+bucketID.Hex()+"/files/"+fileID.Hex(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !called {
		t.Fatal("expected object service DeleteObjectByID call")
	}
}
