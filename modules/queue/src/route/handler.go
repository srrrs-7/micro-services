package route

import (
	queuegrpc "queue/route/grpc"
)

// handler is the QueueServer implementation. UnimplementedQueueServer must be
// embedded by value (per protoc-gen-go-grpc forward-compat contract) so future
// RPCs added to the proto fall back to codes.Unimplemented until handlers land.
type handler struct {
	queuegrpc.UnimplementedQueueServer
}

func NewHandler() handler {
	return handler{}
}
