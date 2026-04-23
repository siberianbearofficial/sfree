package manager

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrNoSources                  = errors.New("no sources configured")
	ErrObjectNotFound             = errors.New("object not found")
	ErrAccessDenied               = errors.New("access denied")
	ErrMultipartUploadNotFound    = errors.New("multipart upload not found")
	ErrMultipartUploadHasNoParts  = errors.New("multipart upload completion requires at least one part")
	ErrMultipartUploadPartOrder   = errors.New("multipart upload parts must be in ascending order")
	ErrMultipartUploadInvalidPart = errors.New("multipart upload invalid part")
)

type InvalidMultipartPartError struct {
	PartNumber int
	Reason     string
}

func (e InvalidMultipartPartError) Error() string {
	return fmt.Sprintf("%v: part %d %s", ErrMultipartUploadInvalidPart, e.PartNumber, e.Reason)
}

func (e InvalidMultipartPartError) Unwrap() error {
	return ErrMultipartUploadInvalidPart
}

type CompleteMultipartPart struct {
	PartNumber int
	ETag       string
}

type PutObjectResult struct {
	File       repository.File
	CleanupErr error
}

type UploadMultipartPartResult struct {
	Part       repository.UploadPart
	ETag       string
	CleanupErr error
}

type CopyObjectResult struct {
	File       repository.File
	CleanupErr error
}

type DeleteObjectResult struct {
	Deleted    bool
	CleanupErr error
}

type CompleteMultipartUploadResult struct {
	File        repository.File
	Upload      repository.MultipartUpload
	ETag        string
	CleanupErrs []error
}

type DeleteBucketContentsResult struct {
	FilesDeleted            int
	MultipartUploadsDeleted int
}

type objectSourceStore interface {
	ListByIDs(ctx context.Context, ids []primitive.ObjectID) ([]repository.Source, error)
}

type ChunkReferenceCounter interface {
	CountByChunk(ctx context.Context, sourceID primitive.ObjectID, name string) (int64, error)
}

type BucketScopedChunkReferenceCounter interface {
	ChunkReferenceCounter
	CountByChunkExcludingBucket(ctx context.Context, bucketID, sourceID primitive.ObjectID, name string) (int64, error)
}

