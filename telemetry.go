package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// serviceName tags every trace, metric, and log so this service's telemetry is
// distinguishable in a shared environment.
const serviceName = "duffel"

// initTelemetry sets up the global OpenTelemetry providers so the rest of the
// program can emit telemetry without knowing how it is exported. The returned
// shutdown function must run before exit, otherwise buffered telemetry from the
// final moments of the process is dropped.
func initTelemetry(ctx context.Context) (func(context.Context) error, error) {
	// build the resource once and reuse it for all three providers so traces,
	// metrics, and logs share identical identifying attributes and can be
	// correlated. Environment-derived attributes come first; the explicit
	// service name is applied last so it can't be overridden by the environment.
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	// the endpoint and credentials for every exporter are left to the OTEL_*
	// environment variables so the collector target can change per deployment
	// without touching this code.
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create otlp trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	// register context and baggage propagation so a trace started upstream
	// continues through this service instead of breaking into disconnected spans.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create otlp metric exporter: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	// register the logger provider globally so the handlers' slog bridge can find
	// it on its own, keeping this the single place that owns telemetry wiring.
	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create otlp log exporter: %w", err)
	}

	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	otellog.SetLoggerProvider(logProvider)

	// give shutdown its own deadline so a slow or unreachable collector can't
	// stall process exit. Every provider is shut down even if an earlier one
	// fails, and the errors are joined so no failure is silently swallowed.
	return func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var errs []error
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}

		if err := meterProvider.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown meter provider: %w", err))
		}

		if err := logProvider.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown logger provider: %w", err))
		}

		return errors.Join(errs...)
	}, nil
}
