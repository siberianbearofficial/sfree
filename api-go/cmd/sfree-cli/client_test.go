package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func configureTestClient(t *testing.T, server string) string {
	t.Helper()
	resetCLIState(t)
	t.Setenv("SFREE_SERVER", server)
	t.Setenv("SFREE_USER", "test-user")
	t.Setenv("SFREE_PASSWORD", "test-pass")
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("test-user:test-pass"))
}

func resetCLIState(t *testing.T) {
	t.Helper()
	flagServer = ""
	flagUser = ""
	flagPassword = ""
	bucketCreateKey = ""
	bucketCreateSources = ""
	keysCreateBucket = ""
	keysCreateSources = ""
	_ = rootCmd.PersistentFlags().Set("server", "")
	_ = rootCmd.PersistentFlags().Set("user", "")
	_ = rootCmd.PersistentFlags().Set("password", "")
	_ = bucketsCreateCmd.Flags().Set("key", "")
	_ = bucketsCreateCmd.Flags().Set("sources", "")
	_ = keysCreateCmd.Flags().Set("bucket", "")
	_ = keysCreateCmd.Flags().Set("sources", "")
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
}

func TestAPIGetSendsAuthAndDecodesResponse(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/sources" {
			t.Fatalf("path = %s, want /api/v1/sources", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	wantAuth := configureTestClient(t, server.URL)

	var out struct {
		OK bool `json:"ok"`
	}
	if err := apiGet("/api/v1/sources", &out); err != nil {
		t.Fatalf("apiGet returned error: %v", err)
	}
	if gotAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", gotAuth, wantAuth)
	}
	if !out.OK {
		t.Fatal("decoded response OK = false, want true")
	}
}

func TestAPIGetReturnsServerErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "source unavailable", http.StatusTeapot)
	}))
	defer server.Close()
	configureTestClient(t, server.URL)

	err := apiGet("/api/v1/sources", &struct{}{})
	if err == nil {
		t.Fatal("apiGet returned nil error")
	}
	if !strings.Contains(err.Error(), "server returned 418") || !strings.Contains(err.Error(), "source unavailable") {
		t.Fatalf("error = %q, want status and body", err.Error())
	}
}

func TestAPIPostSendsAuthJSONAndDecodesResponse(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/buckets" {
			t.Fatalf("path = %s, want /api/v1/buckets", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", ct)
		}
		var req createBucketReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Key != "bucket-one" || len(req.SourceIDs) != 2 || req.SourceIDs[0] != "src-a" || req.SourceIDs[1] != "src-b" {
			t.Fatalf("request = %#v, want bucket-one with two source ids", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"bucket-one"}`))
	}))
	defer server.Close()
	wantAuth := configureTestClient(t, server.URL)

	var out createBucketResp
	err := apiPost("/api/v1/buckets", createBucketReq{Key: "bucket-one", SourceIDs: []string{"src-a", "src-b"}}, &out)
	if err != nil {
		t.Fatalf("apiPost returned error: %v", err)
	}
	if gotAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", gotAuth, wantAuth)
	}
	if out.Key != "bucket-one" {
		t.Fatalf("decoded key = %q, want bucket-one", out.Key)
	}
}

func TestAPIPostReturnsServerErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bucket rejected", http.StatusBadRequest)
	}))
	defer server.Close()
	configureTestClient(t, server.URL)

	err := apiPost("/api/v1/buckets", createBucketReq{}, nil)
	if err == nil {
		t.Fatal("apiPost returned nil error")
	}
	if !strings.Contains(err.Error(), "server returned 400") || !strings.Contains(err.Error(), "bucket rejected") {
		t.Fatalf("error = %q, want status and body", err.Error())
	}
}

func TestAPIUploadSendsAuthMultipartAndDecodesResponse(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(srcPath, []byte("payload bytes"), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/buckets/bucket-1/upload" {
			t.Fatalf("path = %s, want upload path", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("multipart reader: %v", err)
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("next multipart part: %v", err)
		}
		defer func() {
			_ = part.Close()
		}()
		if part.FormName() != "file" {
			t.Fatalf("form name = %q, want file", part.FormName())
		}
		if part.FileName() != "payload.txt" {
			t.Fatalf("file name = %q, want payload.txt", part.FileName())
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		if string(body) != "payload bytes" {
			t.Fatalf("uploaded body = %q, want payload bytes", string(body))
		}
		if _, err := reader.NextPart(); err != io.EOF {
			t.Fatalf("next part error = %v, want EOF", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"file-1","name":"payload.txt"}`))
	}))
	defer server.Close()
	wantAuth := configureTestClient(t, server.URL)

	var out uploadResp
	if err := apiUpload("/api/v1/buckets/bucket-1/upload", srcPath, &out); err != nil {
		t.Fatalf("apiUpload returned error: %v", err)
	}
	if gotAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", gotAuth, wantAuth)
	}
	if out.ID != "file-1" || out.Name != "payload.txt" {
		t.Fatalf("decoded upload response = %#v, want file-1 payload.txt", out)
	}
}

