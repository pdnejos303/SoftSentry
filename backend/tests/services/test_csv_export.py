"""Module 8.3 — RFC 4180 CSV with a UTF-8 BOM (Excel + Thai)."""

from __future__ import annotations

from app.services import csv_export


def test_starts_with_bom() -> None:
    out = csv_export.to_csv_bytes(["a"], [])
    assert out.startswith(csv_export.BOM.encode("utf-8"))


def test_header_and_rows() -> None:
    out = csv_export.to_csv_bytes(["name", "count"], [["Chrome", 3]]).decode("utf-8-sig")
    assert out.splitlines() == ["name,count", "Chrome,3"]


def test_crlf_line_endings() -> None:
    out = csv_export.to_csv_bytes(["a"], [["x"]]).decode("utf-8-sig")
    assert out == "a\r\nx\r\n"


def test_quotes_commas_newlines_escaped() -> None:
    out = csv_export.to_csv_bytes(["name"], [['My, "Cool"\nApp']]).decode("utf-8-sig")
    # Embedded comma + newline force quoting; inner quotes are doubled.
    assert '"My, ""Cool""' in out


def test_none_and_bool_cells() -> None:
    out = csv_export.to_csv_bytes(["a", "b", "c"], [[None, True, False]]).decode("utf-8-sig")
    assert out.splitlines()[1] == ",true,false"


def test_thai_round_trips() -> None:
    out = csv_export.to_csv_bytes(["name"], [["โปรแกรม"]])
    assert "โปรแกรม".encode() in out


def test_iter_streams_row_by_row() -> None:
    chunks = list(csv_export.iter_csv(["a"], [[1], [2]]))
    # One chunk for the header row, one per data row.
    assert len(chunks) == 3
