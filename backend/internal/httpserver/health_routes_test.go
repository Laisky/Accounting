package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/ledger"
	"github.com/Laisky/Accounting/backend/internal/logger"
	"github.com/Laisky/Accounting/backend/internal/storage"
	"github.com/Laisky/Accounting/backend/internal/telemetry"
)

// newOpsRouter builds a root engine with only the operational endpoints (health/readiness) wired.
func newOpsRouter(t *testing.T, db *storage.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(requestLoggerForTest(logger.Setup(false)))
	registerOpsRoutes(router, db, nil)
	return router
}

func opsGet(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// TestHealthzAlwaysOK verifies /healthz is pure liveness: 200 with a nil pool and still 200 after
// the database pool has been closed, because it never touches the database.
func TestHealthzAlwaysOK(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		rec := opsGet(t, newOpsRouter(t, nil), "/healthz")
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"status":"ok"`)
	})

	t.Run("closed db", func(t *testing.T) {
		db, err := storage.Open(context.Background(), "sqlite", "", t.TempDir())
		require.NoError(t, err)
		require.NoError(t, db.Close())

		rec := opsGet(t, newOpsRouter(t, db), "/healthz")
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"status":"ok"`)
	})
}

// TestReadyzSkippedOnMemoryDriver verifies /readyz reports the database as skipped (200) when there
// is no SQL pool, i.e. the in-memory driver.
func TestReadyzSkippedOnMemoryDriver(t *testing.T) {
	rec := opsGet(t, newOpsRouter(t, nil), "/readyz")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"database":"skipped"`)
}

// TestReadyzReportsDatabaseState verifies /readyz pings the pool: 200 database:ok while open, and a
// sanitized 503 database:unavailable after the pool is closed, without leaking the driver error.
func TestReadyzReportsDatabaseState(t *testing.T) {
	db, err := storage.Open(context.Background(), "sqlite", "", t.TempDir())
	require.NoError(t, err)
	router := newOpsRouter(t, db)

	rec := opsGet(t, router, "/readyz")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"database":"ok"`)

	require.NoError(t, db.Close())

	rec = opsGet(t, router, "/readyz")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `"database":"unavailable"`)
	// The sanitized response must not leak driver internals.
	require.NotContains(t, body, "sql:")
	require.NotContains(t, body, "closed")
}

// TestMetricsRendersRequestCounterWhenEnabled verifies /metrics renders the shipped OTel HTTP RED
// counter after a request when the Prometheus reader is enabled.
func TestMetricsRendersRequestCounterWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Setup(false) // telemetry.Init logs via the global logger, as main.go arranges in production.
	// Init installs a global MeterProvider carrying the Prometheus reader; NewMetrics (invoked by
	// registerRoutes below) then binds its instruments to it. Restore the previous provider after.
	prev := otel.GetMeterProvider()
	t.Cleanup(func() { otel.SetMeterProvider(prev) })

	bundle, err := telemetry.Init(context.Background(), config.TelemetryConfig{MetricsEnabled: true})
	require.NoError(t, err)
	require.NotNil(t, bundle)
	t.Cleanup(func() { _ = bundle.Shutdown(context.Background()) })
	handler := bundle.MetricsHandler()
	require.NotNil(t, handler)

	router := gin.New()
	router.Use(requestLoggerForTest(logger.Setup(false)))
	cfg := testConfig()
	RegisterRoutesWithServices(router, cfg, ledger.NewService(), testAuthService(cfg), nil, nil, nil, handler)

	// Drive one request so the http.server.request.count counter has a data point.
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	rec := opsGet(t, router, "/metrics")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "http_server_request_count_total")
	require.Contains(t, body, "http_server_active_requests")
	// The in-memory driver has no SQL pool, so no db.* series is registered.
	require.NotContains(t, body, "db_client_connections")
}

// TestMetricsRendersDBPoolStatsForSQLDriver verifies the SQL driver's connection-pool gauges are
// registered and rendered when a storage handle is wired into the metrics registry.
func TestMetricsRendersDBPoolStatsForSQLDriver(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.Setup(false) // telemetry.Init logs via the global logger, as main.go arranges in production.
	prev := otel.GetMeterProvider()
	t.Cleanup(func() { otel.SetMeterProvider(prev) })

	bundle, err := telemetry.Init(context.Background(), config.TelemetryConfig{MetricsEnabled: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bundle.Shutdown(context.Background()) })
	handler := bundle.MetricsHandler()
	require.NotNil(t, handler)

	db, err := storage.Open(context.Background(), "sqlite", "", t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	router := gin.New()
	router.Use(requestLoggerForTest(logger.Setup(false)))
	cfg := testConfig()
	RegisterRoutesWithServices(router, cfg, ledger.NewService(), testAuthService(cfg), nil, nil, db, handler)

	rec := opsGet(t, router, "/metrics")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "db_client_connections_usage")
	require.Contains(t, body, "db_client_connections_max")
}

// TestMetricsAbsentWhenDisabled verifies /metrics is not registered (404) when no Prometheus
// handler is supplied, i.e. ACCOUNTING_OTEL_METRICS_ENABLED=false.
func TestMetricsAbsentWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(requestLoggerForTest(logger.Setup(false)))
	cfg := testConfig()
	RegisterRoutesWithServices(router, cfg, ledger.NewService(), testAuthService(cfg), nil, nil, nil, nil)

	rec := opsGet(t, router, "/metrics")
	require.Equal(t, http.StatusNotFound, rec.Code)
}
