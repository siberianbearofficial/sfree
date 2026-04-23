package manager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type fakeObjectSources struct {
	sources []repository.Source
	err     error
}

func (f *fakeObjectSources) ListByIDs(context.Context, []primitive.ObjectID) ([]repository.Source, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]repository.Source(nil), f.sources...), nil
}

type fakeObjectFiles struct {
	byName     map[string]repository.File
	countErr   error
	replaceErr error
	deleteErr  error
	listErr    error
	events     *[]string
}

func newFakeObjectFiles(files ...repository.File) *fakeObjectFiles {
	f := &fakeObjectFiles{byName: make(map[string]repository.File)}
	for _, file := range files {
		if file.ID.IsZero() {
			file.ID = primitive.NewObjectID()
		}
		f.byName[fileKey(file.BucketID, file.Name)] = file
	}
	return f
}

func (f *fakeObjectFiles) ReplaceByName(_ context.Context, file repository.File) (*repository.File, *repository.File, error) {
	if f.replaceErr != nil {
		return nil, nil, f.replaceErr
	}
	if file.ID.IsZero() {
		file.ID = primitive.NewObjectID()
	}
	key := fileKey(file.BucketID, file.Name)
	previous, ok := f.byName[key]
	if ok {
		file.ID = previous.ID
		f.byName[key] = file
		return &file, &previous, nil
	}
	f.byName[key] = file
	return &file, nil, nil
}

func (f *fakeObjectFiles) GetByName(_ context.Context, bucketID primitive.ObjectID, name string) (*repository.File, error) {
	file, ok := f.byName[fileKey(bucketID, name)]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	return &file, nil
}

func (f *fakeObjectFiles) GetByID(_ context.Context, id primitive.ObjectID) (*repository.File, error) {
	for _, file := range f.byName {
		if file.ID == id {
			return &file, nil
		}
	}
	return nil, mongo.ErrNoDocuments
}

func (f *fakeObjectFiles) Delete(_ context.Context, id primitive.ObjectID) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for key, file := range f.byName {
		if file.ID == id {
			delete(f.byName, key)
			return nil
		}
	}
	return mongo.ErrNoDocuments
}

func (f *fakeObjectFiles) ListByBucket(_ context.Context, bucketID primitive.ObjectID) ([]repository.File, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var files []repository.File
	for _, file := range f.byName {
		if file.BucketID == bucketID {
			files = append(files, file)
		}
	}
	return files, nil
}

func (f *fakeObjectFiles) DeleteByBucket(_ context.Context, bucketID primitive.ObjectID) error {
	if f.events != nil {
		*f.events = append(*f.events, "delete file metadata")
	}
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for key, file := range f.byName {
		if file.BucketID == bucketID {
			delete(f.byName, key)
		}
	}
	return nil
}

func (f *fakeObjectFiles) CountByChunk(_ context.Context, sourceID primitive.ObjectID, name string) (int64, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	var count int64
	for _, file := range f.byName {
		for _, chunk := range file.Chunks {
			if chunk.SourceID == sourceID && chunk.Name == name {
				count++
				break
			}
		}
	}
	return count, nil
}

func (f *fakeObjectFiles) CountByChunkExcludingBucket(_ context.Context, bucketID, sourceID primitive.ObjectID, name string) (int64, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	var count int64
	for _, file := range f.byName {
		if file.BucketID == bucketID {
			continue
		}
		for _, chunk := range file.Chunks {
			if chunk.SourceID == sourceID && chunk.Name == name {
				count++
				break
			}
		}
	}
	return count, nil
}

type fakeMultipartUploads struct {
	uploads map[string]repository.MultipartUpload
	setErr  error
	delErr  error
	listErr error
	events  *[]string
}

func (f *fakeMultipartUploads) GetByUploadID(_ context.Context, uploadID string) (*repository.MultipartUpload, error) {
	upload, ok := f.uploads[uploadID]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	return &upload, nil
}

func (f *fakeMultipartUploads) SetPart(_ context.Context, uploadID string, part repository.UploadPart) (*repository.UploadPart, error) {
	if f.events != nil {
		*f.events = append(*f.events, "set part")
	}
	if f.setErr != nil {
		return nil, f.setErr
	}
	upload, ok := f.uploads[uploadID]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	var previous *repository.UploadPart
	parts := make([]repository.UploadPart, 0, len(upload.Parts)+1)
	for _, existing := range upload.Parts {
		if existing.PartNumber != part.PartNumber {
			parts = append(parts, existing)
			continue
		}
		partCopy := existing
		previous = &partCopy
	}
	parts = append(parts, part)
	upload.Parts = parts
	f.uploads[uploadID] = upload
	return previous, nil
}

func (f *fakeMultipartUploads) Delete(_ context.Context, uploadID string) error {
	if f.delErr != nil {
		return f.delErr
	}
	if _, ok := f.uploads[uploadID]; !ok {
		return mongo.ErrNoDocuments
	}
	delete(f.uploads, uploadID)
	return nil
}

