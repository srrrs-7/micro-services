package utilgrpc

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// fakeCreds is an interface-satisfying TransportCredentials whose only
// purpose is to act as a sentinel value for option-application tests. The
// embedded interface fills out the method set with nils we never call —
// these tests never trigger a real handshake.
type fakeCreds struct {
	credentials.TransportCredentials
	id string
}

func TestWithTLS_replacesDefaultTransportCreds(t *testing.T) {
	want := &fakeCreds{id: "tls-1"}
	cfg := config{transportCreds: insecure.NewCredentials()}

	WithTLS(want)(&cfg)

	got, ok := cfg.transportCreds.(*fakeCreds)
	if !ok {
		t.Fatalf("transportCreds = %T, want *fakeCreds", cfg.transportCreds)
	}
	if got != want {
		t.Errorf("transportCreds identity mismatch: got=%p want=%p", got, want)
	}
}

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
		WithTLS(insecure.NewCredentials()),
		WithUnaryInterceptors(unary),
		WithStreamInterceptors(stream),
		WithDialOption(grpc.WithUserAgent("test")),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
}

func TestDial_appliesOptionsLeftToRight(t *testing.T) {
	first := &fakeCreds{id: "first"}
	second := &fakeCreds{id: "second"}

	cfg := config{transportCreds: insecure.NewCredentials()}
	for _, opt := range []Option{WithTLS(first), WithTLS(second)} {
		opt(&cfg)
	}

	got, ok := cfg.transportCreds.(*fakeCreds)
	if !ok {
		t.Fatalf("transportCreds = %T, want *fakeCreds", cfg.transportCreds)
	}
	if got.id != "second" {
		t.Errorf("last WithTLS did not win: got id=%q", got.id)
	}
}
