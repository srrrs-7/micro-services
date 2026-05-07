package route

import (
	auditgrpc "audit/route/grpc"
)

// handler is the AuditServer implementation. UnimplementedAuditServer must be
// embedded by value (per protoc-gen-go-grpc forward-compat contract) so future
// RPCs added to the proto fall back to codes.Unimplemented until handlers land.
type handler struct {
	auditgrpc.UnimplementedAuditServer
}

func NewHandler() handler {
	return handler{}
}
