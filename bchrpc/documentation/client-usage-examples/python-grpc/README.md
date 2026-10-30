# Python gRPC examples

These scripts use the generated stubs in [`bchrpc/pb-py`](../../pb-py/README.md). Regenerate them
after changing `bchrpc.proto`:

```bash
cd bchrpc
python -m grpc_tools.protoc -I=. --python_out=./pb-py --grpc_python_out=./pb-py ./bchrpc.proto
```

Install runtime dependencies:

```bash
python3 -m pip install grpcio
```

For a local bchd with a self-signed TLS certificate, set `BCHD_RPC_CERT` to your `rpc.cert` path
and `BCHD_GRPC_TARGET` (default `127.0.0.1:8335` on mainnet).

| Script | RPC | Notes |
| ------ | --- | ----- |
| [`get_blockchain_info_example.py`](./get_blockchain_info_example.py) | `GetBlockchainInfo` | No indexes required |
| [`get_transaction_example.py`](./get_transaction_example.py) | `GetTransaction` | Set `BCHD_TXID_HEX`; needs `--txindex` |
| [`get_slp_token_metadata_example.py`](./get_slp_token_metadata_example.py) | `GetSlpTokenMetadata` | Set `BCHD_TOKEN_ID_HEX`; needs `--slpindex` |
| [`address_subscription.py`](./address_subscription.py) | `GetAddressTransactions`, `SubscribeTransactions` | Existing address example |

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
