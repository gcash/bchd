# gRPC HTTP Gateway Proxy Server

A proxy server can be used when a client application is not able to connect to BCHD via gRPC for one reason or another.


## Build

`$ make`

This will generate all of the files required for the gateway proxy and also the files for the swagger API docs.


## Run the Proxy

`$ ./gw -port 8080 -bchd-grpc-url <BCHD gRPC server url>:8335 -bchd-grpc-certpath <path to self-signed cert>`

If you are using a certificate signed by a CA then you do not need to specify a value for `-bchd-grpc-certpath`.


## Swagger API Docs

The proxy server will also host the static swagger files located in the `./web` directory.
