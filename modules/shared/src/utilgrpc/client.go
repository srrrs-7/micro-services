// Package utilgrpc collects the small set of gRPC client primitives shared
// across services. Per-service wire-level contracts (proto + generated Go)
// live in modules/<svc>/src/route/grpc/; this package supplies the
// connection plumbing every consumer would otherwise re-derive from
// grpc-go's defaults.
//
// Transport security: Dial uses plaintext (insecure.NewCredentials). mTLS
// between services is provided by the Istio Ambient data plane (ztunnel
// HBONE) at the platform layer — see deploy/k8s/istio.md. App-level TLS is
// intentionally not exposed here.
package utilgrpc

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// plaintextTransport is the only transport-credential value Dial ever
// applies. Lifted to a package-level var so the "always plaintext" invariant
// is grep-visible at one place rather than buried inside Dial. mTLS is the
// mesh's responsibility (see package doc).
var plaintextTransport = grpc.WithTransportCredentials(insecure.NewCredentials())

// Option configures a Dial call. Construct values via the WithXxx helpers
// below — the underlying config is intentionally unexported so callers cannot
// reach into it and break the invariants Dial relies on.
type Option func(*config)

type config struct {
	unaryInterceptors  []grpc.UnaryClientInterceptor
	streamInterceptors []grpc.StreamClientInterceptor
	extra              []grpc.DialOption
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

// Dial constructs a gRPC client connection at addr. Transport is plaintext;
// in the Kubernetes deployment, Istio Ambient (ztunnel) wraps inter-pod
// traffic with mTLS at the network layer.
//
// The returned ClientConn is lazy — grpc.NewClient does not block on TCP
// handshake — so callers that want fast-fail behavior must add an explicit
// connect deadline (e.g. WaitForReady on the call, or conn.WaitForStateChange).
func Dial(addr string, opts ...Option) (*grpc.ClientConn, error) {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	dialOpts := []grpc.DialOption{plaintextTransport}
	if len(cfg.unaryInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(cfg.unaryInterceptors...))
	}
	if len(cfg.streamInterceptors) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainStreamInterceptor(cfg.streamInterceptors...))
	}
	dialOpts = append(dialOpts, cfg.extra...)

	return grpc.NewClient(addr, dialOpts...)
}
