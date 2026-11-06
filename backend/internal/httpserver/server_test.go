package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"crypto/tls"

	auditpkg "github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesHealth verifies that the health endpoint responds with an OK status.
func TestRegisterRoutesHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"ok"`)
}

// TestSecurityHeadersAddsHSTSForHTTPS verifies HTTPS responses include strict transport policy.
func TestSecurityHeadersAddsHSTSForHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(securityHeaders(config.Config{}))
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "max-age=63072000; includeSubDomains", rec.Header().Get("Strict-Transport-Security"))
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

// TestSecurityHeadersOmitsHSTSForHTTP verifies plain HTTP responses do not emit HSTS.
func TestSecurityHeadersOmitsHSTSForHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(securityHeaders(config.Config{}))
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Header().Get("Strict-Transport-Security"))
}

// TestSecurityHeadersTrustsForwardedHTTPSFromTrustedProxy verifies proxied HTTPS can emit HSTS.
func TestSecurityHeadersTrustsForwardedHTTPSFromTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(securityHeaders(config.Config{TrustedProxies: []string{"192.0.2.0/24"}}))
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "max-age=63072000; includeSubDomains", rec.Header().Get("Strict-Transport-Security"))
}

// TestSecurityHeadersIgnoresSpoofedForwardedHTTPS verifies direct clients cannot spoof HSTS transport.
func TestSecurityHeadersIgnoresSpoofedForwardedHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(securityHeaders(config.Config{TrustedProxies: []string{"192.0.2.0/24"}}))
	router.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Header().Get("Strict-Transport-Security"))
}

// TestNewServerSQLitePersistenceDriverPersists verifies the relational storage layer is wired
// through server startup: a sqlite-backed server migrates its schema, persists a registered
// account, and a second server instance opened on the same database can still authenticate it.
func TestNewServerSQLitePersistenceDriverPersists(t *testing.T) {
	cfg := testConfig()
	cfg.Persistence.Driver = "sqlite"
	cfg.Persistence.Dir = t.TempDir()
	log := logger.Setup(false)

	server, err := NewServer(cfg, log, nil)
	require.NoError(t, err)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	server.Handler.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	server.Handler.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)

	// The relational schema is materialized as a real sqlite database file.
	_, err = os.Stat(filepath.Join(cfg.Persistence.Dir, "accounting.sqlite3"))
	require.NoError(t, err)

	// A fresh server on the same database re-runs migrations idempotently and authenticates the
	// previously registered account, proving durable relational persistence across instances.
	server2, err := NewServer(cfg, log, nil)
	require.NoError(t, err)
	login2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	login2.Header.Set("Content-Type", "application/json")
	login2Rec := httptest.NewRecorder()
	server2.Handler.ServeHTTP(login2Rec, login2)
	require.Equal(t, http.StatusOK, login2Rec.Code)
}

// TestRegisterRoutesRuntimeConfigExposesOnlyPublicValues verifies frontend config omits backend secrets.
func TestRegisterRoutesRuntimeConfigExposesOnlyPublicValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := config.Config{
		ServerName: "test",
		Auth: config.AuthConfig{
			Email: config.EmailAuthConfig{
				AllowedRegistrationDomains: []string{"example.test"},
				LoginEnabled:               true,
				RegisterEnabled:            false,
				SMTPPassword:               "smtp-secret",
				VerificationRequired:       true,
			},
			External: config.ExternalSSOConfig{
				Enabled:      true,
				LoginURL:     "https://sso.example.test/login",
				MetadataURL:  "https://sso.example.test/runtime-config.json",
				PublicKeyPEM: "-----BEGIN PUBLIC KEY-----\\ntest\\n-----END PUBLIC KEY-----",
			},
			Passkey: config.PasskeyConfig{
				Enabled:       true,
				RPDisplayName: "Accounting Test",
				RPID:          "accounts.example.test",
				RPOrigin:      "https://accounts.example.test",
			},
			TOTP: config.TOTPConfig{
				Enabled: true,
			},
			Turnstile: config.TurnstileConfig{
				Enabled:   true,
				LoginMode: "after_failure",
				SecretKey: "turnstile-secret",
				SiteKey:   "turnstile-site",
			},
		},
		AlertPusher: config.AlertPusherConfig{
			Token: "alert-secret",
		},
	}
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime-config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "smtp-secret")
	require.NotContains(t, rec.Body.String(), "turnstile-secret")
	require.NotContains(t, rec.Body.String(), "alert-secret")
	require.NotContains(t, rec.Body.String(), "sso.example.test")
	require.NotContains(t, rec.Body.String(), "BEGIN PUBLIC KEY")

	var response RuntimeConfigResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "test", response.ServerName)
	require.Equal(t, "/api/v1", response.APIBase)
	require.True(t, response.Auth.EmailLoginEnabled)
	require.False(t, response.Auth.EmailRegisterEnabled)
	require.True(t, response.Auth.EmailVerificationRequired)
	require.Equal(t, []string{"example.test"}, response.Auth.AllowedRegistrationDomains)
	require.True(t, response.Features.TOTPEnabled)
	require.True(t, response.Features.PasskeyEnabled)
	require.True(t, response.Features.TurnstileEnabled)
	require.True(t, response.Features.ExternalSSOEnabled)
	require.True(t, response.SSO.Enabled)
	require.Equal(t, "/api/v1/auth/sso/start", response.SSO.StartPath)
	require.Equal(t, "Accounting Test", response.Passkey.RPDisplayName)
	require.Equal(t, "accounts.example.test", response.Passkey.RPID)
	require.Equal(t, "https://accounts.example.test", response.Passkey.RPOrigin)
	require.Equal(t, "after_failure", response.Turnstile.LoginMode)
	require.Equal(t, "turnstile-site", response.Turnstile.SiteKey)
}

// TestRegisterRoutesRejectsOversizedJSONBody verifies JSON routes cap memory exposure.
func TestRegisterRoutesRejectsOversizedJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	body := `{"email":"person@example.test","password":"correct horse battery staple","padding":"` +
		strings.Repeat("x", int(maxJSONBodyBytes)+1) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Contains(t, rec.Body.String(), "request body too large")
}

