package httpserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/Laisky/zap"
	"github.com/Laisky/zap/zapcore"
	"github.com/Laisky/zap/zaptest/observer"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestServerSSOCredentialLogging uses the production middleware order, including rejected callbacks.
func TestServerSSOCredentialLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.DebugLevel)
	log := logger.Setup(true)
	log = log.WithOptions(zap.WrapCore(func(zapcore.Core) zapcore.Core { return core }))
	cfg := testConfig()
	cfg.Debug = true
	cfg.Persistence.Driver = "memory"
	server, err := NewServer(cfg, log)
	require.NoError(t, err)
	for _, path := range []string{
		"/api/auth/sso/callback?sso_code=secret-code&sso_state=invalid",
		"/api/auth/sso/callback?sso_code=secret-code&sso_code=second-code",
		"/missing?sso_token=secret-token&SSO_CODE=upper-code",
		"/api/auth/sso/callback?sso_code=malformed;code",
		"/api/auth/sso/callback?%73so_code=encoded-code",
	} {
		rec := httptest.NewRecorder()
		server.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	}
	require.NotEmpty(t, logs.All())
	entries := fmt.Sprint(logs.All())
	for _, secret := range []string{"secret-code", "second-code", "secret-token", "upper-code", "malformed", "encoded-code"} {
		require.NotContains(t, entries, secret)
	}
}
