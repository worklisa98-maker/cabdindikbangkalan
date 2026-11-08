package handlers

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/utils"
)

const (
	authRequestTimeout      = 8 * time.Second
	summaryRequestTimeout   = 10 * time.Second
	analyticsRequestTimeout = 15 * time.Second
)

func requestContextWithTimeout(c *gin.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), timeout)
}

func handleOperationalError(c *gin.Context, database *db.DB, module string, action string, err error) bool {
	if err == nil {
		return false
	}

	path := requestRoutePath(c)

	switch {
	case isTimeoutError(err):
		logging.Warnf(module, action, "event=timeout path=%q err=%v", path, err)
		logPoolStatsIfAvailable(database, logging.Warnf, module, action, "event=timeout_pool_stats path=%q", path)
		utils.RespondGatewayTimeout(c)
		return true
	case isDependencyUnavailableError(err):
		logging.Warnf(module, action, "event=dependency_unavailable path=%q err=%v", path, err)
		utils.RespondServiceUnavailable(c)
		return true
	default:
		return false
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "timeout") ||
		strings.Contains(message, "deadline exceeded")
}

func isDependencyUnavailableError(err error) bool {
	if err == nil || isTimeoutError(err) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	message := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"failed to connect",
		"connection is closed",
		"pool closed",
		"server closed the connection unexpectedly",
		"too many clients",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}

	return false
}

func durationMilliseconds(value time.Duration) int64 {
	return value.Milliseconds()
}
