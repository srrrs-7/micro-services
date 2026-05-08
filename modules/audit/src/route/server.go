package route

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	auditgrpc "audit/route/grpc"
	"audit/route/interceptor"
	"shared/utilotel"
)

// NewServer constructs the gRPC server with the project's interceptor chain
// (logging outermost, recovery innermost), the Audit service registered, and
// reflection + the standard grpc.health.v1 service enabled. Health and
// reflection are kept on for dev convenience; tightening for prod is tracked
// in docs/system-design.md §14.
func NewServer(h auditgrpc.AuditServer) *grpc.Server {
	s := grpc.NewServer(
		utilotel.GRPCServerOption(),
		grpc.ChainUnaryInterceptor(
			interceptor.Logging(),
			interceptor.Recovery(),
		),
	)

	auditgrpc.RegisterAuditServer(s, h)

	healthpb.RegisterHealthServer(s, health.NewServer())

	reflection.Register(s)

	return s
}
