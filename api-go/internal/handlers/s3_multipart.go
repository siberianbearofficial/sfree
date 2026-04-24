package handlers

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// S3 XML response types for multipart uploads.

type initiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadId string   `xml:"UploadId"`
}

type completeMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

type completeMultipartUploadRequest struct {
	XMLName xml.Name         `xml:"CompleteMultipartUpload"`
	Parts   []completionPart `xml:"Part"`
}

type completionPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

type listMultipartUploadsResult struct {
	XMLName            xml.Name             `xml:"ListMultipartUploadsResult"`
	Xmlns              string               `xml:"xmlns,attr"`
	Bucket             string               `xml:"Bucket"`
	KeyMarker          string               `xml:"KeyMarker,omitempty"`
	UploadIDMarker     string               `xml:"UploadIdMarker,omitempty"`
	NextKeyMarker      string               `xml:"NextKeyMarker,omitempty"`
	NextUploadIDMarker string               `xml:"NextUploadIdMarker,omitempty"`
	Prefix             string               `xml:"Prefix,omitempty"`
	MaxUploads         int                  `xml:"MaxUploads"`
	Upload             []multipartUploadXML `xml:"Upload"`
	IsTruncated        bool                 `xml:"IsTruncated"`
}

type multipartUploadXML struct {
	Key       string `xml:"Key"`
	UploadId  string `xml:"UploadId"`
	Initiated string `xml:"Initiated"`
}

type listPartsResult struct {
	XMLName              xml.Name  `xml:"ListPartsResult"`
	Xmlns                string    `xml:"xmlns,attr"`
	Bucket               string    `xml:"Bucket"`
	Key                  string    `xml:"Key"`
	UploadId             string    `xml:"UploadId"`
	PartNumberMarker     int       `xml:"PartNumberMarker"`
	NextPartNumberMarker int       `xml:"NextPartNumberMarker,omitempty"`
	MaxParts             int       `xml:"MaxParts"`
	Part                 []partXML `xml:"Part"`
	IsTruncated          bool      `xml:"IsTruncated"`
}

