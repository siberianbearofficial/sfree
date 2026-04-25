package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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
	"go.mongodb.org/mongo-driver/mongo"
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

func TestBatchDeleteFilesDeletesUniqueRequestedFiles(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	fileAID := primitive.NewObjectID()
	fileBID := primitive.NewObjectID()
	shareDeleted := []primitive.ObjectID{}
	objectDeleted := []primitive.ObjectID{}

	router := gin.New()
	router.POST("/buckets/:id/files/batch-delete",
		setUserID(userID.Hex()),
		func(c *gin.Context) {
			handleBatchDeleteFiles(
				c,
				fakeBucketDeleteStore{bucket: testDeleteBucket(bucketID, userID)},
				fakeBatchDeleteFileReader{files: map[primitive.ObjectID]*repository.File{
					fileAID: {ID: fileAID, BucketID: bucketID, Name: "alpha.txt", CreatedAt: time.Now().UTC()},
					fileBID: {ID: fileBID, BucketID: bucketID, Name: "beta.txt", CreatedAt: time.Now().UTC()},
				}},
				&fakeBatchShareLinkCleanup{deletedIDs: &shareDeleted},
				&fakeBatchObjectFileDelete{deletedIDs: &objectDeleted},
				nil,
			)
		},
	)

	body, _ := json.Marshal(batchDeleteFilesRequest{
		FileIDs: []string{fileAID.Hex(), fileAID.Hex(), fileBID.Hex()},
	})
	req, _ := http.NewRequest(http.MethodPost, "/buckets/"+bucketID.Hex()+"/files/batch-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp batchDeleteFilesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.Deleted) != 2 || len(resp.Failed) != 0 || len(resp.Warnings) != 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !reflect.DeepEqual(shareDeleted, []primitive.ObjectID{fileAID, fileBID}) {
		t.Fatalf("unexpected share cleanup ids: %v", shareDeleted)
	}
	if !reflect.DeepEqual(objectDeleted, []primitive.ObjectID{fileAID, fileBID}) {
		t.Fatalf("unexpected object delete ids: %v", objectDeleted)
	}
}

func TestBatchDeleteFilesReportsMissingFilesWithoutStopping(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	missingID := primitive.NewObjectID()

	router := gin.New()
	router.POST("/buckets/:id/files/batch-delete",
		setUserID(userID.Hex()),
		func(c *gin.Context) {
			handleBatchDeleteFiles(
				c,
				fakeBucketDeleteStore{bucket: testDeleteBucket(bucketID, userID)},
				fakeBatchDeleteFileReader{files: map[primitive.ObjectID]*repository.File{
					fileID: {ID: fileID, BucketID: bucketID, Name: "keep-going.txt", CreatedAt: time.Now().UTC()},
				}},
				&fakeBatchShareLinkCleanup{},
				&fakeBatchObjectFileDelete{},
				nil,
			)
		},
	)

	body, _ := json.Marshal(batchDeleteFilesRequest{
		FileIDs: []string{fileID.Hex(), missingID.Hex()},
	})
	req, _ := http.NewRequest(http.MethodPost, "/buckets/"+bucketID.Hex()+"/files/batch-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp batchDeleteFilesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.Deleted) != 1 || resp.Deleted[0].ID != fileID.Hex() {
		t.Fatalf("unexpected deleted response: %+v", resp.Deleted)
	}
	if len(resp.Failed) != 1 || resp.Failed[0].ID != missingID.Hex() || resp.Failed[0].Error != "File not found" {
		t.Fatalf("unexpected failed response: %+v", resp.Failed)
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

type fakeBatchDeleteFileReader struct {
	files map[primitive.ObjectID]*repository.File
	err   error
}

func (f fakeBatchDeleteFileReader) GetByID(_ context.Context, id primitive.ObjectID) (*repository.File, error) {
	if f.err != nil {
		return nil, f.err
	}
	file, ok := f.files[id]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	return file, nil
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

type fakeBatchShareLinkCleanup struct {
	deletedIDs *[]primitive.ObjectID
	err        error
}

func (f *fakeBatchShareLinkCleanup) DeleteByFile(_ context.Context, fileID primitive.ObjectID) error {
	if f.deletedIDs != nil {
		*f.deletedIDs = append(*f.deletedIDs, fileID)
	}
	return f.err
}

type fakeBatchObjectFileDelete struct {
	deletedIDs *[]primitive.ObjectID
	err        error
	result     manager.DeleteObjectResult
}

func (f *fakeBatchObjectFileDelete) DeleteFile(_ context.Context, _ primitive.ObjectID, fileID primitive.ObjectID) (manager.DeleteObjectResult, error) {
	if f.deletedIDs != nil {
		*f.deletedIDs = append(*f.deletedIDs, fileID)
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
