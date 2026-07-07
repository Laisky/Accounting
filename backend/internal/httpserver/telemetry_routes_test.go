package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
)

// newTelemetryTestRouter builds a router with the request-id middleware so telemetry
// responses carry X-Request-ID, mirroring the production middleware chain.
func newTelemetryTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(requestIDMiddleware)
	router.Use(requestLoggerForTest(logger.Setup(false)))
	cfg := testConfig()
	RegisterRoutes(router, cfg, ledger.NewService(), testAuthService(cfg))
	return router
}

func postTelemetry(router *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/telemetry/client", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestClientTelemetryAcceptsVitalsWithRequestID(t *testing.T) {
	router := newTelemetryTestRouter(t)
	rec := postTelemetry(router, `{"kind":"vitals","metricName":"LCP","metricValue":1200,"rating":"good"}`)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

func TestClientTelemetryAcceptsError(t *testing.T) {
	router := newTelemetryTestRouter(t)
	rec := postTelemetry(router, `{"kind":"error","eventId":"e1","errorName":"TypeError","errorMessageHash":"abc123"}`)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

// TestClientTelemetryRejectsSensitiveField proves the strict allowlist: any field outside
// the documented set (here a bookkeeping amount) is rejected, so no sensitive data can be
// logged or forwarded to alerting.
func TestClientTelemetryRejectsSensitiveField(t *testing.T) {
	router := newTelemetryTestRouter(t)
	rec := postTelemetry(router, `{"kind":"vitals","metricName":"LCP","metricValue":1,"amount":9999,"note":"lunch"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestClientTelemetryRejectsUnknownKind(t *testing.T) {
	router := newTelemetryTestRouter(t)
	rec := postTelemetry(router, `{"kind":"note","metricName":"LCP"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
