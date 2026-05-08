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

	for range 3 {
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

func TestHTTPMiddleware_spanNameUsesRequestPattern(t *testing.T) {
	// stdlib ServeMux 1.22+ populates http.Request.Pattern for templated
	// routes. The middleware's spanNameFormatter should pick that up and
	// produce "<METHOD> <pattern>" instead of falling back to serverName.
	rec := installRecordingTracer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := httptest.NewServer(HTTPMiddleware("ignored-server-name")(mux))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/users/42")
	if err != nil {
		t.Fatalf("GET /users/42: %v", err)
	}
	_ = resp.Body.Close()

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if got, want := spans[0].Name(), "GET /users/{id}"; got != want {
		t.Errorf("span name = %q, want %q", got, want)
	}
}

// TestSpanNameFromPattern exercises the three branches of the
// SpanNameFormatter directly — the through-otelhttp test path can only
// drive the stdlib ServeMux behavior (which always populates r.Pattern),
// so the empty-pattern fallback and the chi-style path-only form need
// targeted coverage at the function level.
func TestSpanNameFromPattern(t *testing.T) {
	cases := []struct {
		name      string
		method    string
		pattern   string
		operation string
		want      string
	}{
		{
			name:      "empty pattern falls back to operation",
			method:    http.MethodGet,
			pattern:   "",
			operation: "test-server",
			want:      "test-server",
		},
		{
			name:      "stdlib ServeMux method-prefixed pattern passes through",
			method:    http.MethodGet,
			pattern:   "GET /users/{id}",
			operation: "ignored",
			want:      "GET /users/{id}",
		},
		{
			name:      "chi-style path-only pattern gets method prepended",
			method:    http.MethodGet,
			pattern:   "/users/{id}",
			operation: "ignored",
			want:      "GET /users/{id}",
		},
		{
			name:      "POST chi-style path-only pattern",
			method:    http.MethodPost,
			pattern:   "/users",
			operation: "ignored",
			want:      "POST /users",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &http.Request{Method: tc.method, Pattern: tc.pattern}
			if got := spanNameFromPattern(tc.operation, r); got != tc.want {
				t.Errorf("spanNameFromPattern(%q, &Request{Method:%q, Pattern:%q}) = %q, want %q",
					tc.operation, tc.method, tc.pattern, got, tc.want)
			}
		})
	}
}

func spanNames(spans []sdktrace.ReadOnlySpan) []string {
	out := make([]string, 0, len(spans))
	for _, s := range spans {
		out = append(out, s.Name())
	}
	return out
}
