# JavaScript / Python bchrpc libs
The provided libs are generated with the following command:

```
python -m grpc_tools.protoc -I=../ --python_out=. --grpc_python_out=. ../bchrpc.proto
```

## Dependencies

```
> python -m pip install grpcio-tools
```
