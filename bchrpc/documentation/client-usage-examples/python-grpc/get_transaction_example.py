#!/usr/bin/env python3
"""Example: GetTransaction with a display-order transaction id."""

from __future__ import annotations

import os
import sys

import grpc

# Generated stubs live in bchrpc/pb-py (see pb-py/README.md).
_REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "..", ".."))
_PB_DIR = os.path.join(_REPO_ROOT, "bchrpc", "pb-py")
if _PB_DIR not in sys.path:
    sys.path.insert(0, _PB_DIR)

import bchrpc_pb2 as pb  # noqa: E402
import bchrpc_pb2_grpc as bchrpc  # noqa: E402

from hash_utils import grpc_hash_to_hex_txid, hex_txid_to_grpc_hash  # noqa: E402


def run(
    target: str,
    txid_hex: str,
    cert_path: str | None,
    auth_token: str | None,
) -> None:
    if cert_path:
        creds = grpc.ssl_channel_credentials(
            root_certificates=open(cert_path, "rb").read()
        )
    else:
        creds = grpc.ssl_channel_credentials()

    options = []
    if auth_token:
        options.append(
            ("grpc.default_authority", target.split(":")[0])
        )

    with grpc.secure_channel(target, creds, options=options) as channel:
        stub = bchrpc.bchrpcStub(channel)
        req = pb.GetTransactionRequest()
        req.hash = hex_txid_to_grpc_hash(txid_hex)
        req.include_token_metadata = False

        metadata = ()
        if auth_token:
            metadata = (("AuthenticationToken", auth_token),)

        resp = stub.GetTransaction(req, metadata=metadata)
        tx = resp.transaction
        print("tx id:", grpc_hash_to_hex_txid(tx.hash))
        print("version:", tx.version)
        print("inputs:", len(tx.inputs))
        print("outputs:", len(tx.outputs))


if __name__ == "__main__":
    # Defaults: public mainnet gRPC (replace tx id with one your node has indexed).
    target = os.environ.get("BCHD_GRPC_TARGET", "127.0.0.1:8335")
    txid = os.environ.get(
        "BCHD_TXID_HEX",
        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
    )
    cert = os.environ.get("BCHD_RPC_CERT")
    token = os.environ.get("BCHD_GRPC_AUTH_TOKEN")
    run(target, txid, cert, token)
