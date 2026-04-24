package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestDeleteFileCleansShareLinksBeforeObjectDelete(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	events := []string{}

	router := gin.New()
	router.DELETE("/buckets/:id/files/:file_id",
		setUserID(userID.Hex()),
		func(c *gin.Context) {
			handleDeleteFile(
				c,
				fakeBucketDeleteStore{bucket: testDeleteBucket(bucketID, userID)},
				fakeDeleteFileReader{file: &repository.File{ID: fileID, BucketID: bucketID, CreatedAt: time.Now().UTC()}},
				&fakeShareLinkCleanup{events: &events},
				&fakeObjectFileDelete{events: &events},
				nil,
			)
		},
	)

	req, _ := http.NewRequest(http.MethodDelete, "/buckets/"+bucketID.Hex()+"/files/"+fileID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !reflect.DeepEqual(events, []string{"share_file", "object_file"}) {
		t.Fatalf("unexpected event order: %v", events)
	}
}

func TestDeleteFileShareCleanupFailureReturns500AndSkipsObjectDelete(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	events := []string{}

	router := gin.New()
	router.DELETE("/buckets/:id/files/:file_id",
		setUserID(userID.Hex()),
		func(c *gin.Context) {
			handleDeleteFile(
				c,
				fakeBucketDeleteStore{bucket: testDeleteBucket(bucketID, userID)},
				fakeDeleteFileReader{file: &repository.File{ID: fileID, BucketID: bucketID, CreatedAt: time.Now().UTC()}},
				&fakeShareLinkCleanup{events: &events, err: errors.New("share cleanup failed")},
				&fakeObjectFileDelete{events: &events},
				nil,
			)
		},
	)

	req, _ := http.NewRequest(http.MethodDelete, "/buckets/"+bucketID.Hex()+"/files/"+fileID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !reflect.DeepEqual(events, []string{"share_file"}) {
		t.Fatalf("unexpected event order: %v", events)
	}
}

func TestDeleteBucketCleansShareLinksAndGrantsBeforeBucketDelete(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	events := []string{}

	router := gin.New()
	router.DELETE("/buckets/:id",
		setUserID(userID.Hex()),
		func(c *gin.Context) {
			handleDeleteBucket(
				c,
				&fakeBucketDeleteStore{bucket: testDeleteBucket(bucketID, userID), events: &events},
				&fakeShareLinkCleanup{events: &events},
				&fakeBucketContentsDelete{events: &events},
				nil,
				&fakeBucketGrantCleanup{events: &events},
			)
		},
	)

	req, _ := http.NewRequest(http.MethodDelete, "/buckets/"+bucketID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !reflect.DeepEqual(events, []string{"share_bucket", "bucket_contents", "bucket_grants", "bucket_delete"}) {
		t.Fatalf("unexpected event order: %v", events)
	}
}

func TestDeleteBucketShareCleanupFailureReturns500AndSkipsDeletes(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	events := []string{}

	router := gin.New()
	router.DELETE("/buckets/:id",
		setUserID(userID.Hex()),
		func(c *gin.Context) {
			handleDeleteBucket(
				c,
				&fakeBucketDeleteStore{bucket: testDeleteBucket(bucketID, userID), events: &events},
				&fakeShareLinkCleanup{events: &events, err: errors.New("share cleanup failed")},
				&fakeBucketContentsDelete{events: &events},
				nil,
				nil,
			)
		},
	)

	req, _ := http.NewRequest(http.MethodDelete, "/buckets/"+bucketID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !reflect.DeepEqual(events, []string{"share_bucket"}) {
		t.Fatalf("unexpected event order: %v", events)
	}
}

func TestDeleteBucketGrantCleanupFailureReturns500AndSkipsBucketDelete(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	events := []string{}

	router := gin.New()
	router.DELETE("/buckets/:id",
		setUserID(userID.Hex()),
		func(c *gin.Context) {
			handleDeleteBucket(
				c,
				&fakeBucketDeleteStore{bucket: testDeleteBucket(bucketID, userID), events: &events},
				&fakeShareLinkCleanup{events: &events},
				&fakeBucketContentsDelete{events: &events},
				nil,
				&fakeBucketGrantCleanup{events: &events, err: errors.New("grant cleanup failed")},
			)
		},
	)

	req, _ := http.NewRequest(http.MethodDelete, "/buckets/"+bucketID.Hex(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !reflect.DeepEqual(events, []string{"share_bucket", "bucket_contents", "bucket_grants"}) {
		t.Fatalf("unexpected event order: %v", events)
	}
}

type fakeBucketDeleteStore struct {
	bucket *repository.Bucket
	err    error
	events *[]string
}

func (f fakeBucketDeleteStore) GetByID(_ context.Context, _ primitive.ObjectID) (*repository.Bucket, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.bucket, nil
}

func (f *fakeBucketDeleteStore) Delete(_ context.Context, _ primitive.ObjectID, _ primitive.ObjectID) error {
	if f.events != nil {
		*f.events = append(*f.events, "bucket_delete")
	}
	return f.err
}

type fakeDeleteFileReader struct {
	file *repository.File
	err  error
}

func (f fakeDeleteFileReader) GetByID(_ context.Context, _ primitive.ObjectID) (*repository.File, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.file, nil
}

type fakeShareLinkCleanup struct {
	events *[]string
	err    error
}

func (f *fakeShareLinkCleanup) DeleteByFile(_ context.Context, _ primitive.ObjectID) error {
	if f.events != nil {
		*f.events = append(*f.events, "share_file")
	}
	return f.err
}

func (f *fakeShareLinkCleanup) DeleteByBucket(_ context.Context, _ primitive.ObjectID) error {
	if f.events != nil {
		*f.events = append(*f.events, "share_bucket")
	}
	return f.err
}

type fakeBucketGrantCleanup struct {
	events *[]string
	err    error
}

func (f *fakeBucketGrantCleanup) DeleteByBucket(_ context.Context, _ primitive.ObjectID) error {
	if f.events != nil {
		*f.events = append(*f.events, "bucket_grants")
	}
	return f.err
}

type fakeObjectFileDelete struct {
	events *[]string
	err    error
	result manager.DeleteObjectResult
}

func (f *fakeObjectFileDelete) DeleteFile(_ context.Context, _ primitive.ObjectID, _ primitive.ObjectID) (manager.DeleteObjectResult, error) {
	if f.events != nil {
		*f.events = append(*f.events, "object_file")
	}
	return f.result, f.err
}

type fakeBucketContentsDelete struct {
	events *[]string
	err    error
	result manager.DeleteBucketContentsResult
}

func (f *fakeBucketContentsDelete) DeleteBucketContents(_ context.Context, _ primitive.ObjectID) (manager.DeleteBucketContentsResult, error) {
	if f.events != nil {
		*f.events = append(*f.events, "bucket_contents")
	}
	return f.result, f.err
}

func testDeleteBucket(bucketID, userID primitive.ObjectID) *repository.Bucket {
	return &repository.Bucket{
		ID:        bucketID,
		UserID:    userID,
		Key:       "bucket",
		CreatedAt: time.Now().UTC(),
	}
}
