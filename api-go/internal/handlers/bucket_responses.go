package handlers

import (
	"time"

	"github.com/example/sfree/api-go/internal/repository"
)

type bucketResponse struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	AccessKey string    `json:"access_key"`
	CreatedAt time.Time `json:"created_at"`
	Role      string    `json:"role"`
	Shared    bool      `json:"shared"`
}

func bucketResponseFromAccess(bucket repository.Bucket, role repository.BucketRole, shared bool) bucketResponse {
	return bucketResponse{
		ID:        bucket.ID.Hex(),
		Key:       bucket.Key,
		AccessKey: bucket.AccessKey,
		CreatedAt: bucket.CreatedAt,
		Role:      string(role),
		Shared:    shared,
	}
}