func (f *fakeMultipartUploads) ListByBucket(_ context.Context, bucketID primitive.ObjectID) ([]repository.MultipartUpload, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var uploads []repository.MultipartUpload
	for _, upload := range f.uploads {
		if upload.BucketID == bucketID {
			uploads = append(uploads, upload)
		}
	}
	return uploads, nil
}

func (f *fakeMultipartUploads) CountByPartChunk(_ context.Context, sourceID primitive.ObjectID, name string) (int64, error) {
	var count int64
	for _, upload := range f.uploads {
		for _, part := range upload.Parts {
			for _, chunk := range part.Chunks {
				if chunk.SourceID == sourceID && chunk.Name == name {
					count++
					break
				}
			}
		}
	}
	return count, nil
}

func (f *fakeMultipartUploads) CountByPartChunkExcludingBucket(_ context.Context, bucketID, sourceID primitive.ObjectID, name string) (int64, error) {
	var count int64
	for _, upload := range f.uploads {
		if upload.BucketID == bucketID {
			continue
		}
		for _, part := range upload.Parts {
			for _, chunk := range part.Chunks {
				if chunk.SourceID == sourceID && chunk.Name == name {
					count++
					break
				}
			}
		}
	}
	return count, nil
}

func (f *fakeMultipartUploads) DeleteByBucket(_ context.Context, bucketID primitive.ObjectID) error {
	if f.events != nil {
		*f.events = append(*f.events, "delete multipart metadata")
	}
	if f.delErr != nil {
		return f.delErr
	}
	for uploadID, upload := range f.uploads {
		if upload.BucketID == bucketID {
			delete(f.uploads, uploadID)
		}
	}
	return nil
}

func TestObjectServiceUploadMultipartPartReplacesPartThenDeletesPreviousChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	oldChunk := repository.FileChunk{SourceID: sourceID, Name: "old-part-chunk", Size: 5}
	var deleted []repository.FileChunk
	var events []string
	svc := testObjectService(newFakeObjectFiles(), &deleted)
	svc.sources = &fakeObjectSources{sources: []repository.Source{{ID: sourceID}}}
	svc.multipart = &fakeMultipartUploads{
		uploads: map[string]repository.MultipartUpload{
			"upload-1": {
				BucketID: bucketID,
				UploadID: "upload-1",
				Parts: []repository.UploadPart{
					{PartNumber: 1, ETag: `"old"`, Chunks: []repository.FileChunk{oldChunk}},
				},
			},
		},
		events: &events,
	}
	svc.uploadChunks = func(_ context.Context, r io.Reader, _ []repository.Source, _ int, _ SourceSelector) ([]repository.FileChunk, error) {
		if _, err := io.Copy(io.Discard, r); err != nil {
			return nil, err
		}
		return []repository.FileChunk{{SourceID: sourceID, Name: "new-part-chunk", Size: 4}}, nil
	}
	svc.deleteChunks = func(_ context.Context, chunks []repository.FileChunk) error {
		deleted = append(deleted, chunks...)
		for _, chunk := range chunks {
			events = append(events, "delete "+chunk.Name)
		}
		return nil
	}

	result, err := svc.UploadMultipartPartRecord(
		context.Background(),
		&repository.Bucket{ID: bucketID, SourceIDs: []primitive.ObjectID{sourceID}},
		&repository.MultipartUpload{BucketID: bucketID, UploadID: "upload-1", Parts: []repository.UploadPart{{PartNumber: 1, ETag: `"old"`, Chunks: []repository.FileChunk{oldChunk}}}},
		1,
		bytes.NewBufferString("data"),
		5,
	)
	if err != nil {
		t.Fatalf("UploadMultipartPartRecord returned error: %v", err)
	}
	if result.Part.PartNumber != 1 || len(result.Part.Chunks) != 1 || result.Part.Chunks[0].Name != "new-part-chunk" {
		t.Fatalf("expected replacement part to use new chunk, got %#v", result.Part)
	}
	if len(deleted) != 1 || deleted[0].Name != "old-part-chunk" {
		t.Fatalf("expected previous chunk cleanup after replacement, got %#v", deleted)
	}
	if len(events) != 2 || events[0] != "set part" || events[1] != "delete old-part-chunk" {
		t.Fatalf("expected metadata write before old chunk cleanup, got %#v", events)
	}
	got := svc.multipart.(*fakeMultipartUploads).uploads["upload-1"]
	if len(got.Parts) != 1 || got.Parts[0].Chunks[0].Name != "new-part-chunk" {
		t.Fatalf("expected stored metadata to contain replacement part, got %#v", got.Parts)
	}
}

