#!/bin/sh

# This command requires the node_modules be installed in the pb-js directory
# using the yarn commands found in the README.md
protoc \
  --plugin=protoc-gen-ts=./node_modules/.bin/protoc-gen-ts \
  --js_out=import_style=commonjs,binary:./ \
  --ts_out=service=true:./ \
  --proto_path=../ \
  ../bchrpc.proto