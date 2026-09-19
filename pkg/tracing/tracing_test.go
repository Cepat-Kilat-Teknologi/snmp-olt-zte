package tracing

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestInit_Disabled(t *testing.T) {
	cfg := Config{
		Enabled:     false,
		Endpoint:    "localhost:4317",
		ServiceName: "test-snmp",
		Environment: "test",
		Version:     "v0.0.0-test",
	}

	shutdown, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init(disabled) error: %v", err)
	}

	// Shutdown should be a safe no-op.
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}

	// Global provider must produce noop spans when disabled. Verify by
	// checking it is NOT an *sdktrace.TracerProvider (the real SDK type).
	tp := otel.GetTracerProvider()
	if _, ok := tp.(*sdktrace.TracerProvider); ok {
		t.Error("expected noop provider when disabled, got *sdktrace.TracerProvider")
	}

	// Propagator should still be set (W3C TraceContext + Baggage).
	prop := otel.GetTextMapPropagator()
	fields := prop.Fields()
	want := map[string]bool{"traceparent": false, "tracestate": false, "baggage": false}
	for _, f := range fields {
		want[f] = true
	}
	for k, found := range want {
		if !found {
			t.Errorf("propagator missing field %q", k)
		}
	}
}

func TestInit_Enabled(t *testing.T) {
	cfg := Config{
		Enabled:     true,
		Endpoint:    "localhost:0", // unreachable, but gRPC connect is lazy
		ServiceName: "test-snmp",
		Environment: "test",
		Version:     "v0.0.0-test",
	}

	shutdown, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init(enabled) error: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			t.Logf("shutdown (expected): %v", err)
		}
	}()

	// Global provider must be the real SDK TracerProvider when enabled.
	tp := otel.GetTracerProvider()
	if _, ok := tp.(*sdktrace.TracerProvider); !ok {
		t.Errorf("expected *sdktrace.TracerProvider, got %T", tp)
	}
}

func TestInit_ExporterError(t *testing.T) {
	// Replace the exporter factory with one that always fails.
	orig := newExporter
	newExporter = func(_ context.Context, _ ...otlptracegrpc.Option) (sdktrace.SpanExporter, error) {
		return nil, errors.New("exporter boom")
	}
	t.Cleanup(func() { newExporter = orig })

	cfg := Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "test-snmp",
		Environment: "test",
		Version:     "v0.0.0-test",
	}

	_, err := Init(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error from failing exporter, got nil")
	}
	if err.Error() != "exporter boom" {
		t.Errorf("error = %q, want 'exporter boom'", err)
	}
}

func TestInit_ResourceMergeError(t *testing.T) {
	// Replace the resource merge function with one that always fails.
	orig := mergeResources
	mergeResources = func(_, _ *resource.Resource) (*resource.Resource, error) {
		return nil, errors.New("merge boom")
	}
	t.Cleanup(func() { mergeResources = orig })

	cfg := Config{
		Enabled:     true,
		Endpoint:    "localhost:0",
		ServiceName: "test-snmp",
		Environment: "test",
		Version:     "v0.0.0-test",
	}

	_, err := Init(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error from failing resource merge, got nil")
	}
	if err.Error() != "merge boom" {
		t.Errorf("error = %q, want 'merge boom'", err)
	}
}

func TestInit_ShutdownIdempotent(t *testing.T) {
	cfg := Config{Enabled: false}
	shutdown, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Calling shutdown multiple times must not panic.
	for i := 0; i < 3; i++ {
		if err := shutdown(context.Background()); err != nil {
			t.Errorf("shutdown call %d: %v", i, err)
		}
	}
}
