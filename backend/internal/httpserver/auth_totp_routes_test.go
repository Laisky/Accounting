package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesAuthTOTPFlow verifies TOTP status, setup, confirmation, login challenge, and disable endpoints.
func TestRegisterRoutesAuthTOTPFlow(t *testing.T) {
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

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/totp/status", nil)
	statusReq.AddCookie(sessionCookie)
	statusRec := httptest.NewRecorder()
	router.ServeHTTP(statusRec, statusReq)
	require.Equal(t, http.StatusOK, statusRec.Code)
	require.Contains(t, statusRec.Body.String(), `"enabled":false`)

	setupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/setup", nil)
	setupReq.AddCookie(sessionCookie)
	setupRec := httptest.NewRecorder()
	router.ServeHTTP(setupRec, setupReq)
	require.Equal(t, http.StatusCreated, setupRec.Code)
	require.Contains(t, setupRec.Body.String(), "otpauth://totp/")
	require.NotContains(t, setupRec.Body.String(), `"secret"`)

	session, err := authService.SessionFromToken(t.Context(), sessionCookie.Value)
	require.NoError(t, err)
	setup, err := authService.SetupTOTP(t.Context(), auth.TOTPSetupRequest{
		Actor:   auth.Actor{UserID: session.UserID, Email: session.UserEmail, Status: session.Status},
		Session: session,
	})
	require.NoError(t, err)
	code, err := totp.GenerateCodeCustom(setup.Secret, now, routeTestTOTPValidateOpts())
	require.NoError(t, err)

	confirmReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/confirm", bytes.NewBufferString(`{"code":"`+code+`"}`))
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmReq.AddCookie(sessionCookie)
	confirmRec := httptest.NewRecorder()
	router.ServeHTTP(confirmRec, confirmReq)
	require.Equal(t, http.StatusOK, confirmRec.Code)
	require.Contains(t, confirmRec.Body.String(), `"enabled":true`)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)
	require.Contains(t, loginRec.Body.String(), `"totpRequired":true`)
	require.NotContains(t, loginRec.Body.String(), `"session"`)

	wrongPasswordReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"the wrong password value"}`))
	wrongPasswordReq.Header.Set("Content-Type", "application/json")
	wrongPasswordRec := httptest.NewRecorder()
	router.ServeHTTP(wrongPasswordRec, wrongPasswordReq)
	require.Equal(t, http.StatusUnauthorized, wrongPasswordRec.Code)
	require.NotContains(t, wrongPasswordRec.Body.String(), "totpRequired")

	now = now.Add(30 * time.Second)
	nextCode, err := totp.GenerateCodeCustom(setup.Secret, now, routeTestTOTPValidateOpts())
	require.NoError(t, err)
	loginReq = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple","totp_code":"`+nextCode+`"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec = httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)
	// A correct code must issue a real session, not repeat the challenge.
	require.Contains(t, loginRec.Body.String(), `"session"`)
	require.NotContains(t, loginRec.Body.String(), "totpRequired")
	require.NotEmpty(t, loginRec.Header().Get("Set-Cookie"))

	now = now.Add(30 * time.Second)
	disableCode, err := totp.GenerateCodeCustom(setup.Secret, now, routeTestTOTPValidateOpts())
	require.NoError(t, err)
	disableReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/disable", bytes.NewBufferString(`{"code":"`+disableCode+`"}`))
	disableReq.Header.Set("Content-Type", "application/json")
	disableReq.AddCookie(sessionCookie)
	disableRec := httptest.NewRecorder()
	router.ServeHTTP(disableRec, disableReq)
	require.Equal(t, http.StatusOK, disableRec.Code)
	require.Contains(t, disableRec.Body.String(), `"enabled":false`)
}

// TestRegisterRoutesAuthTOTPDisabled verifies runtime-disabled TOTP rejects setup while status remains safe.
func TestRegisterRoutesAuthTOTPDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Auth.TOTP.Enabled = false
	authService := testAuthService(cfg)
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	sessionCookie := registerAndLogin(t, router, cfg)

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/totp/status", nil)
	statusReq.AddCookie(sessionCookie)
	statusRec := httptest.NewRecorder()
	router.ServeHTTP(statusRec, statusReq)
	require.Equal(t, http.StatusOK, statusRec.Code)
	require.Contains(t, statusRec.Body.String(), `"enabled":false`)

	setupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/setup", nil)
	setupReq.AddCookie(sessionCookie)
	setupRec := httptest.NewRecorder()
	router.ServeHTTP(setupRec, setupReq)
	require.Equal(t, http.StatusBadRequest, setupRec.Code)
	require.Contains(t, setupRec.Body.String(), "totp setup failed")
}

// routeTestTOTPValidateOpts returns the TOTP options used by auth route tests.
func routeTestTOTPValidateOpts() totp.ValidateOpts {
	return totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}
}
