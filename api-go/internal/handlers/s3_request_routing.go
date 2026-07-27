package handlers

import "github.com/gin-gonic/gin"

const (
	s3BucketContextKey = "s3Bucket"
	s3ObjectContextKey = "s3Object"
)

func S3BucketKey(c *gin.Context) string {
	if bucketKey := c.GetString(s3BucketContextKey); bucketKey != "" {
		return bucketKey
	}
	return c.Param("bucket")
}

func SetS3BucketKey(c *gin.Context, bucketKey string) {
	c.Set(s3BucketContextKey, bucketKey)
	upsertRouteParam(c, "bucket", bucketKey)
}

func s3ObjectKey(c *gin.Context) string {
	if objectKey := c.GetString(s3ObjectContextKey); objectKey != "" {
		return objectKey
	}
	return trimS3ObjectParam(c.Param("object"))
}

func SetS3ObjectKey(c *gin.Context, objectKey string) {
	c.Set(s3ObjectContextKey, objectKey)
	rawParam := ""
	if objectKey != "" {
		rawParam = "/" + objectKey
	}
	upsertRouteParam(c, "object", rawParam)
}

func trimS3ObjectParam(raw string) string {
	if raw == "/" {
		return ""
	}
	for len(raw) > 0 && raw[0] == '/' {
		raw = raw[1:]
	}
	return raw
}

func upsertRouteParam(c *gin.Context, key, value string) {
	for i := range c.Params {
		if c.Params[i].Key == key {
			c.Params[i].Value = value
			return
		}
	}
	c.Params = append(c.Params, gin.Param{Key: key, Value: value})
}
