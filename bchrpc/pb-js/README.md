# JavaScript / TypeScript bchrp libs 
## Protoc - [grpc-web](https://github.com/improbable-eng/grpc-web)

```
protoc \
  --plugin=protoc-gen-ts=../node_modules/.bin/protoc-gen-ts \
  --js_out=import_style=commonjs,binary:bchrpc \
  --ts_out=service=true:bchrpc \
  ./bchrpc.proto
```

## Dependencies

```
> yarn add ts-protoc-gen
> yarn add @improbable-eng/grpc-web
> yarn add @types/google-protobuf
> yarn add google-protobuf
```