func TestObjectServiceUploadMultipartPartSetFailureDeletesOnlyNewChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	oldChunk := repository.FileChunk{SourceID: sourceID, Name: "old-part-chunk", Size: 5}
	var deleted []repository.FileChunk
	var events []string
	svc := testObjectService(newFakeObjectFiles(), &deleted)
	svc.sources = &fakeObjectSources{sources: []repository.Source{{ID: sourceID}}}
	svc.multipart = &fakeMultipartUploads{
		uploads: map[string]repository.MultipartUpload{
			"upload-1": {
				BucketID: bucketID,
				UploadID: "upload-1",
				Parts: []repository.UploadPart{
					{PartNumber: 1, ETag: `"old"`, Chunks: []repository.FileChunk{oldChunk}},
				},
			},
		},
		setErr: errors.New("set part failed"),
		events: &events,
	}
	svc.uploadChunks = func(_ context.Context, r io.Reader, _ []repository.Source, _ int, _ SourceSelector) ([]repository.FileChunk, error) {
		if _, err := io.Copy(io.Discard, r); err != nil {
			return nil, err
		}
		return []repository.FileChunk{{SourceID: sourceID, Name: "new-part-chunk", Size: 4}}, nil
	}
	svc.deleteChunks = func(_ context.Context, chunks []repository.FileChunk) error {
		deleted = append(deleted, chunks...)
		for _, chunk := range chunks {
			events = append(events, "delete "+chunk.Name)
		}
		return nil
	}

	_, err := svc.UploadMultipartPartRecord(
		context.Background(),
		&repository.Bucket{ID: bucketID, SourceIDs: []primitive.ObjectID{sourceID}},
		&repository.MultipartUpload{BucketID: bucketID, UploadID: "upload-1", Parts: []repository.UploadPart{{PartNumber: 1, ETag: `"old"`, Chunks: []repository.FileChunk{oldChunk}}}},
		1,
		bytes.NewBufferString("data"),
		5,
	)
	if err == nil {
		t.Fatal("expected metadata replacement error")
	}
	if len(deleted) != 1 || deleted[0].Name != "new-part-chunk" {
		t.Fatalf("expected new chunk cleanup after metadata failure, got %#v", deleted)
	}
	if len(events) != 2 || events[0] != "set part" || events[1] != "delete new-part-chunk" {
		t.Fatalf("expected failed metadata write before new chunk cleanup, got %#v", events)
	}
	got := svc.multipart.(*fakeMultipartUploads).uploads["upload-1"]
	if len(got.Parts) != 1 || got.Parts[0].Chunks[0].Name != "old-part-chunk" {
		t.Fatalf("expected existing metadata to remain unchanged, got %#v", got.Parts)
	}
}

func TestObjectServiceAbortMultipartUploadDeletesChunksThenMetadata(t *testing.T) {
	bucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	chunkA := repository.FileChunk{SourceID: sourceID, Name: "part-1", Size: 5}
	chunkB := repository.FileChunk{SourceID: sourceID, Name: "part-2", Size: 7}
	var deleted []repository.FileChunk
	svc := testObjectService(newFakeObjectFiles(), &deleted)
	svc.multipart = &fakeMultipartUploads{uploads: map[string]repository.MultipartUpload{
		"upload-1": {
			BucketID: bucketID,
			UploadID: "upload-1",
			Parts: []repository.UploadPart{
				{PartNumber: 1, Chunks: []repository.FileChunk{chunkA}},
				{PartNumber: 2, Chunks: []repository.FileChunk{chunkB}},
			},
		},
	}}

	if err := svc.AbortMultipartUpload(context.Background(), "upload-1"); err != nil {
		t.Fatalf("AbortMultipartUpload returned error: %v", err)
	}
	if len(deleted) != 2 || deleted[0].Name != chunkA.Name || deleted[1].Name != chunkB.Name {
		t.Fatalf("expected all part chunks to be deleted, got %#v", deleted)
	}
	if _, err := svc.multipart.GetByUploadID(context.Background(), "upload-1"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected multipart upload metadata to be deleted, got %v", err)
	}
}

func TestObjectServiceAbortMultipartUploadKeepsMetadataWhenCleanupFails(t *testing.T) {
	bucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	cleanupErr := errors.New("delete part chunk failed")
	chunk := repository.FileChunk{SourceID: sourceID, Name: "part-1", Size: 5}
	var deleted []repository.FileChunk
	svc := testObjectService(newFakeObjectFiles(), &deleted)
	svc.multipart = &fakeMultipartUploads{uploads: map[string]repository.MultipartUpload{
		"upload-1": {
			BucketID: bucketID,
			UploadID: "upload-1",
			Parts: []repository.UploadPart{
				{PartNumber: 1, Chunks: []repository.FileChunk{chunk}},
			},
		},
	}}
	svc.deleteChunks = func(_ context.Context, chunks []repository.FileChunk) error {
		deleted = append(deleted, chunks...)
		return cleanupErr
	}

	err := svc.AbortMultipartUpload(context.Background(), "upload-1")
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("expected cleanup error, got %v", err)
	}
	if len(deleted) != 1 || deleted[0].Name != chunk.Name {
		t.Fatalf("expected part cleanup attempt, got %#v", deleted)
	}
	upload, err := svc.multipart.GetByUploadID(context.Background(), "upload-1")
	if err != nil {
		t.Fatalf("expected multipart upload metadata to remain, got %v", err)
	}
	if len(upload.Parts) != 1 || len(upload.Parts[0].Chunks) != 1 || upload.Parts[0].Chunks[0].Name != chunk.Name {
		t.Fatalf("expected retained part metadata, got %#v", upload.Parts)
	}
}

