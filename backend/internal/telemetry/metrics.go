package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds the backend's OpenTelemetry instruments. It is always safe to use: when no
// MeterProvider is configured the global provider is a no-op, so recording is cheap and the
// same call sites work whether or not OTLP metric export is enabled.
type Metrics struct {
	httpRequests   metric.Int64Counter
	httpErrors     metric.Int64Counter
	httpDuration   metric.Float64Histogram
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
	entryWrites, _ := m.Int64Counter("ledger.entry.writes", metric.WithDescription("Ledger entry create/update/delete operations"))
	importPreviews, _ := m.Int64Counter("imports.preview.count", metric.WithDescription("Import previews requested"))
	importApplies, _ := m.Int64Counter("imports.apply.count", metric.WithDescription("Import batches applied"))
	loginOutcomes, _ := m.Int64Counter("auth.login.outcome", metric.WithDescription("Login attempts by outcome"))
	clientErrors, _ := m.Int64Counter("client.error.count", metric.WithDescription("Client-reported frontend errors"))
	webVitals, _ := m.Float64Histogram("client.web_vitals", metric.WithDescription("Sampled Web Vitals values"))
	return &Metrics{
		httpRequests:   httpRequests,
		httpErrors:     httpErrors,
		httpDuration:   httpDuration,
		entryWrites:    entryWrites,
		importPreviews: importPreviews,
		importApplies:  importApplies,
		loginOutcomes:  loginOutcomes,
		clientErrors:   clientErrors,
		webVitals:      webVitals,
	}
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
