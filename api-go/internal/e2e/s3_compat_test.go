//go:build e2e
// +build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

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