func fileKey(bucketID primitive.ObjectID, name string) string {
	return bucketID.Hex() + "/" + name
}

func testObjectService(files *fakeObjectFiles, deleted *[]repository.FileChunk) *objectService {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	return &objectService{
		sources: &fakeObjectSources{sources: []repository.Source{{ID: primitive.NewObjectID()}}},
		files:   files,
		uploadChunks: func(_ context.Context, r io.Reader, _ []repository.Source, _ int, _ SourceSelector) ([]repository.FileChunk, error) {
			if _, err := io.Copy(io.Discard, r); err != nil {
				return nil, err
			}
			return []repository.FileChunk{{SourceID: primitive.NewObjectID(), Name: "new-chunk", Size: 4}}, nil
		},
		deleteChunks: func(_ context.Context, chunks []repository.FileChunk) error {
			*deleted = append(*deleted, chunks...)
			return nil
		},
		now: func() time.Time {
			return now
		},
	}
}

func TestObjectWriteServiceWithSourceClientFactoryUsesFactoryForUploads(t *testing.T) {
	t.Parallel()

	sourceID := primitive.NewObjectID()
	calls := 0
	svc := NewObjectWriteServiceWithSourceClientFactory(nil, nil, func(_ context.Context, src *repository.Source) (SourceClient, error) {
		calls++
		if src.ID != sourceID {
			t.Fatalf("expected source %s, got %s", sourceID.Hex(), src.ID.Hex())
		}
		return &stubSourceClient{}, nil
	})

	chunks, err := svc.service.uploadChunks(context.Background(), bytes.NewReader([]byte("payload")), []repository.Source{{ID: sourceID}}, len("payload"), &RoundRobinSelector{})
	if err != nil {
		t.Fatalf("upload chunks: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected one chunk, got %d", len(chunks))
	}
	if calls != 1 {
		t.Fatalf("expected factory to be called once, got %d", calls)
	}
}

func TestObjectServicePutObjectUpdatesFileAndDeletesOldChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	oldSourceID := primitive.NewObjectID()
	existing := repository.File{
		ID:        primitive.NewObjectID(),
		BucketID:  bucketID,
		Name:      "object.txt",
		CreatedAt: time.Now().UTC(),
		Chunks:    []repository.FileChunk{{SourceID: oldSourceID, Name: "old-chunk", Size: 9}},
	}
	files := newFakeObjectFiles(existing)
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	result, err := svc.PutObject(context.Background(), &repository.Bucket{ID: bucketID}, "object.txt", bytes.NewBufferString("data"), 5, "text/plain", map[string]string{"owner": "alice"})
	if err != nil {
		t.Fatalf("PutObject returned error: %v", err)
	}
	if result.File.ID != existing.ID {
		t.Fatalf("expected update to preserve file id")
	}
	if len(result.File.Chunks) != 1 || result.File.Chunks[0].Name != "new-chunk" {
		t.Fatalf("expected saved file to use uploaded chunk, got %#v", result.File.Chunks)
	}
	if result.File.ContentType != "text/plain" || result.File.UserMetadata["owner"] != "alice" {
		t.Fatalf("expected saved object metadata, got content_type=%q metadata=%#v", result.File.ContentType, result.File.UserMetadata)
	}
	if len(deleted) != 1 || deleted[0].Name != "old-chunk" {
		t.Fatalf("expected old chunk cleanup, got %#v", deleted)
	}
}

func TestObjectServicePutObjectPersistsChecksumETagIndependentOfLifecycleMetadata(t *testing.T) {
	bucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	files := newFakeObjectFiles()
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)
	names := []string{"first-provider-name", "second-provider-name"}
	uploadCalls := 0
	svc.uploadChunks = func(_ context.Context, r io.Reader, _ []repository.Source, _ int, _ SourceSelector) ([]repository.FileChunk, error) {
		if _, err := io.Copy(io.Discard, r); err != nil {
			return nil, err
		}
		name := names[uploadCalls]
		uploadCalls++
		return []repository.FileChunk{{SourceID: sourceID, Name: name, Size: 4, Checksum: "same-content-checksum"}}, nil
	}
	times := []time.Time{
		time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
	}
	nowCalls := 0
	svc.now = func() time.Time {
		tm := times[nowCalls]
		nowCalls++
		return tm
	}

	first, err := svc.PutObject(context.Background(), &repository.Bucket{ID: bucketID}, "object.txt", bytes.NewBufferString("data"), 5, "text/plain", nil)
	if err != nil {
		t.Fatalf("first PutObject returned error: %v", err)
	}
	second, err := svc.PutObject(context.Background(), &repository.Bucket{ID: bucketID}, "object.txt", bytes.NewBufferString("data"), 5, "text/plain", nil)
	if err != nil {
		t.Fatalf("second PutObject returned error: %v", err)
	}
	if first.File.ETag == "" {
		t.Fatal("expected first put to persist an ETag")
	}
	if second.File.ETag != first.File.ETag {
		t.Fatalf("expected same content checksum to keep ETag stable, first=%s second=%s", first.File.ETag, second.File.ETag)
	}
	if ObjectETag(second.File) != second.File.ETag {
		t.Fatalf("expected served ETag to use persisted value")
	}
}

