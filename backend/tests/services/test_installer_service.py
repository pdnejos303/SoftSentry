"""Config-trailer round trip — must stay byte-compatible with the Go reader."""

from __future__ import annotations

from app.services import installer_service


def test_build_then_parse_round_trip() -> None:
    binary = b"\x4d\x5a" + b"fake-pe-bytes" * 100  # MZ header + filler
    blob = installer_service.build_installer(
        binary=binary, server_url="http://192.168.1.50:8001", token="tok-abc"
    )
    # Original binary is a prefix; trailer is appended.
    assert blob.startswith(binary)
    assert blob.endswith(installer_service.MAGIC)

    parsed = installer_service.parse_trailer(blob)
    assert parsed == ("http://192.168.1.50:8001", "tok-abc")


def test_original_length_recoverable() -> None:
    binary = b"X" * 4096
    blob = installer_service.build_installer(
        binary=binary, server_url="http://s", token="t"
    )
    trailer = installer_service.build_trailer(server_url="http://s", token="t")
    original_len = len(blob) - installer_service.FOOTER_LEN - (len(trailer) - installer_service.FOOTER_LEN)
    assert original_len == len(binary)


def test_parse_rejects_plain_binary() -> None:
    assert installer_service.parse_trailer(b"no trailer here") is None


def test_parse_rejects_bad_magic() -> None:
    blob = installer_service.build_installer(binary=b"AAAA", server_url="http://s", token="t")
    assert installer_service.parse_trailer(blob[:-1] + b"!") is None
