// Package telemetry configures OpenTelemetry providers for the backend.
package telemetry

import (
	"context"

	"github.com/Laisky/errors/v2"
	"github.com/Laisky/zap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
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
}

// Init receives runtime config, initializes OpenTelemetry tracing when enabled, and returns providers for shutdown.
func Init(ctx context.Context, cfg config.TelemetryConfig) (*ProviderBundle, error) {
	if !cfg.Enabled {
		return nil, nil //nolint:nilnil // Disabled telemetry intentionally returns no providers and no error.
	}

	if cfg.Endpoint == "" {
		return nil, errors.New("ACCOUNTING_OTEL_EXPORTER_OTLP_ENDPOINT is required when ACCOUNTING_OTEL_ENABLED is true")
	}

	resource, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "build OpenTelemetry resource")
	}

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

	metricExporter, err := otlpmetrichttp.New(ctx, buildMetricExporterOptions(cfg)...)
	if err != nil {
		return nil, errors.Wrap(err, "create OTLP metric exporter")
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(resource),
	)
	otel.SetMeterProvider(meterProvider)

	logger.Logger.Info("OpenTelemetry tracing and metrics initialized",
		zap.String("endpoint", cfg.Endpoint),
		zap.Bool("insecure", cfg.Insecure),
		zap.String("service", cfg.ServiceName),
		zap.String("environment", cfg.Environment))

	return &ProviderBundle{tracerProvider: tracerProvider, meterProvider: meterProvider}, nil
}

// Shutdown receives a context, flushes telemetry providers, and returns shutdown errors.
func (p *ProviderBundle) Shutdown(ctx context.Context) error {
	if p == nil || p.tracerProvider == nil {
		return nil
	}

	if err := p.tracerProvider.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "shutdown tracer provider")
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
