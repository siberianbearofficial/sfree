package handlers

import (
	"net/http"

	"github.com/example/s3aas/api-go/internal/db"
	"github.com/gin-gonic/gin"
)

// DBProbe godoc
// @Summary Check Mongo connection
// @Tags database
// @Success 200 {string} string ""
// @Failure 503 {string} string ""
// @Router /dbz [get]
func DBProbe(m *db.Mongo) gin.HandlerFunc {
	return func(c *gin.Context) {
		if m == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		if err := m.Client.Ping(c.Request.Context(), nil); err != nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusOK)
	}
}
