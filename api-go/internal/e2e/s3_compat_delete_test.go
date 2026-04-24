//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestS3CompatDeleteObjects(t *testing.T) {
	env, ok := loadS3E2EEnv()
	if !ok {
		t.Skip("E2E_S3_ENDPOINT not set; skipping S3 E2E tests")
	}

	ts, _ := newTestServer(t)
	ensureMinIOBucket(t, env)

	suffix := uniqueSuffix()
	username, password := createTestUser(t, ts, suffix)
	sourceID := createS3Source(t, ts, username, password, "src-"+suffix, env)
	bucket := createBucket(t, ts, username, password, "bkt-"+suffix, sourceID)

	t.Cleanup(func() {
		apiDelete(t, ts, "/api/v1/buckets/"+bucket.ID, username, password)
		apiDelete(t, ts, "/api/v1/sources/"+sourceID, username, password)
	})

	objectKeys := []string{
		"multi-delete/a-" + suffix + ".txt",
		"multi-delete/b-" + suffix + ".txt",
		"multi-delete/c-" + suffix + ".txt",
	}
	s3URL := func(key string) string {
		return fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, key)
	}
	for _, key := range objectKeys {
		status, body := s3Do(t, http.MethodPut, s3URL(key), bucket.AccessKey, bucket.AccessSecret, env.Region, []byte("content "+key))
		if status != http.StatusOK {
			t.Fatalf("PUT %s: expected 200, got %d: %s", key, status, body)
		}
	}
	t.Cleanup(func() {
		for _, key := range objectKeys {
			s3Do(t, http.MethodDelete, s3URL(key), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
		}
	})

	type deleteResultXML struct {
		Deleted []struct {
			Key string `xml:"Key"`
		} `xml:"Deleted"`
		Errors []struct {
			Key  string `xml:"Key"`
			Code string `xml:"Code"`
		} `xml:"Error"`
	}
	deleteURL := ts.URL + "/api/s3/" + bucket.Key + "?delete"
	deleteBody := []byte(fmt.Sprintf(`<Delete>
		<Object><Key>%s</Key></Object>
		<Object><Key>%s</Key></Object>
		<Object><Key>%s</Key></Object>
	</Delete>`, objectKeys[0], objectKeys[1], "multi-delete/missing-"+suffix+".txt"))
	status, body := s3Do(t, http.MethodPost, deleteURL, bucket.AccessKey, bucket.AccessSecret, env.Region, deleteBody)
	if status != http.StatusOK {
		t.Fatalf("DeleteObjects: expected 200, got %d: %s", status, body)
	}
	var result deleteResultXML
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode DeleteObjects response: %v body=%s", err, body)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("DeleteObjects expected no errors, got %+v", result.Errors)
	}
	if len(result.Deleted) != 3 {
		t.Fatalf("DeleteObjects expected 3 Deleted entries including missing key, got %+v", result.Deleted)
	}

	listURL := ts.URL + "/api/s3/" + bucket.Key + "?list-type=2&prefix=multi-delete/"
	status, body = s3Do(t, http.MethodGet, listURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("LIST after DeleteObjects: expected 200, got %d: %s", status, body)
	}
	if strings.Contains(string(body), objectKeys[0]) || strings.Contains(string(body), objectKeys[1]) {
		t.Fatalf("LIST after DeleteObjects included deleted keys: %s", body)
	}
	if !strings.Contains(string(body), objectKeys[2]) {
		t.Fatalf("LIST after DeleteObjects missing survivor key %q: %s", objectKeys[2], body)
	}

	quietBody := []byte(fmt.Sprintf(`<Delete><Quiet>true</Quiet><Object><Key>%s</Key></Object></Delete>`, objectKeys[2]))
	status, body = s3Do(t, http.MethodPost, deleteURL, bucket.AccessKey, bucket.AccessSecret, env.Region, quietBody)
	if status != http.StatusOK {
		t.Fatalf("DeleteObjects quiet: expected 200, got %d: %s", status, body)
	}
	result = deleteResultXML{}
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode quiet DeleteObjects response: %v body=%s", err, body)
	}
	if len(result.Deleted) != 0 || len(result.Errors) != 0 {
		t.Fatalf("quiet DeleteObjects expected no Deleted or Error entries, got %+v", result)
	}

	var tooMany strings.Builder
	tooMany.WriteString("<Delete>")
	for i := 0; i < 1001; i++ {
		fmt.Fprintf(&tooMany, "<Object><Key>too-many-%d</Key></Object>", i)
	}
	tooMany.WriteString("</Delete>")
	status, body = s3Do(t, http.MethodPost, deleteURL, bucket.AccessKey, bucket.AccessSecret, env.Region, []byte(tooMany.String()))
	if status != http.StatusBadRequest {
		t.Fatalf("DeleteObjects too many keys: expected 400, got %d: %s", status, body)
	}
	assertS3Error(t, body, "InvalidRequest")

	status, body = s3Do(t, http.MethodPost, deleteURL, bucket.AccessKey, bucket.AccessSecret, env.Region, []byte("<Delete><Object><Key>broken"))
	if status != http.StatusBadRequest {
		t.Fatalf("DeleteObjects malformed XML: expected 400, got %d: %s", status, body)
	}
	assertS3Error(t, body, "MalformedXML")
}

