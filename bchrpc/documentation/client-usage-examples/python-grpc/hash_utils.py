"""Helpers for bchrpc hash fields (little-endian bytes vs display hex)."""

from __future__ import annotations


def hex_txid_to_grpc_hash(hex_txid: str) -> bytes:
    """Convert a 64-char explorer/wallet tx or token id to gRPC ``bytes`` hash."""
    cleaned = hex_txid.strip().lower()
    if cleaned.startswith("0x"):
        cleaned = cleaned[2:]
    if len(cleaned) != 64:
        raise ValueError(
            f"expected 64 hex characters (32 bytes), got {len(cleaned)}; "
            "do not pass the hex string as UTF-8 in a bytes field"
        )
    return bytes.fromhex(cleaned)[::-1]


def grpc_hash_to_hex_txid(data: bytes) -> str:
    """Convert gRPC hash bytes to display-order hex."""
    if len(data) != 32:
        raise ValueError(f"expected 32 bytes, got {len(data)}")
    return data[::-1].hex()
