"""Aggregate the data each report template needs (spec 8.1 / 8.2).

Pure data assembly — reuses the dashboard / license / alert services and adds
two small group-by queries (machines-by-OS, top vulnerabilities). Returns plain
dicts so the result is easy to snapshot-test and to feed straight into Jinja.
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, datetime, timedelta
from typing import Any

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.cve import CveRecord, Vulnerability
from app.models.machine import Machine
from app.models.software import SoftwareRecord
from app.services import (
    alert_service,
    dashboard_service,
    license_service,
    machine_service,
    risk_service,
    software_service,
    vulnerability_service,
)

_UNAUTH_TYPES = ["unauthorized_software", "blacklisted_software"]
_BIG_PAGE = 10_000

# Trend SVG geometry (drawn as an inline polyline in the template).
_SVG_W = 600
_SVG_H = 140


def _pct(part: int, whole: int) -> float:
    return round(part / whole * 100, 1) if whole else 0.0


async def _machines_by_os(session: AsyncSession) -> list[dict[str, Any]]:
    rows = (
        await session.execute(
            select(Machine.os, func.count())
            .where(Machine.deleted_at.is_(None))
            .group_by(Machine.os)
            .order_by(func.count().desc())
        )
    ).all()
    total = sum(int(c) for _, c in rows)
    return [{"os": os_, "count": int(c), "pct": _pct(int(c), total)} for os_, c in rows]


async def _top_vulnerabilities(session: AsyncSession, *, limit: int = 10) -> list[dict[str, Any]]:
    rows = (
        await session.execute(
            select(
                CveRecord.cve_id,
                CveRecord.severity,
                CveRecord.description,
                func.count(func.distinct(Machine.id)).label("affected"),
            )
            .join(Vulnerability, Vulnerability.cve_id == CveRecord.cve_id)
            .join(SoftwareRecord, SoftwareRecord.id == Vulnerability.software_id)
            .join(Machine, Machine.id == SoftwareRecord.machine_id)
            .where(
                Machine.deleted_at.is_(None),
                SoftwareRecord.uninstalled_at.is_(None),
                Vulnerability.is_dismissed.is_(False),
            )
            .group_by(CveRecord.cve_id, CveRecord.severity, CveRecord.description)
            .order_by(func.count(func.distinct(Machine.id)).desc())
            .limit(limit)
        )
    ).all()
    return [
        {
            "cve_id": cve_id,
            "severity": severity,
            "description": (description or "")[:200],
            "affected_count": int(affected),
        }
        for cve_id, severity, description, affected in rows
    ]


def _trend_svg(points: list[dict[str, Any]]) -> dict[str, Any]:
    """Normalize the 30-day trend into polyline coords for a fixed viewBox."""
    series = ("critical", "high", "medium", "low")
    peak = max((p[s] for p in points for s in series), default=0) or 1
    n = max(len(points) - 1, 1)
    polylines: dict[str, str] = {}
    for s in series:
        coords = []
        for idx, p in enumerate(points):
            x = round(idx / n * _SVG_W, 1)
            y = round(_SVG_H - (p[s] / peak) * _SVG_H, 1)
            coords.append(f"{x},{y}")
        polylines[s] = " ".join(coords)
    return {"width": _SVG_W, "height": _SVG_H, "peak": peak, "polylines": polylines}


async def org_summary_context(
    session: AsyncSession, *, generated_by: str | None, org_name: str = "SoftSentry"
) -> dict[str, Any]:
    overview = await dashboard_service.overview(session)
    compliance = await license_service.compliance_summary(session)
    risky = await dashboard_service.risk_scores(session, limit=10)
    for r in risky:
        score = r["risk_score"]
        r["risk_score"] = round(float(score), 1) if isinstance(score, int | float) else 0.0
    trend = await dashboard_service.vuln_trend(session, days=30)
    os_breakdown = await _machines_by_os(session)
    top_vulns = await _top_vulnerabilities(session, limit=10)
    unauth_items, unauth_total = await alert_service.list_alerts(
        session, types=_UNAUTH_TYPES, status="active", page=1, page_size=_BIG_PAGE
    )

    return {
        "org_name": org_name,
        "generated_by": generated_by or "system",
        "generated_at": datetime.now(tz=UTC),
        "overview": overview,
        "compliance": compliance,
        "risky_machines": risky,
        "trend": _trend_svg(trend),
        "machines_by_os": os_breakdown,
        "top_vulnerabilities": top_vulns,
        "unauthorized": unauth_items,
        "unauthorized_total": unauth_total,
    }


async def machine_detail_context(
    session: AsyncSession, machine_uuid: uuid_lib.UUID
) -> dict[str, Any] | None:
    machine = await machine_service.get_machine(session, machine_uuid)
    if machine is None:
        return None

    detail = await machine_service.machine_detail(session, machine)
    breakdown = await risk_service.risk_breakdown(session, machine)
    software_items, software_total = await software_service.machine_software(
        session, machine, page=1, page_size=_BIG_PAGE
    )
    vuln_items, vuln_total = await vulnerability_service.list_vulnerabilities(
        session, machine_uuid=machine_uuid, page=1, page_size=_BIG_PAGE
    )

    # Signature status tally from the active inventory.
    sig_counts: dict[str, int] = {}
    for s in software_items:
        key = str(s.get("signature_status") or "unknown")
        sig_counts[key] = sig_counts.get(key, 0) + 1

    # History limited to the trailing 30 days (spec 8.2 §7).
    history_items, _ = await software_service.machine_history(session, machine.id, limit=500)
    cutoff = datetime.now(tz=UTC) - timedelta(days=30)
    recent_history = []
    for h in history_items:
        occurred = _aware(h.get("occurred_at"))
        if occurred is not None and occurred >= cutoff:
            recent_history.append(h)

    alert_items, alert_total = await alert_service.list_alerts(
        session, machine_uuid=machine_uuid, page=1, page_size=_BIG_PAGE
    )

    return {
        "generated_at": datetime.now(tz=UTC),
        "machine": detail,
        "risk_color": risk_service.risk_color(machine.risk_score),
        "risk_breakdown": breakdown,
        "software": software_items,
        "software_total": software_total,
        "vulnerabilities": vuln_items,
        "vulnerabilities_total": vuln_total,
        "signature_counts": sig_counts,
        "history": recent_history,
        "alerts": alert_items,
        "alerts_total": alert_total,
    }


def _aware(value: Any) -> datetime | None:
    if not isinstance(value, datetime):
        return None
    return value if value.tzinfo is not None else value.replace(tzinfo=UTC)
