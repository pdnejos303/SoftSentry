"""RFC 4180 CSV generation with a UTF-8 BOM for Excel (spec 8.3).

Excel only auto-detects UTF-8 when the file starts with a BOM; without it Thai
text renders as mojibake. Python's `csv` module already handles RFC 4180 quoting
(embedded commas, quotes, newlines), so we lean on it and only add the BOM and
an incremental (row-at-a-time) wrapper suitable for a StreamingResponse.
"""

from __future__ import annotations

import csv
import io
from collections.abc import Iterable, Iterator, Sequence

# Byte-order mark Excel uses to detect UTF-8.
BOM = "﻿"


def _cell(value: object) -> str:
    """Render one cell. None → empty; everything else via str()."""
    if value is None:
        return ""
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


def iter_csv(
    header: Sequence[str],
    rows: Iterable[Sequence[object]],
) -> Iterator[bytes]:
    """Yield UTF-8 CSV bytes one row at a time (RFC 4180, CRLF, BOM on row 0)."""
    buffer = io.StringIO()
    writer = csv.writer(buffer, lineterminator="\r\n")

    writer.writerow(header)
    yield (BOM + buffer.getvalue()).encode("utf-8")
    buffer.seek(0)
    buffer.truncate(0)

    for row in rows:
        writer.writerow([_cell(v) for v in row])
        yield buffer.getvalue().encode("utf-8")
        buffer.seek(0)
        buffer.truncate(0)


def to_csv_bytes(header: Sequence[str], rows: Iterable[Sequence[object]]) -> bytes:
    """Materialize the whole CSV (for small payloads / tests)."""
    return b"".join(iter_csv(header, rows))
