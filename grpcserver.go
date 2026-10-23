package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gcash/bchd/bchrpc"
	"github.com/gorilla/mux"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// AuthenticationTokenKey is the key used in the context to authenticate clients.
// If this is set to anything other than "" in the config, then the server expects
// the client to set a key value in the context metadata to 'AuthenticationToken: cfg.AuthToken'
const AuthenticationTokenKey = "AuthenticationToken"

var prometheusEnabled = false

// grpcReadHeaderTimeout and grpcIdleTimeout bound how long the gRPC HTTP
// server waits for request headers and keeps idle connections.  There are no
// whole-request read or write timeouts because streams are long-lived.
var (
	grpcReadHeaderTimeout = 10 * time.Second
	grpcIdleTimeout       = 5 * time.Minute
)

// grpcWriteTimeout is how long a gRPC client may take none of a reply being
// written to it.  A stream whose client takes nothing for longer is reset, and
// a connection to which nothing can be written for longer is closed, so a
// client that stops reading cannot hold a call's goroutines and the reply for
// good, as BCHN's -rpcservertimeout does for its HTTP clients.  It only runs
// while a reply is being written, so a subscription with nothing to send is
// kept however long it waits.
var grpcWriteTimeout = 30 * time.Second

// grpcWritePartSize is how much of a reply is written to a gRPC stream each
// time its write deadline is renewed: a client must take this much in each
// grpcWriteTimeout.
const grpcWritePartSize = 64 * 1024

func newGrpcServer(netAddrs []net.Addr, rpcCfg *bchrpc.GrpcServerConfig, svr *server) (*bchrpc.GrpcServer, error) {
	if len(netAddrs) == 0 {
		return nil, nil
	}
	// Only the first configured listen address is wired up; the function
	// returns a single server instance.
	addr := netAddrs[0]
	rpcCfg.NetMgr = svr
	opts := grpcServerOptions(newGrpcCallLimiter(cfg.GrpcMaxConcurrentCalls))
	creds, err := credentials.NewServerTLSFromFile(cfg.RPCCert, cfg.RPCKey)
	if err != nil {
		return nil, err
	}
	opts = append(opts, grpc.Creds(creds))
	server := grpc.NewServer(opts...)

	rpcCfg.Server = server

	httpServer := newGrpcHTTPServer(addr.String(), grpcHTTPHandler(server))

	rpcCfg.HTTPServer = httpServer

	gRPCServer := bchrpc.NewGrpcServer(rpcCfg)

	if len(cfg.PrometheusListen) != 0 {
		// init Prometheus metrics
		grpc_prometheus.EnableHandlingTimeHistogram()
		grpc_prometheus.Register(server)

		router := mux.NewRouter()
		router.Handle("/metrics", promhttp.Handler())

		prometheusHTTPServer := &http.Server{
			Addr:         cfg.PrometheusListen,
			Handler:      router,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		prometheusEnabled = true

		go func() {
			if err := prometheusHTTPServer.ListenAndServeTLS(cfg.RPCCert, cfg.RPCKey); err != nil {
				grpcLog.Tracef("Finished serving Prometheus metrics %v", err)
			}
		}()
	}

	// Serve only after metrics are set up, since the interceptors read
	// prometheusEnabled.
	grpcLog.Infof("Experimental gRPC server listening on %s", addr)
	go func() {
		if err := httpServer.ListenAndServeTLS(cfg.RPCCert, cfg.RPCKey); err != nil {
			grpcLog.Tracef("Finished serving expimental gRPC: %v", err)
		}
	}()

	return gRPCServer, nil
}

// grpcServerOptions returns the gRPC server's interceptors, with calls
// bounding the unary calls once they are authorized.
func grpcServerOptions(calls *grpcCallLimiter) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StreamInterceptor(interceptStreaming),
		grpc.ChainUnaryInterceptor(interceptUnary, calls.intercept),
	}
}

// grpcCallQueueFactor is how many unary gRPC calls may wait for a place, as a
// multiple of how many may be in progress.
const grpcCallQueueFactor = 4