func TestObjectServicePutObjectOverwriteReplacesMetadata(t *testing.T) {
	bucketID := primitive.NewObjectID()
	existing := repository.File{
		ID:           primitive.NewObjectID(),
		BucketID:     bucketID,
		Name:         "object.txt",
		ContentType:  "text/plain",
		UserMetadata: map[string]string{"old": "value"},
		Chunks:       []repository.FileChunk{{SourceID: primitive.NewObjectID(), Name: "old-chunk", Size: 9}},
	}
	files := newFakeObjectFiles(existing)
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	result, err := svc.PutObject(context.Background(), &repository.Bucket{ID: bucketID}, "object.txt", bytes.NewBufferString("data"), 5, "application/json", nil)
	if err != nil {
		t.Fatalf("PutObject returned error: %v", err)
	}
	if result.File.ContentType != "application/json" {
		t.Fatalf("expected replacement content type, got %q", result.File.ContentType)
	}
	if len(result.File.UserMetadata) != 0 {
		t.Fatalf("expected metadata to be replaced with empty set, got %#v", result.File.UserMetadata)
	}
}

func TestObjectServicePutObjectCreateFailureDeletesUploadedChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	files := newFakeObjectFiles()
	files.replaceErr = errors.New("replace failed")
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	_, err := svc.PutObject(context.Background(), &repository.Bucket{ID: bucketID}, "object.txt", bytes.NewBufferString("data"), 5, "application/octet-stream", nil)
	if err == nil {
		t.Fatal("expected PutObject create error")
	}
	if len(deleted) != 1 || deleted[0].Name != "new-chunk" {
		t.Fatalf("expected uploaded chunk cleanup after create failure, got %#v", deleted)
	}
}

func TestObjectServicePutObjectOverwriteFailureDeletesUploadedChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	existing := repository.File{
		ID:       primitive.NewObjectID(),
		BucketID: bucketID,
		Name:     "object.txt",
		Chunks:   []repository.FileChunk{{SourceID: primitive.NewObjectID(), Name: "old-chunk", Size: 9}},
	}
	files := newFakeObjectFiles(existing)
	files.replaceErr = errors.New("replace failed")
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	_, err := svc.PutObject(context.Background(), &repository.Bucket{ID: bucketID}, "object.txt", bytes.NewBufferString("data"), 5, "application/octet-stream", nil)
	if err == nil {
		t.Fatal("expected PutObject overwrite error")
	}
	if len(deleted) != 1 || deleted[0].Name != "new-chunk" {
		t.Fatalf("expected uploaded chunk cleanup after overwrite failure, got %#v", deleted)
	}
}

func TestObjectServiceCopyObjectPreservesChunksAndCleansOverwrittenDestination(t *testing.T) {
	userID := primitive.NewObjectID()
	sourceBucketID := primitive.NewObjectID()
	destBucketID := primitive.NewObjectID()
	sourceChunk := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "source-chunk", Size: 12}
	oldDestChunk := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "old-dest-chunk", Size: 8}
	sourceETag := `"source-etag"`
	files := newFakeObjectFiles(
		repository.File{ID: primitive.NewObjectID(), BucketID: sourceBucketID, Name: "source.txt", ETag: sourceETag, ContentType: "image/png", UserMetadata: map[string]string{"owner": "alice"}, Chunks: []repository.FileChunk{sourceChunk}},
		repository.File{ID: primitive.NewObjectID(), BucketID: destBucketID, Name: "dest.txt", Chunks: []repository.FileChunk{oldDestChunk}},
	)
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	result, err := svc.CopyObject(
		context.Background(),
		&repository.Bucket{ID: sourceBucketID, UserID: userID},
		&repository.Bucket{ID: destBucketID, UserID: userID},
		"source.txt",
		"dest.txt",
	)
	if err != nil {
		t.Fatalf("CopyObject returned error: %v", err)
	}
	if len(result.File.Chunks) != 1 || result.File.Chunks[0].Name != sourceChunk.Name {
		t.Fatalf("expected copied file to reference source chunk, got %#v", result.File.Chunks)
	}
	if result.File.ContentType != "image/png" || result.File.UserMetadata["owner"] != "alice" {
		t.Fatalf("expected copied metadata, got content_type=%q metadata=%#v", result.File.ContentType, result.File.UserMetadata)
	}
	if result.File.ETag != sourceETag {
		t.Fatalf("expected copy to preserve source ETag, got %s", result.File.ETag)
	}
	if len(deleted) != 1 || deleted[0].Name != oldDestChunk.Name {
		t.Fatalf("expected overwritten destination chunk cleanup, got %#v", deleted)
	}
}

