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

func newGrpcServer(netAddrs []net.Addr, rpcCfg *bchrpc.GrpcServerConfig, svr *server) (*bchrpc.GrpcServer, error) {
	if len(netAddrs) == 0 {
		return nil, nil
	}
	// Only the first configured listen address is wired up; the function
	// returns a single server instance.
	addr := netAddrs[0]
	rpcCfg.NetMgr = svr
	opts := []grpc.ServerOption{grpc.StreamInterceptor(interceptStreaming), grpc.UnaryInterceptor(interceptUnary)}
	creds, err := credentials.NewServerTLSFromFile(cfg.RPCCert, cfg.RPCKey)
	if err != nil {
		return nil, err
	}
	opts = append(opts, grpc.Creds(creds))
	server := grpc.NewServer(opts...)

	allowAllOrigins := grpcweb.WithOriginFunc(func(origin string) bool {
		return true
	})
	wrappedGrpc := grpcweb.WrapServer(server, allowAllOrigins)

	rpcCfg.Server = server

	handler := func(resp http.ResponseWriter, req *http.Request) {
		if wrappedGrpc.IsGrpcWebRequest(req) || wrappedGrpc.IsAcceptableGrpcCorsRequest(req) {
			wrappedGrpc.ServeHTTP(resp, req)
		} else {
			server.ServeHTTP(resp, req)
		}
	}

	httpServer := newGrpcHTTPServer(addr.String(), http.HandlerFunc(handler))

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

// newGrpcHTTPServer returns the HTTP server for the gRPC listener.
func newGrpcHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: grpcReadHeaderTimeout,
		IdleTimeout:       grpcIdleTimeout,
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

	p, ok := peer.FromContext(ss.Context())
	if ok {
		grpcLog.Infof("Streaming method %s invoked by %s", info.FullMethod,
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
		grpcLog.Errorf("Streaming method %s invoked by %s errored: %v",
			info.FullMethod, p.Addr.String(), err)
	}
	return err
}

// interceptUnary runs a unary call through panic recovery, then
// authentication and the service readiness check, then metrics.  Metrics come
// last so that the handler never runs for an unauthenticated caller.
func interceptUnary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer recoverGrpcPanic(info.FullMethod, &err)

	p, ok := peer.FromContext(ctx)
	if ok {
		grpcLog.Infof("Unary method %s invoked by %s", info.FullMethod,
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
		grpcLog.Errorf("Unary method %s invoked by %s errored: %v",
			info.FullMethod, p.Addr.String(), err)
	}
	return resp, err
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
