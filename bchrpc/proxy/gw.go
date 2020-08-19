package main

import (
	"flag"
	"net/http"

	"github.com/golang/glog"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	gw "github.com/gcash/bchd/bchrpc/proxy/gen"
)

var (
	// command-line options:
	// gRPC server endpoint
	grpcServerEndpoint = flag.String("bchd-grpc-url", "localhost:8335", "BCHD gRPC server endpoint")
	grpcRootCertPath   = flag.String("bchd-grpc-certpath", "", "BCHD gRPC server self-signed root certificate path")
	proxyPort          = flag.String("port", "8080", "port for the proxy server")
)

func run() error {
	var err error
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register gRPC server endpoint
	// Note: Make sure the gRPC server is running properly and accessible
	mux := runtime.NewServeMux()
	var creds credentials.TransportCredentials
	if *grpcRootCertPath != "" {
		creds, err = credentials.NewClientTLSFromFile(*grpcRootCertPath, "")
		if err != nil {
			glog.Fatal(err)
		}
	} else {
		creds = credentials.NewTLS(nil)
	}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds), grpc.WithMaxMsgSize(4294967295)}
	err = gw.RegisterBchrpcHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
	if err != nil {
		return err
	}

	// TODO: consider serving static files for swagger-ui

	// Start HTTP server (and proxy calls to gRPC server endpoint)
	return http.ListenAndServe(":"+*proxyPort, mux)
}

func main() {
	flag.Parse()
	defer glog.Flush()

	if err := run(); err != nil {
		glog.Fatal(err)
	}
}
