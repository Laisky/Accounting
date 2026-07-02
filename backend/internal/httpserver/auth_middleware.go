package httpserver

import (
	"net/http"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
)

// AttachSession receives auth dependencies and returns middleware that hydrates authenticated identity context when available.
func AttachSession(cfg config.Config, authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := gmw.GetLogger(c)

		sessionCookie, err := c.Request.Cookie(cfg.Auth.Session.CookieName)
		if err != nil || sessionCookie.Value == "" {
			c.Next()
			return
		}

		session, err := authService.SessionFromToken(c.Request.Context(), sessionCookie.Value)
		if err != nil {
			log.Debug("session rejected", zap.Error(err))
			c.Next()
			return
		}

		c.Request = c.Request.WithContext(auth.ContextWithSession(c.Request.Context(), session))
		c.Next()
	}
}

// RequireSession returns middleware that rejects requests without an authenticated actor context.
func RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		log := gmw.GetLogger(c)
		if _, ok := auth.ActorFromContext(c.Request.Context()); !ok {
			log.Debug("authenticated actor missing")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		c.Next()
	}
}
