package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
)

type HealthHandler struct {
	DB *db.DB
}

func (h HealthHandler) Check(c *gin.Context) {
	logging.Infof("health", "check", "event=hit")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if h.DB == nil || h.DB.Pool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "degraded",
			"db":     "unavailable",
		})
		return
	}

	if err := h.DB.Pool.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "degraded",
			"db":     "error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"db":     "ok",
	})
}
