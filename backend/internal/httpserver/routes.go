package httpserver

import (
	"net/http"
	"time"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/ledger"
)

// RegisterRoutes binds API endpoints to the provided Gin router.
func RegisterRoutes(router *gin.Engine, cfg config.Config, service *ledger.Service) {
	api := router.Group("/api")

	api.GET("/health", func(c *gin.Context) {
		log := gmw.GetLogger(c)
		log.Debug("health check")
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	api.GET("/runtime-config", func(c *gin.Context) {
		log := gmw.GetLogger(c)
		log.Debug("runtime config requested")
		c.JSON(http.StatusOK, gin.H{
			"serverName": cfg.ServerName,
			"apiBase":    "/api",
		})
	})

	api.GET("/ledger/summary", func(c *gin.Context) {
		log := gmw.GetLogger(c)
		summary := service.Summary(c.Request.Context())
		log.Debug("ledger summary requested")
		c.JSON(http.StatusOK, summary)
	})
}
