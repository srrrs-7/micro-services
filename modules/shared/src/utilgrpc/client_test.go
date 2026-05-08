package utilgrpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"
)

func TestWithUnaryInterceptors_accumulatesAcrossCalls(t *testing.T) {
	noop := func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ grpc.UnaryInvoker, _ ...grpc.CallOption) error {
		return nil
	}

	var cfg config
	WithUnaryInterceptors(noop, noop)(&cfg)
	WithUnaryInterceptors(noop)(&cfg)

	if got, want := len(cfg.unaryInterceptors), 3; got != want {
		t.Errorf("len(unaryInterceptors) = %d, want %d", got, want)
	}
}

func TestWithStreamInterceptors_accumulates(t *testing.T) {
	noop := func(_ context.Context, _ *grpc.StreamDesc, _ *grpc.ClientConn, _ string, _ grpc.Streamer, _ ...grpc.CallOption) (grpc.ClientStream, error) {
		return nil, nil
	}

	var cfg config
	WithStreamInterceptors(noop)(&cfg)
	WithStreamInterceptors(noop, noop)(&cfg)

	if got, want := len(cfg.streamInterceptors), 3; got != want {
		t.Errorf("len(streamInterceptors) = %d, want %d", got, want)
	}
}

func TestWithDialOption_accumulates(t *testing.T) {
	var cfg config
	WithDialOption(grpc.WithUserAgent("ua-1"))(&cfg)
	WithDialOption(grpc.WithUserAgent("ua-2"), grpc.WithDisableRetry())(&cfg)

	if got, want := len(cfg.extra), 3; got != want {
		t.Errorf("len(extra) = %d, want %d", got, want)
	}
}

func TestDial_returnsConnAndDoesNotErrorWithNoOptions(t *testing.T) {
	conn, err := Dial("localhost:0")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if conn == nil {
		t.Fatal("Dial returned nil ClientConn")
	}
	if got, want := conn.Target(), "localhost:0"; got != want {
		t.Errorf("conn.Target() = %q, want %q", got, want)
	}
}

func TestDial_acceptsAllOptionTypesTogether(t *testing.T) {
	unary := func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ grpc.UnaryInvoker, _ ...grpc.CallOption) error {
		return nil
	}
	stream := func(_ context.Context, _ *grpc.StreamDesc, _ *grpc.ClientConn, _ string, _ grpc.Streamer, _ ...grpc.CallOption) (grpc.ClientStream, error) {
		return nil, nil
	}

	conn, err := Dial("localhost:0",
		WithUnaryInterceptors(unary),
		WithStreamInterceptors(stream),
		WithDialOption(grpc.WithUserAgent("test")),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
}
