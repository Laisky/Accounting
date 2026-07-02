package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auditpkg "github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesPasskeyCeremonies verifies passkey begin endpoints expose WebAuthn options without secrets.
func TestRegisterRoutesPasskeyCeremonies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	auditService := auditpkg.NewService(auditpkg.NewMemoryStore())
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg), auditService)

	loginBeginReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkeys/login/begin", nil)
	loginBeginRec := httptest.NewRecorder()
	router.ServeHTTP(loginBeginRec, loginBeginReq)
	require.Equal(t, http.StatusCreated, loginBeginRec.Code)

	var loginStart auth.PasskeyLoginStart
	err := json.Unmarshal(loginBeginRec.Body.Bytes(), &loginStart)
	require.NoError(t, err)
	require.NotEmpty(t, loginStart.FlowID)
	require.NotContains(t, loginBeginRec.Body.String(), "private")
	require.Contains(t, loginBeginRec.Body.String(), `"rpId":"example.test"`)

	loginFinishReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkeys/login/finish", bytes.NewBufferString(`{"flowId":"`+loginStart.FlowID+`","credential":{"id":"bad","rawId":"bad","type":"public-key","response":{}}}`))
	loginFinishReq.Header.Set("Content-Type", "application/json")
	loginFinishRec := httptest.NewRecorder()
	router.ServeHTTP(loginFinishRec, loginFinishReq)
	require.Equal(t, http.StatusUnauthorized, loginFinishRec.Code)

	sessionCookie := registerAndLogin(t, router, cfg)
	registerBeginReq := httptest.NewRequest(http.MethodPost, "/api/auth/passkeys/register/begin", nil)
	registerBeginReq.AddCookie(sessionCookie)
	registerBeginRec := httptest.NewRecorder()
	router.ServeHTTP(registerBeginRec, registerBeginReq)
	require.Equal(t, http.StatusCreated, registerBeginRec.Code)

	var registrationStart auth.PasskeyRegistrationStart
	err = json.Unmarshal(registerBeginRec.Body.Bytes(), &registrationStart)
	require.NoError(t, err)
	require.NotEmpty(t, registrationStart.FlowID)
	require.Contains(t, registerBeginRec.Body.String(), `"residentKey":"required"`)
	require.Contains(t, registerBeginRec.Body.String(), `"userVerification":"required"`)

	auditReq := httptest.NewRequest(http.MethodGet, "/api/audit?page=1&page_size=20", nil)
	auditReq.AddCookie(sessionCookie)
	auditRec := httptest.NewRecorder()
	router.ServeHTTP(auditRec, auditReq)
	require.Equal(t, http.StatusOK, auditRec.Code)
	require.Contains(t, auditRec.Body.String(), `"action":"auth.passkey_registration_started"`)
}

// TestRegisterRoutesPasskeyManagement verifies passkey list, rename, and delete endpoints return public metadata.
func TestRegisterRoutesPasskeyManagement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	store := auth.NewMemoryStore()
	authService := auth.NewService(auth.Config{
		EmailLoginEnabled:         cfg.Auth.Email.LoginEnabled,
		EmailRegisterEnabled:      cfg.Auth.Email.RegisterEnabled,
		EmailVerificationRequired: cfg.Auth.Email.VerificationRequired,
		SessionTTL:                cfg.Auth.Session.TTL,
		TOTPEnabled:               cfg.Auth.TOTP.Enabled,
		PasskeyEnabled:            cfg.Auth.Passkey.Enabled,
		PasskeyRPDisplayName:      cfg.Auth.Passkey.RPDisplayName,
		PasskeyRPID:               cfg.Auth.Passkey.RPID,
		PasskeyRPOrigin:           cfg.Auth.Passkey.RPOrigin,
	}, store, auth.NoopTurnstileVerifier{})
	auditService := auditpkg.NewService(auditpkg.NewMemoryStore())
	RegisterRoutes(router, cfg, ledger.NewService(), authService, auditService)

	hash, err := auth.HashPassword("correct horse battery staple")
	require.NoError(t, err)
	_, err = store.CreateUser(t.Context(), auth.UserRecord{
		User: auth.User{
			ID:            "user-owner",
			Email:         "user-owner@example.test",
			Status:        auth.UserStatusActive,
			EmailVerified: true,
			CreatedAt:     time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
		},
		PasswordHash: hash,
	})
	require.NoError(t, err)
	_, err = store.CreatePasskey(t.Context(), auth.PasskeyCredential{
		ID:             "passkey-1",
		UserID:         "user-owner",
		Label:          "Laptop",
		CredentialID:   []byte("credential-1"),
		PublicKey:      []byte("public-key"),
		BackupEligible: true,
		BackupState:    true,
		Transports:     []string{"internal"},
		CreatedAt:      time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	sessionCookie := loginSeededUser(t, router, cfg, "user-owner")
	listReq := httptest.NewRequest(http.MethodGet, "/api/auth/passkeys", nil)
	listReq.AddCookie(sessionCookie)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)
	require.Contains(t, listRec.Body.String(), `"label":"Laptop"`)
	require.Contains(t, listRec.Body.String(), `"total":1`)
	require.NotContains(t, listRec.Body.String(), "public-key")
	require.NotContains(t, listRec.Body.String(), "credential-1")

	updateReq := httptest.NewRequest(http.MethodPut, "/api/auth/passkeys/passkey-1", bytes.NewBufferString(`{"label":"Work laptop"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.AddCookie(sessionCookie)
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	require.Equal(t, http.StatusOK, updateRec.Code)
	require.Contains(t, updateRec.Body.String(), `"label":"Work laptop"`)

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/auth/passkeys/passkey-1", nil)
	deleteReq.AddCookie(sessionCookie)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	require.Equal(t, http.StatusOK, deleteRec.Code)

	listReq = httptest.NewRequest(http.MethodGet, "/api/auth/passkeys", nil)
	listReq.AddCookie(sessionCookie)
	listRec = httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)
	require.JSONEq(t, `{"items":[],"page":1,"pageSize":50,"total":0}`, listRec.Body.String())
}

// TestRegisterRoutesPasskeysCanBeDisabled verifies disabled passkeys fail closed.
func TestRegisterRoutesPasskeysCanBeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	cfg.Auth.Passkey.Enabled = false
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkeys/login/begin", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "passkey login start failed")
}
