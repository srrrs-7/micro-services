package utilotel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

func TestHTTPMiddleware_skipsGETHealthByDefault(t *testing.T) {
	rec := installRecordingTracer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/foo", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := httptest.NewServer(HTTPMiddleware("test")(mux))
	t.Cleanup(srv.Close)

	for _, path := range []string{"/health", "/foo"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1 (only /foo); names=%v", len(spans), spanNames(spans))
	}
}

func TestHTTPMiddleware_customFilterDisablesTracing(t *testing.T) {
	rec := installRecordingTracer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	traceNothing := WithRequestFilter(func(*http.Request) bool { return false })
	srv := httptest.NewServer(HTTPMiddleware("test", traceNothing)(mux))
	t.Cleanup(srv.Close)

	for i := 0; i < 3; i++ {
		resp, err := http.Get(srv.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		_ = resp.Body.Close()
	}

	if got := len(rec.Ended()); got != 0 {
		t.Errorf("got %d spans, want 0; custom filter should disable tracing", got)
	}
}

func TestHTTPMiddleware_postHealthIsTraced(t *testing.T) {
	// The default filter only skips GET /health — POST /health (or any
	// other method) should still be traced.
	rec := installRecordingTracer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := httptest.NewServer(HTTPMiddleware("test")(mux))
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/health", "application/json", http.NoBody)
	if err != nil {
		t.Fatalf("POST /health: %v", err)
	}
	_ = resp.Body.Close()

	if got := len(rec.Ended()); got != 1 {
		t.Errorf("got %d spans, want 1 (POST /health is not skipped)", got)
	}
}

func spanNames(spans []sdktrace.ReadOnlySpan) []string {
	out := make([]string, 0, len(spans))
	for _, s := range spans {
		out = append(out, s.Name())
	}
	return out
}