// grpcCallLimiter bounds the unary gRPC calls in progress, from the time a
// call's request has been read and its caller authorized until its handler has
// made the reply.  Any client may call when no auth token is set, and a call
// can make the node read a block and build a reply larger still.  A call over
// the limit waits for a place, while fewer than grpcCallQueueFactor times the
// limit wait, and is refused with ResourceExhausted otherwise, as BCHN's HTTP
// server runs 16 requests at once and queues 64 more.  The reply is sent
// without a place, as BCHN's worker threads hand theirs to the connection, so
// a client that takes a reply slowly does not keep other calls waiting, and
// the write deadline ends one that takes none of it.  Subscriptions have their
// own limit.
type grpcCallLimiter struct {
	places     chan struct{} // a value for each call in progress
	waiting    atomic.Int64  // calls waiting for a place
	maxWaiting int64
}

// newGrpcCallLimiter returns a limiter of max unary calls in progress.
func newGrpcCallLimiter(max int) *grpcCallLimiter {
	return &grpcCallLimiter{
		places:     make(chan struct{}, max),
		maxWaiting: grpcCallQueueFactor * int64(max),
	}
}

// intercept runs a unary call once it holds a place, waiting for one until
// its context ends.  The place is freed when the handler returns or panics,
// before the reply is sent.
func (l *grpcCallLimiter) intercept(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	select {
	case l.places <- struct{}{}:
	default:
		if l.waiting.Add(1) > l.maxWaiting {
			l.waiting.Add(-1)
			return nil, status.Errorf(codes.ResourceExhausted,
				"too many calls, the limit is %d", cap(l.places))
		}
		select {
		case l.places <- struct{}{}:
			l.waiting.Add(-1)
		case <-ctx.Done():
			l.waiting.Add(-1)
			return nil, status.FromContextError(ctx.Err()).Err()
		}
	}
	defer func() { <-l.places }()
	return handler(ctx, req)
}

// grpcHTTPHandler returns the handler of the gRPC listener, which passes
// gRPC-Web requests and their CORS preflights to the gRPC-Web wrapper of
// server and the others to server, and writes the replies of both with write
// deadlines.
func grpcHTTPHandler(server *grpc.Server) http.Handler {
	allowAllOrigins := grpcweb.WithOriginFunc(func(origin string) bool {
		return true
	})
	wrappedGrpc := grpcweb.WrapServer(server, allowAllOrigins)

	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp = &grpcDeadlineWriter{ResponseWriter: resp,
			rc: http.NewResponseController(resp)}
		if wrappedGrpc.IsGrpcWebRequest(req) || wrappedGrpc.IsAcceptableGrpcCorsRequest(req) {
			wrappedGrpc.ServeHTTP(resp, req)
		} else {
			server.ServeHTTP(resp, req)
		}
	})
}

// grpcDeadlineWriter is the ResponseWriter of a gRPC request.  It renews the
// write deadline of the request's stream before each grpcWritePartSize bytes
// it writes and before it flushes, and clears it once the write returns, so a
// stream is reset only when its client takes none of a part for
// grpcWriteTimeout, however large the reply and however long the stream is
// idle between replies.
type grpcDeadlineWriter struct {
	http.ResponseWriter
	rc *http.ResponseController
}

// Write writes p in parts of grpcWritePartSize, renewing the write deadline
// before each.
func (w *grpcDeadlineWriter) Write(p []byte) (int, error) {
	n := 0
	for {
		part := p[n:min(len(p), n+grpcWritePartSize)]
		_ = w.rc.SetWriteDeadline(time.Now().Add(grpcWriteTimeout))
		m, err := w.ResponseWriter.Write(part)
		n += m
		if err != nil || n == len(p) {
			_ = w.rc.SetWriteDeadline(time.Time{})
			return n, err
		}
	}
}

// Flush sends what has been written, under the write deadline.  gRPC requires
// the writer to be an http.Flusher.
func (w *grpcDeadlineWriter) Flush() {
	_ = w.rc.SetWriteDeadline(time.Now().Add(grpcWriteTimeout))
	_ = w.rc.Flush()
	_ = w.rc.SetWriteDeadline(time.Time{})
}

// newGrpcHTTPServer returns the HTTP server for the gRPC listener.  An HTTP/2
// connection to which nothing can be written for grpcWriteTimeout is closed,
// as a client that does not read its socket stalls every stream on it, which
// the deadline of each stream cannot end.
func newGrpcHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: grpcReadHeaderTimeout,
		IdleTimeout:       grpcIdleTimeout,
		HTTP2: &http.HTTP2Config{
			WriteByteTimeout: grpcWriteTimeout,
		},
	}
}

