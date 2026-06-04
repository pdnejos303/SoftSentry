"""License compliance: CRUD support, live install counts, status calculation,
installation drill-down, and alert reconciliation (Module 6).

Install counts are computed **live** from the inventory (SQL ILIKE on
`software_name`, DISTINCT live machine) rather than materialized — correct by
construction and cheap at the scale we target. The alert reconciliation drives
the daily `license_compliance_check` worker and the manual refresh endpoint.
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, date, datetime
from typing import TYPE_CHECKING

from sqlalchemy import and_, func, select

from app.models.alert import OPEN_STATUSES, STATUS_RESOLVED, Alert
from app.models.license import License
from app.models.machine import Machine
from app.models.software import SoftwareRecord

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

# ── Statuses (mirror schemas.license.LicenseStatus) ───────────────────────────
STATUS_COMPLIANT = "compliant"
STATUS_OVER_USED = "over_used"
STATUS_EXPIRED = "expired"
STATUS_EXPIRING = "expiring_soon"

# Alert types (land in the shared `alerts` table).
TYPE_EXPIRING = "license_expiring"
TYPE_EXPIRED = "license_expired"
TYPE_OVERUSED = "license_overused"
_LICENSE_ALERT_TYPES = (TYPE_EXPIRING, TYPE_EXPIRED, TYPE_OVERUSED)

# Days-until-expiry that opens the "expiring soon" window.
_EXPIRING_WINDOW = 90


# ── Pure helpers (unit-tested in tests/services/test_license_service.py) ──────
def days_until(expires_at: date | None, today: date) -> int | None:
    if expires_at is None:
        return None
    return (expires_at - today).days


def compute_status(
    installed: int, purchased: int, expires_at: date | None, today: date
) -> str:
    """Single badge status. Precedence: expired > over_used > expiring_soon."""
    d = days_until(expires_at, today)
    if d is not None and d < 0:
        return STATUS_EXPIRED
    if installed > purchased:
        return STATUS_OVER_USED
    if d is not None and d <= _EXPIRING_WINDOW:
        return STATUS_EXPIRING
    return STATUS_COMPLIANT


def expiring_subbucket(days: int) -> int:
    """Coarse 30/60/90 sub-class for the compliance widget."""
    for m in (30, 60, 90):
        if days <= m:
            return m
    return 90


def expiry_severity(days: int) -> str:
    if days <= 7:
        return "high"
    if days <= 30:
        return "medium"
    return "low"


def _desired_alert(installed: int, lic: License, today: date) -> tuple[str, str, str, str] | None:
    """The single open alert a license should have, as
    (type, dedup_key, severity, detail) — or None when compliant.

    dedup_key bands let a license escalate (90→30→7) with one alert per band and
    auto-resolve looser bands; renewal (status→compliant) resolves everything.
    """
    d = days_until(lic.expires_at, today)
    if d is not None and d < 0:
        return (TYPE_EXPIRED, "expired", "critical", f"Expired {-d} day(s) ago")
    if installed > lic.purchased_count:
        over = installed - lic.purchased_count
        return (
            TYPE_OVERUSED,
            "overused",
            "high",
            f"{installed} installed vs {lic.purchased_count} purchased ({over} over)",
        )
    if d is not None and d <= _EXPIRING_WINDOW:
        band = expiring_subbucket(d)
        return (
            TYPE_EXPIRING,
            f"expiring_{band}",
            expiry_severity(d),
            f"Expires in {d} day(s) ({lic.expires_at})",
        )
    return None


# ── Live install counts ───────────────────────────────────────────────────────
async def installed_counts(
    session: AsyncSession, license_ids: list[int]
) -> dict[int, int]:
    """{license_id: distinct live machines whose active software matches}."""
    if not license_ids:
        return {}
    stmt = (
        select(License.id, func.count(func.distinct(Machine.id)))
        .select_from(License)
        .outerjoin(
            SoftwareRecord,
            and_(
                SoftwareRecord.name.ilike(License.software_name),
                SoftwareRecord.uninstalled_at.is_(None),
            ),
        )
        .outerjoin(
            Machine,
            and_(Machine.id == SoftwareRecord.machine_id, Machine.deleted_at.is_(None)),
        )
        .where(License.id.in_(license_ids))
        .group_by(License.id)
    )
    rows = (await session.execute(stmt)).all()
    return {lic_id: int(count) for lic_id, count in rows}


def to_out(lic: License, installed: int, today: date) -> dict[str, object]:
    """Build the LicenseOut payload (status + counts computed)."""
    return {
        "uuid": lic.uuid,
        "software_name": lic.software_name,
        "publisher": lic.publisher,
        "purchased_count": lic.purchased_count,
        "purchased_at": lic.purchased_at,
        "expires_at": lic.expires_at,
        "cost_total": float(lic.cost_total) if lic.cost_total is not None else None,
        "currency": lic.currency,
        "vendor_contact": lic.vendor_contact,
        "notes": lic.notes,
        "has_license_key": lic.license_key is not None,
        "installed_count": installed,
        "status": compute_status(installed, lic.purchased_count, lic.expires_at, today),
        "days_until_expiry": days_until(lic.expires_at, today),
        "created_at": lic.created_at,
        "updated_at": lic.updated_at,
    }


# ── List / get / create / update / delete ─────────────────────────────────────
async def list_licenses(
    session: AsyncSession,
    *,
    q: str | None,
    status_filter: str | None,
    page: int,
    page_size: int,
) -> tuple[list[dict[str, object]], int]:
    stmt = select(License)
    if q:
        like = f"%{q}%"
        stmt = stmt.where(
            License.software_name.ilike(like) | License.publisher.ilike(like)
        )
    licenses = list((await session.execute(stmt.order_by(License.software_name))).scalars().all())
    counts = await installed_counts(session, [lic.id for lic in licenses])
    today = date.today()

    items = [to_out(lic, counts.get(lic.id, 0), today) for lic in licenses]
    if status_filter:
        items = [i for i in items if i["status"] == status_filter]
    total = len(items)
    start = (page - 1) * page_size
    return items[start : start + page_size], total


async def get_license(session: AsyncSession, license_uuid: uuid_lib.UUID) -> License | None:
    return (
        await session.execute(select(License).where(License.uuid == license_uuid))
    ).scalar_one_or_none()


async def detail(session: AsyncSession, lic: License) -> dict[str, object]:
    counts = await installed_counts(session, [lic.id])
    out = to_out(lic, counts.get(lic.id, 0), date.today())
    out["license_key"] = lic.license_key
    return out


async def create_license(session: AsyncSession, data: dict[str, object]) -> License:
    lic = License(**data)
    session.add(lic)
    await session.flush()
    return lic


def update_license(lic: License, data: dict[str, object]) -> None:
    for key, value in data.items():
        setattr(lic, key, value)


async def delete_license(session: AsyncSession, lic: License) -> None:
    await session.delete(lic)


# ── Installation drill-down ───────────────────────────────────────────────────
async def installations(
    session: AsyncSession, lic: License
) -> list[dict[str, object]]:
    rows = (
        await session.execute(
            select(
                Machine.uuid,
                Machine.hostname,
                Machine.os,
                SoftwareRecord.name,
                SoftwareRecord.version,
                Machine.last_seen_at,
            )
            .join(SoftwareRecord, SoftwareRecord.machine_id == Machine.id)
            .where(
                SoftwareRecord.name.ilike(lic.software_name),
                SoftwareRecord.uninstalled_at.is_(None),
                Machine.deleted_at.is_(None),
            )
            .order_by(Machine.hostname)
        )
    ).all()

    # One row per machine (a machine may carry two matching SKUs — spec edge 5).
    seen: set[uuid_lib.UUID] = set()
    items: list[dict[str, object]] = []
    for m_uuid, hostname, os_name, sw_name, sw_version, last_seen in rows:
        if m_uuid in seen:
            continue
        seen.add(m_uuid)
        items.append(
            {
                "machine_uuid": m_uuid,
                "hostname": hostname,
                "os": os_name,
                "software_name": sw_name,
                "software_version": sw_version,
                "last_seen_at": last_seen,
            }
        )
    return items


# ── Compliance summary ────────────────────────────────────────────────────────
async def compliance_summary(session: AsyncSession) -> dict[str, int]:
    licenses = list((await session.execute(select(License))).scalars().all())
    counts = await installed_counts(session, [lic.id for lic in licenses])
    today = date.today()

    tally = {
        "compliant": 0,
        "over_used": 0,
        "expired": 0,
        "expiring_30": 0,
        "expiring_60": 0,
        "expiring_90": 0,
    }
    for lic in licenses:
        installed = counts.get(lic.id, 0)
        status = compute_status(installed, lic.purchased_count, lic.expires_at, today)
        if status == STATUS_EXPIRING:
            d = days_until(lic.expires_at, today)
            assert d is not None  # expiring_soon implies a future expiry date
            tally[f"expiring_{expiring_subbucket(d)}"] += 1
        else:
            tally[status] += 1

    total = len(licenses)
    rate = round(tally["compliant"] / total * 100) if total else 0
    return {"total": total, "compliance_rate": rate, **tally}


# ── Alert reconciliation (worker + manual refresh) ────────────────────────────
async def reconcile_alerts(session: AsyncSession) -> dict[str, int]:
    """Ensure each license has exactly the one open alert its status implies
    (or none). Escalation across bands and renewal both resolve stale alerts.
    Caller commits."""
    licenses = list((await session.execute(select(License))).scalars().all())
    counts = await installed_counts(session, [lic.id for lic in licenses])
    today = date.today()
    now = datetime.now(tz=UTC)

    open_alerts = (
        (
            await session.execute(
                select(Alert).where(
                    Alert.type.in_(_LICENSE_ALERT_TYPES),
                    Alert.status.in_(OPEN_STATUSES),
                )
            )
        )
        .scalars()
        .all()
    )
    by_license: dict[int | None, list[Alert]] = {}
    for a in open_alerts:
        by_license.setdefault(a.license_id, []).append(a)

    raised = 0
    resolved = 0
    for lic in licenses:
        desired = _desired_alert(counts.get(lic.id, 0), lic, today)
        current = by_license.get(lic.id, [])
        keep: Alert | None = None
        if desired is not None:
            a_type, key, severity, detail = desired
            title = f"License: {lic.software_name}"
            keep = next(
                (a for a in current if a.type == a_type and a.dedup_key == key), None
            )
            if keep is not None:
                keep.severity = severity
                keep.detail = detail
                keep.title = title
            else:
                session.add(
                    Alert(
                        license_id=lic.id,
                        type=a_type,
                        dedup_key=key,
                        severity=severity,
                        title=title,
                        detail=detail,
                    )
                )
                raised += 1
        # Resolve every other open license alert for this license.
        for a in current:
            if a is keep:
                continue
            a.status = STATUS_RESOLVED
            a.resolved_at = now
            resolved += 1

    await session.flush()
    return {"licenses": len(licenses), "alerts_raised": raised, "alerts_resolved": resolved}