func TestObjectServiceCopyObjectDerivesChecksumETagForLegacySource(t *testing.T) {
	userID := primitive.NewObjectID()
	sourceBucketID := primitive.NewObjectID()
	destBucketID := primitive.NewObjectID()
	sourceChunk := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "source-provider-name", Size: 12, Checksum: "content-checksum"}
	files := newFakeObjectFiles(
		repository.File{
			ID:        primitive.NewObjectID(),
			BucketID:  sourceBucketID,
			Name:      "source.txt",
			CreatedAt: time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
			Chunks:    []repository.FileChunk{sourceChunk},
		},
	)
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	result, err := svc.CopyObject(
		context.Background(),
		&repository.Bucket{ID: sourceBucketID, UserID: userID},
		&repository.Bucket{ID: destBucketID, UserID: userID},
		"source.txt",
		"dest.txt",
	)
	if err != nil {
		t.Fatalf("CopyObject returned error: %v", err)
	}
	want := newObjectETag(repository.File{Chunks: []repository.FileChunk{sourceChunk}})
	if result.File.ETag != want {
		t.Fatalf("expected checksum-derived destination ETag %s, got %s", want, result.File.ETag)
	}
}

func TestObjectServiceCopyObjectRejectsCrossUserBuckets(t *testing.T) {
	svc := testObjectService(newFakeObjectFiles(), &[]repository.FileChunk{})

	_, err := svc.CopyObject(
		context.Background(),
		&repository.Bucket{ID: primitive.NewObjectID(), UserID: primitive.NewObjectID()},
		&repository.Bucket{ID: primitive.NewObjectID(), UserID: primitive.NewObjectID()},
		"source.txt",
		"dest.txt",
	)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expected access denied, got %v", err)
	}
}

func TestObjectServiceDeleteObjectRemovesMetadataAndUnreferencedChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	chunk := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "delete-me", Size: 3}
	files := newFakeObjectFiles(repository.File{ID: primitive.NewObjectID(), BucketID: bucketID, Name: "object.txt", Chunks: []repository.FileChunk{chunk}})
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	result, err := svc.DeleteObject(context.Background(), bucketID, "object.txt")
	if err != nil {
		t.Fatalf("DeleteObject returned error: %v", err)
	}
	if !result.Deleted {
		t.Fatalf("expected DeleteObject to report deletion")
	}
	if len(deleted) != 1 || deleted[0].Name != chunk.Name {
		t.Fatalf("expected deleted chunk cleanup, got %#v", deleted)
	}
	if _, err := files.GetByName(context.Background(), bucketID, "object.txt"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected metadata delete, got %v", err)
	}
}

func TestObjectServiceDeleteFileRemovesMetadataAndUnreferencedChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	chunk := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "delete-me", Size: 3}
	files := newFakeObjectFiles(repository.File{ID: fileID, BucketID: bucketID, Name: "object.txt", Chunks: []repository.FileChunk{chunk}})
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	result, err := svc.DeleteFile(context.Background(), bucketID, fileID)
	if err != nil {
		t.Fatalf("DeleteFile returned error: %v", err)
	}
	if !result.Deleted {
		t.Fatalf("expected DeleteFile to report deletion")
	}
	if len(deleted) != 1 || deleted[0].Name != chunk.Name {
		t.Fatalf("expected deleted chunk cleanup, got %#v", deleted)
	}
	if _, err := files.GetByID(context.Background(), fileID); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected metadata delete, got %v", err)
	}
}

func TestObjectServiceDeleteFileRejectsWrongBucket(t *testing.T) {
	fileID := primitive.NewObjectID()
	files := newFakeObjectFiles(repository.File{ID: fileID, BucketID: primitive.NewObjectID(), Name: "object.txt"})
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	_, err := svc.DeleteFile(context.Background(), primitive.NewObjectID(), fileID)
	if !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("expected object not found, got %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("expected no chunk cleanup, got %#v", deleted)
	}
	if _, err := files.GetByID(context.Background(), fileID); err != nil {
		t.Fatalf("expected metadata to remain, got %v", err)
	}
}

