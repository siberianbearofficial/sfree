package handlers

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
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
	XMLName     xml.Name             `xml:"ListMultipartUploadsResult"`
	Xmlns       string               `xml:"xmlns,attr"`
	Bucket      string               `xml:"Bucket"`
	Upload      []multipartUploadXML `xml:"Upload"`
	IsTruncated bool                 `xml:"IsTruncated"`
}

type multipartUploadXML struct {
	Key       string `xml:"Key"`
	UploadId  string `xml:"UploadId"`
	Initiated string `xml:"Initiated"`
}

type listPartsResult struct {
	XMLName     xml.Name  `xml:"ListPartsResult"`
	Xmlns       string    `xml:"xmlns,attr"`
	Bucket      string    `xml:"Bucket"`
	Key         string    `xml:"Key"`
	UploadId    string    `xml:"UploadId"`
	Part        []partXML `xml:"Part"`
	IsTruncated bool      `xml:"IsTruncated"`
}

type partXML struct {
	PartNumber   int    `xml:"PartNumber"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	LastModified string `xml:"LastModified"`
}

// lookupBucket resolves the bucket from the request and validates access.
func lookupBucket(c *gin.Context, bucketRepo *repository.BucketRepository) (*repository.Bucket, bool) {
	ctx := c.Request.Context()
	bucketKey := c.Param("bucket")
	bucketDoc, err := bucketRepo.GetByKey(ctx, bucketKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
			return nil, false
		}
		slog.ErrorContext(ctx, "lookup bucket", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return nil, false
	}
	accessKey := c.GetString("accessKey")
	if accessKey == "" || bucketDoc.AccessKey != accessKey {
		writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
		return nil, false
	}
	return bucketDoc, true
}

// PostObject dispatches POST requests on S3 object paths.
// ?uploads → CreateMultipartUpload
// ?uploadId=X → CompleteMultipartUpload
func PostObject(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, chunkSize int) gin.HandlerFunc {
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
			completeMultipartUpload(c, bucketRepo, sourceRepo, fileRepo, mpRepo, chunkSize)
			return
		}
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "missing uploads or uploadId parameter")
	}
}

// PostBucket dispatches POST requests on S3 bucket paths.
// ?delete -> DeleteObjects
func PostBucket(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	deleteObjectsHandler := DeleteObjects(bucketRepo, sourceRepo, fileRepo)
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
	putHandler := PutObject(bucketRepo, sourceRepo, fileRepo, chunkSize)
	copyHandler := CopyObject(bucketRepo, sourceRepo, fileRepo)
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
				uploadPart(c, bucketRepo, sourceRepo, mpRepo, chunkSize)
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
	deleteHandler := DeleteObject(bucketRepo, sourceRepo, fileRepo)
	return func(c *gin.Context) {
		if mpRepo != nil {
			if _, ok := c.GetQuery("uploadId"); ok {
				abortMultipartUpload(c, sourceRepo, mpRepo)
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
	getHandler := GetObject(bucketRepo, sourceRepo, fileRepo)
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
// ?list-type=2 → ListObjectsV2
// otherwise → ListObjects
func ListObjectsOrUploads(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository) gin.HandlerFunc {
	listHandler := ListObjects(bucketRepo, fileRepo)
	listV2Handler := ListObjectsV2(bucketRepo, fileRepo)
	return func(c *gin.Context) {
		if mpRepo != nil {
			if _, ok := c.GetQuery("uploads"); ok {
				listMultipartUploads(c, bucketRepo, mpRepo)
				return
			}
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
	objectKey := strings.TrimPrefix(c.Param("object"), "/")
	if objectKey == "" {
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "empty object key")
		return
	}
	uploadID := primitive.NewObjectID().Hex()
	mu := repository.MultipartUpload{
		BucketID:  bucketDoc.ID,
		ObjectKey: objectKey,
		UploadID:  uploadID,
		CreatedAt: time.Now().UTC(),
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

func uploadPart(c *gin.Context, bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, mpRepo *repository.MultipartUploadRepository, chunkSize int) {
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

	sources, err := sourceRepo.ListByIDs(ctx, bucketDoc.SourceIDs)
	if err != nil {
		slog.ErrorContext(ctx, "upload part: list sources", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}
	if len(sources) == 0 {
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "no sources configured")
		return
	}

	// Delete old part chunks if re-uploading the same part number.
	for _, p := range mu.Parts {
		if p.PartNumber == partNum && len(p.Chunks) > 0 {
			if delErr := manager.DeleteFileChunks(ctx, sourceRepo, p.Chunks); delErr != nil {
				slog.WarnContext(ctx, "upload part: delete old part chunks", slog.String("error", delErr.Error()))
			}
			break
		}
	}

	selector := manager.SelectorForBucket(bucketDoc, sources)
	chunks, err := manager.UploadFileChunksWithStrategy(ctx, c.Request.Body, sources, chunkSize, nil, selector)
	if err != nil {
		slog.ErrorContext(ctx, "upload part: upload chunks", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}

	var totalSize int64
	h := md5.New()
	for _, ch := range chunks {
		totalSize += ch.Size
		_, _ = h.Write([]byte(ch.Name))
	}
	etag := fmt.Sprintf("\"%s\"", hex.EncodeToString(h.Sum(nil)))

	part := repository.UploadPart{
		PartNumber: partNum,
		ETag:       etag,
		Size:       totalSize,
		Chunks:     chunks,
	}
	if err := mpRepo.SetPart(ctx, uploadID, part); err != nil {
		slog.ErrorContext(ctx, "upload part: set part", slog.String("error", err.Error()))
		// Best-effort cleanup of the uploaded chunks.
		_ = manager.DeleteFileChunks(ctx, sourceRepo, chunks)
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}

	c.Header("ETag", etag)
	c.Status(http.StatusOK)
}

func completeMultipartUpload(c *gin.Context, bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, chunkSize int) {
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
	if mu.BucketID != bucketDoc.ID {
		writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
		return
	}

	var req completeMultipartUploadRequest
	if err := xml.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		writeS3Error(c, http.StatusBadRequest, "MalformedXML", "could not parse CompleteMultipartUpload body")
		return
	}
	if len(req.Parts) == 0 {
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "at least one part is required")
		return
	}

	// Validate part numbers are ascending.
	for i := 1; i < len(req.Parts); i++ {
		if req.Parts[i].PartNumber <= req.Parts[i-1].PartNumber {
			writeS3Error(c, http.StatusBadRequest, "InvalidPartOrder", "part numbers must be in ascending order")
			return
		}
	}

	// Build lookup of uploaded parts.
	partMap := make(map[int]repository.UploadPart, len(mu.Parts))
	for _, p := range mu.Parts {
		partMap[p.PartNumber] = p
	}

	// Validate all requested parts exist and ETags match.
	for _, rp := range req.Parts {
		up, exists := partMap[rp.PartNumber]
		if !exists {
			writeS3Error(c, http.StatusBadRequest, "InvalidPart", fmt.Sprintf("part %d not uploaded", rp.PartNumber))
			return
		}
		// Normalize ETags for comparison (strip quotes).
		reqETag := strings.Trim(rp.ETag, "\"")
		upETag := strings.Trim(up.ETag, "\"")
		if reqETag != upETag {
			writeS3Error(c, http.StatusBadRequest, "InvalidPart", fmt.Sprintf("ETag mismatch for part %d", rp.PartNumber))
			return
		}
	}
	allChunks := completedMultipartChunks(req.Parts, partMap)

	fileDoc := repository.File{
		BucketID:  bucketDoc.ID,
		Name:      mu.ObjectKey,
		CreatedAt: time.Now().UTC(),
		Chunks:    allChunks,
	}
	_, previousFile, err := fileRepo.ReplaceByName(ctx, fileDoc)
	if err != nil {
		slog.ErrorContext(ctx, "complete multipart: save file", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}
	if previousFile != nil {
		if delErr := deleteFileChunksIfUnreferenced(ctx, sourceRepo, fileRepo, previousFile.Chunks); delErr != nil {
			slog.WarnContext(ctx, "complete multipart: delete old chunks", slog.String("error", delErr.Error()))
		}
	}

	// Delete chunks for parts that were uploaded but NOT referenced in the completion request.
	requestedParts := make(map[int]bool, len(req.Parts))
	for _, rp := range req.Parts {
		requestedParts[rp.PartNumber] = true
	}
	for _, p := range mu.Parts {
		if !requestedParts[p.PartNumber] {
			if delErr := manager.DeleteFileChunks(ctx, sourceRepo, p.Chunks); delErr != nil {
				slog.WarnContext(ctx, "complete multipart: delete unreferenced part", slog.String("error", delErr.Error()))
			}
		}
	}

	// Clean up the multipart upload record.
	if err := mpRepo.Delete(ctx, uploadID); err != nil {
		slog.WarnContext(ctx, "complete multipart: delete upload record", slog.String("error", err.Error()))
	}

	// Compute multipart ETag: md5-of-part-md5s-N
	h := md5.New()
	for _, rp := range req.Parts {
		up := partMap[rp.PartNumber]
		partHash := strings.Trim(up.ETag, "\"")
		raw, _ := hex.DecodeString(partHash)
		_, _ = h.Write(raw)
	}
	etag := fmt.Sprintf("\"%s-%d\"", hex.EncodeToString(h.Sum(nil)), len(req.Parts))

	c.XML(http.StatusOK, completeMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: fmt.Sprintf("/%s/%s", c.Param("bucket"), mu.ObjectKey),
		Bucket:   c.Param("bucket"),
		Key:      mu.ObjectKey,
		ETag:     etag,
	})
}

func completedMultipartChunks(parts []completionPart, partMap map[int]repository.UploadPart) []repository.FileChunk {
	var allChunks []repository.FileChunk
	chunkOrder := 0
	for _, rp := range parts {
		up := partMap[rp.PartNumber]
		for _, ch := range up.Chunks {
			ch.Order = chunkOrder
			allChunks = append(allChunks, ch)
			chunkOrder++
		}
	}
	return allChunks
}

func abortMultipartUpload(c *gin.Context, sourceRepo *repository.SourceRepository, mpRepo *repository.MultipartUploadRepository) {
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

	// Delete all part chunks from sources.
	for _, p := range mu.Parts {
		if delErr := manager.DeleteFileChunks(ctx, sourceRepo, p.Chunks); delErr != nil {
			slog.WarnContext(ctx, "abort multipart: delete part chunks",
				slog.Int("part_number", p.PartNumber),
				slog.String("error", delErr.Error()),
			)
		}
	}

	if err := mpRepo.Delete(ctx, uploadID); err != nil {
		slog.WarnContext(ctx, "abort multipart: delete upload record", slog.String("error", err.Error()))
	}

	c.Status(http.StatusNoContent)
}

func listMultipartUploads(c *gin.Context, bucketRepo *repository.BucketRepository, mpRepo *repository.MultipartUploadRepository) {
	ctx := c.Request.Context()
	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}

	uploads, err := mpRepo.ListByBucket(ctx, bucketDoc.ID)
	if err != nil {
		slog.ErrorContext(ctx, "list multipart uploads", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}

	entries := make([]multipartUploadXML, 0, len(uploads))
	for _, mu := range uploads {
		entries = append(entries, multipartUploadXML{
			Key:       mu.ObjectKey,
			UploadId:  mu.UploadID,
			Initiated: mu.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	c.XML(http.StatusOK, listMultipartUploadsResult{
		Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:      c.Param("bucket"),
		Upload:      entries,
		IsTruncated: false,
	})
}

func listParts(c *gin.Context, bucketRepo *repository.BucketRepository, mpRepo *repository.MultipartUploadRepository) {
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

	// Sort parts by part number.
	sorted := make([]repository.UploadPart, len(mu.Parts))
	copy(sorted, mu.Parts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PartNumber < sorted[j].PartNumber
	})

	parts := make([]partXML, 0, len(sorted))
	for _, p := range sorted {
		parts = append(parts, partXML{
			PartNumber:   p.PartNumber,
			ETag:         p.ETag,
			Size:         p.Size,
			LastModified: mu.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	c.XML(http.StatusOK, listPartsResult{
		Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:      c.Param("bucket"),
		Key:         mu.ObjectKey,
		UploadId:    uploadID,
		Part:        parts,
		IsTruncated: false,
	})
}
