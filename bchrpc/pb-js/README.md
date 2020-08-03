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
