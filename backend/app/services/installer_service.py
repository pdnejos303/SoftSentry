"""Build a self-installing agent download by appending a config trailer.

The agent binary is a plain PE/Mach-O executable. We append a small JSON config
(server URL + deployment token) plus a fixed footer so the downloaded
``SoftSentry-Setup.exe`` knows where to call home without the user typing
anything. The agent reads the same trailer back (see Go ``internal/installer``).

Layout (bytes appended after the original executable):

    [ config JSON ][ length uint64 little-endian (8) ][ magic (8) ]

To read: take the last 16 bytes; if the trailing 8 match ``MAGIC`` the preceding
8 are the JSON length, and the JSON sits immediately before the length field.
The original executable length is ``total - 16 - json_len`` (used by the agent
to copy itself without the trailer).
"""

from __future__ import annotations

import json
import struct

# 8-byte magic identifying a SoftSentry config trailer. Keep in sync with the Go
# reader in agent/internal/installer.
MAGIC = b"SSCFGv1\x00"
_LEN_FMT = "<Q"  # uint64 little-endian
FOOTER_LEN = struct.calcsize(_LEN_FMT) + len(MAGIC)  # 16


def build_trailer(*, server_url: str, token: str) -> bytes:
    """Return the bytes to append to an agent binary for self-install."""
    config = json.dumps(
        {"server_url": server_url, "token": token},
        separators=(",", ":"),
    ).encode("utf-8")
    return config + struct.pack(_LEN_FMT, len(config)) + MAGIC


def build_installer(*, binary: bytes, server_url: str, token: str) -> bytes:
    """Append the config trailer to a raw agent binary."""
    return binary + build_trailer(server_url=server_url, token=token)


def parse_trailer(blob: bytes) -> tuple[str, str] | None:
    """Inverse of :func:`build_installer` — used by tests and tooling.

    Returns ``(server_url, token)`` or None when no valid trailer is present.
    """
    if len(blob) < FOOTER_LEN:
        return None
    if blob[-len(MAGIC) :] != MAGIC:
        return None
    (json_len,) = struct.unpack(_LEN_FMT, blob[-FOOTER_LEN : -len(MAGIC)])
    start = len(blob) - FOOTER_LEN - json_len
    if start < 0:
        return None
    try:
        data = json.loads(blob[start : len(blob) - FOOTER_LEN].decode("utf-8"))
        return str(data["server_url"]), str(data["token"])
    except (UnicodeDecodeError, json.JSONDecodeError, KeyError, TypeError):
        return None
