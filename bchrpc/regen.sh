#!/bin/sh
set -e

# TODO merge (remove) this file with proxy/Makefile - we should only have 1 file for auto-generated gRPC code

protoc --go_out=plugins=grpc:./pb --go_opt=paths=source_relative bchrpc.proto