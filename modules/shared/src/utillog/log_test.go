package utillog

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
)

// captureStdout swaps os.Stdout for a pipe so the test can read what
// NewLogger actually writes. NewLogger pins os.Stdout at construction
// time, so this swap MUST happen before NewLogger is called.
func captureStdout(t *testing.T) (read func() string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	origLogger := slog.Default()
	t.Cleanup(func() {
		os.Stdout = orig
		slog.SetDefault(origLogger)
	})

	return func() string {
		_ = w.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read pipe: %v", err)
		}
		return string(b)
	}
}

func TestNewLogger_installsJSONHandlerAtDebugLevel(t *testing.T) {
	read := captureStdout(t)
	NewLogger()

	slog.Debug("hello", "k", "v")

	var got map[string]any
	if err := json.NewDecoder(strings.NewReader(read())).Decode(&got); err != nil {
		t.Fatalf("decoded log line is not JSON: %v", err)
	}

	if got["level"] != "DEBUG" {
		t.Errorf("level = %v, want DEBUG", got["level"])
	}
	if got["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", got["msg"])
	}
	if got["k"] != "v" {
		t.Errorf("k = %v, want v", got["k"])
	}
	if _, hasTime := got["time"]; !hasTime {
		t.Error("expected time field in JSON output")
	}
}

func TestNewLogger_handlerEnabledForAllStandardLevels(t *testing.T) {
	read := captureStdout(t)
	NewLogger()

	h := slog.Default().Handler()
	for _, lv := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
		if !h.Enabled(context.Background(), lv) {
			t.Errorf("handler.Enabled(%v) = false, want true (debug-level handler)", lv)
		}
	}
	_ = read // not asserting on output here, but the cleanup needs to run
}

func TestNewLogger_overwritesPreviousDefault(t *testing.T) {
	captureStdout(t) // swap stdout & save original default for restore
	prev := slog.New(slog.NewJSONHandler(io.Discard, nil))
	slog.SetDefault(prev)

	NewLogger()

	if slog.Default() == prev {
		t.Error("NewLogger did not replace the default logger")
	}
}
