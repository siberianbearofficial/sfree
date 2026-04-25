package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type bucketPreflightDecision string

const (
	bucketPreflightDecisionReady           bucketPreflightDecision = "ready"
	bucketPreflightDecisionConfirmRequired bucketPreflightDecision = "confirm_required"
	bucketPreflightDecisionBlocked         bucketPreflightDecision = "blocked"
)

type bucketPreflightRequest struct {
	SourceIDs []string `json:"source_ids" binding:"required,min=1"`
}

type bucketPreflightSourceResponse struct {
	SourceID             string    `json:"source_id"`
	SourceName           string    `json:"source_name"`
	SourceType           string    `json:"source_type"`
	Status               string    `json:"status"`
	ReasonCode           string    `json:"reason_code"`
	Message              string    `json:"message"`
	QuotaTotalBytes      *int64    `json:"quota_total_bytes" extensions:"x-nullable"`
	QuotaUsedBytes       *int64    `json:"quota_used_bytes" extensions:"x-nullable"`
	QuotaFreeBytes       *int64    `json:"quota_free_bytes" extensions:"x-nullable"`
	RequiresConfirmation bool      `json:"requires_confirmation"`
	BlocksCreation       bool      `json:"blocks_creation"`
	CheckedAt            time.Time `json:"checked_at"`
}

type bucketPreflightResponse struct {
	Decision                string                          `json:"decision"`
	Message                 string                          `json:"message"`
	HealthySourceCount      int                             `json:"healthy_source_count"`
	DegradedSourceCount     int                             `json:"degraded_source_count"`
	UnhealthySourceCount    int                             `json:"unhealthy_source_count"`
	NearCapacitySourceCount int                             `json:"near_capacity_source_count"`
	Sources                 []bucketPreflightSourceResponse `json:"sources"`
}

type bucketPreflightConflictResponse struct {
	Error string `json:"error"`
	bucketPreflightResponse
}

type bucketPreflightInputError struct {
	status  int
	message string
}

func (e bucketPreflightInputError) Error() string {
	return e.message
}

func bucketPreflightConflict(summary bucketPreflightResponse) bucketPreflightConflictResponse {
	return bucketPreflightConflictResponse{
		Error:                   summary.Message,
		bucketPreflightResponse: summary,
	}
}

func bucketPreflightForSources(ctx context.Context, userID primitive.ObjectID, sourceRepo sourceGetter, sourceIDHexes []string, factory manager.SourceClientFactory) (bucketPreflightResponse, []primitive.ObjectID, error) {
	resolvedSources, sourceIDs, err := loadOwnedBucketSources(ctx, userID, sourceRepo, sourceIDHexes)
	if err != nil {
		return bucketPreflightResponse{}, nil, err
	}

	resp := bucketPreflightResponse{
		Decision: string(bucketPreflightDecisionReady),
		Sources:  make([]bucketPreflightSourceResponse, 0, len(resolvedSources)),
	}

	for _, src := range resolvedSources {
		health, err := manager.CheckSourceHealth(ctx, &src, factory)
		if err == manager.ErrUnsupportedSourceType {
			health = manager.SourceHealth{
				SourceID:   src.ID.Hex(),
				SourceType: string(src.Type),
				Status:     manager.SourceHealthDegraded,
				CheckedAt:  time.Now().UTC(),
				ReasonCode: "health_unsupported",
				Message:    "SFree could not verify source health for this source type.",
			}
		} else if err != nil {
			return bucketPreflightResponse{}, nil, err
		}

		item := bucketPreflightSourceResponse{
			SourceID:        src.ID.Hex(),
			SourceName:      src.Name,
			SourceType:      string(src.Type),
			Status:          string(health.Status),
			ReasonCode:      health.ReasonCode,
			Message:         health.Message,
			QuotaTotalBytes: health.Quota.TotalBytes,
			QuotaUsedBytes:  health.Quota.UsedBytes,
			QuotaFreeBytes:  health.Quota.FreeBytes,
			CheckedAt:       health.CheckedAt,
		}

		switch health.Status {
		case manager.SourceHealthUnhealthy:
			item.BlocksCreation = true
			resp.UnhealthySourceCount++
		case manager.SourceHealthDegraded:
			item.RequiresConfirmation = true
			resp.DegradedSourceCount++
		default:
			resp.HealthySourceCount++
		}
		if health.ReasonCode == "quota_low" {
			item.RequiresConfirmation = true
			resp.NearCapacitySourceCount++
		}
		if item.BlocksCreation {
			resp.Decision = string(bucketPreflightDecisionBlocked)
		} else if item.RequiresConfirmation && resp.Decision != string(bucketPreflightDecisionBlocked) {
			resp.Decision = string(bucketPreflightDecisionConfirmRequired)
		}

		resp.Sources = append(resp.Sources, item)
	}

	switch bucketPreflightDecision(resp.Decision) {
	case bucketPreflightDecisionBlocked:
		resp.Message = "Remove unhealthy sources before creating this bucket."
	case bucketPreflightDecisionConfirmRequired:
		resp.Message = "Selected sources include degraded or near-capacity providers. Confirm the risk before creating this bucket."
	default:
		resp.Message = "Selected sources passed the current bucket preflight."
	}

	return resp, sourceIDs, nil
}

