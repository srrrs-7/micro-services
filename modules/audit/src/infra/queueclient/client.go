// Package queueclient is audit's wrapper for talking to the queue gRPC
// service (audit-worker is the planned consumer per docs/system-design.md
// §4.1 / §8.2). It is the **single** place inside the audit module that may
// import `queue/route/grpc` cross-service — every other audit package must
// reference queue types through this wrapper. See coding-standards.md §2 for
// the contract-surface exemption that makes this import legal.
package queueclient

import (
	"context"

	"google.golang.org/grpc"

	queuegrpc "queue/route/grpc"
	"shared/utilgrpc"
)

// Option re-exports utilgrpc.Option so consumers configure Client without
// importing shared/utilgrpc directly.
type Option = utilgrpc.Option

// Re-export proto-generated types so audit code references them via this
// package only. New fields/messages added to queue.proto regenerate inside
// `queue/route/grpc`; if a caller needs to name a new type, add a matching
// alias here.
type (
	CreateTopicRequest = queuegrpc.CreateTopicRequest
	Topic              = queuegrpc.Topic
	PublishRequest     = queuegrpc.PublishRequest
	PublishResponse    = queuegrpc.PublishResponse
	ConsumeRequest     = queuegrpc.ConsumeRequest
	ConsumeResponse    = queuegrpc.ConsumeResponse
	LeasedMessage      = queuegrpc.LeasedMessage
	AckRequest         = queuegrpc.AckRequest
)

// Client wraps the proto-generated QueueClient with connection lifecycle.
// Construct one per process (or one per target queue-api instance) and
// share it across goroutines — the underlying *grpc.ClientConn multiplexes.
type Client struct {
	conn *grpc.ClientConn
	rpc  queuegrpc.QueueClient
}

// New dials addr via shared/utilgrpc (plaintext default; switch via
// utilgrpc.WithTLS) and returns a ready-to-use Client.
func New(addr string, opts ...Option) (*Client, error) {
	conn, err := utilgrpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, rpc: queuegrpc.NewQueueClient(conn)}, nil
}

// Close releases the underlying ClientConn. Safe to call once per Client.
func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) CreateTopic(ctx context.Context, req *CreateTopicRequest) (*Topic, error) {
	return c.rpc.CreateTopic(ctx, req)
}

func (c *Client) Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
	return c.rpc.Publish(ctx, req)
}

func (c *Client) Consume(ctx context.Context, req *ConsumeRequest) (*ConsumeResponse, error) {
	return c.rpc.Consume(ctx, req)
}

// Ack returns nil on success — the proto rpc returns google.protobuf.Empty
// which carries no payload, so we collapse it to a bare error.
func (c *Client) Ack(ctx context.Context, req *AckRequest) error {
	_, err := c.rpc.Ack(ctx, req)
	return err
}