// TestRegisterRoutesLedgerSummary verifies the ledger summary endpoint returns domain-shaped data.
func TestRegisterRoutesLedgerSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response ledger.Summary
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "book-household", response.BookID)
	require.Equal(t, "Household", response.BookName)
	require.Equal(t, "USD", response.Currency)
	require.Equal(t, 1, response.EntryCount)
	require.NotEmpty(t, response.Categories)
	require.NotEmpty(t, response.Accounts)
}

// TestRegisterRoutesExchangeRates verifies authenticated sessions can read supported exchange rates.
func TestRegisterRoutesExchangeRates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthServiceWithUsers(t, cfg, "user-owner")
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/exchange-rates", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"currency":"USD"`)
	require.Contains(t, rec.Body.String(), `"currency":"CNY"`)
}

// TestRegisterRoutesAuthSessionFlow verifies register, login cookie issuance, and logout clearing.
func TestRegisterRoutesAuthSessionFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Auth.Session.CookieSecure = true
	cfg.Auth.Session.TTL = time.Hour
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	authService := testAuthService(cfg).WithClock(func() time.Time {
		return now
	})
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)
	require.NotContains(t, registerRec.Body.String(), "password")
	var registerResponse struct {
		User auth.User `json:"user"`
	}
	err := json.Unmarshal(registerRec.Body.Bytes(), &registerResponse)
	require.NoError(t, err)
	require.Equal(t, now, registerResponse.User.CreatedAt)
	require.Equal(t, now, registerResponse.User.UpdatedAt)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)
	require.NotContains(t, loginRec.Body.String(), "correct horse battery staple")

	cookies := loginRec.Result().Cookies()
	require.Len(t, cookies, 1)
	sessionCookie := cookies[0]
	require.Equal(t, cfg.Auth.Session.CookieName, sessionCookie.Name)
	require.True(t, sessionCookie.HttpOnly)
	require.True(t, sessionCookie.Secure)
	require.Equal(t, http.SameSiteLaxMode, sessionCookie.SameSite)
	require.NotEmpty(t, sessionCookie.Value)

	_, err = authService.SessionFromToken(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	require.Equal(t, http.StatusOK, logoutRec.Code)

	clearedCookies := logoutRec.Result().Cookies()
	require.Len(t, clearedCookies, 1)
	require.Equal(t, cfg.Auth.Session.CookieName, clearedCookies[0].Name)
	require.Equal(t, -1, clearedCookies[0].MaxAge)

	_, err = authService.SessionFromToken(t.Context(), sessionCookie.Value)
	require.Error(t, err)
}

