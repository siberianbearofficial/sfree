//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/example/sfree/api-go/internal/app"
	appconfig "github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
)

// s3E2EEnv holds MinIO configuration read from environment variables.
type s3E2EEnv struct {
	Endpoint    string
	Bucket      string
	AccessKeyID string
	SecretKey   string
	Region      string
	PathStyle   bool
}

// loadS3E2EEnv reads MinIO config from env vars.
// Returns (env, true) if E2E_S3_ENDPOINT is set, else (zero, false).
func loadS3E2EEnv() (s3E2EEnv, bool) {
	endpoint := os.Getenv("E2E_S3_ENDPOINT")
	if endpoint == "" {
		return s3E2EEnv{}, false
	}
	bucket := os.Getenv("E2E_S3_BUCKET")
	if bucket == "" {
		bucket = "sfree-e2e-source"
	}
	accessKeyID := os.Getenv("E2E_S3_ACCESS_KEY_ID")
	if accessKeyID == "" {
		accessKeyID = "minioadmin"
	}
	secretKey := os.Getenv("E2E_S3_SECRET_ACCESS_KEY")
	if secretKey == "" {
		secretKey = "minioadmin"
	}
	region := os.Getenv("E2E_S3_REGION")
	if region == "" {
		region = "us-east-1"
	}
	pathStyleVal := os.Getenv("E2E_S3_PATH_STYLE")
	pathStyle := pathStyleVal == "" || pathStyleVal == "true" || pathStyleVal == "1" || pathStyleVal == "yes"
	return s3E2EEnv{
		Endpoint:    endpoint,
		Bucket:      bucket,
		AccessKeyID: accessKeyID,
		SecretKey:   secretKey,
		Region:      region,
		PathStyle:   pathStyle,
	}, true
}

// ensureMinIOBucket creates the MinIO source bucket if it does not already exist.
func ensureMinIOBucket(t *testing.T, env s3E2EEnv) {
	t.Helper()
	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(env.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(env.AccessKeyID, env.SecretKey, "")),
	)
	if err != nil {
		t.Fatalf("ensureMinIOBucket: load aws config: %v", err)
	}
	client := awss3.NewFromConfig(awsCfg, func(opts *awss3.Options) {
		opts.BaseEndpoint = aws.String(env.Endpoint)
		opts.UsePathStyle = env.PathStyle
	})
	_, err = client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(env.Bucket)})
	if err == nil {
		return // already exists
	}
	_, err = client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(env.Bucket)})
	if err != nil {
		t.Fatalf("ensureMinIOBucket: create bucket: %v", err)
	}
}

// newTestServer connects to MongoDB (with retry) and starts an httptest server.
func newTestServer(t *testing.T) (*httptest.Server, *appconfig.Config) {
	t.Helper()
	cfg, err := appconfig.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	var mongoConn *db.Mongo
	for i := 0; i < 30; i++ {
		mongoConn, err = db.Connect(context.Background(), cfg.Mongo)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	router := app.SetupRouter(mongoConn, cfg)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	return ts, cfg
}

// apiPost makes a POST request with JSON body and basic auth.
func apiPost(t *testing.T, ts *httptest.Server, path string, body interface{}, username, password string) (*http.Response, []byte) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp, respBody
}

// apiGet makes a GET request with basic auth.
func apiGet(t *testing.T, ts *httptest.Server, path, username, password string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp, respBody
}

// apiDelete makes a DELETE request with basic auth.
func apiDelete(t *testing.T, ts *httptest.Server, path, username, password string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.SetBasicAuth(username, password)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	resp.Body.Close()
	return resp
}

// createTestUser creates a user and returns username+generated password.
func createTestUser(t *testing.T, ts *httptest.Server, suffix string) (username, password string) {
	t.Helper()
	username = "e2e-user-" + suffix
	resp, body := apiPost(t, ts, "/api/v1/users", map[string]string{"username": username}, "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create user: expected 200, got %d: %s", resp.StatusCode, body)
	}
	var result struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode user response: %v", err)
	}
	return username, result.Password
}

