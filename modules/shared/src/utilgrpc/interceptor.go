package utilgrpc

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor returns a UnaryClientInterceptor that emits one slog
// line per outbound RPC, mirroring the server-side interceptor used by
// audit / queue. The "direction=out" field disambiguates from the
// server-side log line of the same call.
//
// Wire it in via grpc.WithChainUnaryInterceptor(utilgrpc.LoggingInterceptor())
// when calling Dial.
func LoggingInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		st, _ := status.FromError(err)
		// slog.InfoContext so the otelslog bridge can attach trace_id /
		// span_id from the active client span on ctx — outbound calls
		// inherit the caller's trace context, so this lets a single
		// trace stitch the producer span, the wire, and the consumer
		// span together in Tempo and have all three logs in Loki land
		// on the same trace_id.
		slog.InfoContext(ctx, "grpc call",
			"direction", "out",
			"method", method,
			"target", cc.Target(),
			"code", st.Code().String(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return err
	}
}
