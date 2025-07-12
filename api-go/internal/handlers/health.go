package handlers

// Package handlers implements HTTP handlers.

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Healthz godoc
// @Summary Show health
// @Tags system
// @Success 200 {string} string ""
// @Router /healthz [get]
func Healthz(c *gin.Context) {
	c.Status(http.StatusOK)
}

// Readyz godoc
// @Summary Show readiness
// @Tags system
// @Success 200 {string} string ""
// @Router /readyz [get]
func Readyz(c *gin.Context) {
	c.Status(http.StatusOK)
}

// PublicationReady godoc
// @Summary Show publication readiness
// @Tags system
// @Success 200 {string} string ""
// @Router /publication/ready [get]
func PublicationReady(c *gin.Context) {
	c.Status(http.StatusOK)
}
