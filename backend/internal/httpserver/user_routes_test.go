package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/auth"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
)

// TestRegisterRoutesUserProfileSupportsBaseCurrency verifies profile read and update contracts.
func TestRegisterRoutesUserProfileSupportsBaseCurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	authService := testAuthService(cfg)
	RegisterRoutes(router, cfg, ledger.NewService(), authService)

	sessionCookie := registerAndLogin(t, router, cfg)
	getReq := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	getReq.AddCookie(sessionCookie)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)

	var getResponse struct {
		User auth.User `json:"user"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResponse))
	require.Equal(t, auth.DefaultBaseCurrency, getResponse.User.BaseCurrency)

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/users/me", bytes.NewBufferString(`{"baseCurrency":"eur"}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.AddCookie(sessionCookie)
	patchRec := httptest.NewRecorder()
	router.ServeHTTP(patchRec, patchReq)
	require.Equal(t, http.StatusOK, patchRec.Code)

	var patchResponse struct {
		User auth.User `json:"user"`
	}
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &patchResponse))
	require.Equal(t, "EUR", patchResponse.User.BaseCurrency)
}

// TestRegisterRoutesUserProfileRejectsInvalidPatch verifies user profile mutations fail closed.
func TestRegisterRoutesUserProfileRejectsInvalidPatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	log := logger.Setup(false)
	router.Use(requestLoggerForTest(log))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))

	sessionCookie := registerAndLogin(t, router, cfg)
	for _, body := range []string{`{"baseCurrency":"JPY"}`, `{"baseCurrency":"USD","unknown":true}`, `{}`} {
		req := httptest.NewRequest(http.MethodPatch, "/api/users/me", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(sessionCookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}
