package interceptor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// captureSlog redirects the default slog logger to an in-memory JSON
// handler for the duration of the test, returning the buffer for assertions.
// Restoration is registered via t.Cleanup so the global default is always
// returned to its original value.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// decodeOnly parses exactly one JSON object from buf. If a second decode
// succeeds, the test fails — interceptors here log one line per call and
// silently picking the first would mask regressions.
func decodeOnly(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	dec := json.NewDecoder(buf)
	var first map[string]any
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("decode log line: %v", err)
	}
	var second map[string]any
	if err := dec.Decode(&second); err == nil {
		t.Fatalf("expected exactly one log line, got at least two: %v then %v", first, second)
	}
	return first
}

func TestLogging_invokesHandlerOnceAndReturnsItsResult(t *testing.T) {
	captureSlog(t)

	var calls int
	handler := func(_ context.Context, _ any) (any, error) {
		calls++
		return "ok-resp", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/audit.v1.Audit/Ingest"}

	resp, err := Logging()(context.Background(), "req", info, handler)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if resp != "ok-resp" {
		t.Errorf("resp = %v, want %q", resp, "ok-resp")
	}
	if calls != 1 {
		t.Errorf("handler calls = %d, want 1", calls)
	}
}

func TestLogging_logsExpectedFieldsOnSuccess(t *testing.T) {
	buf := captureSlog(t)
	handler := func(_ context.Context, _ any) (any, error) { return nil, nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/audit.v1.Audit/Ingest"}

	if _, err := Logging()(context.Background(), nil, info, handler); err != nil {
		t.Fatalf("Logging: %v", err)
	}

	got := decodeOnly(t, buf)
	if got["msg"] != "grpc call" {
		t.Errorf("msg = %v, want %q", got["msg"], "grpc call")
	}
	if got["method"] != "/audit.v1.Audit/Ingest" {
		t.Errorf("method = %v, want %q", got["method"], "/audit.v1.Audit/Ingest")
	}
	if got["code"] != codes.OK.String() {
		t.Errorf("code = %v, want %q", got["code"], codes.OK.String())
	}
	if _, ok := got["duration_ms"]; !ok {
		t.Error("duration_ms field missing")
	}
}

func TestLogging_logsStatusCodeAndPropagatesGRPCError(t *testing.T) {
	buf := captureSlog(t)
	want := status.Error(codes.PermissionDenied, "denied")
	handler := func(_ context.Context, _ any) (any, error) { return nil, want }
	info := &grpc.UnaryServerInfo{FullMethod: "/audit.v1.Audit/Ingest"}

	_, err := Logging()(context.Background(), nil, info, handler)
	if !errors.Is(err, want) {
		t.Errorf("propagated err = %v, want %v", err, want)
	}

	got := decodeOnly(t, buf)
	if got["code"] != codes.PermissionDenied.String() {
		t.Errorf("code = %v, want %q", got["code"], codes.PermissionDenied.String())
	}
}

func TestLogging_logsUnknownCodeForPlainError(t *testing.T) {
	buf := captureSlog(t)
	want := errors.New("plain")
	handler := func(_ context.Context, _ any) (any, error) { return nil, want }
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/M"}

	_, err := Logging()(context.Background(), nil, info, handler)
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}

	got := decodeOnly(t, buf)
	if got["code"] != codes.Unknown.String() {
		t.Errorf("code = %v, want %q", got["code"], codes.Unknown.String())
	}
}
