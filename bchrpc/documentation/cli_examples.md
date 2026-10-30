# Command-line gRPC examples (grpcurl and curl)

These examples call a bchd gRPC server on **localhost:8335** (mainnet default). Testnet uses
**18335**. TLS is required; for the default self-signed certificate use grpcurl's `-insecure` flag
or point `-cacert` at `rpc.cert` from the bchd data directory.

Install [grpcurl](https://github.com/fullstorydev/grpcurl) and run commands from the repository
root so `-import-path` / `-proto` resolve correctly.

## GetBlockchainInfo

No `bytes` fields in the request.

```bash
grpcurl -insecure \
  -import-path bchrpc -proto bchrpc/bchrpc.proto \
  -d '{}' \
  localhost:8335 pb.bchrpc/GetBlockchainInfo
```

## GetTransaction (tx index required)

Replace `TXID_HEX` with a 64-character transaction id from an explorer. The shell snippet converts
display-order hex to little-endian bytes and base64 for JSON.

```bash
TXID_HEX='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
HASH_B64=$(python3 -c "import base64,sys; h=sys.argv[1]; print(base64.b64encode(bytes.fromhex(h)[::-1]).decode())" "$TXID_HEX")

grpcurl -insecure \
  -import-path bchrpc -proto bchrpc/bchrpc.proto \
  -d "{\"hash\":\"${HASH_B64}\"}" \
  localhost:8335 pb.bchrpc/GetTransaction
```

Optional token metadata (requires SLP index):

```bash
grpcurl -insecure \
  -import-path bchrpc -proto bchrpc/bchrpc.proto \
  -d "{\"hash\":\"${HASH_B64}\",\"include_token_metadata\":true}" \
  localhost:8335 pb.bchrpc/GetTransaction
```

## GetAddressTransactions (address index required)

Addresses are plain JSON strings (cashaddr).

```bash
grpcurl -insecure \
  -import-path bchrpc -proto bchrpc/bchrpc.proto \
  -d '{"address":"qrj279sdafw2r9zkcw7uncdhks2s3lpzmvwlle5243","nb_fetch":10}' \
  localhost:8335 pb.bchrpc/GetAddressTransactions
```

## GetSlpTokenMetadata (SLP index required)

`TOKEN_ID_HEX` is the token id as shown on token explorers (genesis tx id, 64 hex chars).

```bash
TOKEN_ID_HEX='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
TOKEN_B64=$(python3 -c "import base64,sys; h=sys.argv[1]; print(base64.b64encode(bytes.fromhex(h)[::-1]).decode())" "$TOKEN_ID_HEX")

grpcurl -insecure \
  -import-path bchrpc -proto bchrpc/bchrpc.proto \
  -d "{\"token_ids\":[\"${TOKEN_B64}\"]}" \
  localhost:8335 pb.bchrpc/GetSlpTokenMetadata
```

## Authentication token

If bchd runs with `--grpcauthtoken`, add a metadata header (grpcurl `-H`):

```bash
grpcurl -insecure -H 'AuthenticationToken: your_token_here' \
  -import-path bchrpc -proto bchrpc/bchrpc.proto \
  -d '{}' \
  localhost:8335 pb.bchrpc/GetBlockchainInfo
```

## curl via the HTTP gateway proxy

The [gRPC HTTP gateway](../proxy/README.md) exposes selected RPCs as POST JSON under `/v1/...`.
Start the proxy, then use curl against the proxy URL (not bchd's gRPC port directly).

```bash
# Example: proxy listening on :8080, backend bchd with rpc.cert in ~/.bchd/
curl -sS -X POST 'http://127.0.0.1:8080/v1/GetBlockchainInfo' \
  -H 'Content-Type: application/json' \
  -d '{}'
```

`GetTransaction` with the same base64 hash as grpcurl:

```bash
curl -sS -X POST 'http://127.0.0.1:8080/v1/GetTransaction' \
  -H 'Content-Type: application/json' \
  -d "{\"hash\":\"${HASH_B64}\"}"
```

`GetAddressTransactions`:

```bash
curl -sS -X POST 'http://127.0.0.1:8080/v1/GetAddressTransactions' \
  -H 'Content-Type: application/json' \
  -d '{"address":"qrj279sdafw2r9zkcw7uncdhks2s3lpzmvwlle5243","nb_fetch":10}'
```

Streaming RPCs (`SubscribeTransactions`, `SubscribeBlocks`, etc.) are not practical with plain curl;
use a gRPC client or grpcurl.