type partXML struct {
	PartNumber   int    `xml:"PartNumber"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	LastModified string `xml:"LastModified"`
}

type multipartUploadAbortStore interface {
	GetByUploadID(ctx context.Context, uploadID string) (*repository.MultipartUpload, error)
	Delete(ctx context.Context, uploadID string) error
}

type multipartUploadPager interface {
	ListByBucketPage(ctx context.Context, bucketID primitive.ObjectID, prefix, keyMarker, uploadIDMarker string, limit int) ([]repository.MultipartUpload, bool, error)
}

type multipartUploadGetter interface {
	GetByUploadID(ctx context.Context, uploadID string) (*repository.MultipartUpload, error)
}

type multipartUploadListPage struct {
	entries            []multipartUploadXML
	isTruncated        bool
	nextKeyMarker      string
	nextUploadIDMarker string
}

type multipartPartsPage struct {
	parts                []partXML
	isTruncated          bool
	nextPartNumberMarker int
}

// PostObject dispatches POST requests on S3 object paths.
// ?uploads → CreateMultipartUpload
// ?uploadId=X → CompleteMultipartUpload
func PostObject(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, chunkSize int) gin.HandlerFunc {
	return PostObjectWithFactory(bucketRepo, sourceRepo, fileRepo, mpRepo, chunkSize, nil)
}

func PostObjectWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, chunkSize int, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil || mpRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		if _, ok := c.GetQuery("uploads"); ok {
			createMultipartUpload(c, bucketRepo, mpRepo)
			return
		}
		if _, ok := c.GetQuery("uploadId"); ok {
			completeMultipartUpload(c, bucketRepo, sourceRepo, fileRepo, mpRepo, factory)
			return
		}
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "missing uploads or uploadId parameter")
	}
}

// PostBucket dispatches POST requests on S3 bucket paths.
// ?delete -> DeleteObjects
func PostBucket(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return PostBucketWithFactory(bucketRepo, sourceRepo, fileRepo, nil)
}

func PostBucketWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	deleteObjectsHandler := DeleteObjectsWithFactory(bucketRepo, sourceRepo, fileRepo, factory)
	return func(c *gin.Context) {
		if _, ok := c.GetQuery("delete"); ok {
			deleteObjectsHandler(c)
			return
		}
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "missing delete parameter")
	}
}

// PutObjectOrPart dispatches PUT requests.
// x-amz-copy-source → CopyObject
// ?uploadId=X&partNumber=N → UploadPart
// otherwise → PutObject
func PutObjectOrPart(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, chunkSize int) gin.HandlerFunc {
	return PutObjectOrPartWithFactory(bucketRepo, sourceRepo, fileRepo, mpRepo, chunkSize, nil)
}

func PutObjectOrPartWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, chunkSize int, factory manager.SourceClientFactory) gin.HandlerFunc {
	putHandler := PutObjectWithFactory(bucketRepo, sourceRepo, fileRepo, chunkSize, factory)
	copyHandler := CopyObjectWithFactory(bucketRepo, sourceRepo, fileRepo, factory)
	return func(c *gin.Context) {
		if c.GetHeader("x-amz-copy-source") != "" {
			if _, ok := c.GetQuery("uploadId"); ok {
				writeS3Error(c, http.StatusNotImplemented, "NotImplemented", "UploadPartCopy is not supported")
				return
			}
			copyHandler(c)
			return
		}
		if mpRepo != nil {
			if _, ok := c.GetQuery("uploadId"); ok {
				uploadPart(c, bucketRepo, sourceRepo, mpRepo, chunkSize, factory)
				return
			}
		}
		putHandler(c)
	}
}

// DeleteObjectOrAbort dispatches DELETE requests.
// ?uploadId=X → AbortMultipartUpload
// otherwise → DeleteObject
func DeleteObjectOrAbort(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository) gin.HandlerFunc {
	return DeleteObjectOrAbortWithFactory(bucketRepo, sourceRepo, fileRepo, mpRepo, nil)
}

func DeleteObjectOrAbortWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	deleteHandler := DeleteObjectWithFactory(bucketRepo, sourceRepo, fileRepo, factory)
	return func(c *gin.Context) {
		if mpRepo != nil {
			if _, ok := c.GetQuery("uploadId"); ok {
				abortMultipartUpload(c, bucketRepo, sourceRepo, mpRepo, factory)
				return
			}
		}
		deleteHandler(c)
	}
}

// GetObjectOrParts dispatches GET requests on object paths.
// ?uploadId=X → ListParts
// otherwise → GetObject
func GetObjectOrParts(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository) gin.HandlerFunc {
	return GetObjectOrPartsWithFactory(bucketRepo, sourceRepo, fileRepo, mpRepo, nil)
}

func GetObjectOrPartsWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	getHandler := GetObjectWithFactory(bucketRepo, sourceRepo, fileRepo, factory)
	return func(c *gin.Context) {
		if mpRepo != nil {
			if _, ok := c.GetQuery("uploadId"); ok {
				listParts(c, bucketRepo, mpRepo)
				return
			}
		}
		getHandler(c)
	}
}

// ListObjectsOrUploads dispatches GET requests on bucket paths.
// ?uploads → ListMultipartUploads
// ?location → GetBucketLocation
// ?list-type=2 → ListObjectsV2
// otherwise → ListObjects
func ListObjectsOrUploads(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository) gin.HandlerFunc {
	listHandler := ListObjects(bucketRepo, fileRepo)
	listV2Handler := ListObjectsV2(bucketRepo, fileRepo)
	locationHandler := GetBucketLocation(bucketRepo)
	return func(c *gin.Context) {
		if mpRepo != nil {
			if _, ok := c.GetQuery("uploads"); ok {
				listMultipartUploads(c, bucketRepo, mpRepo)
				return
			}
		}
		if _, ok := c.GetQuery("location"); ok {
			locationHandler(c)
			return
		}
		if c.Query("list-type") == "2" {
			listV2Handler(c)
			return
		}
		listHandler(c)
	}
}

func createMultipartUpload(c *gin.Context, bucketRepo *repository.BucketRepository, mpRepo *repository.MultipartUploadRepository) {
	ctx := c.Request.Context()
	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}
	objectKey := s3ObjectKey(c)
	if objectKey == "" {
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "empty object key")
		return
	}
	uploadID := primitive.NewObjectID().Hex()
	mu := repository.MultipartUpload{
		BucketID:     bucketDoc.ID,
		ObjectKey:    objectKey,
		UploadID:     uploadID,
		CreatedAt:    time.Now().UTC(),
		ContentType:  requestObjectContentType(c.Request),
		UserMetadata: requestObjectUserMetadata(c.Request),
	}
	if _, err := mpRepo.Create(ctx, mu); err != nil {
		slog.ErrorContext(ctx, "create multipart upload", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}
	c.XML(http.StatusOK, initiateMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:   c.Param("bucket"),
		Key:      objectKey,
		UploadId: uploadID,
	})
}

func parseMultipartMaxUploads(c *gin.Context) (int, bool) {
	maxUploads := 1000
	raw := c.Query("max-uploads")
	if raw == "" {
		return maxUploads, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		writeS3Error(c, http.StatusBadRequest, "InvalidArgument", "max-uploads must be a non-negative integer")
		return 0, false
	}
	if parsed > maxUploads {
		parsed = maxUploads
	}
	return parsed, true
}

func parseMultipartMaxParts(c *gin.Context) (int, bool) {
	maxParts := 1000
	raw := c.Query("max-parts")
	if raw == "" {
		return maxParts, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		writeS3Error(c, http.StatusBadRequest, "InvalidArgument", "max-parts must be a non-negative integer")
		return 0, false
	}
	if parsed > maxParts {
		parsed = maxParts
	}
	return parsed, true
}

func buildMultipartUploadListPage(ctx context.Context, pager multipartUploadPager, bucketID primitive.ObjectID, prefix, keyMarker, uploadIDMarker string, maxUploads int) (multipartUploadListPage, error) {
	queryLimit := maxUploads
	if maxUploads == 0 {
		queryLimit = 1
	}

	uploads, hasMore, err := pager.ListByBucketPage(ctx, bucketID, prefix, keyMarker, uploadIDMarker, queryLimit)
	if err != nil {
		return multipartUploadListPage{}, err
	}

	page := multipartUploadListPage{
		entries: make([]multipartUploadXML, 0, len(uploads)),
	}
	if maxUploads == 0 {
		page.isTruncated = len(uploads) > 0 || hasMore
		return page, nil
	}

	for _, mu := range uploads {
		page.entries = append(page.entries, multipartUploadXML{
			Key:       mu.ObjectKey,
			UploadId:  mu.UploadID,
			Initiated: mu.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	page.isTruncated = hasMore
	if hasMore && len(uploads) > 0 {
		last := uploads[len(uploads)-1]
		page.nextKeyMarker = last.ObjectKey
		page.nextUploadIDMarker = last.UploadID
	}
	return page, nil
}

func buildMultipartPartsPage(mu *repository.MultipartUpload, partNumberMarker, maxParts int) multipartPartsPage {
	sorted := make([]repository.UploadPart, len(mu.Parts))
	copy(sorted, mu.Parts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PartNumber < sorted[j].PartNumber
	})

	start := 0
	for start < len(sorted) && sorted[start].PartNumber <= partNumberMarker {
		start++
	}

	if maxParts == 0 {
		page := multipartPartsPage{parts: []partXML{}}
		if start < len(sorted) {
			page.isTruncated = true
		}
		return page
	}

	end := start + maxParts
	if end > len(sorted) {
		end = len(sorted)
	}

	parts := make([]partXML, 0, end-start)
	for _, p := range sorted[start:end] {
		parts = append(parts, partXML{
			PartNumber:   p.PartNumber,
			ETag:         p.ETag,
			Size:         p.Size,
			LastModified: mu.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	page := multipartPartsPage{parts: parts}
	if end < len(sorted) {
		page.isTruncated = true
		page.nextPartNumberMarker = sorted[end-1].PartNumber
	}
	return page
}

func uploadPart(c *gin.Context, bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, mpRepo *repository.MultipartUploadRepository, chunkSize int, factory manager.SourceClientFactory) {
	ctx := c.Request.Context()
	uploadID := c.Query("uploadId")
	partNumStr := c.Query("partNumber")
	partNum, err := strconv.Atoi(partNumStr)
	if err != nil || partNum < 1 || partNum > 10000 {
		writeS3Error(c, http.StatusBadRequest, "InvalidArgument", "partNumber must be between 1 and 10000")
		return
	}

	mu, err := mpRepo.GetByUploadID(ctx, uploadID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
			return
		}
		slog.ErrorContext(ctx, "upload part: get upload", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}

	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}
	if mu.BucketID != bucketDoc.ID {
		writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
		return
	}

	objectSvc := manager.NewMultipartPartWriteServiceWithSourceClientFactory(sourceRepo, mpRepo, factory)
	result, err := objectSvc.UploadMultipartPartRecord(ctx, bucketDoc, mu, partNum, c.Request.Body, chunkSize)
	if err != nil {
		switch {
		case errors.Is(err, manager.ErrMultipartUploadNotFound):
			writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
		case isBucketSourceResolutionError(err):
			writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "no sources configured")
		default:
			slog.ErrorContext(ctx, "upload part: store part", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		}
		return
	}
	if result.CleanupErr != nil {
		slog.WarnContext(ctx, "upload part: delete old part chunks", slog.String("error", result.CleanupErr.Error()))
	}

	c.Header("ETag", result.ETag)
	c.Status(http.StatusOK)
}

func completeMultipartUpload(c *gin.Context, bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, factory manager.SourceClientFactory) {
	ctx := c.Request.Context()
	uploadID := c.Query("uploadId")

	mu, err := mpRepo.GetByUploadID(ctx, uploadID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
			return
		}
		slog.ErrorContext(ctx, "complete multipart: get upload", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}

	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}

	var req completeMultipartUploadRequest
	if err := xml.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		writeS3Error(c, http.StatusBadRequest, "MalformedXML", "could not parse CompleteMultipartUpload body")
		return
	}
	requestedParts := make([]manager.CompleteMultipartPart, 0, len(req.Parts))
	for _, rp := range req.Parts {
		requestedParts = append(requestedParts, manager.CompleteMultipartPart{PartNumber: rp.PartNumber, ETag: rp.ETag})
	}

	objectSvc := manager.NewMultipartCompletionServiceWithSourceClientFactory(sourceRepo, fileRepo, mpRepo, factory)
	result, err := objectSvc.CompleteMultipartUploadRecord(ctx, bucketDoc.ID, mu, requestedParts)
	if err != nil {
		switch {
		case errors.Is(err, manager.ErrMultipartUploadNotFound):
			writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
		case errors.Is(err, manager.ErrMultipartUploadHasNoParts):
			writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "at least one part is required")
		case errors.Is(err, manager.ErrMultipartUploadPartOrder):
			writeS3Error(c, http.StatusBadRequest, "InvalidPartOrder", "part numbers must be in ascending order")
		case errors.Is(err, manager.ErrMultipartUploadInvalidPart):
			var partErr manager.InvalidMultipartPartError
			if errors.As(err, &partErr) {
				message := fmt.Sprintf("part %d not uploaded", partErr.PartNumber)
				if partErr.Reason == "etag mismatch" {
					message = fmt.Sprintf("ETag mismatch for part %d", partErr.PartNumber)
				}
				writeS3Error(c, http.StatusBadRequest, "InvalidPart", message)
				return
			}
			writeS3Error(c, http.StatusBadRequest, "InvalidPart", "")
		default:
			slog.ErrorContext(ctx, "complete multipart: mutate object", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		}
		return
	}
	for _, cleanupErr := range result.CleanupErrs {
		slog.WarnContext(ctx, "complete multipart: cleanup", slog.String("error", cleanupErr.Error()))
	}
	c.XML(http.StatusOK, completeMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: fmt.Sprintf("/%s/%s", c.Param("bucket"), result.Upload.ObjectKey),
		Bucket:   c.Param("bucket"),
		Key:      result.Upload.ObjectKey,
		ETag:     result.ETag,
	})
}

func abortMultipartUpload(c *gin.Context, bucketRepo objectBucketReader, sourceRepo *repository.SourceRepository, mpRepo multipartUploadAbortStore, factory manager.SourceClientFactory) {
	ctx := c.Request.Context()
	uploadID := c.Query("uploadId")

	mu, err := mpRepo.GetByUploadID(ctx, uploadID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
			return
		}
		slog.ErrorContext(ctx, "abort multipart: get upload", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}

	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}
	if mu.BucketID != bucketDoc.ID {
		writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
		return
	}

	err = manager.AbortMultipartUploadRecord(ctx, mpRepo, func(ctx context.Context, chunks []repository.FileChunk) error {
		return manager.DeleteFileChunksWithFactory(ctx, sourceRepo, chunks, factory)
	}, mu)
	if err != nil {
		if errors.Is(err, manager.ErrMultipartUploadNotFound) {
			writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
			return
		}
		slog.ErrorContext(ctx, "abort multipart: cleanup", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}

	c.Status(http.StatusNoContent)
}

func listMultipartUploads(c *gin.Context, bucketRepo objectBucketReader, mpRepo multipartUploadPager) {
	ctx := c.Request.Context()
	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}

	maxUploads, ok := parseMultipartMaxUploads(c)
	if !ok {
		return
	}
	prefix := c.Query("prefix")
	keyMarker := c.Query("key-marker")
	uploadIDMarker := c.Query("upload-id-marker")
	if uploadIDMarker != "" && keyMarker == "" {
		writeS3Error(c, http.StatusBadRequest, "InvalidArgument", "upload-id-marker requires key-marker")
		return
	}
	if c.Query("delimiter") != "" {
		writeS3Error(c, http.StatusNotImplemented, "NotImplemented", "delimiter is not supported for ListMultipartUploads")
		return
	}

	page, err := buildMultipartUploadListPage(ctx, mpRepo, bucketDoc.ID, prefix, keyMarker, uploadIDMarker, maxUploads)
	if err != nil {
		slog.ErrorContext(ctx, "list multipart uploads", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}

	c.XML(http.StatusOK, listMultipartUploadsResult{
		Xmlns:              "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:             c.Param("bucket"),
		KeyMarker:          keyMarker,
		UploadIDMarker:     uploadIDMarker,
		NextKeyMarker:      page.nextKeyMarker,
		NextUploadIDMarker: page.nextUploadIDMarker,
		Prefix:             prefix,
		MaxUploads:         maxUploads,
		Upload:             page.entries,
		IsTruncated:        page.isTruncated,
	})
}

func listParts(c *gin.Context, bucketRepo objectBucketReader, mpRepo multipartUploadGetter) {
	ctx := c.Request.Context()
	uploadID := c.Query("uploadId")

	mu, err := mpRepo.GetByUploadID(ctx, uploadID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
			return
		}
		slog.ErrorContext(ctx, "list parts: get upload", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}

	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}
	if mu.BucketID != bucketDoc.ID {
		writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
		return
	}
	maxParts, ok := parseMultipartMaxParts(c)
	if !ok {
		return
	}
	partNumberMarker, err := strconv.Atoi(c.DefaultQuery("part-number-marker", "0"))
	if err != nil || partNumberMarker < 0 {
		writeS3Error(c, http.StatusBadRequest, "InvalidArgument", "part-number-marker must be a non-negative integer")
		return
	}

	page := buildMultipartPartsPage(mu, partNumberMarker, maxParts)

	c.XML(http.StatusOK, listPartsResult{
		Xmlns:                "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:               c.Param("bucket"),
		Key:                  mu.ObjectKey,
		UploadId:             uploadID,
		PartNumberMarker:     partNumberMarker,
		NextPartNumberMarker: page.nextPartNumberMarker,
		MaxParts:             maxParts,
		Part:                 page.parts,
		IsTruncated:          page.isTruncated,
	})
}
