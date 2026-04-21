package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestDecodeDeleteObjectsRequestParsesQuietAndObjects(t *testing.T) {
	t.Parallel()

	body := `<Delete xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Quiet>true</Quiet><Object><Key>a.txt</Key><VersionId>v1</VersionId></Object><Ignored><Nested>value</Nested></Ignored></Delete>`
	req, err := decodeDeleteObjectsRequest(strings.NewReader(body))
	if err != nil {
		t.Fatalf("decode DeleteObjects request: %v", err)
	}
	if !req.Quiet {
		t.Fatal("expected quiet mode")
	}
	if len(req.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(req.Objects))
	}
	if req.Objects[0].Key != "a.txt" || req.Objects[0].VersionID != "v1" {
		t.Fatalf("unexpected object: %+v", req.Objects[0])
	}
}

func TestDecodeDeleteObjectsRequestRejectsTooManyObjectsDuringParse(t *testing.T) {
	t.Parallel()

	var body strings.Builder
	body.WriteString("<Delete>")
	for i := 0; i < maxDeleteObjects+1; i++ {
		fmt.Fprintf(&body, "<Object><Key>too-many-%d</Key></Object>", i)
	}
	body.WriteString("</Delete>")

	req, err := decodeDeleteObjectsRequest(strings.NewReader(body.String()))
	if !errors.Is(err, errDeleteObjectsTooMany) {
		t.Fatalf("expected too many objects error, got %v", err)
	}
	if len(req.Objects) != maxDeleteObjects {
		t.Fatalf("expected parser to retain only %d objects, got %d", maxDeleteObjects, len(req.Objects))
	}
}

func TestDecodeDeleteObjectsRequestRejectsMalformedRoot(t *testing.T) {
	t.Parallel()

	_, err := decodeDeleteObjectsRequest(strings.NewReader("<NotDelete></NotDelete>"))
	if !errors.Is(err, errDeleteObjectsMalformedXML) {
		t.Fatalf("expected malformed XML error, got %v", err)
	}
}

func TestParseCopySource(t *testing.T) {
	t.Parallel()

	bucket, key, ok := parseCopySource("/bucket/a%20b/c.txt")
	if !ok {
		t.Fatal("expected copy source to parse")
	}
	if bucket != "bucket" || key != "a b/c.txt" {
		t.Fatalf("unexpected copy source: bucket=%q key=%q", bucket, key)
	}

	if _, _, ok := parseCopySource("/bucket"); ok {
		t.Fatal("expected missing key to fail")
	}
}

func TestParseObjectRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		raw   string
		total int64
		want  objectRange
		ok    bool
	}{
		{name: "valid bounded", raw: "bytes=2-5", total: 10, want: objectRange{start: 2, end: 5}, ok: true},
		{name: "valid open ended", raw: "bytes=7-", total: 10, want: objectRange{start: 7, end: 9}, ok: true},
		{name: "valid suffix", raw: "bytes=-4", total: 10, want: objectRange{start: 6, end: 9}, ok: true},
		{name: "suffix longer than object", raw: "bytes=-40", total: 10, want: objectRange{start: 0, end: 9}, ok: true},
		{name: "invalid unit", raw: "items=0-1", total: 10},
		{name: "invalid reversed", raw: "bytes=5-2", total: 10},
		{name: "invalid suffix zero", raw: "bytes=-0", total: 10},
		{name: "invalid multi range", raw: "bytes=0-1,3-4", total: 10},
		{name: "out of bounds start", raw: "bytes=10-", total: 10},
		{name: "zero size object", raw: "bytes=0-0", total: 0},
		{name: "end clamped", raw: "bytes=8-20", total: 10, want: objectRange{start: 8, end: 9}, ok: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseObjectRange(tt.raw, tt.total)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, ok)
			}
			if got != tt.want {
				t.Fatalf("expected range %+v, got %+v", tt.want, got)
			}
		})
	}
}

func TestParseListMaxKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		query      string
		want       int
		ok         bool
		wantStatus int
	}{
		{name: "default", want: 1000, ok: true, wantStatus: http.StatusOK},
		{name: "zero", query: "?max-keys=0", want: 0, ok: true, wantStatus: http.StatusOK},
		{name: "cap at 1000", query: "?max-keys=5000", want: 1000, ok: true, wantStatus: http.StatusOK},
		{name: "invalid", query: "?max-keys=abc", ok: false, wantStatus: http.StatusBadRequest},
		{name: "negative", query: "?max-keys=-1", ok: false, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, w := testS3GinContext("/bucket" + tt.query)
			got, ok := parseListMaxKeys(c)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, ok)
			}
			if got != tt.want {
				t.Fatalf("expected max keys %d, got %d", tt.want, got)
			}
			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
			if !tt.ok && !strings.Contains(w.Body.String(), "<Code>InvalidArgument</Code>") {
				t.Fatalf("expected InvalidArgument S3 error, got %q", w.Body.String())
			}
		})
	}
}

func TestFileListBucketEntryAndObjectETag(t *testing.T) {
	t.Parallel()

	sourceID := primitive.NewObjectID()
	file := repository.File{
		Name:      "dir/object.txt",
		CreatedAt: time.Date(2026, 4, 21, 17, 0, 1, 987654321, time.FixedZone("UTC+2", 2*60*60)),
		Chunks: []repository.FileChunk{
			{SourceID: sourceID, Name: "chunk-1", Order: 0, Size: 7},
			{SourceID: sourceID, Name: "chunk-2", Order: 1, Size: 5},
		},
	}

	entry := fileListBucketEntry(file)
	if entry.Key != file.Name {
		t.Fatalf("expected key %q, got %q", file.Name, entry.Key)
	}
	if entry.LastModified != "2026-04-21T15:00:01Z" {
		t.Fatalf("unexpected last modified: %q", entry.LastModified)
	}
	if entry.Size != 12 {
		t.Fatalf("expected size 12, got %d", entry.Size)
	}
	if entry.StorageClass != "STANDARD" {
		t.Fatalf("expected STANDARD storage class, got %q", entry.StorageClass)
	}
	if !strings.HasPrefix(entry.ETag, "\"") || !strings.HasSuffix(entry.ETag, "\"") {
		t.Fatalf("expected quoted ETag, got %q", entry.ETag)
	}
	if entry.ETag != manager.ObjectETag(file) {
		t.Fatalf("expected entry ETag to match ObjectETag")
	}

	changed := file
	changed.Chunks[1].Size = 6
	if manager.ObjectETag(changed) == entry.ETag {
		t.Fatal("expected ETag to change when chunk metadata changes")
	}
}

func TestBuildListBucketPageDelimiterCommonPrefixes(t *testing.T) {
	t.Parallel()

	bucketID := primitive.NewObjectID()
	pager := fakeListBucketFilePager{files: []repository.File{
		testListFile(bucketID, "docs/a.txt"),
		testListFile(bucketID, "docs/b.txt"),
		testListFile(bucketID, "photos/1.jpg"),
		testListFile(bucketID, "root.txt"),
	}}

	contents, commonPrefixes, isTruncated, nextToken, err := buildListBucketPage(context.Background(), pager, bucketID, "", "/", "", 3)
	if err != nil {
		t.Fatalf("build list bucket page: %v", err)
	}
	if isTruncated {
		t.Fatal("expected untruncated page")
	}
	if nextToken != "" {
		t.Fatalf("expected empty next token, got %q", nextToken)
	}
	if got := entryKeys(contents); strings.Join(got, ",") != "root.txt" {
		t.Fatalf("unexpected contents: %v", got)
	}
	if got := commonPrefixKeys(commonPrefixes); strings.Join(got, ",") != "docs/,photos/" {
		t.Fatalf("unexpected common prefixes: %v", got)
	}
}