func loadOwnedBucketSources(ctx context.Context, userID primitive.ObjectID, sourceRepo sourceGetter, sourceIDHexes []string) ([]repository.Source, []primitive.ObjectID, error) {
	if isNilDependency(sourceRepo) {
		return nil, nil, bucketPreflightInputError{status: http.StatusServiceUnavailable, message: "source repository is unavailable"}
	}
	sourceIDs := make([]primitive.ObjectID, 0, len(sourceIDHexes))
	seenSourceIDs := make(map[primitive.ObjectID]struct{}, len(sourceIDHexes))
	sources := make([]repository.Source, 0, len(sourceIDHexes))

	for _, sourceIDHex := range sourceIDHexes {
		sourceID, err := primitive.ObjectIDFromHex(sourceIDHex)
		if err != nil {
			return nil, nil, bucketPreflightInputError{status: http.StatusBadRequest, message: fmt.Sprintf("source_ids contains invalid source id %q", sourceIDHex)}
		}
		if _, ok := seenSourceIDs[sourceID]; ok {
			return nil, nil, bucketPreflightInputError{status: http.StatusBadRequest, message: fmt.Sprintf("source_ids contains duplicate source id %q", sourceIDHex)}
		}
		seenSourceIDs[sourceID] = struct{}{}

		sourceDoc, err := sourceRepo.GetByID(ctx, sourceID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, nil, bucketPreflightInputError{status: http.StatusBadRequest, message: fmt.Sprintf("source_ids contains unknown source id %q", sourceIDHex)}
			}
			return nil, nil, bucketPreflightInputError{status: http.StatusInternalServerError, message: "failed to load sources for bucket preflight"}
		}
		if sourceDoc.UserID != userID {
			return nil, nil, bucketPreflightInputError{status: http.StatusBadRequest, message: fmt.Sprintf("source_ids contains source id %q that is not owned by the authenticated user", sourceIDHex)}
		}

		sources = append(sources, *sourceDoc)
		sourceIDs = append(sourceIDs, sourceID)
	}

	return sources, sourceIDs, nil
}

// BucketPreflight godoc
// @Summary Preflight bucket creation
// @Tags buckets
// @Accept json
// @Produce json
// @Param bucket body bucketPreflightRequest true "Bucket preflight request"
// @Success 200 {object} bucketPreflightResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/preflight [post]
func BucketPreflight(sourceRepo sourceGetter) gin.HandlerFunc {
	return BucketPreflightWithFactory(sourceRepo, nil)
}

func BucketPreflightWithFactory(sourceRepo sourceGetter, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req bucketPreflightRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		userID, ok := authenticatedUserID(c)
		if !ok {
			return
		}
		resp, _, err := bucketPreflightForSources(ctx, userID, sourceRepo, req.SourceIDs, factory)
		if err != nil {
			if inputErr, ok := err.(bucketPreflightInputError); ok {
				c.JSON(inputErr.status, gin.H{"error": inputErr.message})
				return
			}
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}