// TestRegisterRoutesAuthLogoutAllRevokesUserSessions verifies logout-all clears every session for the actor.
func TestRegisterRoutesAuthLogoutAllRevokesUserSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Auth.Session.CookieSecure = true
	authService := testAuthService(cfg)
	auditService := auditpkg.NewService(auditpkg.NewMemoryStore())
	RegisterRoutes(router, cfg, ledger.NewService(), authService, auditService)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)
	var registerBody struct {
		User auth.User `json:"user"`
	}
	err := json.Unmarshal(registerRec.Body.Bytes(), &registerBody)
	require.NoError(t, err)

	firstCookie := loginForTest(t, router, cfg, "person@example.test", "correct horse battery staple")
	secondCookie := loginForTest(t, router, cfg, "person@example.test", "correct horse battery staple")
	require.NotEqual(t, firstCookie.Value, secondCookie.Value)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout-all", nil)
	logoutReq.AddCookie(firstCookie)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	require.Equal(t, http.StatusOK, logoutRec.Code)

	clearedCookies := logoutRec.Result().Cookies()
	require.Len(t, clearedCookies, 1)
	require.Equal(t, cfg.Auth.Session.CookieName, clearedCookies[0].Name)
	require.Equal(t, -1, clearedCookies[0].MaxAge)

	_, err = authService.SessionFromToken(t.Context(), firstCookie.Value)
	require.Error(t, err)
	_, err = authService.SessionFromToken(t.Context(), secondCookie.Value)
	require.Error(t, err)

	events, err := auditService.List(t.Context(), auditpkg.ListRequest{
		ActorID:  registerBody.User.ID,
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.NotEmpty(t, events.Items)
	require.Equal(t, auditpkg.ActionAuthLogoutAll, events.Items[0].Action)
}

// TestRegisterRoutesAuthRejectsUnknownJSONFields verifies mutating auth bodies fail closed.
func TestRegisterRoutesAuthRejectsUnknownJSONFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple","role":"admin"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request body")
}

// TestRegisterRoutesAuthEmailVerificationFlow verifies email verification endpoints activate pending users.
func TestRegisterRoutesAuthEmailVerificationFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Auth.Email.VerificationRequired = true
	sender := &routeEmailSender{}
	authService := testAuthService(cfg).WithEmailSender(sender)
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	sendReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/email/verification?email=person@example.test", nil)
	sendRec := httptest.NewRecorder()
	router.ServeHTTP(sendRec, sendReq)
	require.Equal(t, http.StatusAccepted, sendRec.Code)
	require.NotContains(t, sendRec.Body.String(), "code")
	require.NotContains(t, sendRec.Body.String(), "token")
	require.Len(t, sender.deliveries, 1)
	require.Equal(t, auth.EmailCodePurposeVerification, sender.deliveries[0].purpose)

	confirmReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/email/verification", bytes.NewBufferString(`{"email":"person@example.test","code":"`+sender.deliveries[0].delivery.Code+`"}`))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmRec := httptest.NewRecorder()
	router.ServeHTTP(confirmRec, confirmReq)
	require.Equal(t, http.StatusOK, confirmRec.Code)
	require.Contains(t, confirmRec.Body.String(), `"status":"active"`)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)
}

