"""Inline CSV export endpoints (spec 8.3).

Each route mirrors the filters of the corresponding list endpoint and streams a
BOM-prefixed, RFC 4180 CSV. Registered ahead of the `/{uuid}` routers so the
literal `/export` segment is matched before the UUID path params.
"""

from __future__ import annotations

import uuid as uuid_lib
from collections.abc import Sequence
from datetime import UTC, datetime
from typing import Annotated

from fastapi import APIRouter, Query
from fastapi.responses import StreamingResponse

from app.core.deps import CurrentUser, DBSession
from app.services import csv_export, export_service

router = APIRouter()


def _csv_response(
    stem: str, header: Sequence[str], rows: Sequence[Sequence[object]]
) -> StreamingResponse:
    stamp = datetime.now(tz=UTC).strftime("%Y%m%d")
    filename = f"softsentry-{stem}-{stamp}.csv"
    return StreamingResponse(
        iter(csv_export.iter_csv(header, rows)),
        media_type="text/csv; charset=utf-8",
        headers={"Content-Disposition": f'attachment; filename="{filename}"'},
    )


def _split(value: str | None) -> list[str] | None:
    if not value:
        return None
    return [p.strip() for p in value.split(",") if p.strip()]


@router.get("/machines/export", summary="Export machines as CSV", tags=["exports"])
async def export_machines(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    status_filter: Annotated[str | None, Query(alias="status")] = None,
    os: str | None = None,
    tag: str | None = None,
    sort: str | None = None,
) -> StreamingResponse:
    header, rows = await export_service.machines_csv(
        session, q=q, status=status_filter, os=os, tag=tag, sort=sort
    )
    return _csv_response("machines", header, rows)


@router.get("/software/export", summary="Export software inventory as CSV", tags=["exports"])
async def export_software(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    publisher: str | None = None,
    signature_status: Annotated[str | None, Query(alias="signature_status")] = None,
) -> StreamingResponse:
    header, rows = await export_service.software_csv(
        session, q=q, publisher=publisher, signature_status=signature_status
    )
    return _csv_response("software", header, rows)


@router.get("/vulnerabilities/export", summary="Export vulnerabilities as CSV", tags=["exports"])
async def export_vulnerabilities(
    session: DBSession,
    _: CurrentUser,
    severity: str | None = None,
    machine_uuid: uuid_lib.UUID | None = None,
    show_dismissed: bool = False,
    date_window: Annotated[str | None, Query(alias="matched")] = None,
    q: str | None = None,
    min_confidence: str | None = None,
) -> StreamingResponse:
    header, rows = await export_service.vulnerabilities_csv(
        session,
        severities=_split(severity),
        machine_uuid=machine_uuid,
        show_dismissed=show_dismissed,
        date_window=date_window,
        q=q,
        min_confidence=min_confidence,
    )
    return _csv_response("vulnerabilities", header, rows)


@router.get("/licenses/export", summary="Export licenses as CSV", tags=["exports"])
async def export_licenses(
    session: DBSession,
    _: CurrentUser,
    q: str | None = None,
    status_filter: Annotated[str | None, Query(alias="status")] = None,
) -> StreamingResponse:
    header, rows = await export_service.licenses_csv(session, q=q, status=status_filter)
    return _csv_response("licenses", header, rows)


@router.get("/alerts/export", summary="Export alerts as CSV", tags=["exports"])
async def export_alerts(
    session: DBSession,
    _: CurrentUser,
    type: str | None = None,
    status_filter: Annotated[str | None, Query(alias="status")] = None,
    severity: str | None = None,
    machine_uuid: uuid_lib.UUID | None = None,
) -> StreamingResponse:
    header, rows = await export_service.alerts_csv(
        session,
        types=_split(type),
        status=status_filter,
        severity=severity,
        machine_uuid=machine_uuid,
    )
    return _csv_response("alerts", header, rows)
