package manager

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
