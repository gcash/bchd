#!/bin/sh
set -e

protoc --go_out=plugins=grpc:./pb --go_opt=paths=source_relative bchrpc.proto