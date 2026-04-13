package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// Auth returns a middleware that accepts both HTTP Basic Auth and Bearer JWT tokens.
// If a valid Bearer token is present it takes precedence over Basic Auth.
func Auth(repo *repository.UserRepository, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Resolve JWT: prefer Authorization header, fall back to auth_token cookie.
		tokenStr := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else if cookieToken, err := c.Cookie("auth_token"); err == nil && cookieToken != "" {
			tokenStr = cookieToken
		}

		if tokenStr != "" {
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			sub, _ := claims.GetSubject()
			if sub == "" {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			// Validate that the user still exists.
			oid, err := primitive.ObjectIDFromHex(sub)
			if err != nil {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			if _, err := repo.GetByID(c.Request.Context(), oid); err != nil {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.Set("userID", sub)
			c.Next()
			return
		}

		// Fall back to Basic Auth.
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			c.Header("WWW-Authenticate", `Basic realm="restricted"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if repo == nil {
			slog.ErrorContext(c.Request.Context(), "basic auth: user repository is nil")
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		user, err := repo.GetByUsername(c.Request.Context(), username)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			slog.ErrorContext(c.Request.Context(), "basic auth: failed to get user", slog.String("error", err.Error()))
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set("userID", user.ID.Hex())
		c.Next()
	}
}

// BasicAuth is kept for backward compatibility. Prefer Auth() for new routes.
func BasicAuth(repo *repository.UserRepository) gin.HandlerFunc {
	return Auth(repo, "")
}
