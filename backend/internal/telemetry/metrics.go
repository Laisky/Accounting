package telemetry

import (
	"context"
	"database/sql"

	"github.com/Laisky/zap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/Laisky/Accounting/backend/internal/logger"
)

// Metrics holds the backend's OpenTelemetry instruments. It is always safe to use: when no
// MeterProvider is configured the global provider is a no-op, so recording is cheap and the
// same call sites work whether or not OTLP metric export is enabled.
type Metrics struct {
	meter          metric.Meter
	httpRequests   metric.Int64Counter
	httpErrors     metric.Int64Counter
	httpDuration   metric.Float64Histogram
	activeRequests metric.Int64UpDownCounter
	entryWrites    metric.Int64Counter
	importPreviews metric.Int64Counter
	importApplies  metric.Int64Counter
	loginOutcomes  metric.Int64Counter
	clientErrors   metric.Int64Counter
	webVitals      metric.Float64Histogram
}

// NewMetrics builds the instrument set from the global MeterProvider.
func NewMetrics() *Metrics {
	m := otel.GetMeterProvider().Meter("github.com/Laisky/Accounting/backend")
	httpRequests, _ := m.Int64Counter("http.server.request.count", metric.WithDescription("HTTP requests handled"))
	httpErrors, _ := m.Int64Counter("http.server.error.count", metric.WithDescription("HTTP responses with status >= 500"))
	httpDuration, _ := m.Float64Histogram("http.server.duration", metric.WithUnit("s"), metric.WithDescription("HTTP request duration in seconds"))
	activeRequests, _ := m.Int64UpDownCounter("http.server.active_requests", metric.WithDescription("In-flight HTTP requests"))
	entryWrites, _ := m.Int64Counter("ledger.entry.writes", metric.WithDescription("Ledger entry create/update/delete operations"))
	importPreviews, _ := m.Int64Counter("imports.preview.count", metric.WithDescription("Import previews requested"))
	importApplies, _ := m.Int64Counter("imports.apply.count", metric.WithDescription("Import batches applied"))
	loginOutcomes, _ := m.Int64Counter("auth.login.outcome", metric.WithDescription("Login attempts by outcome"))
	clientErrors, _ := m.Int64Counter("client.error.count", metric.WithDescription("Client-reported frontend errors"))
	webVitals, _ := m.Float64Histogram("client.web_vitals", metric.WithDescription("Sampled Web Vitals values"))
	return &Metrics{
		meter:          m,
		httpRequests:   httpRequests,
		httpErrors:     httpErrors,
		httpDuration:   httpDuration,
		activeRequests: activeRequests,
		entryWrites:    entryWrites,
		importPreviews: importPreviews,
		importApplies:  importApplies,
		loginOutcomes:  loginOutcomes,
		clientErrors:   clientErrors,
		webVitals:      webVitals,
	}
}

// RegisterDBStats registers observable DB connection-pool instruments sourced from sql.DB.Stats().
// It is a no-op when the receiver or the pool is nil (e.g. the in-memory driver), so it never
// panics and the memory driver emits no db.* series.
func (m *Metrics) RegisterDBStats(db *sql.DB) {
	if m == nil || db == nil {
		return
	}
	usage, _ := m.meter.Int64ObservableGauge("db.client.connections.usage",
		metric.WithDescription("Open connections by state (in_use/idle)"))
	maxOpen, _ := m.meter.Int64ObservableGauge("db.client.connections.max",
		metric.WithDescription("Maximum number of open connections allowed"))
	waitCount, _ := m.meter.Int64ObservableCounter("db.client.connections.wait_count",
		metric.WithDescription("Total number of connections waited for"))
	waitDuration, _ := m.meter.Float64ObservableCounter("db.client.connections.wait_duration",
		metric.WithUnit("s"), metric.WithDescription("Total time blocked waiting for a connection"))

	inUse := metric.WithAttributes(attribute.String("state", "in_use"))
	idle := metric.WithAttributes(attribute.String("state", "idle"))
	callback := func(_ context.Context, o metric.Observer) error {
		stats := db.Stats()
		o.ObserveInt64(usage, int64(stats.InUse), inUse)
		o.ObserveInt64(usage, int64(stats.Idle), idle)
		o.ObserveInt64(maxOpen, int64(stats.MaxOpenConnections))
		o.ObserveInt64(waitCount, stats.WaitCount)
		o.ObserveFloat64(waitDuration, stats.WaitDuration.Seconds())
		return nil
	}
	if _, err := m.meter.RegisterCallback(callback, usage, maxOpen, waitCount, waitDuration); err != nil {
		logger.Logger.Warn("register db pool metrics callback", zap.Error(err))
	}
}

// IncActiveRequests increments the in-flight request gauge for a matched route.
func (m *Metrics) IncActiveRequests(ctx context.Context, method, route string) {
	if m == nil {
		return
	}
	m.activeRequests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("http.method", method),
		attribute.String("http.route", route),
	))
}

// DecActiveRequests decrements the in-flight request gauge for a matched route.
func (m *Metrics) DecActiveRequests(ctx context.Context, method, route string) {
	if m == nil {
		return
	}
	m.activeRequests.Add(ctx, -1, metric.WithAttributes(
		attribute.String("http.method", method),
		attribute.String("http.route", route),
	))
}

// RecordHTTP records one HTTP request's RED metrics (rate, errors, duration).
func (m *Metrics) RecordHTTP(ctx context.Context, method, route string, status int, seconds float64) {
	if m == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("http.method", method),
		attribute.String("http.route", route),
		attribute.Int("http.status_code", status),
	)
	m.httpRequests.Add(ctx, 1, attrs)
	m.httpDuration.Record(ctx, seconds, attrs)
	if status >= 500 {
		m.httpErrors.Add(ctx, 1, attrs)
	}
}

// RecordEntryWrite counts a ledger entry mutation by operation (create/update/delete).
func (m *Metrics) RecordEntryWrite(ctx context.Context, op string) {
	if m == nil {
		return
	}
	m.entryWrites.Add(ctx, 1, metric.WithAttributes(attribute.String("op", op)))
}

// RecordImportPreview counts an import preview request.
func (m *Metrics) RecordImportPreview(ctx context.Context) {
	if m == nil {
		return
	}
	m.importPreviews.Add(ctx, 1)
}

// RecordImportApply counts an applied import batch.
func (m *Metrics) RecordImportApply(ctx context.Context) {
	if m == nil {
		return
	}
	m.importApplies.Add(ctx, 1)
}

// RecordLoginOutcome counts a login attempt by outcome (success/failure/totp_required).
func (m *Metrics) RecordLoginOutcome(ctx context.Context, outcome string) {
	if m == nil {
		return
	}
	m.loginOutcomes.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
}

// RecordClientError counts a client-reported frontend error.
func (m *Metrics) RecordClientError(ctx context.Context) {
	if m == nil {
		return
	}
	m.clientErrors.Add(ctx, 1)
}

// RecordWebVital records a sampled Web Vital measurement.
func (m *Metrics) RecordWebVital(ctx context.Context, name string, value float64, rating string) {
	if m == nil {
		return
	}
	m.webVitals.Record(ctx, value, metric.WithAttributes(
		attribute.String("metric", name),
		attribute.String("rating", rating),
	))
}
