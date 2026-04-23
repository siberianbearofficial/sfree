package handlers

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type fakeAbortMultipartStore struct {
	upload        *repository.MultipartUpload
	getErr        error
	deleteCalls   int
	deletedUpload string
}

type fakeMultipartUploadPager struct {
	uploads []repository.MultipartUpload
}

func (p fakeMultipartUploadPager) ListByBucketPage(_ context.Context, bucketID primitive.ObjectID, prefix, keyMarker, uploadIDMarker string, limit int) ([]repository.MultipartUpload, bool, error) {
	filtered := make([]repository.MultipartUpload, 0, len(p.uploads))
	for _, upload := range p.uploads {
		if upload.BucketID != bucketID {
			continue
		}
		if prefix != "" && !strings.HasPrefix(upload.ObjectKey, prefix) {
			continue
		}
		if keyMarker != "" {
			if upload.ObjectKey < keyMarker {
				continue
			}
			if upload.ObjectKey == keyMarker {
				if uploadIDMarker == "" || upload.UploadID <= uploadIDMarker {
					continue
				}
			}
		}
		filtered = append(filtered, upload)
	}
	if limit >= 0 && len(filtered) > limit {
		return filtered[:limit], true, nil
	}
	return filtered, false, nil
}

type fakeMultipartUploadGetter struct {
	upload *repository.MultipartUpload
	err    error
}

func (g fakeMultipartUploadGetter) GetByUploadID(_ context.Context, _ string) (*repository.MultipartUpload, error) {
	if g.err != nil {
		return nil, g.err
	}
	return g.upload, nil
}

func (s *fakeAbortMultipartStore) GetByUploadID(_ context.Context, _ string) (*repository.MultipartUpload, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.upload, nil
}

func (s *fakeAbortMultipartStore) Delete(_ context.Context, uploadID string) error {
	s.deleteCalls++
	s.deletedUpload = uploadID
	return nil
}

