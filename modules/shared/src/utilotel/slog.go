package utilotel

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// teeHandler fans out slog records to multiple handlers. Used by Init to
// keep stdout JSON logging visible in dev while also pushing every record
// through the OTel log SDK to the Collector → Loki.
//
// The slog package does not ship a multi-handler primitive, hence this
// 30-line shim. Records are cloned per handler so handler.WithAttrs /
// WithGroup mutations on the underlying handlers do not leak across.
type teeHandler struct {
	handlers []slog.Handler
}

func newTeeHandler(handlers ...slog.Handler) slog.Handler {
	return &teeHandler{handlers: handlers}
}

func (t *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range t.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (t *teeHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range t.handlers {
		// r.Clone is required because some handlers mutate the record
		// (e.g., adding processor-supplied attrs) before formatting.
		if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (t *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		out[i] = h.WithAttrs(attrs)
	}
	return &teeHandler{handlers: out}
}

func (t *teeHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(t.handlers))
	for i, h := range t.handlers {
		out[i] = h.WithGroup(name)
	}
	return &teeHandler{handlers: out}
}

// installSlogBridge wraps the current slog default with a tee that fans
// out to the OTel log bridge for serviceName, so every slog.Info /
// slog.Error call also flows to the Collector → Loki. Trace context on
// the slog call site (via ctx) is auto-included by otelslog as
// trace_id / span_id record attributes.
//
// Called by Init after the LoggerProvider is set globally; safe to call
// once Init has succeeded.
func installSlogBridge(serviceName string) {
	current := slog.Default().Handler()
	bridged := otelslog.NewHandler(serviceName)
	slog.SetDefault(slog.New(newTeeHandler(current, bridged)))
}
