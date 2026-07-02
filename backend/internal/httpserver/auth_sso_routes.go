package httpserver

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	gmw "github.com/Laisky/gin-middlewares/v7"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
)

const ssoCallbackPath = "/api/auth/sso/callback"
const ssoStartPath = "/api/auth/sso/start"

// registerExternalSSORoutes receives auth dependencies and registers external SSO entry and callback endpoints.
func registerExternalSSORoutes(api *gin.RouterGroup, cfg config.Config, authService *auth.Service, auditService *audit.Service, limiter *authRateLimiter) {
	api.GET("/auth/sso/start", func(c *gin.Context) {
		log := gmw.GetLogger(c)
		if !cfg.Auth.External.Enabled {
			c.JSON(http.StatusNotFound, gin.H{"error": "external sso login is disabled"})
			return
		}
		if !requireAuthRateLimit(c, limiter, "auth.sso.start", c.ClientIP()) {
			return
		}

		state, err := auth.NewSessionToken()
		if err != nil {
			log.Debug("external sso state creation failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "external sso start failed"})
			return
		}
		callbackURL, err := buildSSOCallbackURL(c.Request, cfg, state)
		if err != nil {
			log.Debug("external sso callback url rejected", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "external sso start failed"})
			return
		}
		redirectURL, err := buildSSOLoginRedirectURL(cfg.Auth.External.LoginURL, callbackURL)
		if err != nil {
			log.Debug("external sso login url rejected", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "external sso start failed"})
			return
		}

		setSSOStateCookie(c, cfg, state)
		c.Redirect(http.StatusFound, redirectURL)
	})

	api.GET("/auth/sso/callback", func(c *gin.Context) {
		log := gmw.GetLogger(c)
		if !cfg.Auth.External.Enabled {
			c.JSON(http.StatusNotFound, gin.H{"error": "external sso login is disabled"})
			return
		}
		clearSSOStateCookie(c, cfg)
		if !verifySSOState(c, cfg) {
			log.Debug("external sso state rejected")
			c.JSON(http.StatusBadRequest, gin.H{"error": "external sso callback failed"})
			return
		}

		token := strings.TrimSpace(c.Query("sso_token"))
		if token == "" {
			log.Debug("external sso token missing")
			c.JSON(http.StatusBadRequest, gin.H{"error": "external sso callback failed"})
			return
		}

		result, err := authService.LoginWithExternalSSO(c.Request.Context(), auth.ExternalSSOLoginRequest{Token: token})
		if err != nil {
			log.Debug("external sso login rejected", zap.Error(err))
			recordAuditEvent(c, auditService, audit.RecordRequest{
				Action:     audit.ActionAuthLoginFailed,
				TargetType: "user",
				Metadata:   map[string]string{"method": "external_sso"},
			})
			c.JSON(http.StatusUnauthorized, gin.H{"error": "external sso callback failed"})
			return
		}

		setSessionCookie(c, cfg, authService, result.SessionToken, result.Session.ExpiresAt)
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    result.User.ID,
			ActorEmail: result.User.Email,
			Action:     audit.ActionAuthLogin,
			TargetType: "user",
			TargetID:   result.User.ID,
			Metadata:   map[string]string{"method": "external_sso"},
		})
		c.Redirect(http.StatusFound, externalSSOSuccessRedirect(cfg))
	})
}

// buildSSOCallbackURL receives request data, config, and state and returns the backend SSO callback URL.
func buildSSOCallbackURL(request *http.Request, cfg config.Config, state string) (string, error) {
	rawCallbackURL := strings.TrimSpace(cfg.Auth.External.CallbackURL)
	if rawCallbackURL == "" {
		rawCallbackURL = requestOrigin(request) + ssoCallbackPath
	}

	callbackURL, err := url.Parse(rawCallbackURL)
	if err != nil {
		return "", errors.Wrap(err, "parse external sso callback url")
	}
	if callbackURL.Scheme != "http" && callbackURL.Scheme != "https" {
		return "", errors.WithStack(errors.New("external sso callback url protocol is unsupported"))
	}
	if callbackURL.Host == "" {
		return "", errors.WithStack(errors.New("external sso callback url host is required"))
	}

	query := callbackURL.Query()
	query.Set("state", state)
	callbackURL.RawQuery = query.Encode()
	return callbackURL.String(), nil
}

// buildSSOLoginRedirectURL receives the SSO login URL and callback URL and returns the provider redirect URL.
func buildSSOLoginRedirectURL(rawLoginURL string, callbackURL string) (string, error) {
	loginURL, err := url.Parse(strings.TrimSpace(rawLoginURL))
	if err != nil {
		return "", errors.Wrap(err, "parse external sso login url")
	}
	if loginURL.Scheme != "http" && loginURL.Scheme != "https" {
		return "", errors.WithStack(errors.New("external sso login url protocol is unsupported"))
	}
	if loginURL.Host == "" {
		return "", errors.WithStack(errors.New("external sso login url host is required"))
	}

	query := loginURL.Query()
	query.Set("redirect_to", callbackURL)
	loginURL.RawQuery = query.Encode()
	return loginURL.String(), nil
}

// externalSSOSuccessRedirect receives config and returns the post-login clean redirect target.
func externalSSOSuccessRedirect(cfg config.Config) string {
	target := strings.TrimSpace(cfg.Auth.External.SuccessRedirectURL)
	if target == "" {
		return "/"
	}

	return target
}

// externalSSOStartPath receives config and returns the public local SSO start path when enabled.
func externalSSOStartPath(cfg config.Config) string {
	if !cfg.Auth.External.Enabled {
		return ""
	}

	return ssoStartPath
}

// requestOrigin receives an HTTP request and returns the externally visible request origin.
func requestOrigin(request *http.Request) string {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")); forwardedProto == "http" || forwardedProto == "https" {
		scheme = forwardedProto
	}

	return scheme + "://" + request.Host
}

// setSSOStateCookie receives state data and writes a short-lived hashed anti-CSRF cookie.
func setSSOStateCookie(c *gin.Context, cfg config.Config, state string) {
	ttl := cfg.Auth.External.StateTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     externalSSOStateCookieName(cfg),
		Value:    auth.HashSessionToken(state),
		Path:     "/api/auth/sso",
		MaxAge:   int(ttl.Seconds()),
		Secure:   cfg.Auth.Session.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().UTC().Add(ttl),
	})
}

// clearSSOStateCookie receives a Gin context and expires the SSO state cookie.
func clearSSOStateCookie(c *gin.Context, cfg config.Config) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     externalSSOStateCookieName(cfg),
		Value:    "",
		Path:     "/api/auth/sso",
		MaxAge:   -1,
		Secure:   cfg.Auth.Session.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0).UTC(),
	})
}

// verifySSOState receives callback request data and reports whether the state matches the state cookie.
func verifySSOState(c *gin.Context, cfg config.Config) bool {
	state := strings.TrimSpace(c.Query("state"))
	if state == "" {
		return false
	}
	stateCookie, err := c.Request.Cookie(externalSSOStateCookieName(cfg))
	if err != nil || stateCookie.Value == "" {
		return false
	}

	expectedHash := auth.HashSessionToken(state)
	return subtle.ConstantTimeCompare([]byte(expectedHash), []byte(stateCookie.Value)) == 1
}

// externalSSOStateCookieName receives config and returns the configured state cookie name.
func externalSSOStateCookieName(cfg config.Config) string {
	name := strings.TrimSpace(cfg.Auth.External.StateCookieName)
	if name == "" {
		return "accounting_sso_state"
	}

	return name
}
