"""Pure CSV parsing/validation for bulk policy import (Module 4.3).

Returns (valid_rows, errors) so the caller can report "Add N, skip M" and
insert atomically. Handles a UTF-8 BOM and quoted fields transparently.
"""

from __future__ import annotations

import csv
import io
from typing import Literal

from app.schemas.policy import BulkImportError

Kind = Literal["whitelist", "blacklist"]

_BASE_COLUMNS = ("name_pattern", "publisher_pattern", "version_constraint", "notes")
_BLACKLIST_EXTRA = ("severity", "reason")
_VALID_SEVERITIES = {"high", "medium", "low"}


def _clean(value: str | None) -> str | None:
    """Trim; treat blank as absent (None)."""
    if value is None:
        return None
    v = value.strip()
    return v or None


def parse_policy_csv(
    data: bytes, *, kind: Kind
) -> tuple[list[dict[str, str | None]], list[BulkImportError]]:
    required_cols = set(_BASE_COLUMNS)
    if kind == "blacklist":
        required_cols |= set(_BLACKLIST_EXTRA)

    text = data.decode("utf-8-sig")  # strips a leading BOM if present
    reader = csv.DictReader(io.StringIO(text))

    if reader.fieldnames is None:
        return [], [BulkImportError(row=0, message="CSV is empty")]

    header = {(h or "").strip() for h in reader.fieldnames}
    missing = required_cols - header
    if missing:
        cols = ", ".join(sorted(missing))
        return [], [BulkImportError(row=0, message=f"missing required column(s): {cols}")]

    rows: list[dict[str, str | None]] = []
    errors: list[BulkImportError] = []
    for i, raw in enumerate(reader, start=1):
        row, err = _parse_row(raw, kind=kind, line=i)
        if err is not None:
            errors.append(err)
        elif row is not None:
            rows.append(row)
    return rows, errors


def _parse_row(
    raw: dict[str, str | None], *, kind: Kind, line: int
) -> tuple[dict[str, str | None] | None, BulkImportError | None]:
    name_pattern = _clean(raw.get("name_pattern"))
    if not name_pattern:
        return None, BulkImportError(row=line, message="name_pattern is required")

    row: dict[str, str | None] = {
        "name_pattern": name_pattern,
        "publisher_pattern": _clean(raw.get("publisher_pattern")),
        "version_constraint": _clean(raw.get("version_constraint")),
        "notes": _clean(raw.get("notes")),
    }

    if kind == "blacklist":
        severity = (_clean(raw.get("severity")) or "high").lower()
        if severity not in _VALID_SEVERITIES:
            return None, BulkImportError(
                row=line, message=f"severity must be high/medium/low, got {severity!r}"
            )
        reason = _clean(raw.get("reason"))
        if not reason:
            return None, BulkImportError(row=line, message="reason is required")
        row["severity"] = severity
        row["reason"] = reason

    return row, None
