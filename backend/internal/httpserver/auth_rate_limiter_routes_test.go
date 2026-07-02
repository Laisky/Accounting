package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesAuthRateLimitLogin verifies login limits are scoped by route, IP, and normalized email.
func TestRegisterRoutesAuthRateLimitLogin(t *testing.T) {
	router, _ := testAuthRateLimitRouter(t)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"wrong password"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"PERSON@example.test","password":"wrong password"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.JSONEq(t, `{"error":"rate limit exceeded"}`, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "person@example.test")

	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"other@example.test","password":"wrong password"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestRegisterRoutesAuthRateLimitRegister verifies registration attempts are limited generically.
func TestRegisterRoutesAuthRateLimitRegister(t *testing.T) {
	router, _ := testAuthRateLimitRouter(t)

	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"limited@example.test","password":"correct horse battery staple"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.NotEqual(t, http.StatusTooManyRequests, rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"limited@example.test","password":"correct horse battery staple"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.NotContains(t, rec.Body.String(), "limited@example.test")
}

// TestRegisterRoutesAuthRateLimitPasswordReset verifies password reset requests do not reveal account existence.
func TestRegisterRoutesAuthRateLimitPasswordReset(t *testing.T) {
	router, _ := testAuthRateLimitRouter(t)

	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset/request", bytes.NewBufferString(`{"email":"missing@example.test"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusAccepted, rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset/request", bytes.NewBufferString(`{"email":"missing@example.test"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.NotContains(t, rec.Body.String(), "missing@example.test")
}

// TestRegisterRoutesAuthRateLimitPasskeyLogin verifies public passkey login endpoints are limited.
func TestRegisterRoutesAuthRateLimitPasskeyLogin(t *testing.T) {
	router, _ := testAuthRateLimitRouter(t)

	for range 2 {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/passkeys/login/begin", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkeys/login/begin", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	for range 2 {
		req = httptest.NewRequest(http.MethodPost, "/api/auth/passkeys/login/finish", bytes.NewBufferString(`{"flowId":"flow-1","credential":{}}`))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/passkeys/login/finish", bytes.NewBufferString(`{"flowId":"flow-1","credential":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

// testAuthRateLimitRouter returns a route test router with public auth limits set low.
func testAuthRateLimitRouter(t *testing.T) (*gin.Engine, config.Config) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Auth.RateLimit = config.AuthRateLimitConfig{
		Enabled: true,
		Limit:   2,
		Window:  time.Minute,
	}
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	return router, cfg
}
