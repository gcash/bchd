#!/bin/sh
# Regenerate the Go protobuf, gRPC and REST gateway code in pb/ and the
# swagger file in proxy/web/ from bchrpc.proto.  The Go plugins are pinned by
# the tool directives in go.mod; pb-js and pb-py are generated separately.
set -e

PROTOC_VERSION=36.1

# The plugins format their output with the Go toolchain they are built with.
export GOTOOLCHAIN=go1.27.1

cd "$(dirname "$0")"

if [ "$(protoc --version)" != "libprotoc $PROTOC_VERSION" ]; then
	echo "regen.sh: protoc $PROTOC_VERSION is required, found: $(protoc --version)" >&2
	exit 1
fi

go_plugin=$(go tool -n protoc-gen-go)
grpc_plugin=$(go tool -n protoc-gen-go-grpc)
gateway_plugin=$(go tool -n protoc-gen-grpc-gateway)
openapi_plugin=$(go tool -n protoc-gen-openapiv2)

protoc -I. \
	--plugin=protoc-gen-go="$go_plugin" \
	--plugin=protoc-gen-go-grpc="$grpc_plugin" \
	--plugin=protoc-gen-grpc-gateway="$gateway_plugin" \
	--plugin=protoc-gen-openapiv2="$openapi_plugin" \
	--go_out=pb \
	--go_opt=paths=source_relative \
	--go-grpc_out=pb \
	--go-grpc_opt=paths=source_relative,require_unimplemented_servers=false \
	--grpc-gateway_out=pb \
	--grpc-gateway_opt=paths=source_relative,generate_unbound_methods=true \
	--grpc-gateway_opt=grpc_api_configuration=proxy/bchrpc.yaml \
	--openapiv2_out=proxy/web \
	--openapiv2_opt=generate_unbound_methods=true,json_names_for_fields=false \
	--openapiv2_opt=grpc_api_configuration=proxy/bchrpc.yaml \
	bchrpc.proto
