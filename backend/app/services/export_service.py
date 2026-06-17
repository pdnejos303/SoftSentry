"""CSV row builders for the inline export endpoints (spec 8.3).

Each builder reuses the same list service (hence the same filters) as its
dashboard table, then flattens the dict rows into CSV cells. A single large page
is requested; for the fleet sizes we target this stays within memory and the
StreamingResponse still emits the file row-by-row.
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import date, datetime
from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from app.services import (
    alert_service,
    license_service,
    machine_service,
    software_service,
    vulnerability_service,
)

# Upper bound on rows pulled into a single export (spec: 50k).
EXPORT_CAP = 50_000

# (header, rows) — rows is a list of cell sequences ready for csv_export.
Export = tuple[list[str], list[list[object]]]


def _dt(value: Any) -> str:
    return value.isoformat() if isinstance(value, datetime | date) else ""


def _tags(value: Any) -> str:
    return "; ".join(value) if isinstance(value, list) else ""


async def machines_csv(
    session: AsyncSession,
    *,
    q: str | None = None,
    status: str | None = None,
    os: str | None = None,
    tag: str | None = None,
    sort: str | None = None,
) -> Export:
    items, _ = await machine_service.list_machines(
        session,
        q=q,
        status=status,
        os=os,
        tag=tag,
        page=1,
        page_size=EXPORT_CAP,
        sort=sort,
    )
    header = [
        "uuid",
        "hostname",
        "os",
        "os_version",
        "agent_version",
        "status",
        "last_seen_at",
        "last_scan_at",
        "tags",
        "software_count",
        "risk_score",
    ]
    rows = [
        [
            i["uuid"],
            i["hostname"],
            i["os"],
            i["os_version"],
            i["agent_version"],
            i["status"],
            _dt(i["last_seen_at"]),
            _dt(i["last_scan_at"]),
            _tags(i["tags"]),
            i["software_count"],
            i["risk_score"],
        ]
        for i in items
    ]
    return header, rows


async def machine_software_csv(session: AsyncSession, machine: Any) -> Export:
    """Software inventory of one machine (used by the machine_detail CSV report)."""
    items, _ = await software_service.machine_software(
        session, machine, page=1, page_size=EXPORT_CAP
    )
    header = ["name", "version", "publisher", "install_date", "signature_status"]
    rows = [
        [
            i["name"],
            i["version"],
            i.get("publisher") or "",
            _dt(i.get("install_date")),
            i.get("signature_status") or "",
        ]
        for i in items
    ]
    return header, rows


async def software_csv(
    session: AsyncSession,
    *,
    q: str | None = None,
    publisher: str | None = None,
    signature_status: str | None = None,
) -> Export:
    items, _ = await software_service.cross_software(
        session,
        q=q,
        publisher=publisher,
        signature_status=signature_status,
        page=1,
        page_size=EXPORT_CAP,
    )
    header = ["name", "version", "publisher", "installed_count", "signature_status"]
    rows = [
        [
            i["name"],
            i["version"],
            i.get("publisher") or "",
            i["installed_count"],
            i.get("signature_status") or "",
        ]
        for i in items
    ]
    return header, rows


async def vulnerabilities_csv(
    session: AsyncSession,
    *,
    severities: list[str] | None = None,
    machine_uuid: uuid_lib.UUID | None = None,
    show_dismissed: bool = False,
    date_window: str | None = None,
    q: str | None = None,
    min_confidence: str | None = None,
) -> Export:
    items, _ = await vulnerability_service.list_vulnerabilities(
        session,
        severities=severities,
        machine_uuid=machine_uuid,
        show_dismissed=show_dismissed,
        date_window=date_window,
        q=q,
        min_confidence=min_confidence,
        page=1,
        page_size=EXPORT_CAP,
    )
    header = [
        "cve_id",
        "severity",
        "cvss_score",
        "machine_hostname",
        "software_name",
        "software_version",
        "recommended_version",
        "match_confidence",
        "is_dismissed",
        "matched_at",
    ]
    rows = [
        [
            i["cve_id"],
            i["severity"],
            i.get("cvss_score") if i.get("cvss_score") is not None else "",
            i["machine_hostname"],
            i["software_name"],
            i["software_version"],
            i.get("recommended_version") or "",
            i["match_confidence"],
            i["is_dismissed"],
            _dt(i["matched_at"]),
        ]
        for i in items
    ]
    return header, rows


async def licenses_csv(
    session: AsyncSession,
    *,
    q: str | None = None,
    status: str | None = None,
) -> Export:
    items, _ = await license_service.list_licenses(
        session, q=q, status_filter=status, page=1, page_size=EXPORT_CAP
    )
    header = [
        "software_name",
        "publisher",
        "purchased_count",
        "installed_count",
        "status",
        "purchased_at",
        "expires_at",
        "days_until_expiry",
        "cost_total",
        "currency",
        "vendor_contact",
    ]
    rows = [
        [
            i["software_name"],
            i.get("publisher") or "",
            i["purchased_count"],
            i["installed_count"],
            i["status"],
            _dt(i.get("purchased_at")),
            _dt(i.get("expires_at")),
            i.get("days_until_expiry") if i.get("days_until_expiry") is not None else "",
            i.get("cost_total") if i.get("cost_total") is not None else "",
            i.get("currency") or "",
            i.get("vendor_contact") or "",
        ]
        for i in items
    ]
    return header, rows


async def alerts_csv(
    session: AsyncSession,
    *,
    types: list[str] | None = None,
    status: str | None = None,
    severity: str | None = None,
    machine_uuid: uuid_lib.UUID | None = None,
) -> Export:
    items, _ = await alert_service.list_alerts(
        session,
        types=types,
        status=status,
        severity=severity,
        machine_uuid=machine_uuid,
        page=1,
        page_size=EXPORT_CAP,
    )
    header = [
        "type",
        "severity",
        "status",
        "title",
        "machine_hostname",
        "software_name",
        "software_version",
        "created_at",
        "resolved_at",
    ]
    rows = [
        [
            i["type"],
            i["severity"],
            i["status"],
            i["title"],
            i.get("machine_hostname") or "",
            i.get("software_name") or "",
            i.get("software_version") or "",
            _dt(i.get("created_at")),
            _dt(i.get("resolved_at")),
        ]
        for i in items
    ]
    return header, rows


async def all_machine_software_csv(session: AsyncSession) -> Export:
    """Software inventory ของทุกเครื่อง (ใช้กับ all_machine_detail CSV report)."""
    machines, _ = await machine_service.list_machines(
        session, page=1, page_size=EXPORT_CAP
    )
    header = ["hostname", "name", "version", "publisher", "install_date", "signature_status"]
    rows: list[list[object]] = []
    for m in machines:
        machine = await machine_service.get_machine(session, m["uuid"])
        if machine is None:
            continue
        items, _ = await software_service.machine_software(
            session, machine, page=1, page_size=EXPORT_CAP
        )
        for i in items:
            rows.append(
                [
                    m["hostname"],
                    i["name"],
                    i["version"],
                    i.get("publisher") or "",
                    _dt(i.get("install_date")),
                    i.get("signature_status") or "",
                ]
            )
    return header, rows