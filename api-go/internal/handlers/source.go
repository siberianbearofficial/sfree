package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type createGDriveSourceRequest struct {
	Name string `json:"name" binding:"required"`
	Key  string `json:"key" binding:"required"`
}

type createSourceResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateGDriveSource godoc
// @Summary Create gdrive source
// @Tags sources
// @Accept json
// @Produce json
// @Param source body createGDriveSourceRequest true "Source to create"
// @Success 200 {object} createSourceResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources/gdrive [post]
func CreateGDriveSource(repo *repository.SourceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createGDriveSourceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("create gdrive source: invalid request: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		if repo == nil {
			log.Print("create gdrive source: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		source := repository.Source{
			UserID:    userID,
			Type:      repository.SourceTypeGDrive,
			Name:      req.Name,
			Key:       req.Key,
			CreatedAt: time.Now().UTC(),
		}
		created, err := repo.Create(c.Request.Context(), source)
		if err != nil {
			log.Printf("create gdrive source: failed to create: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, createSourceResponse{
			ID:        created.ID.Hex(),
			Name:      created.Name,
			Type:      string(created.Type),
			Key:       created.Key,
			CreatedAt: created.CreatedAt,
		})
	}
}
