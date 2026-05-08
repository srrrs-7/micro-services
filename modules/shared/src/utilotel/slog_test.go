package utilotel

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// fanRecorder is a slog.Handler that records every Handle invocation —
// used to verify the tee handler dispatches a record to every member.
type fanRecorder struct {
	calls []string
}

func (f *fanRecorder) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (f *fanRecorder) Handle(_ context.Context, r slog.Record) error {
	f.calls = append(f.calls, r.Message)
	return nil
}
func (f *fanRecorder) WithAttrs(_ []slog.Attr) slog.Handler { return f }
func (f *fanRecorder) WithGroup(_ string) slog.Handler      { return f }

func TestTeeHandler_dispatchesEachRecordToEveryHandler(t *testing.T) {
	a := &fanRecorder{}
	b := &fanRecorder{}

	tee := newTeeHandler(a, b)
	logger := slog.New(tee)

	logger.Info("first")
	logger.Warn("second")

	if got, want := len(a.calls), 2; got != want {
		t.Errorf("handler A calls = %d, want %d", got, want)
	}
	if got, want := len(b.calls), 2; got != want {
		t.Errorf("handler B calls = %d, want %d", got, want)
	}
	if a.calls[0] != "first" || a.calls[1] != "second" {
		t.Errorf("handler A messages = %v", a.calls)
	}
}

func TestTeeHandler_EnabledIfAnyMemberEnabled(t *testing.T) {
	always := &fanRecorder{}
	never := disabledHandler{}

	tee := newTeeHandler(never, always)

	if !tee.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("tee.Enabled = false; want true (always-enabled member should win)")
	}

	teeAllOff := newTeeHandler(disabledHandler{}, disabledHandler{})
	if teeAllOff.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("tee with all-off members reported enabled")
	}
}

func TestTeeHandler_endToEndWithJSONHandler(t *testing.T) {
	// Exercise a realistic stack: stdout-shaped JSON handler + recorder.
	// Confirms a slog.Info call routes to both, with the same structured
	// payload, before bridge-wiring is added on top.
	var buf bytes.Buffer
	jsonHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	rec := &fanRecorder{}

	logger := slog.New(newTeeHandler(jsonHandler, rec))
	logger.Info("ping", "k", "v")

	if !strings.Contains(buf.String(), `"msg":"ping"`) {
		t.Errorf("json output missing msg: %s", buf.String())
	}
	if len(rec.calls) != 1 || rec.calls[0] != "ping" {
		t.Errorf("recorder calls = %v", rec.calls)
	}
}

type disabledHandler struct{}

func (disabledHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (disabledHandler) Handle(context.Context, slog.Record) error { return nil }
func (disabledHandler) WithAttrs([]slog.Attr) slog.Handler        { return disabledHandler{} }
func (disabledHandler) WithGroup(string) slog.Handler             { return disabledHandler{} }
