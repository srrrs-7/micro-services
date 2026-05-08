package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"shared/utilotel"
)

// installRecordingTracer swaps the global TracerProvider for one whose
// SpanProcessor records every ended span. The previous provider is
// restored on test cleanup.
func installRecordingTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))

	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})
	return rec
}

func TestRouteTag_renamesSpanToChiRoutePattern(t *testing.T) {
	rec := installRecordingTracer(t)

	r := chi.NewMux()
	r.Use(utilotel.HTTPMiddleware("auth-test"))
	r.Use(RouteTag())
	r.Get("/auth/v1/users/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/v1/users/42", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if got, want := spans[0].Name(), "GET /auth/v1/users/{id}"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}
}

func TestRouteTag_unmatchedRouteLeavesSpanNameAlone(t *testing.T) {
	rec := installRecordingTracer(t)

	r := chi.NewMux()
	r.Use(utilotel.HTTPMiddleware("auth-test"))
	r.Use(RouteTag())
	r.Get("/exists", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	// Pattern is empty for unmatched routes, so the otelhttp default
	// name remains. Just assert the retag did NOT produce a stray
	// "GET " (method + empty pattern).
	if got := spans[0].Name(); got == "GET " {
		t.Errorf("span name = %q, want non-stray name when no route matched", got)
	}
}
