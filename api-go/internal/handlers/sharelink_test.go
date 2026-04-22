package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeShareBucketReader struct {
	bucket *repository.Bucket
}

func (r fakeShareBucketReader) GetByID(_ context.Context, _ primitive.ObjectID) (*repository.Bucket, error) {
	return r.bucket, nil
}

type fakeShareFileReader struct {
	file *repository.File
}

func (r fakeShareFileReader) GetByID(_ context.Context, _ primitive.ObjectID) (*repository.File, error) {
	return r.file, nil
}

type fakeShareLinkCreator struct {
	created []repository.ShareLink
}

func (r *fakeShareLinkCreator) Create(_ context.Context, sl repository.ShareLink) (*repository.ShareLink, error) {
	sl.ID = primitive.NewObjectID()
	r.created = append(r.created, sl)
	return &sl, nil
}

func TestCreateShareLinkAcceptsEmptyBody(t *testing.T) {
	t.Parallel()
	creator, w := serveCreateShareLinkTest(t, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(creator.created) != 1 {
		t.Fatalf("expected one share link, got %d", len(creator.created))
	}
	if creator.created[0].ExpiresAt != nil {
		t.Fatalf("expected no expiry, got %v", creator.created[0].ExpiresAt)
	}
}

func TestCreateShareLinkAcceptsValidExpiryBody(t *testing.T) {
	t.Parallel()
	body := []byte(`{"expires_in":3600}`)
	creator, w := serveCreateShareLinkTest(t, body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(creator.created) != 1 {
		t.Fatalf("expected one share link, got %d", len(creator.created))
	}
	if creator.created[0].ExpiresAt == nil {
		t.Fatal("expected expiry to be set")
	}
	if time.Until(*creator.created[0].ExpiresAt) <= 0 {
		t.Fatalf("expected future expiry, got %v", creator.created[0].ExpiresAt)
	}
}

func TestCreateShareLinkRejectsMalformedJSONBody(t *testing.T) {
	t.Parallel()
	creator, w := serveCreateShareLinkTest(t, []byte(`{"expires_in":`))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if len(creator.created) != 0 {
		t.Fatalf("expected no share links, got %d", len(creator.created))
	}
}

func TestCreateShareLinkRejectsWrongTypeExpiresIn(t *testing.T) {
	t.Parallel()
	body, err := json.Marshal(map[string]any{"expires_in": "3600"})
	if err != nil {
		t.Fatal(err)
	}
	creator, w := serveCreateShareLinkTest(t, body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if len(creator.created) != 0 {
		t.Fatalf("expected no share links, got %d", len(creator.created))
	}
}

func serveCreateShareLinkTest(t *testing.T, body []byte) (*fakeShareLinkCreator, *httptest.ResponseRecorder) {
	t.Helper()
	ownerID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	creator := &fakeShareLinkCreator{}
	router := gin.New()
	router.POST(
		"/buckets/:id/files/:file_id/share",
		setUserID(ownerID.Hex()),
		createShareLink(
			fakeShareBucketReader{bucket: &repository.Bucket{ID: bucketID, UserID: ownerID}},
			fakeShareFileReader{file: &repository.File{ID: fileID, BucketID: bucketID, Name: "report.pdf"}},
			creator,
			nil,
		),
	)

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	req, _ := http.NewRequest(http.MethodPost, "/buckets/"+bucketID.Hex()+"/files/"+fileID.Hex()+"/share", reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return creator, w
}
