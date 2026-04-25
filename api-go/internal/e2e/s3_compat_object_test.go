//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// against the SFree S3-compatible API.
func TestS3CompatObjectLifecycle(t *testing.T) {
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

	objectKey := "hello-" + suffix + ".txt"
	objectContent := []byte("Hello, SFree S3 compatibility!")
	s3URL := func(key string) string {
		return fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, key)
	}
	listURL := ts.URL + "/api/s3/" + bucket.Key

	// PUT object
	status, body := s3Do(t, http.MethodPut, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, objectContent)
	if status != http.StatusOK {
		t.Fatalf("PUT object: expected 200, got %d: %s", status, body)
	}

	// LIST objects — verify the new object appears
	status, body = s3Do(t, http.MethodGet, listURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("LIST objects: expected 200, got %d: %s", status, body)
	}
	if !strings.Contains(string(body), objectKey) {
		t.Fatalf("LIST objects: expected key %q in response, got: %s", objectKey, body)
	}

	// GET object — verify content
	status, body = s3Do(t, http.MethodGet, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("GET object: expected 200, got %d: %s", status, body)
	}
	if !bytes.Equal(body, objectContent) {
		t.Fatalf("GET object: content mismatch; want %q got %q", objectContent, body)
	}

	// PUT same key again (overwrite) — ETag must differ from the original
	updatedContent := []byte("Updated content for overwrite test.")
	status, body = s3Do(t, http.MethodPut, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, updatedContent)
	if status != http.StatusOK {
		t.Fatalf("PUT overwrite: expected 200, got %d: %s", status, body)
	}

	// GET after overwrite — content must be updated
	status, body = s3Do(t, http.MethodGet, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("GET after overwrite: expected 200, got %d: %s", status, body)
	}
	if !bytes.Equal(body, updatedContent) {
		t.Fatalf("GET after overwrite: content mismatch; want %q got %q", updatedContent, body)
	}

	// DELETE object
	status, body = s3Do(t, http.MethodDelete, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusNoContent {
		t.Fatalf("DELETE object: expected 204, got %d: %s", status, body)
	}

	// LIST after delete — object must not appear
	status, body = s3Do(t, http.MethodGet, listURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("LIST after delete: expected 200, got %d: %s", status, body)
	}
	if strings.Contains(string(body), objectKey) {
		t.Fatalf("LIST after delete: key %q should be absent, got: %s", objectKey, body)
	}

	// GET deleted object — must return 404
	status, _ = s3Do(t, http.MethodGet, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusNotFound {
		t.Fatalf("GET deleted object: expected 404, got %d", status)
	}
}

func TestS3CompatMissingObjectReturnsNoSuchKey(t *testing.T) {
	env, ok := loadS3E2EEnv()
	if !ok {
		t.Skip("E2E_S3_ENDPOINT not set; skipping S3 E2E tests")
	}

	suffix := uniqueSuffix()
	ts, bucket := setupS3CompatTest(t, env, suffix)
	s3URL := fmt.Sprintf("%s/api/s3/%s/missing-%s.txt", ts.URL, bucket.Key, suffix)

	status, body := s3Do(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusNotFound {
		t.Fatalf("GET missing object: expected 404, got %d: %s", status, body)
	}

	var errResp struct {
		Code string `xml:"Code"`
	}
	if err := xml.Unmarshal(body, &errResp); err != nil {
		t.Fatalf("decode missing object error: %v body=%s", err, body)
	}
	if errResp.Code != "NoSuchKey" {
		t.Fatalf("GET missing object: expected NoSuchKey, got %q", errResp.Code)
	}
}

func TestS3CompatCopyObject(t *testing.T) {
	env, ok := loadS3E2EEnv()
	if !ok {
		t.Skip("E2E_S3_ENDPOINT not set; skipping S3 E2E tests")
	}

	ts, _ := newTestServer(t)
	ensureMinIOBucket(t, env)

	suffix := uniqueSuffix()
	username, password := createTestUser(t, ts, suffix)
	sourceID := createS3Source(t, ts, username, password, "src-"+suffix, env)
	sourceBucket := createBucket(t, ts, username, password, "src-bkt-"+suffix, sourceID)
	destBucket := createBucket(t, ts, username, password, "dst-bkt-"+suffix, sourceID)

	t.Cleanup(func() {
		apiDelete(t, ts, "/api/v1/buckets/"+destBucket.ID, username, password)
		apiDelete(t, ts, "/api/v1/buckets/"+sourceBucket.ID, username, password)
		apiDelete(t, ts, "/api/v1/sources/"+sourceID, username, password)
	})

	sourceKey := "copy-source-" + suffix + ".txt"
	sameBucketKey := "copy-same-" + suffix + ".txt"
	crossBucketKey := "copy-cross-" + suffix + ".txt"
	objectContent := []byte("copy me without duplicating chunks")
	s3URL := func(bucket createBucketResult, key string) string {
		return fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, key)
	}

	status, body := s3Do(t, http.MethodPut, s3URL(sourceBucket, sourceKey), sourceBucket.AccessKey, sourceBucket.AccessSecret, env.Region, objectContent)
	if status != http.StatusOK {
		t.Fatalf("PUT source object: expected 200, got %d: %s", status, body)
	}

	copyHeaders := map[string]string{"x-amz-copy-source": "/" + sourceBucket.Key + "/" + sourceKey}
	status, _, body = s3DoWithHeaders(t, http.MethodPut, s3URL(sourceBucket, sameBucketKey), sourceBucket.AccessKey, sourceBucket.AccessSecret, env.Region, nil, copyHeaders)
	if status != http.StatusOK {
		t.Fatalf("CopyObject same bucket: expected 200, got %d: %s", status, body)
	}
	var result struct {
		ETag         string `xml:"ETag"`
		LastModified string `xml:"LastModified"`
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode CopyObject result: %v body=%s", err, body)
	}
	if result.ETag == "" || result.LastModified == "" {
		t.Fatalf("CopyObject result missing ETag or LastModified: %+v body=%s", result, body)
	}
	status, body = s3Do(t, http.MethodGet, s3URL(sourceBucket, sameBucketKey), sourceBucket.AccessKey, sourceBucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK || !bytes.Equal(body, objectContent) {
		t.Fatalf("GET same-bucket copy mismatch: status=%d body=%q", status, body)
	}

	status, _, body = s3DoWithHeaders(t, http.MethodPut, s3URL(destBucket, crossBucketKey), destBucket.AccessKey, destBucket.AccessSecret, env.Region, nil, copyHeaders)
	if status != http.StatusOK {
		t.Fatalf("CopyObject cross bucket: expected 200, got %d: %s", status, body)
	}
	status, body = s3Do(t, http.MethodGet, s3URL(destBucket, crossBucketKey), destBucket.AccessKey, destBucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK || !bytes.Equal(body, objectContent) {
		t.Fatalf("GET cross-bucket copy mismatch: status=%d body=%q", status, body)
	}

	replaceHeaders := map[string]string{
		"x-amz-copy-source":        "/" + sourceBucket.Key + "/" + sourceKey,
		"x-amz-metadata-directive": "REPLACE",
	}
	status, body = s3Do(t, http.MethodPut, s3URL(sourceBucket, "copy-replace-"+suffix+".txt"), sourceBucket.AccessKey, sourceBucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("PUT replace-control object: expected 200, got %d: %s", status, body)
	}
	status, _, body = s3DoWithHeaders(t, http.MethodPut, s3URL(sourceBucket, "copy-replace-"+suffix+".txt"), sourceBucket.AccessKey, sourceBucket.AccessSecret, env.Region, nil, replaceHeaders)
	if status != http.StatusNotImplemented {
		t.Fatalf("CopyObject REPLACE: expected 501, got %d: %s", status, body)
	}
	assertS3Error(t, body, "NotImplemented")

	status, body = s3Do(t, http.MethodDelete, s3URL(sourceBucket, sourceKey), sourceBucket.AccessKey, sourceBucket.AccessSecret, env.Region, nil)
	if status != http.StatusNoContent {
		t.Fatalf("DELETE source after copy: expected 204, got %d: %s", status, body)
	}
	status, body = s3Do(t, http.MethodGet, s3URL(sourceBucket, sameBucketKey), sourceBucket.AccessKey, sourceBucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK || !bytes.Equal(body, objectContent) {
		t.Fatalf("GET same-bucket copy after source delete mismatch: status=%d body=%q", status, body)
	}
	status, body = s3Do(t, http.MethodGet, s3URL(destBucket, crossBucketKey), destBucket.AccessKey, destBucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK || !bytes.Equal(body, objectContent) {
		t.Fatalf("GET cross-bucket copy after source delete mismatch: status=%d body=%q", status, body)
	}
}

func TestS3CompatObjectMetadataHeaders(t *testing.T) {
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

	objectKey := "metadata-" + suffix + ".json"
	copyKey := "metadata-copy-" + suffix + ".json"
	objectContent := []byte(`{"ok":true}`)
	s3URL := func(key string) string {
		return fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, key)
	}
	putHeaders := map[string]string{
		"Content-Type":        "application/json",
		"x-amz-meta-owner":    "Alice",
		"X-Amz-Meta-Trace-ID": "trace-123",
	}

	status, _, body := s3DoWithHeaders(t, http.MethodPut, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, objectContent, putHeaders)
	if status != http.StatusOK {
		t.Fatalf("PUT object: expected 200, got %d: %s", status, body)
	}

	status, headers, body := s3DoWithHeaders(t, http.MethodHead, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("HEAD object: expected 200, got %d: %s", status, body)
	}
	assertObjectMetadataHeaders(t, headers, "application/json", "Alice", "trace-123")

	status, headers, body = s3DoWithHeaders(t, http.MethodGet, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil, nil)
	if status != http.StatusOK || !bytes.Equal(body, objectContent) {
		t.Fatalf("GET full object mismatch: status=%d body=%q", status, body)
	}
	assertObjectMetadataHeaders(t, headers, "application/json", "Alice", "trace-123")

	status, headers, body = s3DoWithHeaders(t, http.MethodGet, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{"Range": "bytes=0-1"})
	if status != http.StatusPartialContent || string(body) != `{"` {
		t.Fatalf("GET range mismatch: status=%d body=%q", status, body)
	}
	if got, want := headers.Get("Content-Length"), "2"; got != want {
		t.Fatalf("GET range Content-Length mismatch: got %q want %q", got, want)
	}
	assertObjectMetadataHeaders(t, headers, "application/json", "Alice", "trace-123")

	copyHeaders := map[string]string{"x-amz-copy-source": "/" + bucket.Key + "/" + objectKey}
	status, _, body = s3DoWithHeaders(t, http.MethodPut, s3URL(copyKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil, copyHeaders)
	if status != http.StatusOK {
		t.Fatalf("CopyObject: expected 200, got %d: %s", status, body)
	}
	status, headers, body = s3DoWithHeaders(t, http.MethodHead, s3URL(copyKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("HEAD copied object: expected 200, got %d: %s", status, body)
	}
	assertObjectMetadataHeaders(t, headers, "application/json", "Alice", "trace-123")

	status, _, body = s3DoWithHeaders(t, http.MethodPut, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, []byte("plain"), map[string]string{"Content-Type": "text/plain"})
	if status != http.StatusOK {
		t.Fatalf("PUT overwrite object: expected 200, got %d: %s", status, body)
	}
	status, headers, body = s3DoWithHeaders(t, http.MethodHead, s3URL(objectKey), bucket.AccessKey, bucket.AccessSecret, env.Region, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("HEAD overwritten object: expected 200, got %d: %s", status, body)
	}
	if got := headers.Get("Content-Type"); got != "text/plain" {
		t.Fatalf("expected overwritten Content-Type, got %q", got)
	}
	if got := headers.Get("x-amz-meta-owner"); got != "" {
		t.Fatalf("expected overwritten metadata to be empty, got %q", got)
	}
}

func assertObjectMetadataHeaders(t *testing.T, headers http.Header, contentType, owner, traceID string) {
	t.Helper()
	if got := headers.Get("Content-Type"); got != contentType {
		t.Fatalf("Content-Type mismatch: got %q want %q", got, contentType)
	}
	if got := headers.Get("x-amz-meta-owner"); got != owner {
		t.Fatalf("x-amz-meta-owner mismatch: got %q want %q", got, owner)
	}
	if got := headers.Get("x-amz-meta-trace-id"); got != traceID {
		t.Fatalf("x-amz-meta-trace-id mismatch: got %q want %q", got, traceID)
	}
}

func TestS3CompatGetObjectRange(t *testing.T) {
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

	objectKey := "range-" + suffix + ".txt"
	objectContent := []byte("abcdefghijklmnopqrstuvwxyz")
	s3URL := fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, objectKey)

	status, body := s3Do(t, http.MethodPut, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, objectContent)
	if status != http.StatusOK {
		t.Fatalf("PUT object: expected 200, got %d: %s", status, body)
	}
	t.Cleanup(func() {
		s3Do(t, http.MethodDelete, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	})

	status, headers, body := s3DoWithHeaders(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{"Range": "bytes=0-4"})
	if status != http.StatusPartialContent {
		t.Fatalf("GET range 0-4: expected 206, got %d: %s", status, body)
	}
	if got, want := string(body), "abcde"; got != want {
		t.Fatalf("GET range 0-4 body mismatch: got %q, want %q", got, want)
	}
	if got, want := headers.Get("Content-Range"), "bytes 0-4/26"; got != want {
		t.Fatalf("GET range 0-4 Content-Range mismatch: got %q, want %q", got, want)
	}
	if got := headers.Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("GET range 0-4 Accept-Ranges mismatch: got %q", got)
	}

	status, _, body = s3DoWithHeaders(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{"Range": "bytes=20-"})
	if status != http.StatusPartialContent || string(body) != "uvwxyz" {
		t.Fatalf("GET open-ended range mismatch: status=%d body=%q", status, body)
	}

	status, _, body = s3DoWithHeaders(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{"Range": "bytes=-3"})
	if status != http.StatusPartialContent || string(body) != "xyz" {
		t.Fatalf("GET suffix range mismatch: status=%d body=%q", status, body)
	}

	status, headers, body = s3DoWithHeaders(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{"Range": "bytes=30-40"})
	if status != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("GET invalid range: expected 416, got %d: %s", status, body)
	}
	if got, want := headers.Get("Content-Range"), "bytes */26"; got != want {
		t.Fatalf("GET invalid range Content-Range mismatch: got %q, want %q", got, want)
	}
	if got := headers.Get("Content-Length"); got == strconv.Itoa(len(objectContent)) {
		t.Fatalf("GET invalid range must not reuse object Content-Length %q", got)
	}

	status, headers, body = s3DoWithHeaders(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("GET full object: expected 200, got %d: %s", status, body)
	}
	if !bytes.Equal(body, objectContent) {
		t.Fatalf("GET full object body mismatch: got %q", body)
	}
	if got := headers.Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("GET full object Accept-Ranges mismatch: got %q", got)
	}
}

// TestS3CompatHeadObject tests the HEAD object operation.
func TestS3CompatHeadObject(t *testing.T) {
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

	objectKey := "head-" + suffix + ".txt"
	objectContent := []byte("Head object test content")
	s3URL := fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, objectKey)

	// Upload
	status, body := s3Do(t, http.MethodPut, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, objectContent)
	if status != http.StatusOK {
		t.Fatalf("PUT object: expected 200, got %d: %s", status, body)
	}

	// HEAD via signed request
	status, _ = s3Do(t, http.MethodHead, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("HEAD object: expected 200, got %d", status)
	}

	// HEAD non-existent object — expect 404
	status, _ = s3Do(t, http.MethodHead, s3URL+"-missing", bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusNotFound {
		t.Fatalf("HEAD missing object: expected 404, got %d", status)
	}

	// Cleanup
	s3Do(t, http.MethodDelete, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
}

func TestS3CompatObjectConditionalReads(t *testing.T) {
	env, ok := loadS3E2EEnv()
	if !ok {
		t.Skip("E2E_S3_ENDPOINT not set; skipping S3 E2E tests")
	}

	suffix := uniqueSuffix()
	ts, bucket := setupS3CompatTest(t, env, suffix)

	objectKey := "conditional-" + suffix + ".txt"
	objectContent := []byte("conditional read test payload")
	s3URL := fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, objectKey)

	status, body := s3Do(t, http.MethodPut, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, objectContent)
	if status != http.StatusOK {
		t.Fatalf("PUT object: expected 200, got %d: %s", status, body)
	}

	status, headers, body := s3DoWithHeaders(t, http.MethodHead, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, nil)
	if status != http.StatusOK {
		t.Fatalf("HEAD object: expected 200, got %d: %s", status, body)
	}
	etag := headers.Get("ETag")
	lastModified := headers.Get("Last-Modified")
	if etag == "" || lastModified == "" {
		t.Fatalf("expected ETag and Last-Modified, got etag=%q last-modified=%q", etag, lastModified)
	}

	status, _, body = s3DoWithHeaders(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{"If-Match": etag})
	if status != http.StatusOK || !bytes.Equal(body, objectContent) {
		t.Fatalf("GET If-Match mismatch: status=%d body=%q", status, body)
	}

	status, _, body = s3DoWithHeaders(t, http.MethodHead, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{"If-None-Match": etag})
	if status != http.StatusNotModified || len(body) != 0 {
		t.Fatalf("HEAD If-None-Match mismatch: status=%d body=%q", status, body)
	}

	status, _, body = s3DoWithHeaders(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{"If-Match": `"other-etag"`})
	if status != http.StatusPreconditionFailed {
		t.Fatalf("GET If-Match mismatch: expected 412, got %d: %s", status, body)
	}

	status, _, body = s3DoWithHeaders(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{"If-Modified-Since": lastModified})
	if status != http.StatusNotModified {
		t.Fatalf("GET If-Modified-Since: expected 304, got %d: %s", status, body)
	}

	status, _, body = s3DoWithHeaders(t, http.MethodHead, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{
		"If-Unmodified-Since": time.Unix(0, 0).UTC().Format(http.TimeFormat),
	})
	if status != http.StatusPreconditionFailed {
		t.Fatalf("HEAD If-Unmodified-Since: expected 412, got %d: %s", status, body)
	}

	status, _, body = s3DoWithHeaders(t, http.MethodGet, s3URL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil, map[string]string{
		"If-None-Match":     `"other-etag"`,
		"If-Modified-Since": time.Now().UTC().Add(time.Hour).Format(http.TimeFormat),
	})
	if status != http.StatusOK || !bytes.Equal(body, objectContent) {
		t.Fatalf("GET combined conditional headers mismatch: status=%d body=%q", status, body)
	}
}
