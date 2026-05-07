package interceptor

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecovery_passesThroughOnNoPanic(t *testing.T) {
	captureSlog(t)
	handler := func(_ context.Context, _ any) (any, error) { return "x", nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/M"}

	resp, err := Recovery()(context.Background(), nil, info, handler)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if resp != "x" {
		t.Errorf("resp = %v, want %q", resp, "x")
	}
}

func TestRecovery_propagatesHandlerError(t *testing.T) {
	captureSlog(t)
	want := errors.New("biz")
	handler := func(_ context.Context, _ any) (any, error) { return nil, want }
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/M"}

	resp, err := Recovery()(context.Background(), nil, info, handler)
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
	if resp != nil {
		t.Errorf("resp = %v, want nil", resp)
	}
}

func TestRecovery_convertsStringPanicToInternalAndLogs(t *testing.T) {
	buf := captureSlog(t)
	handler := func(_ context.Context, _ any) (any, error) { panic("boom") }
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/M"}

	resp, err := Recovery()(context.Background(), nil, info, handler)

	if resp != nil {
		t.Errorf("resp = %v, want nil", resp)
	}
	if err == nil {
		t.Fatal("err = nil, want status.Internal")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("err is not a gRPC status: %v", err)
	}
	if st.Code() != codes.Internal {
		t.Errorf("status code = %v, want %v", st.Code(), codes.Internal)
	}

	got := decodeOnly(t, buf)
	if got["msg"] != "panic recovered" {
		t.Errorf("msg = %v, want %q", got["msg"], "panic recovered")
	}
	if got["panic"] != "boom" {
		t.Errorf("panic = %v, want %q", got["panic"], "boom")
	}
	if got["method"] != "/svc/M" {
		t.Errorf("method = %v, want %q", got["method"], "/svc/M")
	}
	stack, ok := got["stack"].(string)
	if !ok || stack == "" {
		t.Error("stack field missing or not a non-empty string")
	}
}

func TestRecovery_capturesNonStringPanicValue(t *testing.T) {
	buf := captureSlog(t)
	handler := func(_ context.Context, _ any) (any, error) { panic(42) }
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/M"}

	_, err := Recovery()(context.Background(), nil, info, handler)
	if err == nil {
		t.Fatal("err = nil, want status.Internal")
	}

	got := decodeOnly(t, buf)
	// slog encodes int as JSON number; json.Decode unmarshals into float64.
	if got["panic"] != float64(42) {
		t.Errorf("panic = %v (%T), want 42", got["panic"], got["panic"])
	}
}
