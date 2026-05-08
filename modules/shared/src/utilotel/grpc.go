package utilotel

import (
	"shared/utilgrpc"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

// GRPCServerOption returns a grpc.ServerOption that registers the OTel
// stats handler on the server. The stats handler emits one trace span per
// RPC and the standard rpc.server.* metrics — it covers traces and metrics
// in a single hook.
//
// Use alongside the project's existing interceptor chain:
//
//	grpc.NewServer(
//	    utilotel.GRPCServerOption(),
//	    grpc.ChainUnaryInterceptor(interceptor.Logging(), interceptor.Recovery()),
//	)
//
// Health-check noise (grpc.health.v1.Health/Check) is filtered at the
// Collector via the configured filter processor; this option deliberately
// does not install a per-RPC filter.
func GRPCServerOption() grpc.ServerOption {
	return grpc.StatsHandler(otelgrpc.NewServerHandler())
}

// GRPCClientOption returns a utilgrpc.Option that registers the OTel stats
// handler on a client connection. It composes naturally with the existing
// utilgrpc.WithUnaryInterceptors / WithTLS option set:
//
//	conn, err := utilgrpc.Dial(addr,
//	    utilotel.GRPCClientOption(),
//	    utilgrpc.WithUnaryInterceptors(utilgrpc.LoggingInterceptor()),
//	)
func GRPCClientOption() utilgrpc.Option {
	return utilgrpc.WithDialOption(grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
}
