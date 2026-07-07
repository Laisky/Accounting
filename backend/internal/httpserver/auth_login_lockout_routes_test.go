package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRegisterRoutesLoginLockoutReturnsRetryAfter verifies account lockout surfaces as HTTP 429.
func TestRegisterRoutesLoginLockoutReturnsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	registerReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, registerReq)
	require.Equal(t, http.StatusCreated, registerRec.Code)

	for range 6 {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"wrong password"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"email":"person@example.test","password":"correct horse battery staple"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.Positive(t, retryAfter)
	require.JSONEq(t, `{"code":"login_temporarily_locked","message":"login temporarily locked"}`, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "person@example.test")
}
