# gRPC request data encoding

Many bchrpc fields use Protocol Buffers `bytes` or `string` types. Wallets and block explorers
show transaction ids and SLP token ids as **64-character hex strings** (32 bytes in **display /
big-endian** order). Several RPC requests expect **raw 32-byte values in little-endian order**, the
same in-memory layout Bitcoin uses internally. Passing a hex string in a `string` field, or passing
the hex digits as UTF-8 in a `bytes` field, produces errors such as:

```text
invalid hash length of 64, want 32
```

The sections below list the common field types and how to prepare values for Python, grpcurl, and
curl against the [HTTP gateway](../proxy/README.md).

## Transaction and block hashes

| Where you see it | Format |
| ---------------- | ------ |
| Explorer, wallet, `chainhash.Hash.String()` in Go | 64 hex chars, **big-endian** (display order) |
| `bytes hash` in `GetTransactionRequest`, `GetBlockInfoRequest`, block `info.hash`, etc. | **32 raw bytes**, **little-endian** |

**Conversion**

- **Display hex → gRPC bytes:** decode hex to 32 bytes, then reverse byte order.
- **gRPC bytes → display hex:** reverse byte order, then encode as hex.

The [Python examples](./client-usage-examples/python-grpc/hash_utils.py) provide `hex_txid_to_grpc_hash`
and `grpc_hash_to_hex_txid` helpers. The same logic appears in existing examples when printing hashes
(for example `bytearray(hash[::-1]).hex()` in [client_usage.md](./client_usage.md)).

## SLP token ids

An SLP token id is the **genesis transaction id** of the token. In API messages it is still a
32-byte hash in **little-endian** form, same rules as transaction hashes:

- `GetSlpTokenMetadataRequest.token_ids` — repeated `bytes`
- `TransactionFilter.slp_token_ids` — repeated `bytes`
- Fields named `token_id` inside responses — `bytes` in little-endian form

Use the same conversion as for transaction ids. The token id string shown on [slp.tokenview.com](https://slp.tokenview.com/)
or in a wallet is display-order hex.

## Bitcoin Cash addresses

Address fields are **strings**, not bytes:

- `GetAddressTransactionsRequest.address` — lowercase **cashaddr** (with or without the `bitcoincash:` /
  `bchtest:` / `bchreg:` prefix; the `cashaddress:` prefix is also accepted per the `.proto` comments).
- Example: `qrj279sdafw2r9zkcw7uncdhks2s3lpzmvwlle5243` or `bitcoincash:qrj279sdafw2r9zkcw7uncdhks2s3lpzmvwlle5243`

Do not hex-encode addresses into `bytes`.

## Other `bytes` fields

| Field | Meaning |
| ----- | ------- |
| `SubmitTransactionRequest.serialized_tx` | Raw transaction serialization |
| `GetSlpParsedScriptRequest.slp_opreturn_script` | Raw OP_RETURN script bytes |
| `CashToken.category_id` | 32-byte token category (little-endian hash) |
| Mempool / block `transaction_hash` | Little-endian tx hash |

When in doubt, check the comment on the field in [bchrpc.proto](../bchrpc.proto).

## JSON, grpcurl, and curl

Tools that send **JSON** (grpcurl, the gRPC HTTP gateway) encode `bytes` fields as **standard base64**
(not hex). You must supply **already little-endian** 32-byte values, then base64-encode them.

Example: display tx id `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa` (64 `a` chars)
is 32 bytes of `0xaa` in display order; little-endian bytes are still 32 × `0xaa`, and base64 is
`qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq`.

See [cli_examples.md](./cli_examples.md) for grpcurl and curl commands against a local node or proxy.

## Index requirements

Some RPCs return `NotFound` or empty data if the node was started without the right indexes. Common
cases:

| RPC | Needs |
| --- | ----- |
| `GetTransaction`, `GetRawTransaction` | `--txindex` |
| `GetAddressTransactions`, `GetAddressUnspentOutputs` | `--addrindex` |
| SLP fields on transactions / addresses | `--slpindex` |
| `GetSlpTokenMetadata` | `--slpindex` |

See [server_configuration.md](./server_configuration.md) for startup flags. Default gRPC port is
**8335** on mainnet (**18335** on testnet3/regtest); see [default_ports.md](../../docs/default_ports.md).