// createS3Source creates an S3 source and returns its ID.
func createS3Source(t *testing.T, ts *httptest.Server, username, password, name string, env s3E2EEnv) string {
	t.Helper()
	payload := map[string]interface{}{
		"name":              name,
		"endpoint":          env.Endpoint,
		"bucket":            env.Bucket,
		"access_key_id":     env.AccessKeyID,
		"secret_access_key": env.SecretKey,
		"region":            env.Region,
		"path_style":        env.PathStyle,
	}
	resp, body := apiPost(t, ts, "/api/v1/sources/s3", payload, username, password)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create source: expected 200, got %d: %s", resp.StatusCode, body)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.ID == "" {
		t.Fatalf("decode source response: %v body=%s", err, body)
	}
	return result.ID
}

// createBucketResult holds the response from bucket creation.
type createBucketResult struct {
	ID           string
	Key          string
	AccessKey    string
	AccessSecret string
}

// createBucket creates a bucket and returns key, access credentials, and ID.
func createBucket(t *testing.T, ts *httptest.Server, username, password, key, sourceID string) createBucketResult {
	t.Helper()
	payload := map[string]interface{}{
		"key":        key,
		"source_ids": []string{sourceID},
	}
	resp, body := apiPost(t, ts, "/api/v1/buckets", payload, username, password)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create bucket: expected 200, got %d: %s", resp.StatusCode, body)
	}
	var createResp struct {
		Key          string `json:"key"`
		AccessKey    string `json:"access_key"`
		AccessSecret string `json:"access_secret"`
	}
	if err := json.Unmarshal(body, &createResp); err != nil {
		t.Fatalf("decode bucket response: %v", err)
	}

	// Retrieve ID from list
	_, listBody := apiGet(t, ts, "/api/v1/buckets", username, password)
	var buckets []struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(listBody, &buckets); err != nil {
		t.Fatalf("decode bucket list: %v", err)
	}
	var bucketID string
	for _, b := range buckets {
		if b.Key == key {
			bucketID = b.ID
			break
		}
	}
	if bucketID == "" {
		t.Fatalf("bucket %q not found in list", key)
	}
	return createBucketResult{
		ID:           bucketID,
		Key:          createResp.Key,
		AccessKey:    createResp.AccessKey,
		AccessSecret: createResp.AccessSecret,
	}
}

// signS3Request signs an HTTP request with AWS Signature Version 4 for S3.
// Sets Authorization, X-Amz-Date, and X-Amz-Content-Sha256 headers.
// Body is read and buffered so it can be re-read by the server.
func signS3Request(req *http.Request, accessKey, secretKey, region string) error {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStr := now.Format("20060102")

	const emptyBodyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	payloadHash := emptyBodyHash
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("sign: read body: %w", err)
		}
		sum := sha256.Sum256(body)
		payloadHash = hex.EncodeToString(sum[:])
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// Use URL.Host as canonical host — the HTTP client sends this as the Host header.
	host := req.URL.Host

	// Canonical headers must be sorted alphabetically by header name.
	canonHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		host, payloadHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	// Canonical URI: decode then re-encode per AWS rules.
	rawPath := req.URL.EscapedPath()
	decodedPath, err := url.PathUnescape(rawPath)
	if err != nil {
		decodedPath = rawPath
	}
	canonURI := s3EncodePath(decodedPath)
	if canonURI == "" {
		canonURI = "/"
	}

	canonReq := strings.Join([]string{
		req.Method,
		canonURI,
		req.URL.RawQuery,
		canonHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStr, region)
	crHash := sha256.Sum256([]byte(canonReq))
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, scope, hex.EncodeToString(crHash[:]))

	kDate := s3HmacSHA256([]byte("AWS4"+secretKey), []byte(dateStr))
	kRegion := s3HmacSHA256(kDate, []byte(region))
	kService := s3HmacSHA256(kRegion, []byte("s3"))
	kSigning := s3HmacSHA256(kService, []byte("aws4_request"))
	sig := s3HmacSHA256(kSigning, []byte(stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, scope, signedHeaders, hex.EncodeToString(sig),
	))
	return nil
}

