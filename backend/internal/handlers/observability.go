package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
)

const (
	loginSlowThreshold            = 1500 * time.Millisecond
	loginLookupSlowThreshold      = 750 * time.Millisecond
	loginPasswordVerifyThreshold  = 500 * time.Millisecond
	publicSummarySlowThreshold    = 1200 * time.Millisecond
	dashboardSummarySlowThreshold = 1500 * time.Millisecond
	analyticsSlowThreshold        = 4000 * time.Millisecond
	readinessPingTimeout          = time.Second
)

type endpointObservation struct {
	c         *gin.Context
	database  *db.DB
	module    string
	action    string
	kind      string
	threshold time.Duration
	startedAt time.Time
}

func beginEndpointObservation(c *gin.Context, database *db.DB, module, action, kind string, threshold time.Duration) *endpointObservation {
	return &endpointObservation{
		c:         c,
		database:  database,
		module:    module,
		action:    action,
		kind:      kind,
		threshold: threshold,
		startedAt: time.Now(),
	}
}

func (o *endpointObservation) Finish() {
	if o == nil {
		return
	}

	duration := time.Since(o.startedAt)
	status := o.c.Writer.Status()
	if status == 0 {
		status = http.StatusOK
	}
	path := requestRoutePath(o.c)
	durationMs := durationMilliseconds(duration)
	thresholdMs := durationMilliseconds(o.threshold)

	switch {
	case duration >= o.threshold:
		logging.Warnf(
			o.module,
			o.action,
			"event=slow_request kind=%s path=%q method=%s status=%d duration_ms=%d threshold_ms=%d",
			o.kind,
			path,
			o.c.Request.Method,
			status,
			durationMs,
			thresholdMs,
		)
		logPoolStatsIfAvailable(o.database, logging.Warnf, o.module, o.action, "reason=slow_request path=%q", path)
	case status >= http.StatusInternalServerError:
		logging.Warnf(
			o.module,
			o.action,
			"event=request_failed kind=%s path=%q method=%s status=%d duration_ms=%d threshold_ms=%d",
			o.kind,
			path,
			o.c.Request.Method,
			status,
			durationMs,
			thresholdMs,
		)
	}
}

type logFunc func(module, action, format string, args ...any)

func logPoolStatsIfAvailable(database *db.DB, logf logFunc, module, action, prefixFormat string, prefixArgs ...any) {
	poolStats := database.PoolStats()
	if !poolStats.Available {
		logf(module, action, prefixFormat+" pool_stats=unavailable", prefixArgs...)
		return
	}

	args := append(prefixArgs,
		poolStats.TotalConns,
		poolStats.AcquiredConns,
		poolStats.IdleConns,
		poolStats.MaxConns,
		poolStats.ConstructingConns,
		poolStats.AcquireCount,
		poolStats.EmptyAcquireCount,
		poolStats.CanceledAcquireCount,
		durationMilliseconds(poolStats.EmptyAcquireWaitTime),
		durationMilliseconds(poolStats.AcquireDuration),
	)

	logf(
		module,
		action,
		prefixFormat+" total_conns=%d acquired_conns=%d idle_conns=%d max_conns=%d constructing_conns=%d acquire_count=%d empty_acquire_count=%d canceled_acquire_count=%d empty_acquire_wait_ms=%d acquire_duration_ms=%d",
		args...,
	)
}

func requestRoutePath(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if fullPath := c.FullPath(); fullPath != "" {
		return fullPath
	}
	if c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	return ""
}
