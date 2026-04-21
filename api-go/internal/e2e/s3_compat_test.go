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
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
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

func listS3SourceObjectKeys(t *testing.T, env s3E2EEnv) map[string]struct{} {
	t.Helper()

	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(env.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(env.AccessKeyID, env.SecretKey, "")),
	)
	if err != nil {
		t.Fatalf("list source objects: load aws config: %v", err)
	}
	client := awss3.NewFromConfig(awsCfg, func(opts *awss3.Options) {
		opts.BaseEndpoint = aws.String(env.Endpoint)
		opts.UsePathStyle = env.PathStyle
	})

	keys := map[string]struct{}{}
	var continuation *string
	for {
		out, err := client.ListObjectsV2(ctx, &awss3.ListObjectsV2Input{
			Bucket:            aws.String(env.Bucket),
			ContinuationToken: continuation,
		})
		if err != nil {
			t.Fatalf("list source objects: %v", err)
		}
		for _, object := range out.Contents {
			if object.Key != nil {
				keys[*object.Key] = struct{}{}
			}
		}
		if out.NextContinuationToken == nil {
			return keys
		}
		continuation = out.NextContinuationToken
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
		s3CanonicalQuery(req.URL),
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

func s3CanonicalQuery(u *url.URL) string {
	if u == nil {
		return ""
	}
	values, _ := url.ParseQuery(u.RawQuery)
	type kv struct {
		k string
		v string
	}
	items := make([]kv, 0)
	for k, vs := range values {
		if len(vs) == 0 {
			items = append(items, kv{k: k})
			continue
		}
		for _, v := range vs {
			items = append(items, kv{k: k, v: v})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].k == items[j].k {
			return items[i].v < items[j].v
		}
		return items[i].k < items[j].k
	})
	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(s3EncodeQuery(item.k))
		b.WriteByte('=')
		b.WriteString(s3EncodeQuery(item.v))
	}
	return b.String()
}

func s3EncodeQuery(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
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
	status, _, respBody := s3DoWithHeaders(t, method, rawURL, accessKey, secretKey, region, body, nil)
	return status, respBody
}

