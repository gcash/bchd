#!/usr/bin/env python3
"""Minimal example: GetBlockchainInfo over TLS."""

from __future__ import annotations

import os
import sys

import grpc

_REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "..", ".."))
_PB_DIR = os.path.join(_REPO_ROOT, "bchrpc", "pb-py")
if _PB_DIR not in sys.path:
    sys.path.insert(0, _PB_DIR)

import bchrpc_pb2 as pb  # noqa: E402
import bchrpc_pb2_grpc as bchrpc  # noqa: E402


def run(target: str, cert_path: str | None) -> None:
    if cert_path:
        creds = grpc.ssl_channel_credentials(
            root_certificates=open(cert_path, "rb").read()
        )
    else:
        creds = grpc.ssl_channel_credentials()

    with grpc.secure_channel(target, creds) as channel:
        stub = bchrpc.bchrpcStub(channel)
        resp = stub.GetBlockchainInfo(pb.GetBlockchainInfoRequest())
        print("best_height:", resp.best_height)
        print("best_block_hash (display hex):", resp.best_block_hash[::-1].hex())


if __name__ == "__main__":
    target = os.environ.get("BCHD_GRPC_TARGET", "127.0.0.1:8335")
    cert = os.environ.get("BCHD_RPC_CERT")
    run(target, cert)
