#!/bin/sh

# Golang
protoc -I. bchrpc.proto --go_out=plugins=grpc:pb

# Python
# Dependencies: 
#   - python -m pip install grpcio
#   - python -m pip install grpcio-tools
python -m grpc_tools.protoc -I=./ --python_out=./pb-py --grpc_python_out=./pb-py ./bchrpc.proto

# Node.js
# Dependencie
#  - yarn add ts-protoc-gen
#  - yarn add @improbable-eng/grpc-web
#  - yarn add @types/google-protobuf
#  - yarn add google-protobuf
protoc \
  --plugin=protoc-gen-ts=./node_modules/.bin/protoc-gen-ts \
  --js_out=import_style=commonjs,binary:pb-js \
  --ts_out=service=true:pb-js \
  ./bchrpc.proto
