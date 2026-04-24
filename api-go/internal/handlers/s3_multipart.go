package handlers

import (
	"net/http"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
)

// PostObject dispatches POST requests on S3 object paths.
// ?uploads -> CreateMultipartUpload
// ?uploadId=X -> CompleteMultipartUpload
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
// x-amz-copy-source -> CopyObject
// ?uploadId=X&partNumber=N -> UploadPart
// otherwise -> PutObject
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
// ?uploadId=X -> AbortMultipartUpload
// otherwise -> DeleteObject
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
// ?uploadId=X -> ListParts
// otherwise -> GetObject
func GetObjectOrParts(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository) gin.HandlerFunc {
	return GetObjectOrPartsWithFactory(bucketRepo, sourceRepo, fileRepo, mpRepo, nil)
}

func GetObjectOrPartsWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	getHandler := GetObjectWithFactory(bucketRepo, sourceRepo, fileRepo, factory)
	listHandler := ListObjectsOrUploads(bucketRepo, fileRepo, mpRepo)
	return func(c *gin.Context) {
		if s3ObjectKey(c) == "" {
			listHandler(c)
			return
		}
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
// ?uploads -> ListMultipartUploads
// ?location -> GetBucketLocation
// ?list-type=2 -> ListObjectsV2
// otherwise -> ListObjects
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