// TestRegisterRoutesAuthPasswordResetFlow verifies password reset endpoints update credentials without returning codes.
func TestRegisterRoutesAuthPasswordResetFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	sender := &routeEmailSender{}
	authService := testAuthService(cfg).WithEmailSender(sender)
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", bytes.NewBufferString(`{"email":"person@example.test"}`))
	resetReq.Header.Set("Content-Type", "application/json")
	resetRec := httptest.NewRecorder()
	router.ServeHTTP(resetRec, resetReq)
	require.Equal(t, http.StatusAccepted, resetRec.Code)
	require.NotContains(t, resetRec.Body.String(), "code")
	require.NotContains(t, resetRec.Body.String(), "token")
	require.NotContains(t, resetRec.Body.String(), "correct horse battery staple")
	require.Len(t, sender.deliveries, 1)
	require.Equal(t, auth.EmailCodePurposePasswordReset, sender.deliveries[0].purpose)

	confirmReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", bytes.NewBufferString(`{"email":"person@example.test","code":"`+sender.deliveries[0].delivery.Code+`","newPassword":"new correct horse battery staple"}`))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmRec := httptest.NewRecorder()
	router.ServeHTTP(confirmRec, confirmReq)
	require.Equal(t, http.StatusOK, confirmRec.Code)
	require.NotContains(t, confirmRec.Body.String(), "new correct horse battery staple")

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"new correct horse battery staple"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)
}

// TestRegisterRoutesAuthSessionRequiresCookie verifies the protected session route rejects missing cookies.
func TestRegisterRoutesAuthSessionRequiresCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}

// TestRegisterRoutesPublicEndpointsContinueWithoutCookie verifies API session hydration does not block public routes.
func TestRegisterRoutesPublicEndpointsContinueWithoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime-config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"serverName":"test"`)
}

// TestRegisterRoutesAuthSessionReturnsActor verifies the protected session route exposes stable actor data.
func TestRegisterRoutesAuthSessionReturnsActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthService(cfg)
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	sessionCookie := registerAndLogin(t, router, cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"email":"person@example.test"`)
	require.Contains(t, rec.Body.String(), `"status":"active"`)
	require.NotContains(t, rec.Body.String(), sessionCookie.Value)
}

// TestRegisterRoutesAuthSessionRejectsRevokedCookie verifies revoked sessions cannot authenticate requests.
func TestRegisterRoutesAuthSessionRejectsRevokedCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthService(cfg)
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	sessionCookie := registerAndLogin(t, router, cfg)
	require.NoError(t, authService.Logout(t.Context(), sessionCookie.Value))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}

