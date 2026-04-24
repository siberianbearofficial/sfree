package manager

import (
	"context"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (s *objectService) DeleteBucketContents(ctx context.Context, bucketID primitive.ObjectID) (DeleteBucketContentsResult, error) {
	files, err := s.files.ListByBucket(ctx, bucketID)
	if err != nil {
		return DeleteBucketContentsResult{}, err
	}

	var uploads []repository.MultipartUpload
	if s.multipart != nil {
		uploads, err = s.multipart.ListByBucket(ctx, bucketID)
		if err != nil {
			return DeleteBucketContentsResult{}, err
		}
	}

	chunks := bucketCleanupChunks(files, uploads)
	if err := s.files.DeleteByBucket(ctx, bucketID); err != nil {
		return DeleteBucketContentsResult{}, err
	}
	if s.multipart != nil {
		if err := s.multipart.DeleteByBucket(ctx, bucketID); err != nil {
			return DeleteBucketContentsResult{}, err
		}
	}
	if err := s.deleteBucketChunksIfUnreferenced(ctx, bucketID, chunks); err != nil {
		return DeleteBucketContentsResult{}, err
	}
	return DeleteBucketContentsResult{
		FilesDeleted:            len(files),
		MultipartUploadsDeleted: len(uploads),
	}, nil
}

func (s *objectService) deleteBucketChunksIfUnreferenced(ctx context.Context, bucketID primitive.ObjectID, chunks []repository.FileChunk) error {
	return deleteBucketChunksIfUnreferenced(ctx, bucketID, s.files, s.multipart, s.deleteChunks, chunks)
}

func deleteBucketChunksIfUnreferenced(ctx context.Context, bucketID primitive.ObjectID, files BucketScopedChunkReferenceCounter, multipart objectMultipartStore, deleteChunks objectChunkDeleter, chunks []repository.FileChunk) error {
	seen := make(map[string]struct{}, len(chunks))
	for _, chunk := range chunks {
		ref := chunk.SourceID.Hex() + "/" + chunk.Name
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		count, err := files.CountByChunkExcludingBucket(ctx, bucketID, chunk.SourceID, chunk.Name)
		if err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if multipart != nil {
			count, err = multipart.CountByPartChunkExcludingBucket(ctx, bucketID, chunk.SourceID, chunk.Name)
			if err != nil {
				return err
			}
			if count > 0 {
				continue
			}
		}
		if err := deleteChunks(ctx, []repository.FileChunk{chunk}); err != nil {
			return err
		}
	}
	return nil
}

func bucketCleanupChunks(files []repository.File, uploads []repository.MultipartUpload) []repository.FileChunk {
	var chunks []repository.FileChunk
	for _, file := range files {
		chunks = append(chunks, file.Chunks...)
	}
	for _, upload := range uploads {
		for _, part := range upload.Parts {
			chunks = append(chunks, part.Chunks...)
		}
	}
	return chunks
}
