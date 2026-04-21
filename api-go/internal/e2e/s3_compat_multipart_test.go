//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
)

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

func TestS3CompatMalformedUnsupportedMultipartErrors(t *testing.T) {
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

	assertError := func(name string, gotStatus, wantStatus int, body []byte, wantCode string) {
		t.Helper()
		if gotStatus != wantStatus {
			t.Fatalf("%s: expected status %d, got %d: %s", name, wantStatus, gotStatus, body)
		}
		assertS3Error(t, body, wantCode)
	}

	objectKey := "malformed-errors-" + suffix + ".txt"
	status, body := s3Do(t, http.MethodPost, s3URL(objectKey, nil), bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	assertError("POST object without multipart query", status, http.StatusBadRequest, body, "InvalidRequest")

	copyHeaders := map[string]string{"x-amz-copy-source": "/" + bucket.Key + "/source-" + suffix + ".txt"}
	status, _, body = s3DoWithHeaders(t, http.MethodPut, s3URL(objectKey, url.Values{"uploadId": {"upload-part-copy-id"}}), bucket.AccessKey, bucket.AccessSecret, env.Region, nil, copyHeaders)
	assertError("UploadPartCopy", status, http.StatusNotImplemented, body, "NotImplemented")

	createURL := s3URL(objectKey, url.Values{"uploads": {""}})
	status, body = s3Do(t, http.MethodPost, createURL, bucket.AccessKey, bucket.AccessSecret, env.Region, nil)
	if status != http.StatusOK {
		t.Fatalf("CreateMultipartUpload: expected 200, got %d: %s", status, body)
	}
	var createResult struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(body, &createResult); err != nil || createResult.UploadID == "" {
		t.Fatalf("CreateMultipartUpload decode: uploadId=%q err=%v body=%s", createResult.UploadID, err, body)
	}

	completeURL := s3URL(objectKey, url.Values{"uploadId": {createResult.UploadID}})
	status, body = s3Do(t, http.MethodPost, completeURL, bucket.AccessKey, bucket.AccessSecret, env.Region, []byte("<CompleteMultipartUpload><Part>"))
	assertError("malformed CompleteMultipartUpload XML", status, http.StatusBadRequest, body, "MalformedXML")

	missingUploadURL := s3URL("missing-upload-"+suffix+".txt", url.Values{"uploadId": {"missing-upload-id"}})
	status, body = s3Do(t, http.MethodPost, missingUploadURL, bucket.AccessKey, bucket.AccessSecret, env.Region, []byte("<CompleteMultipartUpload/>"))
	assertError("unknown multipart upload ID", status, http.StatusNotFound, body, "NoSuchUpload")
}
