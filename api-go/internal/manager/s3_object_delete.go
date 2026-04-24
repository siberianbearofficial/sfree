package manager

import (
	"context"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (s *objectService) DeleteObject(ctx context.Context, bucketID primitive.ObjectID, name string) (DeleteObjectResult, error) {
	fileDoc, err := s.files.GetByName(ctx, bucketID, name)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return DeleteObjectResult{}, nil
		}
		return DeleteObjectResult{}, err
	}
	if err := s.files.Delete(ctx, fileDoc.ID); err != nil {
		if err == mongo.ErrNoDocuments {
			return DeleteObjectResult{}, nil
		}
		return DeleteObjectResult{}, err
	}
	return DeleteObjectResult{
		Deleted:    true,
		CleanupErr: s.deleteFileChunksIfUnreferenced(ctx, fileDoc.Chunks),
	}, nil
}

func (s *objectService) DeleteFile(ctx context.Context, bucketID, fileID primitive.ObjectID) (DeleteObjectResult, error) {
	fileDoc, err := s.files.GetByID(ctx, fileID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return DeleteObjectResult{}, ErrObjectNotFound
		}
		return DeleteObjectResult{}, err
	}
	if fileDoc.BucketID != bucketID {
		return DeleteObjectResult{}, ErrObjectNotFound
	}
	if err := s.files.Delete(ctx, fileDoc.ID); err != nil {
		if err == mongo.ErrNoDocuments {
			return DeleteObjectResult{}, ErrObjectNotFound
		}
		return DeleteObjectResult{}, err
	}
	return DeleteObjectResult{
		Deleted:    true,
		CleanupErr: s.deleteFileChunksIfUnreferenced(ctx, fileDoc.Chunks),
	}, nil
}

func (s *objectService) deleteFileChunksIfUnreferenced(ctx context.Context, chunks []repository.FileChunk) error {
	return deleteFileChunksIfUnreferenced(ctx, s.files, s.deleteChunks, chunks)
}

func DeleteFileChunksIfUnreferenced(ctx context.Context, sourceRepo *repository.SourceRepository, fileStore ChunkReferenceCounter, chunks []repository.FileChunk) error {
	return DeleteFileChunksIfUnreferencedWithFactory(ctx, sourceRepo, fileStore, chunks, nil)
}

func DeleteFileChunksIfUnreferencedWithFactory(ctx context.Context, sourceRepo *repository.SourceRepository, fileStore ChunkReferenceCounter, chunks []repository.FileChunk, factory SourceClientFactory) error {
	return deleteFileChunksIfUnreferenced(ctx, fileStore, func(ctx context.Context, chunks []repository.FileChunk) error {
		return DeleteFileChunksWithFactory(ctx, sourceRepo, chunks, factory)
	}, chunks)
}

func deleteFileChunksIfUnreferenced(ctx context.Context, files ChunkReferenceCounter, deleteChunks objectChunkDeleter, chunks []repository.FileChunk) error {
	seen := make(map[string]struct{}, len(chunks))
	for _, chunk := range chunks {
		ref := chunk.SourceID.Hex() + "/" + chunk.Name
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		count, err := files.CountByChunk(ctx, chunk.SourceID, chunk.Name)
		if err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := deleteChunks(ctx, []repository.FileChunk{chunk}); err != nil {
			return err
		}
	}
	return nil
}
