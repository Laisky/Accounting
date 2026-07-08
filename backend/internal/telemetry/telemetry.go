// Package telemetry configures OpenTelemetry providers for the backend.
package telemetry

import (
	"context"
	"net/http"

	"github.com/Laisky/errors/v2"
	"github.com/Laisky/zap"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/Laisky/Accounting/backend/internal/config"
	"github.com/Laisky/Accounting/backend/internal/logger"
)

// ProviderBundle contains telemetry providers that need graceful shutdown.
type ProviderBundle struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	metricsHandler http.Handler
}

// Init receives runtime config, initializes OpenTelemetry tracing and metrics, and returns
// providers for shutdown. Tracing plus OTLP metric push are gated on cfg.Enabled; the Prometheus
// /metrics reader is gated independently on cfg.MetricsEnabled, so a MeterProvider is built (and
// registered as the global provider) whenever either export path is enabled.
func Init(ctx context.Context, cfg config.TelemetryConfig) (*ProviderBundle, error) {
	if !cfg.Enabled && !cfg.MetricsEnabled {
		return nil, nil //nolint:nilnil // Disabled telemetry intentionally returns no providers and no error.
	}

	if cfg.Enabled && cfg.Endpoint == "" {
		return nil, errors.New("ACCOUNTING_OTEL_EXPORTER_OTLP_ENDPOINT is required when ACCOUNTING_OTEL_ENABLED is true")
	}

	resource, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "build OpenTelemetry resource")
	}

	bundle := &ProviderBundle{}

	if cfg.Enabled {
		traceExporter, err := otlptracehttp.New(ctx, buildTraceExporterOptions(cfg)...)
		if err != nil {
			return nil, errors.Wrap(err, "create OTLP trace exporter")
		}
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(resource),
		)
		otel.SetTracerProvider(tracerProvider)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		bundle.tracerProvider = tracerProvider
	}

	// A single MeterProvider feeds every enabled reader: the OTLP push PeriodicReader and/or the
	// Prometheus pull exporter, both reading the same instruments.
	meterOptions := []sdkmetric.Option{sdkmetric.WithResource(resource)}
	if cfg.Enabled {
		metricExporter, err := otlpmetrichttp.New(ctx, buildMetricExporterOptions(cfg)...)
		if err != nil {
			return nil, errors.Wrap(err, "create OTLP metric exporter")
		}
		meterOptions = append(meterOptions, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)))
	}
	if cfg.MetricsEnabled {
		// A dedicated registry (rather than the process-wide default) keeps repeated Init calls,
		// e.g. across tests, from colliding on duplicate collector registration.
		registry := prometheus.NewRegistry()
		promReader, err := otelprom.New(otelprom.WithRegisterer(registry))
		if err != nil {
			return nil, errors.Wrap(err, "create Prometheus metric exporter")
		}
		meterOptions = append(meterOptions, sdkmetric.WithReader(promReader))
		bundle.metricsHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	}
	meterProvider := sdkmetric.NewMeterProvider(meterOptions...)
	otel.SetMeterProvider(meterProvider)
	bundle.meterProvider = meterProvider

	logger.Logger.Info("OpenTelemetry initialized",
		zap.Bool("otlp_push", cfg.Enabled),
		zap.Bool("prometheus_metrics", cfg.MetricsEnabled),
		zap.String("endpoint", cfg.Endpoint),
		zap.Bool("insecure", cfg.Insecure),
		zap.String("service", cfg.ServiceName),
		zap.String("environment", cfg.Environment))

	return bundle, nil
}

// MetricsHandler returns the Prometheus /metrics HTTP handler, or nil when the Prometheus
// reader is disabled. It is nil-safe so callers can pass Init's result straight through.
func (p *ProviderBundle) MetricsHandler() http.Handler {
	if p == nil {
		return nil
	}
	return p.metricsHandler
}

// Shutdown receives a context, flushes telemetry providers, and returns shutdown errors.
func (p *ProviderBundle) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}

	if p.tracerProvider != nil {
		if err := p.tracerProvider.Shutdown(ctx); err != nil {
			return errors.Wrap(err, "shutdown tracer provider")
		}
	}

	if p.meterProvider != nil {
		if err := p.meterProvider.Shutdown(ctx); err != nil {
			return errors.Wrap(err, "shutdown meter provider")
		}
	}

	return nil
}

// buildMetricExporterOptions receives telemetry config and returns OTLP metric exporter options.
func buildMetricExporterOptions(cfg config.TelemetryConfig) []otlpmetrichttp.Option {
	options := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.Endpoint),
		otlpmetrichttp.WithCompression(otlpmetrichttp.GzipCompression),
	}

	if cfg.Insecure {
		options = append(options, otlpmetrichttp.WithInsecure())
	}

	return options
}

// buildResource receives telemetry config and returns the OpenTelemetry resource attributes.
func buildResource(ctx context.Context, cfg config.TelemetryConfig) (*sdkresource.Resource, error) {
	attributes := []attribute.KeyValue{
		attribute.String("service.name", cfg.ServiceName),
	}
	if cfg.Environment != "" {
		attributes = append(attributes, attribute.String("deployment.environment", cfg.Environment))
	}

	resource, err := sdkresource.New(ctx,
		sdkresource.WithFromEnv(),
		sdkresource.WithHost(),
		sdkresource.WithTelemetrySDK(),
		sdkresource.WithProcess(),
		sdkresource.WithAttributes(attributes...),
	)
	if err != nil {
		return nil, errors.Wrap(err, "create OpenTelemetry resource")
	}

	return resource, nil
}

// buildTraceExporterOptions receives telemetry config and returns OTLP trace exporter options.
func buildTraceExporterOptions(cfg config.TelemetryConfig) []otlptracehttp.Option {
	options := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
	}

	if cfg.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}

	return options
}
