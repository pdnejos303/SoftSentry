"""Unit tests for the agent binary distribution service."""

from __future__ import annotations

import json
from pathlib import Path

import pytest
from app.services import binary_service


@pytest.mark.parametrize(
    ("candidate", "current", "expected"),
    [
        ("1.0.1", "1.0.0", True),
        ("1.0.0", "1.0.0", False),
        ("0.9.9", "1.0.0", False),
        ("1.2.0", "1.10.0", False),  # numeric, not lexical
        ("2.0", "1.9.9", True),
        ("1.0.0", "0.1.0", True),
    ],
)
def test_is_newer(candidate: str, current: str, expected: bool) -> None:
    assert binary_service.is_newer(candidate, current) is expected


def _write_manifest(tmp_path: Path, entries: list[dict]) -> None:
    (tmp_path / "manifest.json").write_text(json.dumps({"binaries": entries}))


def test_latest_for_picks_highest_matching_os_arch(tmp_path: Path) -> None:
    _write_manifest(
        tmp_path,
        [
            {
                "version": "1.0.0",
                "os": "windows",
                "arch": "amd64",
                "filename": "a.exe",
                "sha256": "aa",
            },
            {
                "version": "1.2.0",
                "os": "windows",
                "arch": "amd64",
                "filename": "b.exe",
                "sha256": "bb",
            },
            {"version": "9.9.9", "os": "macos", "arch": "arm64", "filename": "c", "sha256": "cc"},
        ],
    )
    latest = binary_service.latest_for(str(tmp_path), "windows", "amd64")
    assert latest is not None
    assert latest.version == "1.2.0"
    assert latest.sha256 == "bb"


def test_latest_for_returns_none_when_no_match(tmp_path: Path) -> None:
    _write_manifest(
        tmp_path,
        [
            {
                "version": "1.0.0",
                "os": "windows",
                "arch": "amd64",
                "filename": "a.exe",
                "sha256": "aa",
            }
        ],
    )
    assert binary_service.latest_for(str(tmp_path), "macos", "arm64") is None


def test_latest_for_returns_none_when_dir_unset_or_missing(tmp_path: Path) -> None:
    assert binary_service.latest_for("", "windows", "amd64") is None
    assert binary_service.latest_for(str(tmp_path / "nope"), "windows", "amd64") is None


def test_resolve_path_returns_existing_file(tmp_path: Path) -> None:
    (tmp_path / "a.exe").write_bytes(b"BINARY")
    _write_manifest(
        tmp_path,
        [
            {
                "version": "1.0.0",
                "os": "windows",
                "arch": "amd64",
                "filename": "a.exe",
                "sha256": "aa",
            }
        ],
    )
    resolved = binary_service.resolve_path(str(tmp_path), "1.0.0", "windows", "amd64")
    assert resolved is not None
    assert resolved.read_bytes() == b"BINARY"


def test_resolve_path_none_for_unknown_version(tmp_path: Path) -> None:
    _write_manifest(
        tmp_path,
        [
            {
                "version": "1.0.0",
                "os": "windows",
                "arch": "amd64",
                "filename": "a.exe",
                "sha256": "aa",
            }
        ],
    )
    assert binary_service.resolve_path(str(tmp_path), "2.0.0", "windows", "amd64") is None


def test_download_url_shape() -> None:
    assert (
        binary_service.download_url("1.0.1", "windows", "amd64")
        == "/api/v1/agents/binary/download/1.0.1/windows/amd64"
    )
