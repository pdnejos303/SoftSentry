"""Agent binary distribution (spec 1.6 / API §agents binary).

Binaries live in a directory (``settings.agent_binary_dir``) alongside a
``manifest.json`` that an admin upload tool maintains:

    {
      "binaries": [
        {"version": "1.0.1", "os": "windows", "arch": "amd64",
         "filename": "softsentry-agent-1.0.1-windows-amd64.exe", "sha256": "..."}
      ]
    }

The directory is optional: when unset or absent, auto-update is simply
disabled and the endpoints report "no update".
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class BinaryEntry:
    version: str
    os: str
    arch: str
    filename: str
    sha256: str


def _version_tuple(v: str) -> tuple[int, ...]:
    """Parse a dotted version into a tuple of ints for numeric comparison.

    Non-numeric components are treated as 0 so a malformed entry can never
    crash the comparison.
    """
    parts: list[int] = []
    for chunk in v.split("."):
        digits = "".join(c for c in chunk if c.isdigit())
        parts.append(int(digits) if digits else 0)
    return tuple(parts)


def is_newer(candidate: str, current: str) -> bool:
    """Return True iff ``candidate`` is a strictly newer version than ``current``."""
    return _version_tuple(candidate) > _version_tuple(current)


def _load_entries(binary_dir: str) -> list[BinaryEntry]:
    if not binary_dir:
        return []
    manifest = Path(binary_dir) / "manifest.json"
    if not manifest.is_file():
        return []
    try:
        data = json.loads(manifest.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return []
    entries: list[BinaryEntry] = []
    for raw in data.get("binaries", []):
        try:
            entries.append(
                BinaryEntry(
                    version=str(raw["version"]),
                    os=str(raw["os"]),
                    arch=str(raw["arch"]),
                    filename=str(raw["filename"]),
                    sha256=str(raw["sha256"]),
                )
            )
        except (KeyError, TypeError):
            continue
    return entries


def latest_for(binary_dir: str, os: str, arch: str) -> BinaryEntry | None:
    """Return the highest-versioned binary matching ``os``/``arch``, or None."""
    matches = [e for e in _load_entries(binary_dir) if e.os == os and e.arch == arch]
    if not matches:
        return None
    return max(matches, key=lambda e: _version_tuple(e.version))


def resolve_path(binary_dir: str, version: str, os: str, arch: str) -> Path | None:
    """Return the on-disk path of the requested binary if it exists, else None."""
    for e in _load_entries(binary_dir):
        if e.version == version and e.os == os and e.arch == arch:
            path = Path(binary_dir) / e.filename
            return path if path.is_file() else None
    return None


def download_url(version: str, os: str, arch: str) -> str:
    """Build the agent-facing download path for a binary."""
    return f"/api/v1/agents/binary/download/{version}/{os}/{arch}"