func TestAPIUploadReturnsServerErrorBody(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(srcPath, []byte("payload bytes"), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		http.Error(w, "upload refused", http.StatusRequestEntityTooLarge)
	}))
	defer server.Close()
	configureTestClient(t, server.URL)

	err := apiUpload("/api/v1/buckets/bucket-1/upload", srcPath, nil)
	if err == nil {
		t.Fatal("apiUpload returned nil error")
	}
	if !strings.Contains(err.Error(), "server returned 413") || !strings.Contains(err.Error(), "upload refused") {
		t.Fatalf("error = %q, want status and body", err.Error())
	}
}

func TestAPIDownloadSendsAuthAndWritesFile(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/buckets/bucket-1/files/file-1/download" {
			t.Fatalf("path = %s, want download path", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("downloaded bytes"))
	}))
	defer server.Close()
	wantAuth := configureTestClient(t, server.URL)
	dest := filepath.Join(t.TempDir(), "out.txt")

	if err := apiDownload("/api/v1/buckets/bucket-1/files/file-1/download", dest); err != nil {
		t.Fatalf("apiDownload returned error: %v", err)
	}
	if gotAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", gotAuth, wantAuth)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(got) != "downloaded bytes" {
		t.Fatalf("downloaded file = %q, want downloaded bytes", string(got))
	}
}

func TestAPIDownloadReturnsServerErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "download missing", http.StatusNotFound)
	}))
	defer server.Close()
	configureTestClient(t, server.URL)

	err := apiDownload("/api/v1/buckets/bucket-1/files/file-1/download", filepath.Join(t.TempDir(), "out.txt"))
	if err == nil {
		t.Fatal("apiDownload returned nil error")
	}
	if !strings.Contains(err.Error(), "server returned 404") || !strings.Contains(err.Error(), "download missing") {
		t.Fatalf("error = %q, want status and body", err.Error())
	}
}

func TestAPIDeleteSendsAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/buckets/bucket-1/files/file-1" {
			t.Fatalf("path = %s, want delete path", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	wantAuth := configureTestClient(t, server.URL)

	if err := apiDelete("/api/v1/buckets/bucket-1/files/file-1"); err != nil {
		t.Fatalf("apiDelete returned error: %v", err)
	}
	if gotAuth != wantAuth {
		t.Fatalf("Authorization = %q, want %q", gotAuth, wantAuth)
	}
}

func TestAPIDeleteReturnsServerErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "delete denied", http.StatusForbidden)
	}))
	defer server.Close()
	configureTestClient(t, server.URL)

	err := apiDelete("/api/v1/buckets/bucket-1/files/file-1")
	if err == nil {
		t.Fatal("apiDelete returned nil error")
	}
	if !strings.Contains(err.Error(), "server returned 403") || !strings.Contains(err.Error(), "delete denied") {
		t.Fatalf("error = %q, want status and body", err.Error())
	}
}
