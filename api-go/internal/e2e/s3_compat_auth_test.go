//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestS3CompatAccessKeyAuth(t *testing.T) {
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

	listURL := ts.URL + "/api/s3/" + bucket.Key

	// Wrong access key — expect 401
	status, _ := s3Do(t, http.MethodGet, listURL, "wrong-key", bucket.AccessSecret, env.Region, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("wrong access key: expected 401, got %d", status)
	}

	// Wrong secret (invalid signature) — expect 401
	status, _ = s3Do(t, http.MethodGet, listURL, bucket.AccessKey, "wrong-secret", env.Region, nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("wrong secret: expected 401, got %d", status)
	}

	// Unsigned request — expect 401
	req, _ := http.NewRequest(http.MethodGet, listURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unsigned request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unsigned request: expected 401, got %d", resp.StatusCode)
	}
}

func TestS3CompatWrongBucketCredentialsDoNotExposeObjectBytes(t *testing.T) {
	env, ok := loadS3E2EEnv()
	if !ok {
		t.Skip("E2E_S3_ENDPOINT not set; skipping S3 E2E tests")
	}

	ts, _ := newTestServer(t)
	ensureMinIOBucket(t, env)

	suffix := uniqueSuffix()
	username, password := createTestUser(t, ts, suffix)
	sourceID := createS3Source(t, ts, username, password, "src-"+suffix, env)
	ownerBucket := createBucket(t, ts, username, password, "owner-bkt-"+suffix, sourceID)
	otherBucket := createBucket(t, ts, username, password, "other-bkt-"+suffix, sourceID)

	t.Cleanup(func() {
		apiDelete(t, ts, "/api/v1/buckets/"+otherBucket.ID, username, password)
		apiDelete(t, ts, "/api/v1/buckets/"+ownerBucket.ID, username, password)
		apiDelete(t, ts, "/api/v1/sources/"+sourceID, username, password)
	})

	objectKey := "private-" + suffix + ".txt"
	objectContent := []byte("private object bytes " + suffix)
	s3URL := fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, ownerBucket.Key, objectKey)

	status, body := s3Do(t, http.MethodPut, s3URL, ownerBucket.AccessKey, ownerBucket.AccessSecret, env.Region, objectContent)
	if status != http.StatusOK {
		t.Fatalf("PUT private object: expected 200, got %d: %s", status, body)
	}
	t.Cleanup(func() {
		s3Do(t, http.MethodDelete, s3URL, ownerBucket.AccessKey, ownerBucket.AccessSecret, env.Region, nil)
	})

	status, body = s3Do(t, http.MethodGet, s3URL, otherBucket.AccessKey, otherBucket.AccessSecret, env.Region, nil)
	if status == http.StatusOK {
		t.Fatalf("GET with wrong bucket credentials: expected non-200, got 200: %s", body)
	}
	for i := 0; i <= len(objectContent)-8; i++ {
		if fragment := objectContent[i : i+8]; bytes.Contains(body, fragment) {
			t.Fatalf("GET with wrong bucket credentials exposed object byte fragment %q in response %q", fragment, body)
		}
	}
}

func TestS3CompatPresignedGetObject(t *testing.T) {
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

	objectKey := "presign-get-" + suffix + ".txt"
	objectContent := []byte("Presigned GET content!")
	s3URL := fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, objectKey)

	// Upload via signed request first.
	status, body := s3Do(t, http.MethodPut, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, objectContent)
	if status != http.StatusOK {
		t.Fatalf("PUT object: expected 200, got %d: %s", status, body)
	}

	// Generate presigned GET URL and download without credentials.
	presignedURL, err := presignS3URL(s3URL, "GET", bucket.AccessKey, bucket.AccessSecret, env.Region, 3600)
	if err != nil {
		t.Fatalf("presign URL: %v", err)
	}

	resp, err := http.Get(presignedURL)
	if err != nil {
		t.Fatalf("presigned GET: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("presigned GET: expected 200, got %d: %s", resp.StatusCode, respBody)
	}
	if !bytes.Equal(respBody, objectContent) {
		t.Fatalf("presigned GET: content mismatch; want %q got %q", objectContent, respBody)
	}

	// Verify ETag and Last-Modified headers are present.
	if resp.Header.Get("ETag") == "" {
		t.Fatal("presigned GET: missing ETag header")
	}
	if resp.Header.Get("Last-Modified") == "" {
		t.Fatal("presigned GET: missing Last-Modified header")
	}

	// Cleanup
	s3Do(t, http.MethodDelete, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
}

// TestS3CompatPresignedPutObject tests uploading a file via presigned URL.
func TestS3CompatPresignedPutObject(t *testing.T) {
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

	objectKey := "presign-put-" + suffix + ".txt"
	objectContent := []byte("Presigned PUT content!")
	s3URL := fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, objectKey)

	// Generate presigned PUT URL.
	presignedURL, err := presignS3URL(s3URL, "PUT", bucket.AccessKey, bucket.AccessSecret, env.Region, 3600)
	if err != nil {
		t.Fatalf("presign URL: %v", err)
	}

	// Upload via presigned URL without credentials in headers.
	req, err := http.NewRequest(http.MethodPut, presignedURL, bytes.NewReader(objectContent))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.ContentLength = int64(len(objectContent))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("presigned PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("presigned PUT: expected 200, got %d", resp.StatusCode)
	}

	// Verify content via signed GET.
	status, body := s3Do(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("GET after presigned PUT: expected 200, got %d: %s", status, body)
	}
	if !bytes.Equal(body, objectContent) {
		t.Fatalf("GET after presigned PUT: content mismatch; want %q got %q", objectContent, body)
	}

	// Cleanup
	s3Do(t, http.MethodDelete, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
}

func TestS3CompatPresignedURLExpired(t *testing.T) {
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

	objectKey := "presign-expired-" + suffix + ".txt"
	s3URL := fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, objectKey)

	// Generate presigned URL with 1-second expiry.
	presignedURL, err := presignS3URL(s3URL, "GET", bucket.AccessKey, bucket.AccessSecret, env.Region, 1)
	if err != nil {
		t.Fatalf("presign URL: %v", err)
	}

	// Wait for expiry.
	time.Sleep(2 * time.Second)

	resp, err := http.Get(presignedURL)
	if err != nil {
		t.Fatalf("expired presigned GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired presigned GET: expected 401, got %d", resp.StatusCode)
	}
}
