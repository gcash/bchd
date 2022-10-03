package main

import (
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gcash/bchutil"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"github.com/gcash/bchd/bchrpc/proxy/middlewares"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	gw "github.com/gcash/bchd/bchrpc/pb"
)

var (
	defaultHomeDir = bchutil.AppDataDir("bchd", false)

	// BCHD backend
	grpcServerEndpoint = flag.String("bchd-grpc-url", "localhost:8335", "BCHD gRPC server endpoint")
	grpcRootCertPath   = flag.String("bchd-grpc-certpath", "", "BCHD gRPC server self-signed root certificate path")

	// HTTP proxy
	proxyAddr = flag.String("http-addr", ":8080", "addr:port for the proxy server - defaults to :8080")
	useHTTPS  = flag.Bool("https", false, "Run the gRPC proxy using HTTPS - default false")
	httpsCert = flag.String("https-cert", filepath.Join(defaultHomeDir, "rpc.cert"), "The certificate to serve HTTPS - defaults to rpc.cert in HOME dir")
	httpsKey  = flag.String("https-key", filepath.Join(defaultHomeDir, "rpc.key"), "The key to serve HTTPS - defaults to rpc.key in HOME dir")
)

// GrpcProxy is the grpc gateway proxy and static webpage server implementation
type GrpcProxy struct {
	ctx    context.Context
	server *http.Server
}

func (proxy *GrpcProxy) serveHTTP(ctx context.Context) error {
	var err error

	// Register gRPC server endpoint
	// Note: Make sure the gRPC server is running properly and accessible
	marshaller := &runtime.JSONPb{}
	marshaller.UseProtoNames = true
	marshaller.EmitUnpopulated = true
	grpcGateway := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, marshaller),
	)
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
	err = gw.RegisterBchrpcHandlerFromEndpoint(ctx, grpcGateway, *grpcServerEndpoint, opts)
	if err != nil {
		return err
	}

	router := mux.NewRouter()

	// serve static files for swagger-ui
	fileServer := http.FileServer(http.Dir("./web/"))
	router.PathPrefix("/").Handler(fileServer).Methods("GET")

	// mount the gRPC router + middlewares on /v1
	grpcGatewayRouter := router.PathPrefix("/v1").Subrouter()
	grpcGatewayRouter.Use(middlewares.NoCacheMiddleware)
	grpcGatewayRouter.Use(middlewares.CorsMiddleware)
	grpcGatewayRouter.NewRoute().HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// limit reading requests to max 50 MB
		r.Body = http.MaxBytesReader(w, r.Body, 50*1024*1024)

		if r.Method == "OPTIONS" { // browsers will do an OPTIONS request first to check CORS headers
			w.WriteHeader(http.StatusNoContent)
		} else {
			grpcGateway.ServeHTTP(w, r) // this calls the gRPC server endpoint
		}
	})

	// Start HTTP server
	proxy.server = &http.Server{
		Addr:         *proxyAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
	}

	if *useHTTPS {
		fmt.Printf("Serving HTTPS at %s\n", *proxyAddr)
		if err := proxy.server.ListenAndServeTLS(*httpsCert, *httpsKey); err != http.ErrServerClosed {
			glog.Errorf("Error serving HTTP %+v", err)
		}
	} else {
		fmt.Printf("Serving HTTP at %s\n", *proxyAddr)
		if err := proxy.server.ListenAndServe(); err != http.ErrServerClosed {
			glog.Errorf("Error serving HTTP %+v", err)
		}
	}

	return err
}

// Shutdown shuts down the proxy server
func (proxy *GrpcProxy) Shutdown() {
	if proxy.server != nil {
		err := proxy.server.Shutdown(proxy.ctx)
		if err != nil {
			glog.Errorf("Error shutting down HTTP server %+v", err)
		}
	}
}

func main() {
	flag.Parse()
	defer glog.Flush()

	// Create the app context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxy := &GrpcProxy{
		ctx:    ctx,
		server: nil,
	}
	if err := proxy.serveHTTP(ctx); err != nil {
		glog.Fatalf("Error starting HTTP server: %+v", err)
	}
}
