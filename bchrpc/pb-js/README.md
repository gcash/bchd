# JavaScript / TypeScript bchrpc libs
## Protoc - [grpc-web](https://github.com/improbable-eng/grpc-web)
The provided libs are generated with the following `protoc` command:

```
protoc \
  --plugin=protoc-gen-ts=./node_modules/.bin/protoc-gen-ts \
  --js_out=import_style=commonjs,binary:./ \
  --ts_out=service=grpc-web:./ \
  --proto_path=../ \
  ../bchrpc.proto
```

## Dependencies

```
> yarn install
```

`protoc` no longer includes the JavaScript generator, so `protoc-gen-js` from
[protobuf-javascript](https://github.com/protocolbuffers/protobuf-javascript/releases)
has to be on the `PATH`.  The provided libs are generated with version 4.0.3.
