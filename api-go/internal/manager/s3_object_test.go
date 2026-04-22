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

func fileKey(bucketID primitive.ObjectID, name string) string {
	return bucketID.Hex() + "/" + name
}

func testObjectService(files *fakeObjectFiles, deleted *[]repository.FileChunk) *ObjectService {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	return &ObjectService{
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

	result, err := svc.PutObject(context.Background(), &repository.Bucket{ID: bucketID}, "object.txt", bytes.NewBufferString("data"), 5)
	if err != nil {
		t.Fatalf("PutObject returned error: %v", err)
	}
	if result.File.ID != existing.ID {
		t.Fatalf("expected update to preserve file id")
	}
	if len(result.File.Chunks) != 1 || result.File.Chunks[0].Name != "new-chunk" {
		t.Fatalf("expected saved file to use uploaded chunk, got %#v", result.File.Chunks)
	}
	if len(deleted) != 1 || deleted[0].Name != "old-chunk" {
		t.Fatalf("expected old chunk cleanup, got %#v", deleted)
	}
}

func TestObjectServicePutObjectCreateFailureDeletesUploadedChunks(t *testing.T) {
	bucketID := primitive.NewObjectID()
	files := newFakeObjectFiles()
	files.replaceErr = errors.New("replace failed")
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)

	_, err := svc.PutObject(context.Background(), &repository.Bucket{ID: bucketID}, "object.txt", bytes.NewBufferString("data"), 5)
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

	_, err := svc.PutObject(context.Background(), &repository.Bucket{ID: bucketID}, "object.txt", bytes.NewBufferString("data"), 5)
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
	files := newFakeObjectFiles(
		repository.File{ID: primitive.NewObjectID(), BucketID: sourceBucketID, Name: "source.txt", Chunks: []repository.FileChunk{sourceChunk}},
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
	if len(deleted) != 1 || deleted[0].Name != oldDestChunk.Name {
		t.Fatalf("expected overwritten destination chunk cleanup, got %#v", deleted)
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
	fileOnlyChunk := repository.FileChunk{SourceID: sourceID, Name: "file-only", Size: 3}
	fileSharedChunk := repository.FileChunk{SourceID: sourceID, Name: "file-shared", Size: 5}
	partOnlyChunk := repository.FileChunk{SourceID: sourceID, Name: "part-only", Size: 7}
	partSharedChunk := repository.FileChunk{SourceID: sourceID, Name: "part-shared", Size: 11}
	files := newFakeObjectFiles(
		repository.File{ID: primitive.NewObjectID(), BucketID: bucketID, Name: "object.txt", Chunks: []repository.FileChunk{fileOnlyChunk, fileSharedChunk}},
		repository.File{ID: primitive.NewObjectID(), BucketID: otherBucketID, Name: "survivor.txt", Chunks: []repository.FileChunk{fileSharedChunk}},
	)
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
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)
	svc.multipart = uploads

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
}

func TestObjectServiceDeleteBucketContentsReturnsChunkCleanupError(t *testing.T) {
	bucketID := primitive.NewObjectID()
	cleanupErr := errors.New("delete chunks failed")
	files := newFakeObjectFiles(repository.File{
		ID:       primitive.NewObjectID(),
		BucketID: bucketID,
		Name:     "object.txt",
		Chunks:   []repository.FileChunk{{SourceID: primitive.NewObjectID(), Name: "delete-me", Size: 3}},
	})
	var deleted []repository.FileChunk
	svc := testObjectService(files, &deleted)
	svc.deleteChunks = func(context.Context, []repository.FileChunk) error {
		return cleanupErr
	}

	_, err := svc.DeleteBucketContents(context.Background(), bucketID)
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("expected cleanup error, got %v", err)
	}
	if _, err := files.GetByName(context.Background(), bucketID, "object.txt"); err != nil {
		t.Fatalf("expected file metadata to remain after cleanup failure, got %v", err)
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
	if len(deleted) != 1 || deleted[0].Name != "delete-me" {
		t.Fatalf("expected chunk cleanup before metadata delete failure, got %#v", deleted)
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
	part2Chunk := repository.FileChunk{SourceID: primitive.NewObjectID(), Name: "part-2", Size: 7}
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
			BucketID:  bucketID,
			ObjectKey: "object.txt",
			UploadID:  "upload-1",
			Parts: []repository.UploadPart{
				{PartNumber: 1, ETag: `"d41d8cd98f00b204e9800998ecf8427e"`, Chunks: []repository.FileChunk{part1ChunkA, part1ChunkB}},
				{PartNumber: 2, ETag: `"0cc175b9c0f1b6a831c399e269772661"`, Chunks: []repository.FileChunk{part2Chunk}},
			},
		},
	}}

	result, err := svc.CompleteMultipartUpload(context.Background(), bucketID, "upload-1", []CompleteMultipartPart{
		{PartNumber: 1, ETag: "d41d8cd98f00b204e9800998ecf8427e"},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload returned error: %v", err)
	}
	if result.File.Name != "object.txt" {
		t.Fatalf("expected final object name, got %q", result.File.Name)
	}
	if len(result.File.Chunks) != 2 {
		t.Fatalf("expected two final chunks, got %#v", result.File.Chunks)
	}
	if result.File.Chunks[0].Order != 0 || result.File.Chunks[1].Order != 1 {
		t.Fatalf("expected chunks to be renumbered, got %#v", result.File.Chunks)
	}
	if result.File.Chunks[0].Checksum != part1ChunkA.Checksum {
		t.Fatalf("expected checksum to be preserved")
	}
	if len(deleted) != 2 {
		t.Fatalf("expected old file chunk and unreferenced part cleanup, got %#v", deleted)
	}
	if deleted[0].Name != oldChunk.Name || deleted[1].Name != part2Chunk.Name {
		t.Fatalf("unexpected cleanup order: %#v", deleted)
	}
	if !strings.HasSuffix(result.ETag, `-1"`) {
		t.Fatalf("unexpected multipart etag: %s", result.ETag)
	}
	if _, err := svc.multipart.GetByUploadID(context.Background(), "upload-1"); !errors.Is(err, mongo.ErrNoDocuments) {
		t.Fatalf("expected multipart upload record cleanup, got %v", err)
	}
}
