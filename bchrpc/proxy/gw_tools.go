//go:build tools

package main

// The purpose of this file is to pin dependencies: We ensure go builds the proxy with the same
// dependencies on every machine.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

import (
	_ "github.com/golang/protobuf/protoc-gen-go"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
