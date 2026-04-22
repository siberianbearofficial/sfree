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
	mongoConn, err := connectMongoForE2E(cfg)
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	router, err := app.SetupRouter(mongoConn, cfg)
	if err != nil {
		t.Fatalf("setup router: %v", err)
	}
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	return ts, cfg
}

func connectMongoForE2E(cfg *appconfig.Config) (*db.Mongo, error) {
	deadline := time.Now().Add(30 * time.Second)
	retryInterval := time.Second
	var lastErr error
	for attempts := 1; ; attempts++ {
		mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
		if err == nil {
			return mongoConn, nil
		}
		lastErr = err

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("mongo did not become ready within 30s after %d attempts: %w", attempts, lastErr)
		}

		wait := retryInterval
		if remaining < wait {
			wait = remaining
		}
		timer := time.NewTimer(wait)
		<-timer.C
	}
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

// presignS3URL generates a presigned URL for the given method and raw URL.
func presignS3URL(rawURL, method, accessKey, secretKey, region string, expiresSec int) (string, error) {
	return presignS3URLAt(rawURL, method, accessKey, secretKey, region, time.Now().UTC(), expiresSec)
}

func presignS3URLAt(rawURL, method, accessKey, secretKey, region string, signedAt time.Time, expiresSec int) (string, error) {
	now := signedAt.UTC()
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
