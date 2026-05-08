package utilotel

import (
	"net/http"

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

// HTTPMiddleware returns a chi-compatible middleware that wraps each request
// in an OTel server span and emits the standard http.server.* metrics.
//
// serverName is the operation name otelhttp uses for spans whose route is
// not otherwise resolved. chi callers should additionally apply a route-
// pattern retag middleware so spans carry the templated path (chi resolves
// the pattern *during* ServeHTTP, so the retag must run after the inner
// handler).
func HTTPMiddleware(serverName string, opts ...HTTPOption) func(http.Handler) http.Handler {
	cfg := httpConfig{shouldTrace: defaultShouldTrace}
	for _, opt := range opts {
		opt(&cfg)
	}

	return otelhttp.NewMiddleware(serverName,
		otelhttp.WithFilter(cfg.shouldTrace),
	)
}

func defaultShouldTrace(r *http.Request) bool {
	return r.Method != http.MethodGet || r.URL.Path != "/health"
}
