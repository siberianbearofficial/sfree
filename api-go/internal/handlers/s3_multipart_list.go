package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
	if handleMultipartUploadLookupError(c, "list parts", err) {
		return
	}

	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}
	if handleMultipartBucketMismatch(c, mu.BucketID == bucketDoc.ID) {
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
