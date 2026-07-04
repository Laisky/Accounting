package httpserver

import (
	"net/http"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
)

type userProfileUpdateRequest struct {
	BaseCurrency *string `json:"baseCurrency"`
}

// registerUserRoutes receives an API group and registers authenticated user profile endpoints.
func registerUserRoutes(api *gin.RouterGroup, authService *auth.Service, auditService *audit.Service) {
	api.GET("/users/me", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)
		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		user, err := authService.UserProfile(c.Request.Context(), actor)
		if err != nil {
			log.Debug("user profile failed", zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "user profile not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"user": user})
	})

	api.PATCH("/users/me", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)
		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request userProfileUpdateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		user, err := authService.UpdateUserProfile(c.Request.Context(), auth.UpdateUserProfileRequest{
			Actor:        actor,
			BaseCurrency: request.BaseCurrency,
		})
		if err != nil {
			log.Debug("user profile update rejected", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "user profile update failed"})
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionUserProfileUpdated,
			TargetType: "user",
			TargetID:   actor.UserID,
			Metadata:   map[string]string{"baseCurrency": user.BaseCurrency},
		})
		c.JSON(http.StatusOK, gin.H{"user": user})
	})
}
