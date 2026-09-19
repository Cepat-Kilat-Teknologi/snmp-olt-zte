// Package tracing initializes the OpenTelemetry trace pipeline for snmp-olt-zte.
// When OTEL_ENABLED is false (the default) a noop provider is used, adding zero
// overhead. When enabled, spans are exported via OTLP/gRPC to a collector.
package tracing

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// Config holds the OTel tracing configuration read from environment variables.
type Config struct {
	Enabled     bool   // OTEL_ENABLED (default: false)
	Endpoint    string // OTEL_EXPORTER_OTLP_ENDPOINT (default: localhost:4317)
	ServiceName string // OTEL_SERVICE_NAME (default: snmp-olt-zte)
	Environment string // APP_ENV — attached as deployment.environment resource attribute
	Version     string // build version — attached as service.version resource attribute
}

// ShutdownFunc drains buffered spans and releases resources. Callers should
// defer it after Init returns successfully.
type ShutdownFunc func(ctx context.Context) error

// newExporter creates an OTLP/gRPC span exporter. Package-level var so tests
// can inject a failing factory without a real gRPC connection.
var newExporter = func(ctx context.Context, opts ...otlptracegrpc.Option) (sdktrace.SpanExporter, error) {
	return otlptracegrpc.New(ctx, opts...)
}

// mergeResources combines two resources. Exposed as a var so tests can exercise
// the error return path (which is unreachable with NewSchemaless in production).
var mergeResources = resource.Merge

// Init configures the global TracerProvider and TextMapPropagator. When
// cfg.Enabled is false the global provider stays noop and the returned shutdown
// is a no-op — existing code that doesn't touch OTel is entirely unaffected.
func Init(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	// Always set the propagator so incoming traceparent headers are parsed even
	// when local tracing is off (the service may sit between traced services).
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if !cfg.Enabled {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := newExporter(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(), // TLS should be handled by the collector sidecar/mesh
	)
	if err != nil {
		return nil, err
	}

	// Build a resource with service metadata. Use NewSchemaless to avoid
	// schema-version conflicts with resource.Default() (SDK may bundle a
	// newer semconv schema than v1.26.0).
	res, err := mergeResources(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
	)
	otel.SetTracerProvider(tp)

	shutdown := func(ctx context.Context) error {
		// Give the batcher a bounded window to flush remaining spans.
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx)
	}
	return shutdown, nil
}
