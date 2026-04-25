package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/gin-gonic/gin"
)

const (
	authCookieName      = "auth_token"
	authSessionDuration = 7 * 24 * time.Hour
)

func authSessionMaxAge() int {
	return int(authSessionDuration / time.Second)
}

func useSecureCookies(cfg *config.Config, req *http.Request) bool {
	if req != nil && req.TLS != nil {
		return true
	}
	if cfg == nil || cfg.FrontendURL == "" {
		return false
	}
	frontendURL, err := url.Parse(cfg.FrontendURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(frontendURL.Scheme, "https")
}

func setAuthCookie(c *gin.Context, cfg *config.Config, token string) {
	secure := useSecureCookies(cfg, c.Request)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, token, authSessionMaxAge(), "/", "", secure, true)
}

func clearAuthCookie(c *gin.Context, cfg *config.Config) {
	secure := useSecureCookies(cfg, c.Request)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, "", -1, "/", "", secure, true)
}

// SessionLogin godoc
// @Summary Create auth session
// @Tags auth
// @Success 204 {string} string ""
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/auth/session [post]
func SessionLogin(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		jwtSecret := cfg.JWTSecret
		if jwtSecret == "" {
			jwtSecret = cfg.AccessSecretKey
		}
		token, err := IssueJWT(userIDHex, jwtSecret)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		setAuthCookie(c, cfg, token)
		c.Status(http.StatusNoContent)
	}
}

// SessionLogout godoc
// @Summary Clear auth session
// @Tags auth
// @Success 204 {string} string ""
// @Router /api/v1/auth/session [delete]
func SessionLogout(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		clearAuthCookie(c, cfg)
		c.Status(http.StatusNoContent)
	}
}