func s3DoWithHeaders(t *testing.T, method, rawURL, accessKey, secretKey, region string, body []byte, headers map[string]string) (int, http.Header, []byte) {
	t.Helper()
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, rawURL, reqBody)
	if err != nil {
		t.Fatalf("s3Do new request: %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
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
	return resp.StatusCode, resp.Header.Clone(), respBody
}

// uniqueSuffix returns a short timestamp-based string for naming test resources.
func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func setupS3CompatTest(t *testing.T, env s3E2EEnv, suffix string) (*httptest.Server, createBucketResult) {
	t.Helper()

	ts, _ := newTestServer(t)
	ensureMinIOBucket(t, env)

	username, password := createTestUser(t, ts, suffix)
	sourceID := createS3Source(t, ts, username, password, "src-"+suffix, env)
	bucket := createBucket(t, ts, username, password, "bkt-"+suffix, sourceID)

	t.Cleanup(func() {
		apiDelete(t, ts, "/api/v1/buckets/"+bucket.ID, username, password)
		apiDelete(t, ts, "/api/v1/sources/"+sourceID, username, password)
	})

	return ts, bucket
}

func assertS3Error(t *testing.T, body []byte, wantCode string) {
	t.Helper()

	var result struct {
		Code string `xml:"Code"`
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode S3 error: %v body=%s", err, body)
	}
	if result.Code != wantCode {
		t.Fatalf("S3 error code mismatch: got %q, want %q body=%s", result.Code, wantCode, body)
	}
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
}

func TestS3CompatMultipartUploadLifecycle(t *testing.T) {
	env, ok := loadS3E2EEnv()
	if !ok {
		t.Skip("E2E_S3_ENDPOINT not set; skipping S3 E2E tests")
	}

	suffix := uniqueSuffix()
	ts, bucket := setupS3CompatTest(t, env, suffix)
	s3URL := func(key string, params url.Values) string {
		rawURL := fmt.Sprintf("%s/api/s3/%s/%s", ts.URL, bucket.Key, key)
		if len(params) > 0 {
			rawURL += "?" + params.Encode()
		}
		return rawURL
	}

	objectKey := "multipart-" + suffix + ".txt"
	createURL := s3URL(objectKey, url.Values{"uploads": {""}})
	status, body := s3Do(t, http.MethodPost, createURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("CreateMultipartUpload: expected 200, got %d: %s", status, body)
	}
	var createResult struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(body, &createResult); err != nil || createResult.UploadID == "" {
		t.Fatalf("CreateMultipartUpload decode: uploadId=%q err=%v body=%s", createResult.UploadID, err, body)
	}

	partURL := func(uploadID string, partNumber int) string {
		return s3URL(objectKey, url.Values{
			"partNumber": {strconv.Itoa(partNumber)},
			"uploadId":   {uploadID},
		})
	}
	part1Original := []byte("part-one-original")
	status, headers, body := s3DoWithHeaders(t, http.MethodPut, partURL(createResult.UploadID, 1), bucket.AccessKey, bucket.AccessSecret, env.Region, part1Original, nil)
	if status != http.StatusOK {
		t.Fatalf("UploadPart original part 1: expected 200, got %d: %s", status, body)
	}
	part1OriginalETag := headers.Get("ETag")
	if part1OriginalETag == "" {
		t.Fatal("UploadPart original part 1: missing ETag")
	}

	part1Replacement := []byte("part-one-replacement")
	status, headers, body = s3DoWithHeaders(t, http.MethodPut, partURL(createResult.UploadID, 1), bucket.AccessKey, bucket.AccessSecret, env.Region, part1Replacement, nil)
	if status != http.StatusOK {
		t.Fatalf("UploadPart replacement part 1: expected 200, got %d: %s", status, body)
	}
	part1ETag := headers.Get("ETag")
	if part1ETag == "" {
		t.Fatal("UploadPart replacement part 1: missing ETag")
	}
	if part1ETag == part1OriginalETag {
		t.Fatalf("UploadPart replacement part 1: ETag did not change after re-upload: %s", part1ETag)
	}

	part2 := []byte("part-two")
	status, headers, body = s3DoWithHeaders(t, http.MethodPut, partURL(createResult.UploadID, 2), bucket.AccessKey, bucket.AccessSecret, env.Region, part2, nil)
	if status != http.StatusOK {
		t.Fatalf("UploadPart part 2: expected 200, got %d: %s", status, body)
	}
	part2ETag := headers.Get("ETag")
	if part2ETag == "" {
		t.Fatal("UploadPart part 2: missing ETag")
	}

	listPartsURL := s3URL(objectKey, url.Values{"uploadId": {createResult.UploadID}})
	status, body = s3Do(t, http.MethodGet, listPartsURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("ListParts: expected 200, got %d: %s", status, body)
	}
	var partsResult struct {
		Parts []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
			Size       int64  `xml:"Size"`
		} `xml:"Part"`
	}
	if err := xml.Unmarshal(body, &partsResult); err != nil {
		t.Fatalf("ListParts decode: %v body=%s", err, body)
	}
	if len(partsResult.Parts) != 2 {
		t.Fatalf("ListParts: expected 2 parts, got %d: %+v", len(partsResult.Parts), partsResult.Parts)
	}
	expectedParts := []struct {
		number int
		size   int64
		etag   string
	}{
		{number: 1, size: int64(len(part1Replacement)), etag: part1ETag},
		{number: 2, size: int64(len(part2)), etag: part2ETag},
	}
	for i, want := range expectedParts {
		got := partsResult.Parts[i]
		if got.PartNumber != want.number || got.Size != want.size || got.ETag != want.etag {
			t.Fatalf("ListParts part %d mismatch: got number=%d size=%d etag=%q, want number=%d size=%d etag=%q",
				i, got.PartNumber, got.Size, got.ETag, want.number, want.size, want.etag)
		}
	}

	completeBody := []byte(fmt.Sprintf(
		`<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>%s</ETag></Part><Part><PartNumber>2</PartNumber><ETag>%s</ETag></Part></CompleteMultipartUpload>`,
		part1ETag, part2ETag,
	))
	completeURL := s3URL(objectKey, url.Values{"uploadId": {createResult.UploadID}})
	status, body = s3Do(t, http.MethodPost, completeURL, bucket.AccessKey, bucket.AccessSecret, env.Region, completeBody)
	if status != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload: expected 200, got %d: %s", status, body)
	}

	status, body = s3Do(t, http.MethodGet, s3URL(objectKey, nil), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("GET completed multipart object: expected 200, got %d: %s", status, body)
	}
	if want := append(append([]byte{}, part1Replacement...), part2...); !bytes.Equal(body, want) {
		t.Fatalf("GET completed multipart object: got %q, want %q", body, want)
	}

	status, body = s3Do(t, http.MethodGet, listPartsURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status < 400 {
		t.Fatalf("ListParts completed upload: expected non-2xx, got %d: %s", status, body)
	}
	assertS3Error(t, body, "NoSuchUpload")

	abortKey := "multipart-abort-" + suffix + ".txt"
	beforeAbortKeys := listS3SourceObjectKeys(t, env)
	status, body = s3Do(t, http.MethodPost, s3URL(abortKey, url.Values{"uploads": {""}}), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("CreateMultipartUpload for abort: expected 200, got %d: %s", status, body)
	}
	var abortCreateResult struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(body, &abortCreateResult); err != nil || abortCreateResult.UploadID == "" {
		t.Fatalf("CreateMultipartUpload for abort decode: uploadId=%q err=%v body=%s", abortCreateResult.UploadID, err, body)
	}
	abortPartURL := s3URL(abortKey, url.Values{"partNumber": {"1"}, "uploadId": {abortCreateResult.UploadID}})
	status, body = s3Do(t, http.MethodPut, abortPartURL, bucket.AccessKey, bucket.AccessSecret, env.Region, []byte("abort-me"))
	if status != http.StatusOK {
		t.Fatalf("UploadPart for abort: expected 200, got %d: %s", status, body)
	}
	afterAbortPartKeys := listS3SourceObjectKeys(t, env)
	if len(afterAbortPartKeys) <= len(beforeAbortKeys) {
		t.Fatalf("UploadPart for abort: expected source chunk object to be created; before=%d after=%d", len(beforeAbortKeys), len(afterAbortPartKeys))
	}
	abortURL := s3URL(abortKey, url.Values{"uploadId": {abortCreateResult.UploadID}})
	status, body = s3Do(t, http.MethodDelete, abortURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusNoContent {
		t.Fatalf("AbortMultipartUpload: expected 204, got %d: %s", status, body)
	}
	afterAbortKeys := listS3SourceObjectKeys(t, env)
	if len(afterAbortKeys) != len(beforeAbortKeys) {
		t.Fatalf("AbortMultipartUpload: source chunks were not cleaned up; before=%d after=%d", len(beforeAbortKeys), len(afterAbortKeys))
	}
	for key := range beforeAbortKeys {
		if _, ok := afterAbortKeys[key]; !ok {
			t.Fatalf("AbortMultipartUpload: source key %q disappeared unexpectedly", key)
		}
	}
	status, body = s3Do(t, http.MethodGet, abortURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status < 400 {
		t.Fatalf("ListParts aborted upload: expected non-2xx, got %d: %s", status, body)
	}
	assertS3Error(t, body, "NoSuchUpload")
	status, body = s3Do(t, http.MethodGet, s3URL(abortKey, nil), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusNotFound {
		t.Fatalf("GET aborted multipart object: expected 404, got %d: %s", status, body)
	}

	status, body = s3Do(t, http.MethodPut, s3URL("missing-upload-"+suffix+".txt", url.Values{"partNumber": {"1"}, "uploadId": {"missing-upload-id"}}), bucket.AccessKey, bucket.AccessSecret, env.Region, []byte("missing"))
	if status < 400 {
		t.Fatalf("UploadPart missing upload: expected non-2xx, got %d: %s", status, body)
	}
	assertS3Error(t, body, "NoSuchUpload")
	status, body = s3Do(t, http.MethodDelete, s3URL("empty-upload-"+suffix+".txt", url.Values{"uploadId": {""}}), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status < 400 {
		t.Fatalf("AbortMultipartUpload empty uploadId: expected non-2xx, got %d: %s", status, body)
	}
	assertS3Error(t, body, "NoSuchUpload")
}

func TestS3CompatListObjectsV2PrefixDelimiterAndPagination(t *testing.T) {
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
		"docs/readme-" + suffix + ".txt",
		"photos/2024/a-" + suffix + ".jpg",
		"photos/2024/b-" + suffix + ".jpg",
		"photos/2025/c-" + suffix + ".jpg",
		"photos/root-" + suffix + ".txt",
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

	type listBucketXML struct {
		Prefix                string `xml:"Prefix"`
		Delimiter             string `xml:"Delimiter"`
		IsTruncated           bool   `xml:"IsTruncated"`
		KeyCount              int    `xml:"KeyCount"`
		NextContinuationToken string `xml:"NextContinuationToken"`
		NextMarker            string `xml:"NextMarker"`
		Contents              []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
		CommonPrefixes []struct {
			Prefix string `xml:"Prefix"`
		} `xml:"CommonPrefixes"`
	}
	listURL := ts.URL + "/api/s3/" + bucket.Key
	listWith := func(params url.Values) listBucketXML {
		t.Helper()
		rawURL := listURL
		if len(params) > 0 {
			rawURL += "?" + params.Encode()
		}
		status, body := s3Do(t, http.MethodGet, rawURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
		if status != http.StatusOK {
			t.Fatalf("LIST %s: expected 200, got %d: %s", rawURL, status, body)
		}
		var result listBucketXML
		if err := xml.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode list response: %v body=%s", err, body)
		}
		return result
	}
	keysFrom := func(result listBucketXML) []string {
		keys := make([]string, 0, len(result.Contents))
		for _, entry := range result.Contents {
			keys = append(keys, entry.Key)
		}
		return keys
	}
	prefixesFrom := func(result listBucketXML) []string {
		prefixes := make([]string, 0, len(result.CommonPrefixes))
		for _, prefix := range result.CommonPrefixes {
			prefixes = append(prefixes, prefix.Prefix)
		}
		return prefixes
	}
	has := func(values []string, want string) bool {
		for _, value := range values {
			if value == want {
				return true
			}
		}
		return false
	}

	prefixList := listWith(url.Values{"list-type": {"2"}, "prefix": {"photos/"}})
	prefixKeys := keysFrom(prefixList)
	if prefixList.Prefix != "photos/" || prefixList.KeyCount != 4 || len(prefixKeys) != 4 {
		t.Fatalf("V2 prefix list mismatch: prefix=%q keyCount=%d keys=%v", prefixList.Prefix, prefixList.KeyCount, prefixKeys)
	}
	if has(prefixKeys, "docs/readme-"+suffix+".txt") {
		t.Fatalf("V2 prefix list included non-matching key: %v", prefixKeys)
	}

	delimited := listWith(url.Values{"list-type": {"2"}, "prefix": {"photos/"}, "delimiter": {"/"}})
	delimitedKeys := keysFrom(delimited)
	commonPrefixes := prefixesFrom(delimited)
	if delimited.Delimiter != "/" || delimited.KeyCount != 3 {
		t.Fatalf("V2 delimiter list mismatch: delimiter=%q keyCount=%d keys=%v prefixes=%v", delimited.Delimiter, delimited.KeyCount, delimitedKeys, commonPrefixes)
	}
	if !has(delimitedKeys, "photos/root-"+suffix+".txt") || !has(commonPrefixes, "photos/2024/") || !has(commonPrefixes, "photos/2025/") {
		t.Fatalf("V2 delimiter list missing expected entries: keys=%v prefixes=%v", delimitedKeys, commonPrefixes)
	}

	page1 := listWith(url.Values{"list-type": {"2"}, "prefix": {"photos/"}, "max-keys": {"2"}})
	page1Keys := keysFrom(page1)
	if page1.KeyCount != 2 || len(page1Keys) != 2 || !page1.IsTruncated || page1.NextContinuationToken == "" {
		t.Fatalf("V2 first page mismatch: keyCount=%d keys=%v truncated=%v token=%q", page1.KeyCount, page1Keys, page1.IsTruncated, page1.NextContinuationToken)
	}
	page2 := listWith(url.Values{"list-type": {"2"}, "prefix": {"photos/"}, "max-keys": {"2"}, "continuation-token": {page1.NextContinuationToken}})
	page2Keys := keysFrom(page2)
	if page2.KeyCount != 2 || len(page2Keys) != 2 || page2.IsTruncated {
		t.Fatalf("V2 second page mismatch: keyCount=%d keys=%v truncated=%v", page2.KeyCount, page2Keys, page2.IsTruncated)
	}
	if has(page2Keys, page1Keys[0]) || has(page2Keys, page1Keys[1]) {
		t.Fatalf("V2 pagination repeated keys: page1=%v page2=%v", page1Keys, page2Keys)
	}

	v1Delimited := listWith(url.Values{"prefix": {"photos/"}, "delimiter": {"/"}})
	v1Keys := keysFrom(v1Delimited)
	v1Prefixes := prefixesFrom(v1Delimited)
	if v1Delimited.KeyCount != 3 || !has(v1Prefixes, "photos/2024/") || !has(v1Prefixes, "photos/2025/") || !has(v1Keys, "photos/root-"+suffix+".txt") {
		t.Fatalf("V1 delimiter list mismatch: keyCount=%d keys=%v prefixes=%v nextMarker=%q", v1Delimited.KeyCount, v1Keys, v1Prefixes, v1Delimited.NextMarker)
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