type objectFileStore interface {
	BucketScopedChunkReferenceCounter
	GetByID(ctx context.Context, id primitive.ObjectID) (*repository.File, error)
	GetByName(ctx context.Context, bucketID primitive.ObjectID, name string) (*repository.File, error)
	ListByBucket(ctx context.Context, bucketID primitive.ObjectID) ([]repository.File, error)
	ReplaceByName(ctx context.Context, f repository.File) (*repository.File, *repository.File, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
	DeleteByBucket(ctx context.Context, bucketID primitive.ObjectID) error
}

type objectMultipartStore interface {
	GetByUploadID(ctx context.Context, uploadID string) (*repository.MultipartUpload, error)
	ListByBucket(ctx context.Context, bucketID primitive.ObjectID) ([]repository.MultipartUpload, error)
	CountByPartChunk(ctx context.Context, sourceID primitive.ObjectID, name string) (int64, error)
	CountByPartChunkExcludingBucket(ctx context.Context, bucketID, sourceID primitive.ObjectID, name string) (int64, error)
	SetPart(ctx context.Context, uploadID string, part repository.UploadPart) (*repository.UploadPart, error)
	Delete(ctx context.Context, uploadID string) error
	DeleteByBucket(ctx context.Context, bucketID primitive.ObjectID) error
}

type MultipartUploadAbortStore interface {
	GetByUploadID(ctx context.Context, uploadID string) (*repository.MultipartUpload, error)
	Delete(ctx context.Context, uploadID string) error
}

type objectChunkUploader func(ctx context.Context, r io.Reader, sources []repository.Source, chunkSize int, selector SourceSelector) ([]repository.FileChunk, error)
type objectChunkDeleter func(ctx context.Context, chunks []repository.FileChunk) error

type objectService struct {
	sources      objectSourceStore
	files        objectFileStore
	multipart    objectMultipartStore
	uploadChunks objectChunkUploader
	deleteChunks objectChunkDeleter
	now          func() time.Time
}

type ObjectWriteService struct {
	service *objectService
}

type MultipartPartWriteService struct {
	service *objectService
}

type ObjectDeleteService struct {
	service *objectService
}

type BucketCleanupService struct {
	service *objectService
}

type MultipartCompletionService struct {
	service *objectService
}

func NewObjectWriteService(sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) *ObjectWriteService {
	return NewObjectWriteServiceWithSourceClientFactory(sourceRepo, fileRepo, nil)
}

func NewObjectWriteServiceWithSourceClientFactory(sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, factory SourceClientFactory) *ObjectWriteService {
	svc := newObjectServiceBase(sourceRepo, factory)
	svc.files = fileRepo
	return &ObjectWriteService{service: svc}
}

func NewMultipartPartWriteService(sourceRepo *repository.SourceRepository, mpRepo *repository.MultipartUploadRepository) *MultipartPartWriteService {
	return NewMultipartPartWriteServiceWithSourceClientFactory(sourceRepo, mpRepo, nil)
}

func NewMultipartPartWriteServiceWithSourceClientFactory(sourceRepo *repository.SourceRepository, mpRepo *repository.MultipartUploadRepository, factory SourceClientFactory) *MultipartPartWriteService {
	svc := newObjectServiceBase(sourceRepo, factory)
	svc.multipart = mpRepo
	return &MultipartPartWriteService{service: svc}
}

func NewObjectDeleteService(sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) *ObjectDeleteService {
	return NewObjectDeleteServiceWithSourceClientFactory(sourceRepo, fileRepo, nil)
}

func NewObjectDeleteServiceWithSourceClientFactory(sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, factory SourceClientFactory) *ObjectDeleteService {
	svc := newObjectServiceBase(sourceRepo, factory)
	svc.files = fileRepo
	return &ObjectDeleteService{service: svc}
}

func NewBucketCleanupService(sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository) *BucketCleanupService {
	return NewBucketCleanupServiceWithSourceClientFactory(sourceRepo, fileRepo, mpRepo, nil)
}

func NewBucketCleanupServiceWithSourceClientFactory(sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, factory SourceClientFactory) *BucketCleanupService {
	svc := newObjectServiceBase(sourceRepo, factory)
	svc.files = fileRepo
	svc.multipart = mpRepo
	return &BucketCleanupService{service: svc}
}

func NewMultipartCompletionService(sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository) *MultipartCompletionService {
	return NewMultipartCompletionServiceWithSourceClientFactory(sourceRepo, fileRepo, mpRepo, nil)
}

func NewMultipartCompletionServiceWithSourceClientFactory(sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, factory SourceClientFactory) *MultipartCompletionService {
	svc := newObjectServiceBase(sourceRepo, factory)
	svc.files = fileRepo
	svc.multipart = mpRepo
	return &MultipartCompletionService{service: svc}
}

func newObjectServiceBase(sourceRepo *repository.SourceRepository, factory SourceClientFactory) *objectService {
	return &objectService{
		sources: sourceRepo,
		uploadChunks: func(ctx context.Context, r io.Reader, sources []repository.Source, chunkSize int, selector SourceSelector) ([]repository.FileChunk, error) {
			return UploadFileChunksWithStrategy(ctx, r, sources, chunkSize, factory, selector)
		},
		deleteChunks: func(ctx context.Context, chunks []repository.FileChunk) error {
			return DeleteFileChunksWithFactory(ctx, sourceRepo, chunks, factory)
		},
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *ObjectWriteService) PutObject(ctx context.Context, bucket *repository.Bucket, name string, body io.Reader, chunkSize int, contentType string, userMetadata map[string]string) (PutObjectResult, error) {
	return s.service.PutObject(ctx, bucket, name, body, chunkSize, contentType, userMetadata)
}

func (s *ObjectWriteService) CopyObject(ctx context.Context, sourceBucket, destBucket *repository.Bucket, sourceKey, destKey string) (CopyObjectResult, error) {
	return s.service.CopyObject(ctx, sourceBucket, destBucket, sourceKey, destKey)
}

func (s *MultipartPartWriteService) UploadMultipartPartRecord(ctx context.Context, bucket *repository.Bucket, mu *repository.MultipartUpload, partNumber int, body io.Reader, chunkSize int) (UploadMultipartPartResult, error) {
	return s.service.UploadMultipartPartRecord(ctx, bucket, mu, partNumber, body, chunkSize)
}

func (s *ObjectDeleteService) DeleteObject(ctx context.Context, bucketID primitive.ObjectID, name string) (DeleteObjectResult, error) {
	return s.service.DeleteObject(ctx, bucketID, name)
}

func (s *ObjectDeleteService) DeleteFile(ctx context.Context, bucketID, fileID primitive.ObjectID) (DeleteObjectResult, error) {
	return s.service.DeleteFile(ctx, bucketID, fileID)
}

func (s *BucketCleanupService) DeleteBucketContents(ctx context.Context, bucketID primitive.ObjectID) (DeleteBucketContentsResult, error) {
	return s.service.DeleteBucketContents(ctx, bucketID)
}

func (s *MultipartCompletionService) CompleteMultipartUpload(ctx context.Context, bucketID primitive.ObjectID, uploadID string, requestedParts []CompleteMultipartPart) (CompleteMultipartUploadResult, error) {
	return s.service.CompleteMultipartUpload(ctx, bucketID, uploadID, requestedParts)
}

func (s *MultipartCompletionService) CompleteMultipartUploadRecord(ctx context.Context, bucketID primitive.ObjectID, mu *repository.MultipartUpload, requestedParts []CompleteMultipartPart) (CompleteMultipartUploadResult, error) {
	return s.service.CompleteMultipartUploadRecord(ctx, bucketID, mu, requestedParts)
}

func (s *objectService) PutObject(ctx context.Context, bucket *repository.Bucket, name string, body io.Reader, chunkSize int, contentType string, userMetadata map[string]string) (PutObjectResult, error) {
	sources, err := s.sources.ListByIDs(ctx, bucket.SourceIDs)
	if err != nil {
		return PutObjectResult{}, err
	}
	if len(sources) == 0 {
		return PutObjectResult{}, ErrNoSources
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
	if len(sources) == 0 {
		return UploadMultipartPartResult{}, ErrNoSources
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

func (s *objectService) deleteFileChunksIfUnreferenced(ctx context.Context, chunks []repository.FileChunk) error {
	return deleteFileChunksIfUnreferenced(ctx, s.files, s.deleteChunks, chunks)
}

func (s *objectService) deleteBucketChunksIfUnreferenced(ctx context.Context, bucketID primitive.ObjectID, chunks []repository.FileChunk) error {
	return deleteBucketChunksIfUnreferenced(ctx, bucketID, s.files, s.multipart, s.deleteChunks, chunks)
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

func ObjectETag(file repository.File) string {
	if file.ETag != "" {
		return file.ETag
	}
	return legacyObjectETag(file)
}

func newObjectETag(file repository.File) string {
	if etag, ok := checksumObjectETag(file.Chunks); ok {
		return etag
	}
	return legacyObjectETag(file)
}

func copyObjectETag(source repository.File) string {
	if source.ETag != "" {
		return source.ETag
	}
	if etag, ok := checksumObjectETag(source.Chunks); ok {
		return etag
	}
	return legacyObjectETag(source)
}

func checksumObjectETag(chunks []repository.FileChunk) (string, bool) {
	h := sha256.New()
	for _, chunk := range chunks {
		if chunk.Checksum == "" {
			return "", false
		}
		_, _ = h.Write([]byte(chunk.Checksum))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(strconv.FormatInt(chunk.Size, 10)))
		_, _ = h.Write([]byte(";"))
	}
	return "\"" + hex.EncodeToString(h.Sum(nil)) + "\"", true
}

func legacyObjectETag(file repository.File) string {
	h := sha256.New()
	_, _ = h.Write([]byte(file.Name))
	_, _ = h.Write([]byte(file.CreatedAt.UTC().Format(time.RFC3339Nano)))
	for _, chunk := range file.Chunks {
		_, _ = h.Write([]byte(chunk.SourceID.Hex()))
		_, _ = h.Write([]byte(chunk.Name))
		_, _ = h.Write([]byte(strconv.Itoa(chunk.Order)))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(strconv.FormatInt(chunk.Size, 10)))
	}
	return "\"" + hex.EncodeToString(h.Sum(nil)) + "\""
}

func multipartPartETag(chunks []repository.FileChunk) string {
	h := md5.New()
	for _, chunk := range chunks {
		_, _ = h.Write([]byte(chunk.Name))
	}
	return fmt.Sprintf("\"%s\"", hex.EncodeToString(h.Sum(nil)))
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
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
