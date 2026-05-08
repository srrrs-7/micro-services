// Package utilotel wires the OpenTelemetry SDK for the binaries in this
// monorepo. Init configures the global TracerProvider, MeterProvider, and
// text-map propagator from the standard OTEL_* environment variables;
// HTTPMiddleware / GRPCServerOption / GRPCClientOption add per-request
// instrumentation at the route layer.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset, Init wires noop providers so a
// dev loop without `make obs-up` running stays zero-overhead.
package utilotel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// EnvOTLPEndpoint is the env var the OTLP exporters read for the collector
// address. utilotel.Init treats an unset value as "use noop providers".
const EnvOTLPEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"

const shutdownTimeout = 5 * time.Second

// Init configures the global TracerProvider, MeterProvider, and text-map
// propagator for the calling binary, and starts Go runtime metrics
// collection.
//
// The W3C TraceContext + Baggage propagator is always installed so callers
// can rely on context propagation even when noop providers are in use.
//
// On success, the returned shutdown flushes pending traces and metrics; call
// it from the graceful-shutdown sequence in main(). It is safe to call when
// noop providers are in use.
func Init(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if os.Getenv(EnvOTLPEndpoint) == "" {
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
		otel.SetMeterProvider(metricnoop.NewMeterProvider())
		return func(context.Context) error { return nil }, nil
	}

	// resource.WithFromEnv reads OTEL_SERVICE_NAME / OTEL_RESOURCE_ATTRIBUTES.
	// We pass it last so env-supplied attributes override the explicit
	// service.name fallback above (matching the OTel spec — env wins).
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("utilotel: build resource: %v", err)
	}

	traceExp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("utilotel: otlp trace exporter: %v", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	metricExp, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("utilotel: otlp metric exporter: %v", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	if err := runtime.Start(); err != nil {
		return nil, fmt.Errorf("utilotel: start runtime metrics: %v", err)
	}

	return func(ctx context.Context) error {
		sctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
		defer cancel()
		return errors.Join(tp.Shutdown(sctx), mp.Shutdown(sctx))
	}, nil
}
