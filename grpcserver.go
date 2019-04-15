package main

import (
	"context"
	"errors"
	"github.com/gcash/bchd/bchrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"net"
	"strings"
)

// AuthenticationTokenKey is the key used in the context to authenticate clients.
// If this is set to anything other than "" in the config, then the server expects
// the client to set a key value in the context metadata to 'AuthenticationToken: cfg.AuthToken'
const AuthenticationTokenKey = "AuthenticationToken"

func newGrpcServer(listeners []net.Listener, rpcCfg *bchrpc.GrpcServerConfig, svr *server) (*bchrpc.GrpcServer, error) {
	if len(listeners) != 0 {
		rpcCfg.NetMgr = svr
		opts := []grpc.ServerOption{grpc.StreamInterceptor(interceptStreaming), grpc.UnaryInterceptor(interceptUnary)}
		if !cfg.DisableTLS {
			creds, err := credentials.NewServerTLSFromFile(cfg.RPCCert, cfg.RPCKey)
			if err != nil {
				return nil, err
			}
			opts = append(opts, grpc.Creds(creds))
		}
		server := grpc.NewServer(opts...)

		rpcCfg.Server = server
		rpcCfg.Listeners = listeners
		gRPCServer := bchrpc.NewGrpcServer(rpcCfg)

		for _, lis := range listeners {
			lis := lis
			go func() {
				grpcLog.Infof("Experimental gRPC server listening on %s",
					lis.Addr())
				err := server.Serve(lis)
				grpcLog.Tracef("Finished serving expimental gRPC: %v",
					err)
			}()
		}
		return gRPCServer, nil

	}
	return nil, nil
}

// serviceName returns the package.service segment from the full gRPC method
// name `/package.service/method`.
func serviceName(method string) string {
	// Slice off first /
	method = method[1:]
	// Keep everything before the next /
	return method[:strings.IndexRune(method, '/')]
}

func interceptStreaming(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	p, ok := peer.FromContext(ss.Context())
	if ok {
		grpcLog.Infof("Streaming method %s invoked by %s", info.FullMethod,
			p.Addr.String())
	}

	err := validateAuthenticationToken(ss.Context())
	if err != nil {
		return err
	}

	err = bchrpc.ServiceReady(serviceName(info.FullMethod))
	if err != nil {
		return err
	}
	err = handler(srv, ss)
	if err != nil && ok {
		grpcLog.Errorf("Streaming method %s invoked by %s errored: %v",
			info.FullMethod, p.Addr.String(), err)
	}
	return err
}

func interceptUnary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	p, ok := peer.FromContext(ctx)
	if ok {
		grpcLog.Infof("Unary method %s invoked by %s", info.FullMethod,
			p.Addr.String())
	}

	err = validateAuthenticationToken(ctx)
	if err != nil {
		return nil, err
	}

	err = bchrpc.ServiceReady(serviceName(info.FullMethod))
	if err != nil {
		return nil, err
	}
	resp, err = handler(ctx, req)
	if err != nil && ok {
		grpcLog.Errorf("Unary method %s invoked by %s errored: %v",
			info.FullMethod, p.Addr.String(), err)
	}
	return resp, err
}

func validateAuthenticationToken(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if cfg.GrpcAuthToken != "" && (!ok || len(md.Get(AuthenticationTokenKey)) == 0 || md.Get(AuthenticationTokenKey)[0] != cfg.GrpcAuthToken) {
		return errors.New("invalid authentication token")
	}
	return nil
}
