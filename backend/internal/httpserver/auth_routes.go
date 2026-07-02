package httpserver

import (
	"net/http"

	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
)

// registerAuthRoutes receives an API group and registers authentication endpoints.
func registerAuthRoutes(api *gin.RouterGroup, cfg config.Config, authService *auth.Service, auditService *audit.Service, limiter *authRateLimiter) {
	api.POST("/auth/register", func(c *gin.Context) {
		log := gmw.GetLogger(c)

		var request authRequest
		if !decodeStrictJSON(c, &request) {
			return
		}
		if !requireAuthRateLimit(c, limiter, "auth.register", request.Email) {
			return
		}

		user, err := authService.Register(c.Request.Context(), auth.RegisterRequest{
			Email:          request.Email,
			Password:       request.Password,
			TurnstileToken: request.TurnstileToken,
			RemoteIP:       c.ClientIP(),
		})
		if err != nil {
			log.Debug("registration rejected", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "registration failed"})
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    user.ID,
			ActorEmail: user.Email,
			Action:     audit.ActionAuthRegister,
			TargetType: "user",
			TargetID:   user.ID,
			Metadata:   map[string]string{"method": "password"},
		})
		c.JSON(http.StatusCreated, gin.H{"user": user})
	})

	api.POST("/auth/login", func(c *gin.Context) {
		log := gmw.GetLogger(c)

		var request authRequest
		if !decodeStrictJSON(c, &request) {
			return
		}
		if !requireAuthRateLimit(c, limiter, "auth.login", request.Email) {
			return
		}

		result, err := authService.Login(c.Request.Context(), auth.LoginRequest{
			Email:          request.Email,
			Password:       request.Password,
			TOTPCode:       request.TOTPCode,
			TurnstileToken: request.TurnstileToken,
			RemoteIP:       c.ClientIP(),
		})
		if err != nil {
			log.Debug("login rejected", zap.Error(err))
			recordAuditEvent(c, auditService, audit.RecordRequest{
				Action:     audit.ActionAuthLoginFailed,
				TargetType: "user",
				Metadata:   map[string]string{"method": "password"},
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		if result.TOTPRequired {
			// Password verified but a second factor is still required; prompt the
			// client for a TOTP code without issuing a session. Audit the partial
			// auth so a valid-password-but-blocked-by-2FA signal is not lost.
			log.Debug("login awaiting totp verification")
			recordAuditEvent(c, auditService, audit.RecordRequest{
				ActorID:    result.User.ID,
				ActorEmail: result.User.Email,
				Action:     audit.ActionAuthLoginTOTPChallenge,
				TargetType: "user",
				TargetID:   result.User.ID,
				Metadata:   map[string]string{"method": "password"},
			})
			c.JSON(http.StatusOK, gin.H{"totpRequired": true})
			return
		}

		setSessionCookie(c, cfg, authService, result.SessionToken, result.Session.ExpiresAt)
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    result.User.ID,
			ActorEmail: result.User.Email,
			Action:     audit.ActionAuthLogin,
			TargetType: "user",
			TargetID:   result.User.ID,
			Metadata:   map[string]string{"method": "password"},
		})
		c.JSON(http.StatusOK, gin.H{
			"user":    result.User,
			"session": result.Session,
		})
	})

	api.POST("/auth/logout", func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, hasActor := auth.ActorFromContext(c.Request.Context())
		sessionCookie, err := c.Request.Cookie(cfg.Auth.Session.CookieName)
		if err == nil {
			if logoutErr := authService.Logout(c.Request.Context(), sessionCookie.Value); logoutErr != nil {
				log.Debug("logout session revoke failed", zap.Error(logoutErr))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
				return
			}
		}

		if hasActor {
			recordAuditEvent(c, auditService, audit.RecordRequest{
				ActorID:    actor.UserID,
				ActorEmail: actor.Email,
				Action:     audit.ActionAuthLogout,
				TargetType: "user",
				TargetID:   actor.UserID,
			})
		}
		clearSessionCookie(c, cfg)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api.GET("/auth/session", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		session, ok := auth.SessionFromContext(c.Request.Context())
		if !ok {
			log.Debug("session context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		log.Debug("session requested")
		c.JSON(http.StatusOK, gin.H{
			"actor":   actor,
			"session": session,
		})
	})

	api.GET("/auth/email/verification", func(c *gin.Context) {
		log := gmw.GetLogger(c)

		email := c.Query("email")
		if !requireAuthRateLimit(c, limiter, "auth.email_verification_request", email) {
			return
		}
		delivery, err := authService.RequestEmailVerification(c.Request.Context(), auth.EmailCodeRequest{
			Email: email,
		})
		if err != nil {
			log.Debug("email verification request rejected", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "email verification request failed"})
			return
		}

		log.Debug("email verification requested")
		if delivery.User != nil {
			recordAuditEvent(c, auditService, audit.RecordRequest{
				ActorID:    delivery.User.ID,
				ActorEmail: delivery.User.Email,
				Action:     audit.ActionEmailVerificationRequested,
				TargetType: "user",
				TargetID:   delivery.User.ID,
			})
		}
		c.JSON(http.StatusAccepted, gin.H{"status": "sent", "expiresAt": delivery.ExpiresAt})
	})

	api.POST("/auth/email/verification", func(c *gin.Context) {
		log := gmw.GetLogger(c)

		var request emailCodeConfirmRequest
		if !decodeStrictJSON(c, &request) {
			return
		}
		if !requireAuthRateLimit(c, limiter, "auth.email_verification_confirm", request.Email) {
			return
		}

		user, err := authService.ConfirmEmailVerification(c.Request.Context(), auth.ConfirmEmailRequest{
			Email: request.Email,
			Code:  request.Code,
		})
		if err != nil {
			log.Debug("email verification rejected", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "email verification failed"})
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    user.ID,
			ActorEmail: user.Email,
			Action:     audit.ActionEmailVerified,
			TargetType: "user",
			TargetID:   user.ID,
		})
		c.JSON(http.StatusOK, gin.H{"user": user})
	})

	api.POST("/auth/password-reset/request", func(c *gin.Context) {
		log := gmw.GetLogger(c)

		var request emailCodeRequest
		if !decodeStrictJSON(c, &request) {
			return
		}
		if !requireAuthRateLimit(c, limiter, "auth.password_reset_request", request.Email) {
			return
		}

		delivery, err := authService.RequestPasswordReset(c.Request.Context(), auth.EmailCodeRequest{
			Email: request.Email,
		})
		if err != nil {
			log.Debug("password reset request rejected", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "password reset request failed"})
			return
		}

		log.Debug("password reset requested")
		if delivery.User != nil {
			recordAuditEvent(c, auditService, audit.RecordRequest{
				ActorID:    delivery.User.ID,
				ActorEmail: delivery.User.Email,
				Action:     audit.ActionPasswordResetRequested,
				TargetType: "user",
				TargetID:   delivery.User.ID,
			})
		}
		c.JSON(http.StatusAccepted, gin.H{"status": "sent", "expiresAt": delivery.ExpiresAt})
	})

	api.POST("/auth/password-reset/confirm", func(c *gin.Context) {
		log := gmw.GetLogger(c)

		var request passwordResetConfirmRequest
		if !decodeStrictJSON(c, &request) {
			return
		}
		if !requireAuthRateLimit(c, limiter, "auth.password_reset_confirm", request.Email) {
			return
		}

		user, err := authService.ConfirmPasswordReset(c.Request.Context(), auth.ConfirmPasswordResetRequest{
			Email:       request.Email,
			Code:        request.Code,
			NewPassword: request.NewPassword,
		})
		if err != nil {
			log.Debug("password reset rejected", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "password reset failed"})
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    user.ID,
			ActorEmail: user.Email,
			Action:     audit.ActionPasswordResetConfirmed,
			TargetType: "user",
			TargetID:   user.ID,
		})
		c.JSON(http.StatusOK, gin.H{"user": user})
	})

	api.GET("/auth/totp/status", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		status, err := authService.TOTPStatus(c.Request.Context(), actor)
		if err != nil {
			log.Debug("totp status failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "totp status failed"})
			return
		}

		c.JSON(http.StatusOK, status)
	})

	api.POST("/auth/totp/setup", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, session, ok := authIdentityFromContext(c)
		if !ok {
			log.Debug("auth context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		setup, err := authService.SetupTOTP(c.Request.Context(), auth.TOTPSetupRequest{
			Actor:   actor,
			Session: session,
		})
		if err != nil {
			log.Debug("totp setup failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "totp setup failed"})
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionTOTPSetupRequested,
			TargetType: "user",
			TargetID:   actor.UserID,
		})
		c.JSON(http.StatusCreated, setup)
	})

	api.POST("/auth/totp/confirm", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, session, ok := authIdentityFromContext(c)
		if !ok {
			log.Debug("auth context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request totpCodeRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		status, err := authService.ConfirmTOTP(c.Request.Context(), auth.TOTPConfirmRequest{
			Actor:   actor,
			Session: session,
			Code:    request.Code,
		})
		if err != nil {
			log.Debug("totp confirm failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "totp confirm failed"})
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionTOTPEnabled,
			TargetType: "user",
			TargetID:   actor.UserID,
		})
		c.JSON(http.StatusOK, status)
	})

	api.POST("/auth/totp/disable", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request totpCodeRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		status, err := authService.DisableTOTP(c.Request.Context(), auth.TOTPDisableRequest{
			Actor: actor,
			Code:  request.Code,
		})
		if err != nil {
			log.Debug("totp disable failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "totp disable failed"})
			return
		}

		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionTOTPDisabled,
			TargetType: "user",
			TargetID:   actor.UserID,
		})
		c.JSON(http.StatusOK, status)
	})

	registerExternalSSORoutes(api, cfg, authService, auditService, limiter)
	registerPasskeyRoutes(api, cfg, authService, auditService, limiter)
}

type authRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	TOTPCode       string `json:"totp_code"`
	TurnstileToken string `json:"turnstile_token"`
}

type emailCodeRequest struct {
	Email string `json:"email"`
}

type emailCodeConfirmRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type passwordResetConfirmRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"newPassword"`
}

type totpCodeRequest struct {
	Code string `json:"code"`
}

// authIdentityFromContext receives a Gin context and returns authenticated actor and session data.
func authIdentityFromContext(c *gin.Context) (auth.Actor, auth.Session, bool) {
	actor, ok := auth.ActorFromContext(c.Request.Context())
	if !ok {
		return auth.Actor{}, auth.Session{}, false
	}
	session, ok := auth.SessionFromContext(c.Request.Context())
	if !ok {
		return auth.Actor{}, auth.Session{}, false
	}

	return actor, session, true
}

// recordAuditEvent receives sanitized audit input and records it without interrupting the request.
func recordAuditEvent(c *gin.Context, auditService *audit.Service, request audit.RecordRequest) {
	if auditService == nil {
		return
	}
	if _, err := auditService.Record(c.Request.Context(), request); err != nil {
		log := gmw.GetLogger(c)
		log.Debug("audit event record failed", zap.Error(err))
	}
}
