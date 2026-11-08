package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Details interface{} `json:"details,omitempty"`
	Message string      `json:"message,omitempty"`
}

func setNoStoreHeader(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
}

func RespondData(c *gin.Context, status int, data interface{}) {
	setNoStoreHeader(c)
	c.JSON(status, APIResponse{Data: data})
}

func RespondError(c *gin.Context, status int, message string) {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	setNoStoreHeader(c)
	c.JSON(status, APIResponse{Error: message})
}

func RespondUnauthorized(c *gin.Context) {
	setNoStoreHeader(c)
	c.JSON(http.StatusUnauthorized, APIResponse{Error: "UNAUTHORIZED"})
}

func RespondForbidden(c *gin.Context) {
	setNoStoreHeader(c)
	c.JSON(http.StatusForbidden, APIResponse{Error: "FORBIDDEN"})
}

func RespondBadRequest(c *gin.Context, details interface{}) {
	setNoStoreHeader(c)
	c.JSON(http.StatusBadRequest, APIResponse{Error: "BAD_REQUEST", Details: details})
}

func RespondBadRequestMessage(c *gin.Context, message string) {
	setNoStoreHeader(c)
	c.JSON(http.StatusBadRequest, APIResponse{Error: "BAD_REQUEST", Message: message})
}

func RespondInternalServerError(c *gin.Context) {
	setNoStoreHeader(c)
	c.JSON(http.StatusInternalServerError, APIResponse{Error: "INTERNAL_SERVER_ERROR"})
}

func RespondNotFound(c *gin.Context) {
	setNoStoreHeader(c)
	c.JSON(http.StatusNotFound, APIResponse{Error: "NOT_FOUND"})
}

func RespondTooManyRequests(c *gin.Context) {
	setNoStoreHeader(c)
	c.JSON(http.StatusTooManyRequests, APIResponse{Error: "TOO_MANY_REQUESTS"})
}
