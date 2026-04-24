package manager

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (s *objectService) CompleteMultipartUpload(ctx context.Context, bucketID primitive.ObjectID, uploadID string, requestedParts []CompleteMultipartPart) (CompleteMultipartUploadResult, error) {
	mu, err := s.multipart.GetByUploadID(ctx, uploadID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return CompleteMultipartUploadResult{}, ErrMultipartUploadNotFound
		}
		return CompleteMultipartUploadResult{}, err
	}
	return s.CompleteMultipartUploadRecord(ctx, bucketID, mu, requestedParts)
}

func (s *objectService) CompleteMultipartUploadRecord(ctx context.Context, bucketID primitive.ObjectID, mu *repository.MultipartUpload, requestedParts []CompleteMultipartPart) (CompleteMultipartUploadResult, error) {
	if mu == nil {
		return CompleteMultipartUploadResult{}, ErrMultipartUploadNotFound
	}
	if mu.BucketID != bucketID {
		return CompleteMultipartUploadResult{}, ErrMultipartUploadNotFound
	}
	if len(requestedParts) == 0 {
		return CompleteMultipartUploadResult{}, ErrMultipartUploadHasNoParts
	}
	for i := 1; i < len(requestedParts); i++ {
		if requestedParts[i].PartNumber <= requestedParts[i-1].PartNumber {
			return CompleteMultipartUploadResult{}, ErrMultipartUploadPartOrder
		}
	}

	partMap := make(map[int]repository.UploadPart, len(mu.Parts))
	for _, p := range mu.Parts {
		partMap[p.PartNumber] = p
	}

	var allChunks []repository.FileChunk
	chunkOrder := 0
	for _, rp := range requestedParts {
		up, exists := partMap[rp.PartNumber]
		if !exists {
			return CompleteMultipartUploadResult{}, InvalidMultipartPartError{PartNumber: rp.PartNumber, Reason: "not uploaded"}
		}
		if strings.Trim(rp.ETag, "\"") != strings.Trim(up.ETag, "\"") {
			return CompleteMultipartUploadResult{}, InvalidMultipartPartError{PartNumber: rp.PartNumber, Reason: "etag mismatch"}
		}
		for _, ch := range up.Chunks {
			allChunks = append(allChunks, repository.FileChunk{
				SourceID: ch.SourceID,
				Name:     ch.Name,
				Order:    chunkOrder,
				Size:     ch.Size,
				Checksum: ch.Checksum,
			})
			chunkOrder++
		}
	}

	etag := multipartETag(requestedParts, partMap)
	fileDoc := repository.File{
		BucketID:     bucketID,
		Name:         mu.ObjectKey,
		CreatedAt:    s.now(),
		Chunks:       allChunks,
		ETag:         etag,
		ContentType:  mu.ContentType,
		UserMetadata: cloneStringMap(mu.UserMetadata),
	}
	saved, previousFile, err := s.files.ReplaceByName(ctx, fileDoc)
	if err != nil {
		return CompleteMultipartUploadResult{}, err
	}

	var cleanupErrs []error
	if previousFile != nil {
		if err := s.deleteFileChunksIfUnreferenced(ctx, previousFile.Chunks); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}

	requested := make(map[int]bool, len(requestedParts))
	for _, rp := range requestedParts {
		requested[rp.PartNumber] = true
	}
	for _, p := range mu.Parts {
		if !requested[p.PartNumber] {
			if err := s.deleteChunks(ctx, p.Chunks); err != nil {
				cleanupErrs = append(cleanupErrs, err)
			}
		}
	}
	if err := s.multipart.Delete(ctx, mu.UploadID); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}

	return CompleteMultipartUploadResult{
		File:        *saved,
		Upload:      *mu,
		ETag:        etag,
		CleanupErrs: cleanupErrs,
	}, nil
}

func (s *objectService) AbortMultipartUpload(ctx context.Context, uploadID string) error {
	return AbortMultipartUpload(ctx, s.multipart, s.deleteChunks, uploadID)
}

func (s *objectService) AbortMultipartUploadRecord(ctx context.Context, mu *repository.MultipartUpload) error {
	return AbortMultipartUploadRecord(ctx, s.multipart, s.deleteChunks, mu)
}

func AbortMultipartUpload(ctx context.Context, multipart MultipartUploadAbortStore, deleteChunks func(context.Context, []repository.FileChunk) error, uploadID string) error {
	mu, err := multipart.GetByUploadID(ctx, uploadID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrMultipartUploadNotFound
		}
		return err
	}
	return AbortMultipartUploadRecord(ctx, multipart, deleteChunks, mu)
}

func AbortMultipartUploadRecord(ctx context.Context, multipart MultipartUploadAbortStore, deleteChunks func(context.Context, []repository.FileChunk) error, mu *repository.MultipartUpload) error {
	if mu == nil {
		return ErrMultipartUploadNotFound
	}

	var cleanupErrs []error
	for _, part := range mu.Parts {
		if err := deleteChunks(ctx, part.Chunks); err != nil {
			cleanupErrs = append(cleanupErrs, err)
		}
	}
	if err := errors.Join(cleanupErrs...); err != nil {
		return err
	}
	if err := multipart.Delete(ctx, mu.UploadID); err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrMultipartUploadNotFound
		}
		return err
	}
	return nil
}

func multipartPartETag(chunks []repository.FileChunk) string {
	h := md5.New()
	for _, chunk := range chunks {
		_, _ = h.Write([]byte(chunk.Name))
	}
	return fmt.Sprintf("\"%s\"", hex.EncodeToString(h.Sum(nil)))
}

func multipartETag(requestedParts []CompleteMultipartPart, partMap map[int]repository.UploadPart) string {
	h := md5.New()
	for _, rp := range requestedParts {
		up := partMap[rp.PartNumber]
		partHash := strings.Trim(up.ETag, "\"")
		raw, _ := hex.DecodeString(partHash)
		_, _ = h.Write(raw)
	}
	return fmt.Sprintf("\"%s-%d\"", hex.EncodeToString(h.Sum(nil)), len(requestedParts))
}
