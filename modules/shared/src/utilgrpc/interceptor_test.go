package utilgrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// captureSlog redirects the default slog logger to an in-memory JSON
// handler for the duration of the test, returning the buffer for assertions.
// Restoration is registered via t.Cleanup so tests stay parallel-safe within
// the same package even though they each manipulate the global default.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// newTestConn returns a real, never-dialed *grpc.ClientConn. grpc.NewClient is
// lazy — it does not perform a TCP handshake — so this is cheap and safe to
// pass into the LoggingInterceptor's cc parameter to drive cc.Target().
func newTestConn(t *testing.T, target string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestLoggingInterceptor_invokesUnderlyingExactlyOnceWithSameArgs(t *testing.T) {
	captureSlog(t)
	conn := newTestConn(t, "localhost:0")

	var calls int
	var gotMethod string
	invoker := func(_ context.Context, method string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		calls++
		gotMethod = method
		return nil
	}

	err := LoggingInterceptor()(context.Background(), "/svc/M", "req", "reply", conn, invoker)
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
	if calls != 1 {
		t.Errorf("invoker calls = %d, want 1", calls)
	}
	if gotMethod != "/svc/M" {
		t.Errorf("invoker method = %q, want %q", gotMethod, "/svc/M")
	}
}

func TestLoggingInterceptor_propagatesInvokerError(t *testing.T) {
	captureSlog(t)
	conn := newTestConn(t, "localhost:0")

	want := errors.New("boom")
	invoker := func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return want
	}

	got := LoggingInterceptor()(context.Background(), "/svc/M", nil, nil, conn, invoker)
	if !errors.Is(got, want) {
		t.Errorf("returned err = %v, want %v", got, want)
	}
}

func TestLoggingInterceptor_logsExpectedFieldsOnSuccess(t *testing.T) {
	buf := captureSlog(t)
	conn := newTestConn(t, "passthrough:///audit-api:8080")

	invoker := func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return nil
	}

	if err := LoggingInterceptor()(context.Background(), "/audit.v1.Audit/Ingest", nil, nil, conn, invoker); err != nil {
		t.Fatalf("interceptor: %v", err)
	}

	got := decodeOnly(t, buf)
	if got["msg"] != "grpc call" {
		t.Errorf("msg = %v, want %q", got["msg"], "grpc call")
	}
	if got["direction"] != "out" {
		t.Errorf("direction = %v, want %q", got["direction"], "out")
	}
	if got["method"] != "/audit.v1.Audit/Ingest" {
		t.Errorf("method = %v, want %q", got["method"], "/audit.v1.Audit/Ingest")
	}
	if got["target"] != "passthrough:///audit-api:8080" {
		t.Errorf("target = %v, want %q", got["target"], "passthrough:///audit-api:8080")
	}
	if got["code"] != codes.OK.String() {
		t.Errorf("code = %v, want %q", got["code"], codes.OK.String())
	}
	if _, ok := got["duration_ms"]; !ok {
		t.Error("duration_ms field missing")
	}
}

func TestLoggingInterceptor_logsStatusCodeOnError(t *testing.T) {
	buf := captureSlog(t)
	conn := newTestConn(t, "localhost:0")

	invoker := func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		return status.Error(codes.PermissionDenied, "nope")
	}

	_ = LoggingInterceptor()(context.Background(), "/svc/M", nil, nil, conn, invoker)

	got := decodeOnly(t, buf)
	if got["code"] != codes.PermissionDenied.String() {
		t.Errorf("code = %v, want %q", got["code"], codes.PermissionDenied.String())
	}
}

// decodeOnly parses the single JSON object the test wrote to buf. If the
// interceptor ever logs more than one line per call, this will fail loudly
// rather than silently picking the first.
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
