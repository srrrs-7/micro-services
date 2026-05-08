package utilotel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestInit_unsetEndpoint_returnsNoopShutdown(t *testing.T) {
	t.Setenv(EnvOTLPEndpoint, "")

	shutdown, err := Init(context.Background(), "test-service")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if shutdown == nil {
		t.Fatal("Init returned nil shutdown")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown: %v", err)
	}
}

func TestInit_alwaysSetsW3CPropagator(t *testing.T) {
	t.Setenv(EnvOTLPEndpoint, "")
	if _, err := Init(context.Background(), "test-service"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	prop := otel.GetTextMapPropagator()
	if prop == nil {
		t.Fatal("GetTextMapPropagator() = nil")
	}

	want := map[string]bool{"traceparent": true, "tracestate": true, "baggage": true}
	got := map[string]bool{}
	for _, f := range prop.Fields() {
		got[f] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("propagator missing %q (got %v)", k, prop.Fields())
		}
	}
}

func TestInit_unsetEndpoint_installsNoopProviders(t *testing.T) {
	t.Setenv(EnvOTLPEndpoint, "")
	if _, err := Init(context.Background(), "test-service"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Calling Tracer / Meter through the global providers must not panic
	// even when noop is in effect.
	tracer := otel.Tracer("utilotel-test")
	_, span := tracer.Start(context.Background(), "noop")
	span.End()

	meter := otel.Meter("utilotel-test")
	if _, err := meter.Int64Counter("noop_counter"); err != nil {
		t.Errorf("Int64Counter on noop meter: %v", err)
	}
}