func TestObjectServiceDeleteBucketContentsRemovesMetadataAndUnreferencedChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	otherBucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	var events []string
	fileOnlyChunk := repository.FileChunk{SourceID: sourceID, Name: "file-only", Size: 3}
	fileSharedChunk := repository.FileChunk{SourceID: sourceID, Name: "file-shared", Size: 5}
	partOnlyChunk := repository.FileChunk{SourceID: sourceID, Name: "part-only", Size: 7}
	partSharedChunk := repository.FileChunk{SourceID: sourceID, Name: "part-shared", Size: 11}
	files := newFakeObjectFiles(
		repository.File{ID: primitive.NewObjectID(), BucketID: bucketID, Name: "object.txt", Chunks: []repository.FileChunk{fileOnlyChunk, fileSharedChunk}},
		repository.File{ID: primitive.NewObjectID(), BucketID: otherBucketID, Name: "survivor.txt", Chunks: []repository.FileChunk{fileSharedChunk}},
	)
	files.events = &events
	uploads := &fakeMultipartUploads{uploads: map[string]repository.MultipartUpload{
		"delete-upload": {
			BucketID:  bucketID,
			ObjectKey: "pending.txt",
			UploadID:  "delete-upload",
			Parts: []repository.UploadPart{
				{PartNumber: 1, Chunks: []repository.FileChunk{partOnlyChunk, partSharedChunk}},
			},
		},
		"survivor-upload": {
			BucketID:  otherBucketID,
			ObjectKey: "other-pending.txt",
			UploadID:  "survivor-upload",
			Parts: []repository.UploadPart{
				{PartNumber: 1, Chunks: []repository.FileChunk{partSharedChunk}},
			},
		},
	}}
	uploads.events = &events
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)
	svc.multipart = uploads
	svc.deleteChunks = func(_ context.Context, chunks []repository.FileChunk) error {
		deleted = append(deleted, chunks...)
		for _, chunk := range chunks {
			events = append(events, "delete chunk "+chunk.Name)
		}
		return nil
	}

	result, err := svc.DeleteBucketContents(context.Background(), bucketID)
	if err != nil {
		t.Fatalf("DeleteBucketContents returned error: %v", err)
	}
	if result.FilesDeleted != 1 || result.MultipartUploadsDeleted != 1 {
		t.Fatalf("unexpected delete counts: %+v", result)
	}
	if _, err := files.GetByName(context.Background(), bucketID, "object.txt"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected deleted bucket file metadata to be removed, got %v", err)
	}
	if _, err := files.GetByName(context.Background(), otherBucketID, "survivor.txt"); err != nil {
		t.Fatalf("expected other bucket file metadata to remain, got %v", err)
	}
	if _, err := uploads.GetByUploadID(context.Background(), "delete-upload"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected deleted bucket multipart metadata to be removed, got %v", err)
	}
	if _, err := uploads.GetByUploadID(context.Background(), "survivor-upload"); err != nil {
		t.Fatalf("expected other bucket multipart metadata to remain, got %v", err)
	}
	deletedNames := make(map[string]bool, len(deleted))
	for _, chunk := range deleted {
		deletedNames[chunk.Name] = true
	}
	if !deletedNames[fileOnlyChunk.Name] || !deletedNames[partOnlyChunk.Name] {
		t.Fatalf("expected unreferenced chunks to be deleted, got %#v", deleted)
	}
	if deletedNames[fileSharedChunk.Name] || deletedNames[partSharedChunk.Name] {
		t.Fatalf("expected shared chunks to remain, got %#v", deleted)
	}
	if len(events) < 2 || events[0] != "delete file metadata" || events[1] != "delete multipart metadata" {
		t.Fatalf("expected metadata deletion before chunk cleanup, got %#v", events)
	}
}

func TestObjectServiceDeleteBucketContentsReturnsChunkCleanupError(t *testing.T) {
	bucketID := primitive.NewObjectID()
	cleanupErr := errors.New("delete chunks failed")
	var events []string
	files := newFakeObjectFiles(repository.File{
		ID:       primitive.NewObjectID(),
		BucketID: bucketID,
		Name:     "object.txt",
		Chunks:   []repository.FileChunk{{SourceID: primitive.NewObjectID(), Name: "delete-me", Size: 3}},
	})
	files.events = &events
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)
	svc.deleteChunks = func(_ context.Context, chunks []repository.FileChunk) error {
		for _, chunk := range chunks {
			events = append(events, "delete chunk "+chunk.Name)
		}
		return cleanupErr
	}

	_, err := svc.DeleteBucketContents(context.Background(), bucketID)
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("expected cleanup error, got %v", err)
	}
	if _, err := files.GetByName(context.Background(), bucketID, "object.txt"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected file metadata removed before cleanup failure, got %v", err)
	}
	if len(events) != 2 || events[0] != "delete file metadata" || events[1] != "delete chunk delete-me" {
		t.Fatalf("expected chunk cleanup after metadata deletion, got %#v", events)
	}
}

