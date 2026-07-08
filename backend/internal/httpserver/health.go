package httpserver

import (
	"context"
	"net/http"
	"time"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/storage"
)

// readyzPingTimeout bounds the readiness database probe so a stalled pool cannot hang the check.
const readyzPingTimeout = 2 * time.Second

// registerOpsRoutes binds the operational endpoints on the ROOT engine (no session, no rate
// limiter, no /api/v1 prefix): /healthz (liveness), /readyz (readiness), and, when a Prometheus
// handler is supplied, /metrics. These are ops endpoints and are intentionally absent from the
// OpenAPI JSON contract; /metrics is expected to be restricted at the ingress gateway.
func registerOpsRoutes(router *gin.Engine, db *storage.DB, metricsHandler http.Handler) {
	// /healthz is pure liveness: it never touches the database, so it stays 200 even when the
	// database is down, letting orchestrators distinguish "process alive" from "not ready".
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// /readyz is readiness: it reports the database as skipped for the in-memory driver and
	// otherwise pings the pool, returning 503 with a sanitized message (never the driver error
	// text) when the ping fails.
	router.GET("/readyz", func(c *gin.Context) {
		sqlDB := db.SQLDB() // nil-safe: returns nil for the in-memory driver.
		if sqlDB == nil {
			c.JSON(http.StatusOK, gin.H{
				"status": "ok",
				"checks": gin.H{"database": "skipped"},
			})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), readyzPingTimeout)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			gmw.GetLogger(c).Warn("readiness database ping failed", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"checks": gin.H{"database": "unavailable"},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"checks": gin.H{"database": "ok"},
		})
	})

	if metricsHandler != nil {
		router.GET("/metrics", gin.WrapH(metricsHandler))
	}
}
