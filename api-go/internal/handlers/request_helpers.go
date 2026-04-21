package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func authenticatedUserID(c *gin.Context) (primitive.ObjectID, bool) {
	userIDHex := c.GetString("userID")
	if userIDHex == "" {
		c.Status(http.StatusUnauthorized)
		return primitive.NilObjectID, false
	}
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return primitive.NilObjectID, false
	}
	return userID, true
}

func routeObjectID(c *gin.Context, param string) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Param(param))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return primitive.NilObjectID, false
	}
	return id, true
}
