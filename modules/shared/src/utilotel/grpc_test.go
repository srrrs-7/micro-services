package utilotel

import (
	"testing"

	"shared/utilgrpc"
)

func TestGRPCServerOption_isNonNil(t *testing.T) {
	if GRPCServerOption() == nil {
		t.Fatal("GRPCServerOption() = nil")
	}
}

func TestGRPCClientOption_composesWithDial(t *testing.T) {
	// The client option must satisfy utilgrpc.Option and be acceptable to
	// utilgrpc.Dial. The dial target need not resolve — grpc.NewClient is
	// lazy, so this test does not require a running server.
	conn, err := utilgrpc.Dial("localhost:0", GRPCClientOption())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if conn == nil {
		t.Fatal("Dial returned nil ClientConn")
	}
}
