package httpserver

import (
	"encoding/json"
	"net/http"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
)

type passkeyRegisterFinishRequest struct {
	FlowID     string          `json:"flowId"`
	Label      string          `json:"label"`
	Credential json.RawMessage `json:"credential"`
}

type passkeyLoginFinishRequest struct {
	FlowID     string          `json:"flowId"`
	Credential json.RawMessage `json:"credential"`
}

type passkeyUpdateRequest struct {
	Label string `json:"label"`
}

// registerPasskeyRoutes receives an API group and registers passkey authentication endpoints.
func registerPasskeyRoutes(api *gin.RouterGroup, cfg config.Config, authService *auth.Service, auditService *audit.Service, limiter *authRateLimiter) {
	api.POST("/auth/passkeys/login/begin", func(c *gin.Context) {
		log := gmw.GetLogger(c)

		if !requireAuthRateLimit(c, limiter, "auth.passkey_login_begin", "") {
			return
		}
		start, err := authService.BeginPasskeyLogin(c.Request.Context())
		if err != nil {
			log.Debug("passkey login start failed", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "passkey login start failed")
			return
		}

		c.JSON(http.StatusCreated, start)
	})

	api.POST("/auth/passkeys/login/finish", func(c *gin.Context) {
		log := gmw.GetLogger(c)

		var request passkeyLoginFinishRequest
		if !decodeStrictJSON(c, &request) {
			return
		}
		if !requireAuthRateLimit(c, limiter, "auth.passkey_login_finish", request.FlowID) {
			return
		}

		result, err := authService.FinishPasskeyLogin(c.Request.Context(), auth.PasskeyLoginFinishRequest{
			FlowID: request.FlowID,
		}, request.Credential)
		if err != nil {
			log.Debug("passkey login rejected", zap.Error(err))
			recordAuditEvent(c, auditService, audit.RecordRequest{
				Action:     audit.ActionAuthLoginFailed,
				TargetType: "user",
				Metadata:   map[string]string{"method": "passkey"},
			})
			respondAPIMessage(c, http.StatusUnauthorized, "passkey login failed")
			return
		}

		setSessionCookie(c, cfg, authService, result.SessionToken, result.Session.ExpiresAt)
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    result.User.ID,
			ActorEmail: result.User.Email,
			Action:     audit.ActionPasskeyLogin,
			TargetType: "user",
			TargetID:   result.User.ID,
			Metadata:   map[string]string{"method": "passkey"},
		})
		c.JSON(http.StatusOK, gin.H{
			"user":    result.User,
			"session": result.Session,
		})
	})

	api.GET("/auth/passkeys", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}
		pagination, ok := parseEntryPagination(c)
		if !ok {
			return
		}

		passkeys, err := authService.ListPasskeys(c.Request.Context(), auth.PasskeyListRequest{
			Actor:    actor,
			Page:     pagination.Page,
			PageSize: pagination.PageSize,
		})
		if err != nil {
			log.Debug("passkey list failed", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "passkey list failed")
			return
		}

		c.JSON(http.StatusOK, passkeys)
	})

	api.POST("/auth/passkeys/register/begin", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}

		start, err := authService.BeginPasskeyRegistration(c.Request.Context(), actor)
		if err != nil {
			log.Debug("passkey registration start failed", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "passkey registration start failed")
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionPasskeyRegistrationStarted,
			TargetType: "user",
			TargetID:   actor.UserID,
		})
		c.JSON(http.StatusCreated, start)
	})

	api.POST("/auth/passkeys/register/finish", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}

		var request passkeyRegisterFinishRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		passkey, err := authService.FinishPasskeyRegistration(c.Request.Context(), auth.PasskeyRegistrationFinishRequest{
			Actor:  actor,
			FlowID: request.FlowID,
			Label:  request.Label,
		}, request.Credential)
		if err != nil {
			log.Debug("passkey registration finish failed", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "passkey registration failed")
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionPasskeyRegistered,
			TargetType: "passkey",
			TargetID:   passkey.ID,
		})
		c.JSON(http.StatusCreated, passkey)
	})

	api.PUT("/auth/passkeys/:passkeyID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}

		var request passkeyUpdateRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		passkey, err := authService.UpdatePasskey(c.Request.Context(), auth.PasskeyUpdateRequest{
			Actor:     actor,
			PasskeyID: c.Param("passkeyID"),
			Label:     request.Label,
		})
		if err != nil {
			log.Debug("passkey update failed", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "passkey update failed")
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionPasskeyRenamed,
			TargetType: "passkey",
			TargetID:   passkey.ID,
		})
		c.JSON(http.StatusOK, passkey)
	})

	api.DELETE("/auth/passkeys/:passkeyID", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}

		passkeyID := c.Param("passkeyID")
		if err := authService.DeletePasskey(c.Request.Context(), actor, passkeyID); err != nil {
			log.Debug("passkey delete failed", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "passkey delete failed")
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionPasskeyDeleted,
			TargetType: "passkey",
			TargetID:   passkeyID,
		})
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
