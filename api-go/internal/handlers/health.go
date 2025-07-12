package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Healthz(c *gin.Context) {
	c.Status(http.StatusOK)
}

func Readyz(c *gin.Context) {
	c.Status(http.StatusOK)
}

func PublicationReady(c *gin.Context) {
	c.Status(http.StatusOK)
}
