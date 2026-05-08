package middleware

import (
	"net/http"

	"github.com/go-chi/chi"
	"go.opentelemetry.io/otel/trace"
)

// RouteTag retags the active OTel span with chi's resolved RoutePattern so
// span names carry the templated path (e.g. "POST /auth/v1/users/{id}") in
// place of the raw URL. Must be installed *inside*
// shared/utilotel.HTTPMiddleware — that middleware creates the span; this
// one only renames it after chi finishes routing.
//
// chi populates RouteContext.RoutePattern during the route walk, which
// happens *before* the inner handler runs. Setting the name here picks up
// whatever pattern matched (or stays as the otelhttp default if no route
// matched). The OTel span is still active when this returns, so SetName is
// applied before the parent middleware ends the span.
func RouteTag() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			rc := chi.RouteContext(r.Context())
			if rc == nil {
				return
			}
			pattern := rc.RoutePattern()
			if pattern == "" {
				return
			}
			span := trace.SpanFromContext(r.Context())
			if !span.IsRecording() {
				return
			}
			span.SetName(r.Method + " " + pattern)
		})
	}
}
