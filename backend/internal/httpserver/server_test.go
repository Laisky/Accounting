package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"ok"`)
}

// TestNewServerFilePersistenceDriverWritesSnapshots verifies file-backed stores are wired through server startup.
func TestNewServerFilePersistenceDriverWritesSnapshots(t *testing.T) {
	cfg := testConfig()
	cfg.Persistence.Driver = "file"
	cfg.Persistence.Dir = t.TempDir()
	log := logger.Setup(false)

	server, err := NewServer(cfg, log)
	require.NoError(t, err)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	server.Handler.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	server.Handler.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)

	_, err = os.Stat(filepath.Join(cfg.Persistence.Dir, "auth.json"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(cfg.Persistence.Dir, "audit.json"))
	require.NoError(t, err)
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
				Enabled:         true,
				GraphQLEndpoint: "https://sso.example.test/query",
				LoginURL:        "https://sso.example.test/login",
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

	req := httptest.NewRequest(http.MethodGet, "/api/runtime-config", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "smtp-secret")
	require.NotContains(t, rec.Body.String(), "turnstile-secret")
	require.NotContains(t, rec.Body.String(), "alert-secret")
	require.NotContains(t, rec.Body.String(), "sso.example.test")

	var response RuntimeConfigResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "test", response.ServerName)
	require.Equal(t, "/api", response.APIBase)
	require.True(t, response.Auth.EmailLoginEnabled)
	require.False(t, response.Auth.EmailRegisterEnabled)
	require.True(t, response.Auth.EmailVerificationRequired)
	require.Equal(t, []string{"example.test"}, response.Auth.AllowedRegistrationDomains)
	require.True(t, response.Features.TOTPEnabled)
	require.True(t, response.Features.PasskeyEnabled)
	require.True(t, response.Features.TurnstileEnabled)
	require.True(t, response.Features.ExternalSSOEnabled)
	require.True(t, response.SSO.Enabled)
	require.Equal(t, "/api/auth/sso/start", response.SSO.StartPath)
	require.Equal(t, "Accounting Test", response.Passkey.RPDisplayName)
	require.Equal(t, "accounts.example.test", response.Passkey.RPID)
	require.Equal(t, "https://accounts.example.test", response.Passkey.RPOrigin)
	require.Equal(t, "after_failure", response.Turnstile.LoginMode)
	require.Equal(t, "turnstile-site", response.Turnstile.SiteKey)
}

// TestRegisterRoutesLedgerSummary verifies the ledger summary endpoint returns domain-shaped data.
func TestRegisterRoutesLedgerSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodGet, "/api/ledger/summary", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/exchange-rates", nil)
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
	authService := testAuthService(cfg)
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)
	require.NotContains(t, registerRec.Body.String(), "password")

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
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

	_, err := authService.SessionFromToken(t.Context(), sessionCookie.Value)
	require.NoError(t, err)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
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

// TestRegisterRoutesAuthRejectsUnknownJSONFields verifies mutating auth bodies fail closed.
func TestRegisterRoutesAuthRejectsUnknownJSONFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple","role":"admin"}`))
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

	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	sendReq := httptest.NewRequest(http.MethodGet, "/api/auth/email/verification?email=person@example.test", nil)
	sendRec := httptest.NewRecorder()
	router.ServeHTTP(sendRec, sendReq)
	require.Equal(t, http.StatusAccepted, sendRec.Code)
	require.NotContains(t, sendRec.Body.String(), "code")
	require.NotContains(t, sendRec.Body.String(), "token")
	require.Len(t, sender.deliveries, 1)
	require.Equal(t, auth.EmailCodePurposeVerification, sender.deliveries[0].purpose)

	confirmReq := httptest.NewRequest(http.MethodPost, "/api/auth/email/verification", bytes.NewBufferString(`{"email":"person@example.test","code":"`+sender.deliveries[0].delivery.Code+`"}`))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmRec := httptest.NewRecorder()
	router.ServeHTTP(confirmRec, confirmReq)
	require.Equal(t, http.StatusOK, confirmRec.Code)
	require.Contains(t, confirmRec.Body.String(), `"status":"active"`)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
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

	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	resetReq := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset/request", bytes.NewBufferString(`{"email":"person@example.test"}`))
	resetReq.Header.Set("Content-Type", "application/json")
	resetRec := httptest.NewRecorder()
	router.ServeHTTP(resetRec, resetReq)
	require.Equal(t, http.StatusAccepted, resetRec.Code)
	require.NotContains(t, resetRec.Body.String(), "code")
	require.NotContains(t, resetRec.Body.String(), "token")
	require.NotContains(t, resetRec.Body.String(), "correct horse battery staple")
	require.Len(t, sender.deliveries, 1)
	require.Equal(t, auth.EmailCodePurposePasswordReset, sender.deliveries[0].purpose)

	confirmReq := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset/confirm", bytes.NewBufferString(`{"email":"person@example.test","code":"`+sender.deliveries[0].delivery.Code+`","newPassword":"new correct horse battery staple"}`))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmRec := httptest.NewRecorder()
	router.ServeHTTP(confirmRec, confirmReq)
	require.Equal(t, http.StatusOK, confirmRec.Code)
	require.NotContains(t, confirmRec.Body.String(), "new correct horse battery staple")

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"new correct horse battery staple"}`))
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

	req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/api/runtime-config", nil)
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
	req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/api/runtime-config", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"serverName":"test"`)
}

// registerAndLogin receives a router and config, registers a user, and returns the issued session cookie.
func registerAndLogin(t *testing.T, router *gin.Engine, cfg config.Config) *http.Cookie {
	t.Helper()

	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
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