func TestS3CompatDeleteBucketCleansObjectMultipartAndChunks(t *testing.T) {
	env, ok := loadS3E2EEnv()
	if !ok {
		t.Skip("E2E_S3_ENDPOINT not set; skipping S3 E2E tests")
	}

	ts, cfg := newTestServer(t)
	ensureMinIOBucket(t, env)

	ctx := context.Background()
	mongoConn, err := db.Connect(ctx, cfg.Mongo)
	if err != nil {
		t.Fatalf("connect mongo for assertions: %v", err)
	}
	t.Cleanup(func() {
		_ = mongoConn.Close(ctx)
	})
	fileRepo, err := repository.NewFileRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatalf("create file repository: %v", err)
	}
	mpRepo, err := repository.NewMultipartUploadRepository(context.Background(), mongoConn.DB)
	if err != nil {
		t.Fatalf("create multipart repository: %v", err)
	}

	suffix := uniqueSuffix()
	username, password := createTestUser(t, ts, suffix)
	sourceID := createS3Source(t, ts, username, password, "src-"+suffix, env)
	bucket := createBucket(t, ts, username, password, "bkt-"+suffix, sourceID)
	t.Cleanup(func() {
		apiDelete(t, ts, "/api/v1/buckets/"+bucket.ID, username, password)
		apiDelete(t, ts, "/api/v1/sources/"+sourceID, username, password)
	})

	bucketID, err := primitive.ObjectIDFromHex(bucket.ID)
	if err != nil {
		t.Fatalf("parse bucket id: %v", err)
	}
	s3URL := func(key string, params url.Values) string {
		rawURL := fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, key)
		if len(params) > 0 {
			rawURL += "?" + params.Encode()
		}
		return rawURL
	}

	beforeSourceKeys := listS3SourceObjectKeys(t, env)

	objectKey := "bucket-delete-cleanup/object-" + suffix + ".txt"
	objectBody := []byte("completed object " + suffix)
	status, body := s3Do(t, http.MethodPut, s3URL(objectKey, nil), bucket.AccessKey, bucket.AccessSecret, env.Region, objectBody)
	if status != http.StatusOK {
		t.Fatalf("PUT completed object: expected 200, got %d: %s", status, body)
	}
	status, body = s3Do(t, http.MethodGet, s3URL(objectKey, nil), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("GET completed object before bucket delete: expected 200, got %d: %s", status, body)
	}
	if string(body) != string(objectBody) {
		t.Fatalf("GET completed object before bucket delete: got %q, want %q", body, objectBody)
	}

	activeMultipartKey := "bucket-delete-cleanup/active-multipart-" + suffix + ".txt"
	status, body = s3Do(t, http.MethodPost, s3URL(activeMultipartKey, url.Values{"uploads": {""}}), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("CreateMultipartUpload: expected 200, got %d: %s", status, body)
	}
	var createResult struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(body, &createResult); err != nil || createResult.UploadID == "" {
		t.Fatalf("CreateMultipartUpload decode: uploadId=%q err=%v body=%s", createResult.UploadID, err, body)
	}
	partURL := s3URL(activeMultipartKey, url.Values{
		"partNumber": {"1"},
		"uploadId":   {createResult.UploadID},
	})
	status, body = s3Do(t, http.MethodPut, partURL, bucket.AccessKey, bucket.AccessSecret, env.Region, []byte("active multipart part "+suffix))
	if status != http.StatusOK {
		t.Fatalf("UploadPart active multipart: expected 200, got %d: %s", status, body)
	}

	listUploadsURL := fmt.Sprintf("%s/api/s3/%s?uploads", ts.URL, bucket.Key)
	status, body = s3Do(t, http.MethodGet, listUploadsURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("ListMultipartUploads before bucket delete: expected 200, got %d: %s", status, body)
	}
	var uploadsResult struct {
		Uploads []struct {
			Key      string `xml:"Key"`
			UploadID string `xml:"UploadId"`
		} `xml:"Upload"`
	}
	if err := xml.Unmarshal(body, &uploadsResult); err != nil {
		t.Fatalf("ListMultipartUploads decode: %v body=%s", err, body)
	}
	foundUpload := false
	for _, upload := range uploadsResult.Uploads {
		if upload.Key == activeMultipartKey && upload.UploadID == createResult.UploadID {
			foundUpload = true
			break
		}
	}
	if !foundUpload {
		t.Fatalf("ListMultipartUploads missing active upload key=%q uploadId=%q: %+v", activeMultipartKey, createResult.UploadID, uploadsResult.Uploads)
	}

	if _, err := fileRepo.GetByName(ctx, bucketID, objectKey); err != nil {
		t.Fatalf("expected completed object metadata before bucket delete: %v", err)
	}
	if _, err := mpRepo.GetByUploadID(ctx, createResult.UploadID); err != nil {
		t.Fatalf("expected active multipart metadata before bucket delete: %v", err)
	}

	afterCreateSourceKeys := listS3SourceObjectKeys(t, env)
	createdSourceKeys := sourceKeyDifference(afterCreateSourceKeys, beforeSourceKeys)
	if len(createdSourceKeys) == 0 {
		t.Fatal("expected completed object or multipart upload to create source chunks")
	}

	resp := apiDelete(t, ts, "/api/v1/buckets/"+bucket.ID, username, password)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete bucket: expected 200/204, got %d", resp.StatusCode)
	}

	status, body = s3Do(t, http.MethodGet, s3URL(objectKey, nil), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status < http.StatusBadRequest {
		t.Fatalf("GET completed object after bucket delete: expected non-success, got %d: %s", status, body)
	}
	listObjectsURL := fmt.Sprintf("%s/api/s3/%s?list-type=2&prefix=%s", ts.URL, bucket.Key, url.QueryEscape("bucket-delete-cleanup/"))
	status, body = s3Do(t, http.MethodGet, listObjectsURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status < http.StatusBadRequest {
		t.Fatalf("LIST after bucket delete: expected non-success, got %d: %s", status, body)
	}
	status, body = s3Do(t, http.MethodGet, s3URL(activeMultipartKey, url.Values{"uploadId": {createResult.UploadID}}), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status < http.StatusBadRequest {
		t.Fatalf("ListParts after bucket delete: expected non-success, got %d: %s", status, body)
	}

	files, err := fileRepo.ListByBucket(ctx, bucketID)
	if err != nil {
		t.Fatalf("list file metadata after bucket delete: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected bucket file metadata cleanup, got %d files", len(files))
	}
	uploads, err := mpRepo.ListByBucket(ctx, bucketID)
	if err != nil {
		t.Fatalf("list multipart metadata after bucket delete: %v", err)
	}
	if len(uploads) != 0 {
		t.Fatalf("expected bucket multipart metadata cleanup, got %d uploads", len(uploads))
	}
	if _, err := mpRepo.GetByUploadID(ctx, createResult.UploadID); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected active multipart upload record cleanup, got %v", err)
	}

	afterDeleteSourceKeys := listS3SourceObjectKeys(t, env)
	for key := range createdSourceKeys {
		if _, ok := afterDeleteSourceKeys[key]; ok {
			t.Fatalf("expected bucket delete to remove unreferenced source chunk %q", key)
		}
	}
}

func sourceKeyDifference(after, before map[string]struct{}) map[string]struct{} {
	diff := make(map[string]struct{})
	for key := range after {
		if _, ok := before[key]; !ok {
			diff[key] = struct{}{}
		}
	}
	return diff
}
