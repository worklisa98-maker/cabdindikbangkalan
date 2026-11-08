package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
)

type HealthHandler struct {
	DB *db.DB
}

func (h HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (h HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), readinessPingTimeout)
	defer cancel()

	if h.DB == nil || h.DB.Pool == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"db":     "unavailable",
		})
		return
	}

	if err := h.DB.Pool.Ping(ctx); err != nil {
		logging.Warnf("health", "ready", "event=database_ping_failed err=%v", err)
		logPoolStatsIfAvailable(h.DB, logging.Warnf, "health", "ready", "event=database_ping_failed_pool_stats")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"db":     "error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"db":     "ok",
	})
}

func (h HealthHandler) Check(c *gin.Context) {
	h.Ready(c)
}
