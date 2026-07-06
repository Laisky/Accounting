package httpserver

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Laisky/errors/v2"
	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/config"
)

// RegisterSPA registers either a development proxy or a production static SPA fallback.
func RegisterSPA(router *gin.Engine, cfg config.FrontendConfig) error {
	if strings.TrimSpace(cfg.DevURL) != "" {
		return registerDevProxy(router, cfg.DevURL)
	}

	return registerStaticSPA(router, cfg.DistDir)
}

// registerDevProxy sends non-API requests to the Vite development server.
func registerDevProxy(router *gin.Engine, rawURL string) error {
	target, err := url.Parse(rawURL)
	if err != nil {
		return errors.Wrap(err, "parse frontend dev url")
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	router.NoRoute(func(c *gin.Context) {
		log := gmw.GetLogger(c)
		log.Debug("proxy spa request", zap.String("path", c.Request.URL.Path))
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	return nil
}

// registerStaticSPA serves Vite build output and falls back to index.html for browser routes.
func registerStaticSPA(router *gin.Engine, distDir string) error {
	if strings.TrimSpace(distDir) == "" {
		return errors.WithStack(errors.New("frontend dist dir is empty"))
	}

	indexPath := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return errors.Wrapf(err, "stat spa index %q", indexPath)
	}

	assetsDir := filepath.Join(distDir, "assets")
	if _, err := os.Stat(assetsDir); err == nil {
		router.StaticFS("/assets", http.Dir(assetsDir))
	}

	router.GET("/", func(c *gin.Context) {
		serveIndex(c, indexPath)
	})

	router.NoRoute(func(c *gin.Context) {
		log := gmw.GetLogger(c)
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			respondAPIMessage(c, http.StatusNotFound, "api route not found")
			return
		}

		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}

		cleanPath := filepath.Clean(strings.TrimPrefix(c.Request.URL.Path, "/"))
		if cleanPath != "." {
			filePath := filepath.Join(distDir, cleanPath)
			info, err := os.Stat(filePath)
			if err == nil && !info.IsDir() {
				c.File(filePath)
				return
			}
		}

		if !strings.Contains(c.GetHeader("Accept"), "text/html") {
			log.Debug("static asset not found", zap.String("path", c.Request.URL.Path))
			c.Status(http.StatusNotFound)
			return
		}

		serveIndex(c, indexPath)
	})

	return nil
}

// serveIndex writes the SPA index document for a browser navigation request.
func serveIndex(c *gin.Context, indexPath string) {
	c.Header("Cache-Control", "no-store")
	c.File(indexPath)
}
