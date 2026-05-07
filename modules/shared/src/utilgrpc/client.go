// Package utilgrpc collects the small set of gRPC client primitives shared
// across services. The wire-level contract (proto + generated Go) lives in
// shared/contract/<svc>/v1/; this package supplies the connection plumbing
// every consumer would otherwise re-derive from grpc-go's defaults.
package utilgrpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Option configures a Dial call. Construct values via the WithXxx helpers
// below — the underlying config is intentionally unexported so callers cannot
// reach into it and break the invariants Dial relies on.
type Option func(*config)

type config struct {
	transportCreds     credentials.TransportCredentials
	unaryInterceptors  []grpc.UnaryClientInterceptor
	streamInterceptors []grpc.StreamClientInterceptor
	extra              []grpc.DialOption
}

// WithTLS overrides the default plaintext transport with the given
// credentials. Use credentials/credentials.NewTLS or NewClientTLSFromCert at
// the call site.
func WithTLS(creds credentials.TransportCredentials) Option {
	return func(c *config) { c.transportCreds = creds }
}

// WithUnaryInterceptors appends client-side unary interceptors. Multiple
// calls accumulate; the resulting slice is wired via
// grpc.WithChainUnaryInterceptor, so earlier interceptors wrap later ones.
func WithUnaryInterceptors(is ...grpc.UnaryClientInterceptor) Option {
	return func(c *config) { c.unaryInterceptors = append(c.unaryInterceptors, is...) }
}

// WithStreamInterceptors is the streaming counterpart of WithUnaryInterceptors.
func WithStreamInterceptors(is ...grpc.StreamClientInterceptor) Option {
	return func(c *config) { c.streamInterceptors = append(c.streamInterceptors, is...) }
}

// WithDialOption is the escape hatch for any grpc.DialOption not yet surfaced
// directly on this package. Prefer the typed Withxxx constructors above when
// they cover the case so the call site stays grpc-agnostic.
func WithDialOption(opts ...grpc.DialOption) Option {
	return func(c *config) { c.extra = append(c.extra, opts...) }
}

// Dial constructs a gRPC client connection at addr with the project's
// default options:
//   - plaintext transport (dev / staging-internal). Override with WithTLS to
//     switch.
//
// The returned ClientConn is lazy — grpc.NewClient does not block on TCP
// handshake — so callers that want fast-fail behavior must add an explicit
// connect deadline (e.g. WaitForReady on the call, or conn.WaitForStateChange).
func Dial(addr string, opts ...Option) (*grpc.ClientConn, error) {
	cfg := config{transportCreds: insecure.NewCredentials()}
	for _, opt := range opts {
		opt(&cfg)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(cfg.transportCreds),
	}
	if len(cfg.unaryInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(cfg.unaryInterceptors...))
	}
	if len(cfg.streamInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainStreamInterceptor(cfg.streamInterceptors...))
	}
	dialOpts = append(dialOpts, cfg.extra...)

	return grpc.NewClient(addr, dialOpts...)
}
