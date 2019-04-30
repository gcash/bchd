#!/bin/sh

protoc -I. bchrpc.proto --go_out=plugins=grpc:pb