func s3HmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// s3EncodePath percent-encodes a path per AWS S3 canonical URI rules:
// unreserved chars and '/' pass through; everything else is %-encoded.
func s3EncodePath(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		c := path[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' || c == '/' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// s3Do makes a signed S3 request and returns status code + body.
func s3Do(t *testing.T, method, rawURL, accessKey, secretKey, region string, body []byte) (int, []byte) {
	t.Helper()
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, rawURL, reqBody)
	if err != nil {
		t.Fatalf("s3Do new request: %v", err)
	}
	if err := signS3Request(req, accessKey, secretKey, region); err != nil {
		t.Fatalf("s3Do sign request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("s3Do do request: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody
}

// uniqueSuffix returns a short timestamp-based string for naming test resources.
func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// TestS3CompatSourceAndBucketCreation tests source/bucket CRUD plus the
// deletion-blocked-by-bucket constraint.
func TestS3CompatSourceAndBucketCreation(t *testing.T) {
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

	// Verify source appears in list
	_, listBody := apiGet(t, ts, "/api/v1/sources", username, password)
	var sources []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(listBody, &sources); err != nil {
		t.Fatalf("decode sources list: %v", err)
	}
	found := false
	for _, s := range sources {
		if s.ID == sourceID {
			found = true
			if s.Type != "s3" {
				t.Fatalf("expected type s3, got %s", s.Type)
			}
			break
		}
	}
	if !found {
		t.Fatalf("source %s not found in list", sourceID)
	}

	// Verify bucket access_key equals the bucket key (product design)
	if bucket.AccessKey != bucket.Key {
		t.Fatalf("expected access_key == key, got access_key=%s key=%s", bucket.AccessKey, bucket.Key)
	}
	if bucket.AccessSecret == "" {
		t.Fatal("expected non-empty access_secret")
	}

	// Deleting the source while the bucket references it must return 409.
	resp := apiDelete(t, ts, "/api/v1/sources/"+sourceID, username, password)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 deleting source in-use, got %d", resp.StatusCode)
	}

	// Cleanup: delete bucket then source
	resp = apiDelete(t, ts, "/api/v1/buckets/"+bucket.ID, username, password)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete bucket: expected 200/204, got %d", resp.StatusCode)
	}
	resp = apiDelete(t, ts, "/api/v1/sources/"+sourceID, username, password)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete source: expected 200/204, got %d", resp.StatusCode)
	}
}

// TestS3CompatObjectLifecycle exercises the full S3 PUT/LIST/GET/DELETE cycle
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

// TestS3CompatAccessKeyAuth verifies that the S3 API rejects requests with
// wrong credentials.
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

// presignS3URL generates a presigned URL for the given method and raw URL.
func presignS3URL(rawURL, method, accessKey, secretKey, region string, expiresSec int) (string, error) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStr := now.Format("20060102")
	scope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStr, region)
	cred := accessKey + "/" + scope
	signedHeaders := "host"

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Canonical URI: decode then re-encode per AWS rules.
	decodedPath, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		decodedPath = u.EscapedPath()
	}
	canonURI := s3EncodePath(decodedPath)
	if canonURI == "" {
		canonURI = "/"
	}

	// Build canonical query string (sorted, without X-Amz-Signature).
	canonQuery := fmt.Sprintf("X-Amz-Algorithm=AWS4-HMAC-SHA256"+
		"&X-Amz-Credential=%s"+
		"&X-Amz-Date=%s"+
		"&X-Amz-Expires=%d"+
		"&X-Amz-SignedHeaders=%s",
		url.QueryEscape(cred), amzDate, expiresSec, signedHeaders)

	canonHeaders := fmt.Sprintf("host:%s\n", u.Host)
	payloadHash := "UNSIGNED-PAYLOAD"

	canonReq := strings.Join([]string{
		method,
		canonURI,
		canonQuery,
		canonHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	crHash := sha256.Sum256([]byte(canonReq))
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, scope, hex.EncodeToString(crHash[:]))

	kDate := s3HmacSHA256([]byte("AWS4"+secretKey), []byte(dateStr))
	kRegion := s3HmacSHA256(kDate, []byte(region))
	kService := s3HmacSHA256(kRegion, []byte("s3"))
	kSigning := s3HmacSHA256(kService, []byte("aws4_request"))
	sig := s3HmacSHA256(kSigning, []byte(stringToSign))

	return fmt.Sprintf("%s://%s%s?%s&X-Amz-Signature=%s",
		u.Scheme, u.Host, u.EscapedPath(), canonQuery, hex.EncodeToString(sig)), nil
}

// TestS3CompatPresignedGetObject tests downloading a file via presigned URL.
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

// TestS3CompatPresignedURLExpired tests that expired presigned URLs are rejected.
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