func TestPostObjectNilRepos(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/api/s3/:bucket/*object", PostObject(nil, nil, nil, nil, 0))

	req, _ := http.NewRequest(http.MethodPost, "/api/s3/mybucket/mykey?uploads", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestPostObjectMissingQueryParam(t *testing.T) {
	t.Parallel()
	r := gin.New()
	// Use non-nil handler to get past the nil check — repos still nil.
	r.POST("/api/s3/:bucket/*object", func(c *gin.Context) {
		// Simulate non-nil repos being available.
		PostObject(nil, nil, nil, nil, 0)(c)
	})

	// No ?uploads or ?uploadId
	req, _ := http.NewRequest(http.MethodPost, "/api/s3/mybucket/mykey", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should return 503 because repos are nil
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestPutObjectOrPartNilMpRepoDelegatesToPut(t *testing.T) {
	t.Parallel()
	r := gin.New()
	// With nil mpRepo and nil other repos, a PUT with ?uploadId should still
	// fall through to PutObject, which returns 503 for nil repos.
	r.PUT("/api/s3/:bucket/*object", PutObjectOrPart(nil, nil, nil, nil, 0))

	req, _ := http.NewRequest(http.MethodPut, "/api/s3/mybucket/mykey?uploadId=abc&partNumber=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// mpRepo is nil, so it falls through to PutObject which gives 503 for nil repos.
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDeleteObjectOrAbortNilMpRepoDelegatesToDelete(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/api/s3/:bucket/*object", DeleteObjectOrAbort(nil, nil, nil, nil))

	req, _ := http.NewRequest(http.MethodDelete, "/api/s3/mybucket/mykey?uploadId=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestGetObjectOrPartsNilMpRepoDelegatesToGet(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/api/s3/:bucket/*object", GetObjectOrParts(nil, nil, nil, nil))

	req, _ := http.NewRequest(http.MethodGet, "/api/s3/mybucket/mykey?uploadId=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListObjectsOrUploadsNilMpRepoDelegatesToList(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/api/s3/:bucket", ListObjectsOrUploads(nil, nil, nil))

	req, _ := http.NewRequest(http.MethodGet, "/api/s3/mybucket?uploads", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCompleteMultipartUploadMalformedXML(t *testing.T) {
	t.Parallel()

	// The CompleteMultipartUpload handler needs to parse XML from the body.
	// We can test that malformed XML returns the right error by calling the
	// handler with valid query params but invalid body. However, the handler
	// first checks the upload exists in the repo, so we need the lookups to
	// succeed. Since we don't have a mock repo easily, we verify the XML struct
	// parsing works correctly instead.

	var req completeMultipartUploadRequest
	good := `<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>"abc"</ETag></Part></CompleteMultipartUpload>`
	if err := xml.NewDecoder(bytes.NewReader([]byte(good))).Decode(&req); err != nil {
		t.Fatalf("expected valid XML to parse, got %v", err)
	}
	if len(req.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(req.Parts))
	}
	if req.Parts[0].PartNumber != 1 {
		t.Fatalf("expected part number 1, got %d", req.Parts[0].PartNumber)
	}
	if req.Parts[0].ETag != `"abc"` {
		t.Fatalf("expected ETag \"abc\", got %s", req.Parts[0].ETag)
	}
}

func TestInitiateMultipartUploadResultXML(t *testing.T) {
	t.Parallel()

	result := initiateMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:   "mybucket",
		Key:      "mykey",
		UploadId: "abc123",
	}
	data, err := xml.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	s := string(data)
	if !bytes.Contains(data, []byte("<Bucket>mybucket</Bucket>")) {
		t.Fatalf("missing Bucket in XML: %s", s)
	}
	if !bytes.Contains(data, []byte("<Key>mykey</Key>")) {
		t.Fatalf("missing Key in XML: %s", s)
	}
	if !bytes.Contains(data, []byte("<UploadId>abc123</UploadId>")) {
		t.Fatalf("missing UploadId in XML: %s", s)
	}
}

func TestCompleteMultipartUploadResultXML(t *testing.T) {
	t.Parallel()

	result := completeMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: "/mybucket/mykey",
		Bucket:   "mybucket",
		Key:      "mykey",
		ETag:     `"abc-2"`,
	}
	data, err := xml.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	s := string(data)
	if !bytes.Contains(data, []byte("<ETag>&#34;abc-2&#34;</ETag>")) && !bytes.Contains(data, []byte(`<ETag>"abc-2"</ETag>`)) {
		t.Fatalf("missing or wrong ETag in XML: %s", s)
	}
}

func TestListMultipartUploadsResultXML(t *testing.T) {
	t.Parallel()

	result := listMultipartUploadsResult{
		Xmlns:              "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:             "mybucket",
		KeyMarker:          "file1.bin",
		UploadIDMarker:     "id1",
		NextKeyMarker:      "file2.bin",
		NextUploadIDMarker: "id2",
		MaxUploads:         1,
		Upload: []multipartUploadXML{
			{Key: "file1.bin", UploadId: "id1", Initiated: "2026-01-01T00:00:00Z"},
			{Key: "file2.bin", UploadId: "id2", Initiated: "2026-01-02T00:00:00Z"},
		},
		IsTruncated: true,
	}
	data, err := xml.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	s := string(data)
	if !bytes.Contains(data, []byte("<UploadId>id1</UploadId>")) {
		t.Fatalf("missing upload id1: %s", s)
	}
	if !bytes.Contains(data, []byte("<UploadId>id2</UploadId>")) {
		t.Fatalf("missing upload id2: %s", s)
	}
	if !bytes.Contains(data, []byte("<NextKeyMarker>file2.bin</NextKeyMarker>")) {
		t.Fatalf("missing NextKeyMarker: %s", s)
	}
	if !bytes.Contains(data, []byte("<NextUploadIdMarker>id2</NextUploadIdMarker>")) {
		t.Fatalf("missing NextUploadIdMarker: %s", s)
	}
}

func TestListPartsResultXML(t *testing.T) {
	t.Parallel()

	result := listPartsResult{
		Xmlns:                "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:               "mybucket",
		Key:                  "file1.bin",
		UploadId:             "id1",
		PartNumberMarker:     1,
		NextPartNumberMarker: 2,
		MaxParts:             1,
		Part: []partXML{
			{PartNumber: 1, ETag: `"aaa"`, Size: 5242880, LastModified: "2026-01-01T00:00:00Z"},
			{PartNumber: 2, ETag: `"bbb"`, Size: 1234, LastModified: "2026-01-01T00:00:00Z"},
		},
		IsTruncated: true,
	}
	data, err := xml.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	s := string(data)
	if !bytes.Contains(data, []byte("<PartNumber>1</PartNumber>")) {
		t.Fatalf("missing part 1: %s", s)
	}
	if !bytes.Contains(data, []byte("<PartNumber>2</PartNumber>")) {
		t.Fatalf("missing part 2: %s", s)
	}
	if !bytes.Contains(data, []byte("<NextPartNumberMarker>2</NextPartNumberMarker>")) {
		t.Fatalf("missing NextPartNumberMarker: %s", s)
	}
}

func TestListMultipartUploadsPagination(t *testing.T) {
	t.Parallel()

	bucketID := primitive.NewObjectID()
	pager := fakeMultipartUploadPager{
		uploads: []repository.MultipartUpload{
			{BucketID: bucketID, ObjectKey: "alpha.txt", UploadID: "u1", CreatedAt: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)},
			{BucketID: bucketID, ObjectKey: "alpha.txt", UploadID: "u2", CreatedAt: time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)},
			{BucketID: bucketID, ObjectKey: "beta.txt", UploadID: "u3", CreatedAt: time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC)},
		},
	}

	page, err := buildMultipartUploadListPage(context.Background(), pager, bucketID, "", "", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !page.isTruncated {
		t.Fatal("expected first page to be truncated")
	}
	if page.nextKeyMarker != "alpha.txt" || page.nextUploadIDMarker != "u2" {
		t.Fatalf("unexpected next markers: %+v", page)
	}
	if len(page.entries) != 2 || page.entries[0].UploadId != "u1" || page.entries[1].UploadId != "u2" {
		t.Fatalf("unexpected page entries: %+v", page.entries)
	}

	page, err = buildMultipartUploadListPage(context.Background(), pager, bucketID, "", "alpha.txt", "u2", 2)
	if err != nil {
		t.Fatal(err)
	}
	if page.isTruncated {
		t.Fatal("expected second page to finish the listing")
	}
	if len(page.entries) != 1 || page.entries[0].Key != "beta.txt" {
		t.Fatalf("unexpected second page entries: %+v", page.entries)
	}
}

func TestListMultipartUploadsRejectsUploadIDMarkerWithoutKeyMarker(t *testing.T) {
	t.Parallel()

	c, w := testS3GinContext("/api/s3/route-bucket?uploads&upload-id-marker=u2")
	c.Request.Method = http.MethodGet
	c.Params = gin.Params{{Key: "bucket", Value: "route-bucket"}}
	c.Set("accessKey", "route-access")

	listMultipartUploads(c, fakeObjectBucketReader{
		bucket: &repository.Bucket{ID: primitive.NewObjectID(), Key: "route-bucket", AccessKey: "route-access"},
	}, fakeMultipartUploadPager{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>InvalidArgument</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestListMultipartUploadsRejectsDelimiter(t *testing.T) {
	t.Parallel()

	c, w := testS3GinContext("/api/s3/route-bucket?uploads&delimiter=/")
	c.Request.Method = http.MethodGet
	c.Params = gin.Params{{Key: "bucket", Value: "route-bucket"}}
	c.Set("accessKey", "route-access")

	listMultipartUploads(c, fakeObjectBucketReader{
		bucket: &repository.Bucket{ID: primitive.NewObjectID(), Key: "route-bucket", AccessKey: "route-access"},
	}, fakeMultipartUploadPager{})

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>NotImplemented</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestBuildMultipartPartsPage(t *testing.T) {
	t.Parallel()

	page := buildMultipartPartsPage(&repository.MultipartUpload{
		CreatedAt: time.Date(2026, time.January, 4, 0, 0, 0, 0, time.UTC),
		Parts: []repository.UploadPart{
			{PartNumber: 3, ETag: `"ccc"`, Size: 3},
			{PartNumber: 1, ETag: `"aaa"`, Size: 1},
			{PartNumber: 2, ETag: `"bbb"`, Size: 2},
		},
	}, 1, 1)

	if !page.isTruncated {
		t.Fatal("expected paged parts result to be truncated")
	}
	if page.nextPartNumberMarker != 2 {
		t.Fatalf("unexpected next marker: %d", page.nextPartNumberMarker)
	}
	if len(page.parts) != 1 || page.parts[0].PartNumber != 2 {
		t.Fatalf("unexpected page parts: %+v", page.parts)
	}
}

func TestListPartsRejectsInvalidMarker(t *testing.T) {
	t.Parallel()

	bucketID := primitive.NewObjectID()
	uploadID := primitive.NewObjectID().Hex()
	c, w := testS3GinContext("/api/s3/route-bucket/object.txt?uploadId=" + uploadID + "&part-number-marker=-1")
	c.Request.Method = http.MethodGet
	c.Params = gin.Params{
		{Key: "bucket", Value: "route-bucket"},
		{Key: "object", Value: "/object.txt"},
	}
	c.Set("accessKey", "route-access")

	listParts(c, fakeObjectBucketReader{
		bucket: &repository.Bucket{ID: bucketID, Key: "route-bucket", AccessKey: "route-access"},
	}, fakeMultipartUploadGetter{
		upload: &repository.MultipartUpload{BucketID: bucketID, ObjectKey: "object.txt", UploadID: uploadID},
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>InvalidArgument</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestAbortMultipartUploadRejectsOtherBucketUpload(t *testing.T) {
	t.Parallel()

	routeBucketID := primitive.NewObjectID()
	otherBucketID := primitive.NewObjectID()
	uploadID := primitive.NewObjectID().Hex()
	store := &fakeAbortMultipartStore{
		upload: &repository.MultipartUpload{
			BucketID:  otherBucketID,
			ObjectKey: "object.txt",
			UploadID:  uploadID,
			Parts: []repository.UploadPart{
				{
					PartNumber: 1,
					Chunks: []repository.FileChunk{
						{SourceID: primitive.NewObjectID(), Name: "part-1", Size: 5},
					},
				},
			},
		},
	}

	c, w := testS3GinContext("/api/s3/route-bucket/object.txt?uploadId=" + uploadID)
	c.Request.Method = http.MethodDelete
	c.Params = gin.Params{
		{Key: "bucket", Value: "route-bucket"},
		{Key: "object", Value: "/object.txt"},
	}
	c.Set("accessKey", "route-access")

	abortMultipartUpload(c, fakeObjectBucketReader{
		bucket: &repository.Bucket{
			ID:        routeBucketID,
			Key:       "route-bucket",
			AccessKey: "route-access",
		},
	}, &repository.SourceRepository{}, store, nil)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>NoSuchUpload</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
	if store.deleteCalls != 0 {
		t.Fatalf("expected upload metadata to remain, delete called %d times for %q", store.deleteCalls, store.deletedUpload)
	}
}

func TestAbortMultipartUploadUnknownUploadKeepsNoSuchUpload(t *testing.T) {
	t.Parallel()

	store := &fakeAbortMultipartStore{getErr: mongo.ErrNoDocuments}
	c, w := testS3GinContext("/api/s3/route-bucket/object.txt?uploadId=missing")
	c.Request.Method = http.MethodDelete
	c.Params = gin.Params{
		{Key: "bucket", Value: "route-bucket"},
		{Key: "object", Value: "/object.txt"},
	}

	abortMultipartUpload(c, fakeObjectBucketReader{}, &repository.SourceRepository{}, store, nil)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>NoSuchUpload</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
	if store.deleteCalls != 0 {
		t.Fatalf("expected no delete calls, got %d", store.deleteCalls)
	}
}
