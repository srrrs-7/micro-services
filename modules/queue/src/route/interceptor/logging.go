package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Logging returns a UnaryServerInterceptor that emits one slog line per RPC,
// including the gRPC method, status code, and duration. The inner Recovery
// interceptor converts panics into codes.Internal before this layer observes
// the error, so the "code" field stays meaningful.
func Logging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		st, _ := status.FromError(err)
		slog.Info("grpc call",
			"method", info.FullMethod,
			"code", st.Code().String(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return resp, err
	}
}
