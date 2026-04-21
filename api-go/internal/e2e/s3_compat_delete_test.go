//go:build e2e
// +build e2e

package e2e

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"testing"
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
}