// TestRegisterRoutesAuthSessionRejectsExpiredCookie verifies expired sessions cannot authenticate requests.
func TestRegisterRoutesAuthSessionRejectsExpiredCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	authService := testAuthService(cfg).WithClock(func() time.Time {
		return now
	})
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	sessionCookie := registerAndLogin(t, router, cfg)
	authService.WithClock(func() time.Time {
		return now.Add(2 * time.Hour)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authentication required")
}

// TestRegisterRoutesPublicEndpointsContinueWithExpiredCookie verifies stale cookies do not break public APIs.
func TestRegisterRoutesPublicEndpointsContinueWithExpiredCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	authService := testAuthService(cfg).WithClock(func() time.Time {
		return now
	})
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	sessionCookie := registerAndLogin(t, router, cfg)
	authService.WithClock(func() time.Time {
		return now.Add(2 * time.Hour)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime-config", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"serverName":"test"`)
}

// registerAndLogin receives a router and config, registers a user, and returns the issued session cookie.
func registerAndLogin(t *testing.T, router *gin.Engine, cfg config.Config) *http.Cookie {
	t.Helper()

	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)

	cookies := loginRec.Result().Cookies()
	require.NotEmpty(t, cookies)
	require.Equal(t, cfg.Auth.Session.CookieName, cookies[0].Name)

	return cookies[0]
}

// testConfig returns runtime config for HTTP route tests.
func testConfig() config.Config {
	return config.Config{
		ServerName: "test",
		Auth: config.AuthConfig{
			Email: config.EmailAuthConfig{
				LoginEnabled:         true,
				RegisterEnabled:      true,
				VerificationRequired: false,
				VerificationTTL:      10 * time.Minute,
			},
			Session: config.SessionConfig{
				CookieName:   "accounting_test_session",
				CookieSecure: false,
				TTL:          time.Hour,
			},
			Passkey: config.PasskeyConfig{
				Enabled:       true,
				RPDisplayName: "Accounting Test",
				RPID:          "example.test",
				RPOrigin:      "http://example.test",
			},
			TOTP: config.TOTPConfig{
				Enabled:             true,
				Issuer:              "Accounting Test",
				ReplayCacheDuration: 30 * time.Second,
			},
		},
	}
}

func loginForTest(t *testing.T, router *gin.Engine, cfg config.Config, email string, password string) *http.Cookie {
	t.Helper()

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"`+email+`","password":"`+password+`"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)
	cookies := loginRec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, cfg.Auth.Session.CookieName, cookies[0].Name)
	return cookies[0]
}

// testAuthService receives config and returns an in-memory auth service for route tests.
func testAuthService(cfg config.Config) *auth.Service {
	return auth.NewService(auth.Config{
		AllowedRegistrationDomains: cfg.Auth.Email.AllowedRegistrationDomains,
		EmailLoginEnabled:          cfg.Auth.Email.LoginEnabled,
		EmailRegisterEnabled:       cfg.Auth.Email.RegisterEnabled,
		EmailVerificationRequired:  cfg.Auth.Email.VerificationRequired,
		EmailVerificationTTL:       cfg.Auth.Email.VerificationTTL,
		ExternalSSOEnabled:         cfg.Auth.External.Enabled,
		ExternalSSOAutoProvision:   cfg.Auth.External.AutoProvisionEnabled,
		SessionTTL:                 cfg.Auth.Session.TTL,
		TOTPEnabled:                cfg.Auth.TOTP.Enabled,
		TOTPIssuer:                 cfg.Auth.TOTP.Issuer,
		TOTPReplayCacheDuration:    cfg.Auth.TOTP.ReplayCacheDuration,
		PasskeyEnabled:             cfg.Auth.Passkey.Enabled,
		PasskeyRPDisplayName:       cfg.Auth.Passkey.RPDisplayName,
		PasskeyRPID:                cfg.Auth.Passkey.RPID,
		PasskeyRPOrigin:            cfg.Auth.Passkey.RPOrigin,
		TurnstileEnabled:           cfg.Auth.Turnstile.Enabled,
		TurnstileLoginMode:         cfg.Auth.Turnstile.LoginMode,
	}, auth.NewMemoryStore(), auth.NoopTurnstileVerifier{})
}

type routeEmailDelivery struct {
	delivery auth.EmailCodeDelivery
	purpose  auth.EmailCodePurpose
}

type routeEmailSender struct {
	deliveries []routeEmailDelivery
}

// SendAuthCode receives delivery data and records it for route assertions.
func (s *routeEmailSender) SendAuthCode(_ context.Context, delivery auth.EmailCodeDelivery, purpose auth.EmailCodePurpose) error {
	s.deliveries = append(s.deliveries, routeEmailDelivery{
		delivery: delivery,
		purpose:  purpose,
	})

	return nil
}
