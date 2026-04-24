package handlers

import (
	"context"
	"encoding/xml"
	"log/slog"
	"net/http"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

const s3BucketRegion = "us-east-1"

type bucketAccessKeyReader interface {
	GetByAccessKey(ctx context.Context, accessKey string) (*repository.Bucket, error)
}

type listAllMyBucketsResult struct {
	XMLName xml.Name         `xml:"ListAllMyBucketsResult"`
	Xmlns   string           `xml:"xmlns,attr"`
	Owner   listBucketsOwner `xml:"Owner"`
	Buckets bucketList       `xml:"Buckets"`
}

type listBucketsOwner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName,omitempty"`
}

type bucketList struct {
	Buckets []bucketListEntry `xml:"Bucket"`
}

type bucketListEntry struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
}

type bucketLocationConstraint struct {
	XMLName xml.Name `xml:"LocationConstraint"`
	Xmlns   string   `xml:"xmlns,attr"`
	Value   string   `xml:",chardata"`
}

func lookupBucketByAccessKey(c *gin.Context, repo bucketAccessKeyReader) (*repository.Bucket, bool) {
	ctx := c.Request.Context()
	accessKey := c.GetString("accessKey")
	if accessKey == "" {
		writeS3Error(c, http.StatusForbidden, "AccessDenied", "")
		return nil, false
	}
	bucketDoc, err := repo.GetByAccessKey(ctx, accessKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusForbidden, "InvalidAccessKeyId", "")
			return nil, false
		}
		slog.ErrorContext(ctx, "lookup bucket by access key", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return nil, false
	}
	return bucketDoc, true
}

func setBucketRegionHeader(c *gin.Context) {
	c.Header("x-amz-bucket-region", s3BucketRegion)
}

// ListBucketsS3 godoc
// @Summary List buckets
// @Tags s3
// @Produce xml
// @Success 200 {string} string ""
// @Failure 403 {string} string ""
// @Failure 500 {string} string ""
// @Router / [get]
// @Router /api/s3 [get]
func ListBucketsS3(repo bucketAccessKeyReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketDoc, ok := lookupBucketByAccessKey(c, repo)
		if !ok {
			return
		}
		setBucketRegionHeader(c)
		c.XML(http.StatusOK, listAllMyBucketsResult{
			Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
			Owner: listBucketsOwner{
				ID:          bucketDoc.UserID.Hex(),
				DisplayName: bucketDoc.UserID.Hex(),
			},
			Buckets: bucketList{
				Buckets: []bucketListEntry{{
					Name:         bucketDoc.Key,
					CreationDate: bucketDoc.CreatedAt.UTC().Format(time.RFC3339),
				}},
			},
		})
	}
}

// HeadBucket godoc
// @Summary Head bucket
// @Tags s3
// @Param bucket path string true "Bucket key"
// @Success 200 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Router /{bucket} [head]
// @Router /api/s3/{bucket} [head]
func HeadBucket(bucketRepo objectBucketReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		if _, ok := lookupBucket(c, bucketRepo); !ok {
			return
		}
		setBucketRegionHeader(c)
		c.Status(http.StatusOK)
	}
}

func GetBucketLocation(bucketRepo objectBucketReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		if _, ok := lookupBucket(c, bucketRepo); !ok {
			return
		}
		setBucketRegionHeader(c)
		c.XML(http.StatusOK, bucketLocationConstraint{
			Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
		})
	}
}
