package utilotel

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPOption configures HTTPMiddleware.
type HTTPOption func(*httpConfig)

type httpConfig struct {
	// shouldTrace returns true if the request should produce a span +
	// metrics. Returning false short-circuits the middleware to a
	// transparent pass-through.
	shouldTrace func(*http.Request) bool
}

// WithRequestFilter overrides the default trace-eligibility predicate. fn
// returns true for requests that should be traced. The default skips
// GET /health so liveness probes are not represented as spans.
func WithRequestFilter(fn func(*http.Request) bool) HTTPOption {
	return func(c *httpConfig) { c.shouldTrace = fn }
}

// HTTPMiddleware returns an http.Handler middleware that wraps each request
// in an OTel server span and emits the standard http.server.* metrics.
//
// serverName is the operation name otelhttp uses to seed the span at start.
// otelhttp v0.68+ rewrites the span name *after* the handler runs whenever
// http.Request.Pattern is set — chi v5 (and stdlib ServeMux 1.22+) populate
// that field during routing, so we install a SpanNameFormatter that turns
// it into the canonical "<METHOD> <pattern>" shape (e.g. "GET /auth/v1/users/{id}").
// Routers that don't set r.Pattern fall back to serverName so the span name
// stays meaningful.
func HTTPMiddleware(serverName string, opts ...HTTPOption) func(http.Handler) http.Handler {
	cfg := httpConfig{shouldTrace: defaultShouldTrace}
	for _, opt := range opts {
		opt(&cfg)
	}

	return otelhttp.NewMiddleware(serverName,
		otelhttp.WithFilter(cfg.shouldTrace),
		otelhttp.WithSpanNameFormatter(spanNameFromPattern),
	)
}

// spanNameFromPattern is otelhttp's spanNameFormatter callback. otelhttp
// invokes it twice — once at span Start (where r.Pattern is "" because
// routing has not happened yet, so we return operation) and once after
// the inner handler returns (where r.Pattern carries the resolved
// templated path).
//
// Routers differ on whether the method is part of the stored pattern:
//   - stdlib ServeMux 1.22+ stores e.g. "GET /users/{id}" (method-prefixed)
//   - chi v5 stores e.g. "/users/{id}" (path only)
//
// Detect the method-prefixed form and pass it through; otherwise prepend
// r.Method so both routers produce the same canonical "<METHOD> <pattern>"
// shape.
func spanNameFromPattern(operation string, r *http.Request) string {
	if r.Pattern == "" {
		return operation
	}
	if strings.HasPrefix(r.Pattern, r.Method+" ") {
		return r.Pattern
	}
	return r.Method + " " + r.Pattern
}

func defaultShouldTrace(r *http.Request) bool {
	return r.Method != http.MethodGet || r.URL.Path != "/health"
}
