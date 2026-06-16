"""Overview-dashboard aggregates (Module 7).

KPI counts, top-N risky machines (cheap indexed sort on the stored
`machines.risk_score`), and the 30-day vulnerability trend. Signature-status and
license-compliance charts reuse the existing `signature_service.signature_stats`
and `license_service.compliance_summary` endpoints.
"""

from __future__ import annotations

from datetime import UTC, date, datetime, timedelta
from typing import TYPE_CHECKING

from sqlalchemy import func, select

from app.models.cve import CveRecord, Vulnerability
from app.models.machine import Machine
from app.models.software import SoftwareRecord
from app.services import risk_service
from app.services.machine_service import online_filter

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

_SEVERITIES = ("critical", "high", "medium", "low")


async def overview(session: AsyncSession) -> dict[str, int]:
    now = datetime.now(tz=UTC)
    machines_total = (
        await session.execute(
            select(func.count()).select_from(Machine).where(Machine.deleted_at.is_(None))
        )
    ).scalar_one()

    agents_online = (
        await session.execute(
            select(func.count())
            .select_from(Machine)
            .where(Machine.deleted_at.is_(None), online_filter(now))
        )
    ).scalar_one()

    software_unique = (
        await session.execute(
            select(func.count(func.distinct(func.lower(SoftwareRecord.name))))
            .select_from(SoftwareRecord)
            .join(Machine, Machine.id == SoftwareRecord.machine_id)
            .where(SoftwareRecord.uninstalled_at.is_(None), Machine.deleted_at.is_(None))
        )
    ).scalar_one()

    sev_counts = await _vuln_severity_counts(session)

    return {
        "machines_total": int(machines_total),
        "agents_online": int(agents_online),
        "software_unique": int(software_unique),
        "vuln_critical": sev_counts["critical"],
        "vuln_high": sev_counts["high"],
        "vuln_medium": sev_counts["medium"],
        "vuln_low": sev_counts["low"],
    }


async def _vuln_severity_counts(session: AsyncSession) -> dict[str, int]:
    rows = (
        await session.execute(
            select(CveRecord.severity, func.count(func.distinct(Vulnerability.id)))
            .join(CveRecord, CveRecord.cve_id == Vulnerability.cve_id)
            .join(SoftwareRecord, SoftwareRecord.id == Vulnerability.software_id)
            .join(Machine, Machine.id == SoftwareRecord.machine_id)
            .where(
                Machine.deleted_at.is_(None),
                SoftwareRecord.uninstalled_at.is_(None),
                Vulnerability.is_dismissed.is_(False),
                Vulnerability.match_confidence.in_(["high", "medium"]),
            )
            .group_by(CveRecord.severity)
        )
    ).all()
    counts = {sev: int(c) for sev, c in rows}
    return {sev: counts.get(sev, 0) for sev in _SEVERITIES}


async def risk_scores(session: AsyncSession, *, limit: int = 10) -> list[dict[str, object]]:
    """Top-N machines by stored risk score (desc), only those with score > 0."""
    rows = (
        (
            await session.execute(
                select(Machine)
                .where(Machine.deleted_at.is_(None), Machine.risk_score > 0)
                .order_by(Machine.risk_score.desc(), Machine.id.desc())
                .limit(limit)
            )
        )
        .scalars()
        .all()
    )
    return [
        {
            "machine_uuid": m.uuid,
            "hostname": m.hostname,
            "risk_score": m.risk_score,
            "color": risk_service.risk_color(m.risk_score),
        }
        for m in rows
    ]


async def vuln_trend(
    session: AsyncSession, *, days: int = 30
) -> list[dict[str, date | int]]:
    """Daily matched-CVE counts by severity over the trailing `days` window.
    Missing days are filled with zeros so the line chart has no gaps."""
    today = datetime.now(tz=UTC).date()
    start = today - timedelta(days=days - 1)

    rows = (
        await session.execute(
            select(
                func.date(Vulnerability.matched_at),
                CveRecord.severity,
                func.count(func.distinct(Vulnerability.id)),
            )
            .join(CveRecord, CveRecord.cve_id == Vulnerability.cve_id)
            .join(SoftwareRecord, SoftwareRecord.id == Vulnerability.software_id)
            .join(Machine, Machine.id == SoftwareRecord.machine_id)
            .where(
                Machine.deleted_at.is_(None),
                SoftwareRecord.uninstalled_at.is_(None),
                Vulnerability.is_dismissed.is_(False),
                Vulnerability.match_confidence.in_(["high", "medium"]),
                func.date(Vulnerability.matched_at) >= start,
            )
            .group_by(func.date(Vulnerability.matched_at), CveRecord.severity)
        )
    ).all()

    # (date, severity) → count. `func.date` returns a str on SQLite, a date on PG.
    tally: dict[tuple[date, str], int] = {}
    for raw_day, severity, count in rows:
        day = _coerce_date(raw_day)
        tally[(day, severity)] = int(count)

    points: list[dict[str, date | int]] = [
        {
            "date": start + timedelta(days=offset),
            "critical": tally.get((start + timedelta(days=offset), "critical"), 0),
            "high": tally.get((start + timedelta(days=offset), "high"), 0),
            "medium": tally.get((start + timedelta(days=offset), "medium"), 0),
            "low": tally.get((start + timedelta(days=offset), "low"), 0),
        }
        for offset in range(days)
    ]
    return points


def _coerce_date(value: object) -> date:
    if isinstance(value, date):
        return value
    return date.fromisoformat(str(value))
