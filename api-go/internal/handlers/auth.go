package handlers

import (
	"log"
	"net/http"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// BasicAuth is a middleware that validates HTTP Basic credentials and sets the user ID in context.
func BasicAuth(repo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			c.Header("WWW-Authenticate", `Basic realm="restricted"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if repo == nil {
			log.Print("basic auth: user repository is nil")
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		user, err := repo.GetByUsername(c.Request.Context(), username)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			log.Printf("basic auth: failed to get user: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set("userID", user.ID.Hex())
		c.Next()
	}
}
