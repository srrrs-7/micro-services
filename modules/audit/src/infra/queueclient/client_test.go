package queueclient

import (
	"testing"

	"google.golang.org/grpc"

	"shared/utilgrpc"
)

func TestNew_returnsClientWithLazyConn(t *testing.T) {
	// grpc.NewClient is lazy — no TCP handshake at Dial time — so an
	// unreachable address is fine for confirming wiring.
	c, err := New("localhost:0")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if c == nil {
		t.Fatal("New returned nil Client")
	}
	if c.conn == nil {
		t.Error("Client.conn = nil; expected a *grpc.ClientConn")
	}
	if c.rpc == nil {
		t.Error("Client.rpc = nil; expected a queuegrpc.QueueClient")
	}
}

func TestNew_acceptsCallerOptions(t *testing.T) {
	// Callers should be able to layer additional utilgrpc.Option values
	// on top of the default OTel-prepended chain. We pass a no-op
	// WithDialOption to confirm the option type alias resolves correctly
	// and Dial accepts the combined slice.
	c, err := New("localhost:0",
		utilgrpc.WithDialOption(grpc.WithUserAgent("audit-worker-test")),
	)
	if err != nil {
		t.Fatalf("New with options: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if c == nil {
		t.Fatal("New returned nil Client")
	}
}

func TestClient_CloseIsSafe(t *testing.T) {
	c, err := New("localhost:0")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