func TestBuildListBucketPageContinuationToken(t *testing.T) {
	t.Parallel()

	bucketID := primitive.NewObjectID()
	pager := fakeListBucketFilePager{files: []repository.File{
		testListFile(bucketID, "a.txt"),
		testListFile(bucketID, "b.txt"),
		testListFile(bucketID, "c.txt"),
	}}

	contents, commonPrefixes, isTruncated, nextToken, err := buildListBucketPage(context.Background(), pager, bucketID, "", "", "", 2)
	if err != nil {
		t.Fatalf("build first page: %v", err)
	}
	if !isTruncated {
		t.Fatal("expected first page to be truncated")
	}
	if nextToken != "b.txt" {
		t.Fatalf("expected next token b.txt, got %q", nextToken)
	}
	if len(commonPrefixes) != 0 {
		t.Fatalf("expected no common prefixes, got %v", commonPrefixes)
	}
	if got := entryKeys(contents); strings.Join(got, ",") != "a.txt,b.txt" {
		t.Fatalf("unexpected first page contents: %v", got)
	}

	contents, commonPrefixes, isTruncated, nextToken, err = buildListBucketPage(context.Background(), pager, bucketID, "", "", nextToken, 2)
	if err != nil {
		t.Fatalf("build second page: %v", err)
	}
	if isTruncated {
		t.Fatal("expected second page to be complete")
	}
	if nextToken != "" {
		t.Fatalf("expected empty final token, got %q", nextToken)
	}
	if len(commonPrefixes) != 0 {
		t.Fatalf("expected no common prefixes, got %v", commonPrefixes)
	}
	if got := entryKeys(contents); strings.Join(got, ",") != "c.txt" {
		t.Fatalf("unexpected second page contents: %v", got)
	}
}

func TestBuildListBucketPageMaxKeysZero(t *testing.T) {
	t.Parallel()

	bucketID := primitive.NewObjectID()
	pager := fakeListBucketFilePager{files: []repository.File{
		testListFile(bucketID, "a.txt"),
		testListFile(bucketID, "b.txt"),
	}}

	contents, commonPrefixes, isTruncated, nextToken, err := buildListBucketPage(context.Background(), pager, bucketID, "", "", "", 0)
	if err != nil {
		t.Fatalf("build list bucket page: %v", err)
	}
	if len(contents) != 0 || len(commonPrefixes) != 0 {
		t.Fatalf("expected empty max-keys=0 result, got contents=%v commonPrefixes=%v", contents, commonPrefixes)
	}
	if !isTruncated {
		t.Fatal("expected max-keys=0 to report truncation when keys exist")
	}
	if nextToken != "a.txt" {
		t.Fatalf("expected first key as next token, got %q", nextToken)
	}
}

type fakeListBucketFilePager struct {
	files []repository.File
}

func (p fakeListBucketFilePager) ListByBucketWithPrefixPage(_ context.Context, bucketID primitive.ObjectID, prefix, after string, limit int) ([]repository.File, bool, error) {
	var filtered []repository.File
	for _, file := range p.files {
		if file.BucketID != bucketID || !strings.HasPrefix(file.Name, prefix) || file.Name <= after {
			continue
		}
		filtered = append(filtered, file)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	if limit >= 0 && len(filtered) > limit {
		return filtered[:limit], true, nil
	}
	return filtered, false, nil
}

func testS3GinContext(target string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return c, w
}

func testListFile(bucketID primitive.ObjectID, name string) repository.File {
	return repository.File{
		BucketID:  bucketID,
		Name:      name,
		CreatedAt: time.Unix(1700000000, 0).UTC(),
		Chunks: []repository.FileChunk{
			{SourceID: primitive.NewObjectID(), Name: name + ".chunk", Size: int64(len(name))},
		},
	}
}

func entryKeys(entries []listBucketEntry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}

func commonPrefixKeys(prefixes []listCommonPrefix) []string {
	keys := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		keys = append(keys, prefix.Prefix)
	}
	return keys
}
