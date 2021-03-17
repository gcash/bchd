# gRPC HTTP Gateway Proxy Server

A proxy server can be used when a client application is not able to connect to BCHD via gRPC for one reason or another.


## Build

1. Install the latest grpc-gateway library via:

```
$ go install \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
    github.com/golang/protobuf/protoc-gen-go \
    google.golang.org/grpc/cmd/protoc-gen-go-grpc
```

2. `$ make`

This will generate all of the files required for the gateway proxy and also the files for the swagger API docs.


## Run the Proxy

`$ ./gw -port 8080 -bchd-grpc-url <BCHD gRPC server url>:8335 -bchd-grpc-certpath <path to self-signed cert>`

If you are using a certificate signed by a CA then you do not need to specify a value for `-bchd-grpc-certpath`.


## Swagger API Docs

The proxy server will also host the static swagger files located in the `./web` directory.


## Run tests

```
go test gw_test.go
```

You can specify another BCHD backend using the same parameters as above for `gw.go`.
