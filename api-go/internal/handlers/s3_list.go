package handlers

import (
	"context"
	"encoding/xml"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const listCommonPrefixSkipSuffix = "\U0010FFFF"

type listBucketResult struct {
	XMLName               xml.Name           `xml:"ListBucketResult"`
	Xmlns                 string             `xml:"xmlns,attr"`
	Name                  string             `xml:"Name"`
	Prefix                string             `xml:"Prefix"`
	Marker                string             `xml:"Marker,omitempty"`
	NextMarker            string             `xml:"NextMarker,omitempty"`
	MaxKeys               int                `xml:"MaxKeys"`
	Delimiter             string             `xml:"Delimiter,omitempty"`
	IsTruncated           bool               `xml:"IsTruncated"`
	Contents              []listBucketEntry  `xml:"Contents"`
	CommonPrefixes        []listCommonPrefix `xml:"CommonPrefixes"`
	KeyCount              int                `xml:"KeyCount"`
	ContinuationToken     string             `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string             `xml:"NextContinuationToken,omitempty"`
	StartAfter            string             `xml:"StartAfter,omitempty"`
}

type listBucketEntry struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type listCommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

type listBucketPageItem struct {
	sortKey      string
	entry        listBucketEntry
	hasEntry     bool
	commonPrefix string
}

type listBucketFilePager interface {
	ListByBucketWithPrefixPage(ctx context.Context, bucketID primitive.ObjectID, prefix, after string, limit int) ([]repository.File, bool, error)
}

func parseListMaxKeys(c *gin.Context) (int, bool) {
	maxKeys := 1000
	raw := c.Query("max-keys")
	if raw == "" {
		return maxKeys, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		writeS3Error(c, http.StatusBadRequest, "InvalidArgument", "max-keys must be a non-negative integer")
		return 0, false
	}
	if parsed > maxKeys {
		parsed = maxKeys
	}
	return parsed, true
}

func fileListBucketEntry(file repository.File) listBucketEntry {
	return listBucketEntry{
		Key:          file.Name,
		LastModified: file.CreatedAt.UTC().Format(time.RFC3339),
		ETag:         manager.ObjectETag(file),
		Size:         manager.FileSize(file),
		StorageClass: "STANDARD",
	}
}

func buildListBucketPage(ctx context.Context, fileRepo listBucketFilePager, bucketID primitive.ObjectID, prefix, delimiter, after string, maxKeys int) ([]listBucketEntry, []listCommonPrefix, bool, string, error) {
	batchLimit := maxKeys + 1
	if batchLimit < 1 {
		batchLimit = 1
	}
	if batchLimit > 1001 {
		batchLimit = 1001
	}

	contents := make([]listBucketEntry, 0, maxKeys)
	commonPrefixes := make([]listCommonPrefix, 0, maxKeys)
	seenPrefixes := make(map[string]struct{})
	cursorAfter := after
	lastToken := ""

	for {
		files, hasMore, err := fileRepo.ListByBucketWithPrefixPage(ctx, bucketID, prefix, cursorAfter, batchLimit)
		if err != nil {
			return nil, nil, false, "", err
		}
		if len(files) == 0 {
			return contents, commonPrefixes, false, "", nil
		}

		for _, file := range files {
			if file.Name > cursorAfter {
				cursorAfter = file.Name
			}

			item := listBucketPageItem{
				sortKey:  file.Name,
				entry:    fileListBucketEntry(file),
				hasEntry: true,
			}
			if delimiter != "" {
				remainder := strings.TrimPrefix(file.Name, prefix)
				if idx := strings.Index(remainder, delimiter); idx >= 0 {
					commonPrefix := prefix + remainder[:idx+len(delimiter)]
					skipAfter := commonPrefix + listCommonPrefixSkipSuffix
					if skipAfter > cursorAfter {
						cursorAfter = skipAfter
					}
					if commonPrefix <= after {
						continue
					}
					if _, ok := seenPrefixes[commonPrefix]; ok {
						continue
					}
					seenPrefixes[commonPrefix] = struct{}{}
					item = listBucketPageItem{sortKey: commonPrefix, commonPrefix: commonPrefix}
				}
			}

			if maxKeys == 0 {
				return contents, commonPrefixes, true, item.sortKey, nil
			}
			if len(contents)+len(commonPrefixes) == maxKeys {
				return contents, commonPrefixes, true, lastToken, nil
			}

			if item.hasEntry {
				contents = append(contents, item.entry)
			} else {
				commonPrefixes = append(commonPrefixes, listCommonPrefix{Prefix: item.commonPrefix})
			}
			lastToken = item.sortKey
		}

		if !hasMore {
			return contents, commonPrefixes, false, "", nil
		}
	}
}

// ListObjects godoc
// @Summary List objects
// @Tags s3
// @Produce xml
// @Param bucket path string true "Bucket key"
// @Success 200 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Router /api/s3/{bucket} [get]
func ListObjects(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketKey := c.Param("bucket")
		bucketDoc, ok := lookupBucket(c, bucketRepo)
		if !ok {
			return
		}
		maxKeys, ok := parseListMaxKeys(c)
		if !ok {
			return
		}
		prefix := c.Query("prefix")
		delimiter := c.Query("delimiter")
		marker := c.Query("marker")
		contents, commonPrefixes, isTruncated, nextMarker, err := buildListBucketPage(ctx, fileRepo, bucketDoc.ID, prefix, delimiter, marker, maxKeys)
		if err != nil {
			slog.ErrorContext(ctx, "list objects: list files", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		result := listBucketResult{
			Xmlns:          "http://s3.amazonaws.com/doc/2006-03-01/",
			Name:           bucketKey,
			Prefix:         prefix,
			Marker:         marker,
			NextMarker:     nextMarker,
			MaxKeys:        maxKeys,
			Delimiter:      delimiter,
			IsTruncated:    isTruncated,
			Contents:       contents,
			CommonPrefixes: commonPrefixes,
			KeyCount:       len(contents) + len(commonPrefixes),
		}
		c.XML(http.StatusOK, result)
	}
}

func ListObjectsV2(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketKey := c.Param("bucket")
		bucketDoc, ok := lookupBucket(c, bucketRepo)
		if !ok {
			return
		}
		maxKeys, ok := parseListMaxKeys(c)
		if !ok {
			return
		}
		prefix := c.Query("prefix")
		delimiter := c.Query("delimiter")
		continuationToken := c.Query("continuation-token")
		startAfter := c.Query("start-after")
		after := startAfter
		if continuationToken != "" {
			after = continuationToken
		}
		contents, commonPrefixes, isTruncated, nextToken, err := buildListBucketPage(ctx, fileRepo, bucketDoc.ID, prefix, delimiter, after, maxKeys)
		if err != nil {
			slog.ErrorContext(ctx, "list objects v2: list files", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		result := listBucketResult{
			Xmlns:                 "http://s3.amazonaws.com/doc/2006-03-01/",
			Name:                  bucketKey,
			Prefix:                prefix,
			MaxKeys:               maxKeys,
			Delimiter:             delimiter,
			IsTruncated:           isTruncated,
			Contents:              contents,
			CommonPrefixes:        commonPrefixes,
			KeyCount:              len(contents) + len(commonPrefixes),
			ContinuationToken:     continuationToken,
			NextContinuationToken: nextToken,
			StartAfter:            startAfter,
		}
		c.XML(http.StatusOK, result)
	}
}
