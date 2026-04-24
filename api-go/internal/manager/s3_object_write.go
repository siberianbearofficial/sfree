package manager

import (
	"context"
	"io"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (s *objectService) PutObject(ctx context.Context, bucket *repository.Bucket, name string, body io.Reader, chunkSize int, contentType string, userMetadata map[string]string) (PutObjectResult, error) {
	sources, err := s.sources.ListByIDs(ctx, bucket.SourceIDs)
	if err != nil {
		return PutObjectResult{}, err
	}
	if len(bucket.SourceIDs) == 0 {
		return PutObjectResult{}, ErrNoSources
	}
	if err := requireResolvedSources(bucket.SourceIDs, sources); err != nil {
		return PutObjectResult{}, err
	}

	chunks, err := s.uploadChunks(ctx, body, sources, chunkSize, SelectorForBucket(bucket, sources))
	if err != nil {
		return PutObjectResult{}, err
	}

	fileDoc := repository.File{
		BucketID:     bucket.ID,
		Name:         name,
		CreatedAt:    s.now(),
		Chunks:       chunks,
		ContentType:  contentType,
		UserMetadata: cloneStringMap(userMetadata),
	}
	fileDoc.ETag = newObjectETag(fileDoc)
	currentFile, previousFile, err := s.files.ReplaceByName(ctx, fileDoc)
	if err != nil {
		_ = s.deleteChunks(ctx, chunks)
		return PutObjectResult{}, err
	}
	var cleanupErr error
	if previousFile != nil {
		cleanupErr = s.deleteFileChunksIfUnreferenced(ctx, previousFile.Chunks)
	}
	return PutObjectResult{File: *currentFile, CleanupErr: cleanupErr}, nil
}

func (s *objectService) UploadMultipartPartRecord(ctx context.Context, bucket *repository.Bucket, mu *repository.MultipartUpload, partNumber int, body io.Reader, chunkSize int) (UploadMultipartPartResult, error) {
	if bucket == nil || mu == nil || mu.BucketID != bucket.ID {
		return UploadMultipartPartResult{}, ErrMultipartUploadNotFound
	}

	sources, err := s.sources.ListByIDs(ctx, bucket.SourceIDs)
	if err != nil {
		return UploadMultipartPartResult{}, err
	}
	if len(bucket.SourceIDs) == 0 {
		return UploadMultipartPartResult{}, ErrNoSources
	}
	if err := requireResolvedSources(bucket.SourceIDs, sources); err != nil {
		return UploadMultipartPartResult{}, err
	}

	chunks, err := s.uploadChunks(ctx, body, sources, chunkSize, SelectorForBucket(bucket, sources))
	if err != nil {
		return UploadMultipartPartResult{}, err
	}

	var totalSize int64
	for _, ch := range chunks {
		totalSize += ch.Size
	}
	part := repository.UploadPart{
		PartNumber: partNumber,
		ETag:       multipartPartETag(chunks),
		Size:       totalSize,
		Chunks:     chunks,
	}

	previous, err := s.multipart.SetPart(ctx, mu.UploadID, part)
	if err != nil {
		_ = s.deleteChunks(ctx, chunks)
		if err == mongo.ErrNoDocuments {
			return UploadMultipartPartResult{}, ErrMultipartUploadNotFound
		}
		return UploadMultipartPartResult{}, err
	}

	var cleanupErr error
	if previous != nil {
		cleanupErr = s.deleteChunks(ctx, previous.Chunks)
	}
	return UploadMultipartPartResult{Part: part, ETag: part.ETag, CleanupErr: cleanupErr}, nil
}

func requireResolvedSources(ids []primitive.ObjectID, sources []repository.Source) error {
	resolved := make(map[primitive.ObjectID]struct{}, len(sources))
	for _, source := range sources {
		resolved[source.ID] = struct{}{}
	}
	missing := make([]primitive.ObjectID, 0)
	for _, id := range ids {
		if _, ok := resolved[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return repository.SourcesNotFoundError{IDs: missing}
	}
	return nil
}

func (s *objectService) CopyObject(ctx context.Context, sourceBucket, destBucket *repository.Bucket, sourceKey, destKey string) (CopyObjectResult, error) {
	if sourceBucket.ID != destBucket.ID && sourceBucket.UserID != destBucket.UserID {
		return CopyObjectResult{}, ErrAccessDenied
	}

	sourceFile, err := s.files.GetByName(ctx, sourceBucket.ID, sourceKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return CopyObjectResult{}, ErrObjectNotFound
		}
		return CopyObjectResult{}, err
	}

	chunks := append([]repository.FileChunk(nil), sourceFile.Chunks...)
	copyFile := repository.File{
		BucketID:     destBucket.ID,
		Name:         destKey,
		CreatedAt:    s.now(),
		Chunks:       chunks,
		ContentType:  sourceFile.ContentType,
		UserMetadata: cloneStringMap(sourceFile.UserMetadata),
	}
	copyFile.ETag = copyObjectETag(*sourceFile)
	currentFile, previousFile, err := s.files.ReplaceByName(ctx, copyFile)
	if err != nil {
		return CopyObjectResult{}, err
	}
	var cleanupErr error
	if previousFile != nil {
		cleanupErr = s.deleteFileChunksIfUnreferenced(ctx, previousFile.Chunks)
	}
	return CopyObjectResult{File: *currentFile, CleanupErr: cleanupErr}, nil
}