// serviceName returns the package.service segment from the full gRPC method
// name `/package.service/method`.
func serviceName(method string) string {
	// Slice off first /
	method = method[1:]
	// Keep everything before the next /
	return method[:strings.IndexRune(method, '/')]
}

// interceptStreaming runs a streaming call through panic recovery, then
// authentication and the service readiness check, then metrics.  Metrics come
// last so that the handler never runs for an unauthenticated caller.
func interceptStreaming(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer recoverGrpcPanic(info.FullMethod, &err)

	// This line is logged for every call, before the caller is
	// authenticated, so it is at debug.
	p, ok := peer.FromContext(ss.Context())
	if ok {
		grpcLog.Debugf("Streaming method %s invoked by %s", info.FullMethod,
			p.Addr.String())
	}

	if err := authorizeGrpcCall(ss.Context(), info.FullMethod); err != nil {
		return err
	}

	if prometheusEnabled {
		err = grpc_prometheus.StreamServerInterceptor(srv, ss, info, handler)
	} else {
		err = handler(srv, ss)
	}
	if err != nil && ok {
		logGrpcError("Streaming", info.FullMethod, p.Addr.String(), err)
	}
	return err
}

// interceptUnary runs a unary call through panic recovery, then
// authentication and the service readiness check, then metrics.  Metrics come
// last so that the handler never runs for an unauthenticated caller.
func interceptUnary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer recoverGrpcPanic(info.FullMethod, &err)

	// This line is logged for every call, before the caller is
	// authenticated, so it is at debug.
	p, ok := peer.FromContext(ctx)
	if ok {
		grpcLog.Debugf("Unary method %s invoked by %s", info.FullMethod,
			p.Addr.String())
	}

	if err := authorizeGrpcCall(ctx, info.FullMethod); err != nil {
		return nil, err
	}

	if prometheusEnabled {
		resp, err = grpc_prometheus.UnaryServerInterceptor(ctx, req, info, handler)
	} else {
		resp, err = handler(ctx, req)
	}
	if err != nil && ok {
		logGrpcError("Unary", info.FullMethod, p.Addr.String(), err)
	}
	return resp, err
}

// logGrpcError logs the error a call failed with.  An error that answers the
// caller, such as an unknown transaction, a bad argument, an SLP transaction
// refused with Aborted, an index the node does not run, a limit such as on
// subscriptions or proof searches, the end of a subscription it canceled or of
// the server, or its deadline passing, is logged at debug, so that a client
// polling for something cannot flood the log with errors.
func logGrpcError(kind, method, addr string, err error) {
	format := "%s method %s invoked by %s errored: %v"
	switch status.Code(err) {
	case codes.NotFound, codes.InvalidArgument, codes.OutOfRange,
		codes.AlreadyExists, codes.FailedPrecondition, codes.Canceled,
		codes.Aborted, codes.Unavailable, codes.ResourceExhausted,
		codes.DeadlineExceeded:

		grpcLog.Debugf(format, kind, method, addr, err)
		return
	}
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) {

		grpcLog.Debugf(format, kind, method, addr, err)
		return
	}
	grpcLog.Errorf(format, kind, method, addr, err)
}

// recoverGrpcPanic turns a panic in a gRPC handler into an Internal error, so
// a single call can't stop the process.  It must be deferred.
func recoverGrpcPanic(method string, err *error) {
	if r := recover(); r != nil {
		grpcLog.Errorf("Recovered from panic in gRPC method %s: %v\n%s",
			method, r, debug.Stack())
		*err = status.Error(codes.Internal, "internal error")
	}
}

// authorizeGrpcCall checks the auth token and that the called service is
// ready.
func authorizeGrpcCall(ctx context.Context, method string) error {
	if err := validateAuthenticationToken(ctx); err != nil {
		return err
	}
	return bchrpc.ServiceReady(serviceName(method))
}

func validateAuthenticationToken(ctx context.Context) error {
	if cfg.GrpcAuthToken == "" {
		return nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("invalid authentication token")
	}
	tokens := md.Get(AuthenticationTokenKey)
	if len(tokens) == 0 {
		return errors.New("invalid authentication token")
	}
	// Compare in constant time, over hashes so the lengths always match.
	got := sha256.Sum256([]byte(tokens[0]))
	want := sha256.Sum256([]byte(cfg.GrpcAuthToken))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		return errors.New("invalid authentication token")
	}
	return nil
}
