# Python gRPC examples

These scripts use the generated stubs in [`bchrpc/pb-py`](../../../pb-py/README.md). Regenerate them
after changing `bchrpc.proto`:

```bash
cd bchrpc
python -m grpc_tools.protoc -I=. --python_out=./pb-py --grpc_python_out=./pb-py ./bchrpc.proto
```

Install runtime dependencies (the checked-in stubs need grpcio 1.68.1 and protobuf 5.28.1 or newer):

```bash
python3 -m pip install 'grpcio>=1.68.1' 'protobuf>=5.28.1'
```

Regenerating the stubs also needs `python3 -m pip install grpcio-tools`.

The scripts connect to `BCHD_GRPC_TARGET` (default `127.0.0.1:8335`, a local mainnet node started
with `--grpclisten`). For a node with the default self-signed TLS certificate, set `BCHD_RPC_CERT`
to its `rpc.cert` (`~/.bchd/rpc.cert` on Linux); without it the certificate is checked against the
system CAs and the connection fails.

| Script | RPC | Notes |
| ------ | --- | ----- |
| [`get_blockchain_info_example.py`](./get_blockchain_info_example.py) | `GetBlockchainInfo` | No indexes required |
| [`get_transaction_example.py`](./get_transaction_example.py) | `GetTransaction` | Set `BCHD_TXID_HEX`; needs `--txindex` |
| [`get_slp_token_metadata_example.py`](./get_slp_token_metadata_example.py) | `GetSlpTokenMetadata` | Set `BCHD_TOKEN_ID_HEX`; needs `--slpindex` |
| [`address_subscription.py`](./address_subscription.py) | `GetAddressTransactions`, `SubscribeTransactions` | Older example against a public node; needs the stubs on the Python path |

Hash and address encoding rules: [`../../data_encoding.md`](../../data_encoding.md).

Shared helpers: [`hash_utils.py`](./hash_utils.py).

Verify examples compile (from repo root):

```bash
python3 -m py_compile \
  bchrpc/documentation/client-usage-examples/python-grpc/hash_utils.py \
  bchrpc/documentation/client-usage-examples/python-grpc/get_blockchain_info_example.py \
  bchrpc/documentation/client-usage-examples/python-grpc/get_transaction_example.py \
  bchrpc/documentation/client-usage-examples/python-grpc/get_slp_token_metadata_example.py \
  bchrpc/pb-py/bchrpc_pb2.py \
  bchrpc/pb-py/bchrpc_pb2_grpc.py
```
