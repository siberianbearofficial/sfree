//go:build e2e
// +build e2e

package e2e

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

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
