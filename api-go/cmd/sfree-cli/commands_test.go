package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executeCLI(t *testing.T, args ...string) string {
	t.Helper()
	oldStdout := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = write
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	closeErr := write.Close()
	out, readErr := io.ReadAll(read)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close stdout pipe: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("execute %v returned error: %v\nstdout:\n%s", args, err, string(out))
	}
	os.Stdout = oldStdout
	return string(out)
}

func TestBucketsCreateOutputContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/buckets" {
			t.Fatalf("path = %s, want /api/v1/buckets", r.URL.Path)
		}
		var req createBucketReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Key != "smoke-bucket" || len(req.SourceIDs) != 2 || req.SourceIDs[0] != "src-1" || req.SourceIDs[1] != "src-2" {
			t.Fatalf("request = %#v, want smoke-bucket with source ids", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"smoke-bucket","access_key":"AKIA_TEST","access_secret":"SECRET_TEST","created_at":"2026-04-22T05:00:00Z"}`))
	}))
	defer server.Close()
	configureTestClient(t, server.URL)

	out := executeCLI(t, "buckets", "create", "--key", "smoke-bucket", "--sources", "src-1,src-2")

	for _, want := range []string{
		"Bucket created successfully.",
		"  Bucket Key:      smoke-bucket",
		"  Access Key:      AKIA_TEST",
		"  Access Secret:   SECRET_TEST",
		"  Created:         2026-04-22T05:00:00Z",
		"  aws s3 ls s3://smoke-bucket/ --endpoint-url " + server.URL + "/api/s3",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\noutput:\n%s", want, out)
		}
	}
	if got := awkField(out, "Access Key:"); got != "AKIA_TEST" {
		t.Fatalf("smoke-parsed Access Key = %q, want AKIA_TEST\noutput:\n%s", got, out)
	}
	if got := awkField(out, "Access Secret:"); got != "SECRET_TEST" {
		t.Fatalf("smoke-parsed Access Secret = %q, want SECRET_TEST\noutput:\n%s", got, out)
	}
}

func TestUploadOutputContract(t *testing.T) {
	srcPath := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(srcPath, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write upload source: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/buckets/bucket-1/upload" {
			t.Fatalf("path = %s, want upload path", r.URL.Path)
		}
		part, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		defer func() {
			_ = part.Close()
		}()
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read form file: %v", err)
		}
		if string(body) != "payload" {
			t.Fatalf("uploaded body = %q, want payload", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"file-123","name":"payload.txt","created_at":"2026-04-22T05:00:00Z"}`))
	}))
	defer server.Close()
	configureTestClient(t, server.URL)

	out := executeCLI(t, "upload", "bucket-1", srcPath)

	for _, want := range []string{
		"Uploaded payload.txt",
		"  File ID: file-123",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q\noutput:\n%s", want, out)
		}
	}
	if got := awkField(out, "File ID:"); got != "file-123" {
		t.Fatalf("smoke-parsed File ID = %q, want file-123\noutput:\n%s", got, out)
	}
}

func awkField(output, label string) string {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, label) {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return ""
		}
		return strings.TrimSpace(parts[1])
	}
	return ""
}
