"""Business-KPI metric collection (Telemetry).

Prometheus gauges live in the backend process's memory, so they must be
refreshed *inside* the backend (not the arq worker). ``collect_business_snapshot``
runs the DB queries and is pure-ish (returns a dataclass) so it can be unit
tested without touching the Prometheus registry; ``apply_snapshot`` pushes a
snapshot onto the gauges; ``run_refresh_loop`` ties them together on a timer
started from the FastAPI lifespan.
"""

from __future__ import annotations

import asyncio
import time
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from sqlalchemy import func, select

from app.core import metrics as m
from app.core.logging import logger
from app.models.alert import OPEN_STATUSES, Alert
from app.models.cve import CveRecord, Vulnerability
from app.models.machine import Machine
from app.models.software import SoftwareRecord
from app.services import license_service
from app.services.machine_service import STALE_WITHIN, online_cut_expr

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

_SEVERITIES = ("critical", "high", "medium", "low")
_LICENSE_STATUSES = ("compliant", "over_used", "expiring_soon", "expired")


@dataclass(frozen=True)
class BusinessSnapshot:
    machines: dict[str, int]  # online / stale / offline
    software_unique: int
    alerts_open: dict[str, int]  # severity -> count
    vulnerabilities_open: dict[str, int]  # severity -> count
    licenses: dict[str, int]  # status -> count


async def collect_business_snapshot(session: AsyncSession) -> BusinessSnapshot:
    now = datetime.now(tz=UTC)
    online_expr = online_cut_expr(now)
    stale_cut = now - STALE_WITHIN

    base = select(func.count()).select_from(Machine).where(Machine.deleted_at.is_(None))
    total = (await session.execute(base)).scalar_one()
    online = (
        await session.execute(base.where(Machine.last_seen_at >= online_expr))
    ).scalar_one()
    stale = (
        await session.execute(
            base.where(Machine.last_seen_at < online_expr, Machine.last_seen_at >= stale_cut)
        )
    ).scalar_one()
    # Offline absorbs never-seen machines (last_seen_at IS NULL).
    offline = int(total) - int(online) - int(stale)

    software_unique = (
        await session.execute(
            select(func.count(func.distinct(func.lower(SoftwareRecord.name))))
            .select_from(SoftwareRecord)
            .join(Machine, Machine.id == SoftwareRecord.machine_id)
            .where(SoftwareRecord.uninstalled_at.is_(None), Machine.deleted_at.is_(None))
        )
    ).scalar_one()

    alert_rows = (
        await session.execute(
            select(Alert.severity, func.count())
            .where(Alert.status.in_(OPEN_STATUSES))
            .group_by(Alert.severity)
        )
    ).all()
    alert_counts = {sev: int(c) for sev, c in alert_rows}

    vuln_rows = (
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
    vuln_counts = {sev: int(c) for sev, c in vuln_rows}

    summary = await license_service.compliance_summary(session)
    license_counts = {
        "compliant": summary["compliant"],
        "over_used": summary["over_used"],
        "expired": summary["expired"],
        # compliance_summary splits expiring into 30/60/90 sub-buckets.
        "expiring_soon": summary["expiring_30"] + summary["expiring_60"] + summary["expiring_90"],
    }

    return BusinessSnapshot(
        machines={"online": int(online), "stale": int(stale), "offline": offline},
        software_unique=int(software_unique),
        alerts_open={sev: alert_counts.get(sev, 0) for sev in _SEVERITIES},
        vulnerabilities_open={sev: vuln_counts.get(sev, 0) for sev in _SEVERITIES},
        licenses={st: license_counts.get(st, 0) for st in _LICENSE_STATUSES},
    )


def apply_snapshot(snapshot: BusinessSnapshot) -> None:
    for status, value in snapshot.machines.items():
        m.machines.labels(status=status).set(value)
    m.software_unique.set(snapshot.software_unique)
    for sev in _SEVERITIES:
        m.alerts_open.labels(severity=sev).set(snapshot.alerts_open.get(sev, 0))
        m.vulnerabilities_open.labels(severity=sev).set(snapshot.vulnerabilities_open.get(sev, 0))
    for status in _LICENSE_STATUSES:
        m.licenses.labels(status=status).set(snapshot.licenses.get(status, 0))
    m.business_metrics_refreshed_timestamp_seconds.set(time.time())


async def refresh(session: AsyncSession) -> BusinessSnapshot:
    snapshot = await collect_business_snapshot(session)
    apply_snapshot(snapshot)
    return snapshot


async def run_refresh_loop(
    session_factory: async_sessionmaker[AsyncSession],
    interval_seconds: int,
) -> None:
    """Refresh business gauges every ``interval_seconds`` until cancelled."""
    while True:
        try:
            async with session_factory() as session:
                await refresh(session)
        except asyncio.CancelledError:
            raise
        except Exception:
            logger.exception("metrics.refresh_failed")
        await asyncio.sleep(interval_seconds)