func TestObjectServiceDeleteBucketContentsReturnsFileMetadataDeleteError(t *testing.T) {
	bucketID := primitive.NewObjectID()
	deleteErr := errors.New("delete metadata failed")
	files := newFakeObjectFiles(repository.File{
		ID:       primitive.NewObjectID(),
		BucketID: bucketID,
		Name:     "object.txt",
		Chunks:   []repository.FileChunk{{SourceID: primitive.NewObjectID(), Name: "delete-me", Size: 3}},
	})
	files.deleteErr = deleteErr
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	_, err := svc.DeleteBucketContents(context.Background(), bucketID)
	if !errors.Is(err, deleteErr) {
		t.Fatalf("expected file metadata delete error, got %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("expected no chunk cleanup after metadata delete failure, got %#v", deleted)
	}
	if _, err := files.GetByName(context.Background(), bucketID, "object.txt"); err != nil {
		t.Fatalf("expected file metadata to remain after delete failure, got %v", err)
	}
}

func TestObjectServiceDeleteBucketContentsReturnsMultipartMetadataDeleteError(t *testing.T) {
	bucketID := primitive.NewObjectID()
	deleteErr := errors.New("delete multipart metadata failed")
	files := newFakeObjectFiles(repository.File{
		ID:       primitive.NewObjectID(),
		BucketID: bucketID,
		Name:     "object.txt",
		Chunks:   []repository.FileChunk{{SourceID: primitive.NewObjectID(), Name: "delete-me", Size: 3}},
	})
	uploads := &fakeMultipartUploads{
		uploads: map[string]repository.MultipartUpload{
			"delete-upload": {
				BucketID:  bucketID,
				ObjectKey: "pending.txt",
				UploadID:  "delete-upload",
			},
		},
		delErr: deleteErr,
	}
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)
	svc.multipart = uploads

	_, err := svc.DeleteBucketContents(context.Background(), bucketID)
	if !errors.Is(err, deleteErr) {
		t.Fatalf("expected multipart metadata delete error, got %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("expected no chunk cleanup after multipart metadata delete failure, got %#v", deleted)
	}
	if _, err := files.GetByName(context.Background(), bucketID, "object.txt"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected file metadata deleted before multipart delete failure, got %v", err)
	}
	if _, err := uploads.GetByUploadID(context.Background(), "delete-upload"); err != nil {
		t.Fatalf("expected multipart metadata to remain after delete failure, got %v", err)
	}
}

func TestObjectServiceCompleteMultipartUploadBuildsFinalFileAndCleansUnusedChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	oldChunk := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "old-file-chunk", Size: 2}
	part1ChunkA := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "part-1a", Size: 5, Checksum: "sum-a"}
	part1ChunkB := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "part-1b", Size: 6, Checksum: "sum-b"}
	part2Chunk := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "part-2", Size: 7, Checksum: "sum-c"}
	unusedPartChunk := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "part-3-unused", Size: 8}
	files := newFakeObjectFiles(repository.File{
		ID:       primitive.NewObjectID(),
		BucketID: bucketID,
		Name:     "object.txt",
		Chunks:   []repository.FileChunk{oldChunk},
	})
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)
	svc.multipart = &fakeMultipartUploads{uploads: map[string]repository.MultipartUpload{
		"upload-1": {
			BucketID:     bucketID,
			ObjectKey:    "object.txt",
			UploadID:     "upload-1",
			ContentType:  "application/x-ndjson",
			UserMetadata: map[string]string{"batch": "42"},
			Parts: []repository.UploadPart{
				{PartNumber: 1, ETag: `"d41d8cd98f00b204e9800998ecf8427e"`, Chunks: []repository.FileChunk{part1ChunkA, part1ChunkB}},
				{PartNumber: 2, ETag: `"0cc175b9c0f1b6a831c399e269772661"`, Chunks: []repository.FileChunk{part2Chunk}},
				{PartNumber: 3, ETag: `"900150983cd24fb0d6963f7d28e17f72"`, Chunks: []repository.FileChunk{unusedPartChunk}},
			},
		},
	}}

	result, err := svc.CompleteMultipartUpload(context.Background(), bucketID, "upload-1", []CompleteMultipartPart{
		{PartNumber: 1, ETag: "d41d8cd98f00b204e9800998ecf8427e"},
		{PartNumber: 2, ETag: "0cc175b9c0f1b6a831c399e269772661"},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload returned error: %v", err)
	}
	if result.File.Name != "object.txt" {
		t.Fatalf("expected final object name, got %q", result.File.Name)
	}
	if len(result.File.Chunks) != 3 {
		t.Fatalf("expected three final chunks, got %#v", result.File.Chunks)
	}
	wantChecksums := []string{part1ChunkA.Checksum, part1ChunkB.Checksum, part2Chunk.Checksum}
	for i, want := range wantChecksums {
		if result.File.Chunks[i].Order != i {
			t.Fatalf("chunk %d order: got %d, want %d", i, result.File.Chunks[i].Order, i)
		}
		if result.File.Chunks[i].Checksum != want {
			t.Fatalf("chunk %d checksum: got %q, want %q", i, result.File.Chunks[i].Checksum, want)
		}
	}
	if result.File.ContentType != "application/x-ndjson" || result.File.UserMetadata["batch"] != "42" {
		t.Fatalf("expected multipart metadata to be preserved, got content_type=%q metadata=%#v", result.File.ContentType, result.File.UserMetadata)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected old file chunk and unreferenced part cleanup, got %#v", deleted)
	}
	if deleted[0].Name != oldChunk.Name || deleted[1].Name != unusedPartChunk.Name {
		t.Fatalf("unexpected cleanup order: %#v", deleted)
	}
	if !strings.HasSuffix(result.ETag, `-2"`) {
		t.Fatalf("unexpected multipart etag: %s", result.ETag)
	}
	if result.File.ETag != result.ETag {
		t.Fatalf("expected multipart completion ETag to be persisted, file=%s result=%s", result.File.ETag, result.ETag)
	}
	if ObjectETag(result.File) != result.ETag {
		t.Fatalf("expected served object ETag to match multipart completion ETag")
	}
	if _, err := svc.multipart.GetByUploadID(context.Background(), "upload-1"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected multipart upload record cleanup, got %v", err)
	}
}
