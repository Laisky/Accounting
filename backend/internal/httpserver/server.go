// Package httpserver owns HTTP routing, middleware, API handlers, and SPA serving.
package httpserver

import (
	"net/http"
	"time"

	"github.com/Laisky/errors/v2"
	gmw "github.com/Laisky/gin-middlewares/v7"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/ledger"
)

// NewServer builds an HTTP server with API routes, middleware, and SPA fallback.
func NewServer(cfg config.Config, log glog.Logger) (*http.Server, error) {
	if log == nil {
		return nil, errors.WithStack(errors.New("logger is nil"))
	}

	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(
		gin.Recovery(),
		gmw.NewLoggerMiddleware(
			gmw.WithLogger(log),
			gmw.WithLevel(log.Level().String()),
		),
		securityHeaders,
	)

	service := ledger.NewService()
	RegisterRoutes(router, cfg, service)

	if err := RegisterSPA(router, cfg.Frontend); err != nil {
		log.Info("spa disabled", zap.Error(err))
	}

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}, nil
}

// securityHeaders adds baseline browser security headers to each response.
func securityHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Header("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'")
	c.Next()
}
