"""Machine listing/detail/admin logic for the dashboard.

Status is derived from last_seen_at at query time (protocol §2 stale detection),
never stored, so a quiet agent ages to stale→offline without a writer.
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING

from sqlalchemy import String, and_, cast, func, or_, select
from sqlalchemy.sql.elements import ColumnElement

from app.models.agent_config import AgentConfig
from app.models.machine import Machine
from app.models.software import SoftwareRecord
from app.services import risk_service

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

ONLINE_WITHIN = timedelta(minutes=5)
STALE_WITHIN = timedelta(hours=1)

_SORTABLE = {
    "hostname": Machine.hostname,
    "last_seen_at": Machine.last_seen_at,
    "last_scan_at": Machine.last_scan_at,
    "enrolled_at": Machine.enrolled_at,
}


def compute_status(last_seen_at: datetime | None, now: datetime) -> str:
    if last_seen_at is None:
        return "offline"
    # SQLite (test backend) hands back naive datetimes; treat them as UTC.
    if last_seen_at.tzinfo is None:
        last_seen_at = last_seen_at.replace(tzinfo=UTC)
    delta = now - last_seen_at
    if delta < ONLINE_WITHIN:
        return "online"
    if delta < STALE_WITHIN:
        return "stale"
    return "offline"


def _status_condition(status: str, now: datetime) -> ColumnElement[bool] | None:
    online_cut = now - ONLINE_WITHIN
    stale_cut = now - STALE_WITHIN
    if status == "online":
        return Machine.last_seen_at >= online_cut
    if status == "stale":
        return and_(Machine.last_seen_at >= stale_cut, Machine.last_seen_at < online_cut)
    if status == "offline":
        return or_(Machine.last_seen_at < stale_cut, Machine.last_seen_at.is_(None))
    return None


async def _active_software_counts(session: AsyncSession, machine_ids: list[int]) -> dict[int, int]:
    if not machine_ids:
        return {}
    rows = await session.execute(
        select(SoftwareRecord.machine_id, func.count())
        .where(
            SoftwareRecord.machine_id.in_(machine_ids),
            SoftwareRecord.uninstalled_at.is_(None),
        )
        .group_by(SoftwareRecord.machine_id)
    )
    return dict(rows.tuples().all())


async def list_machines(
    session: AsyncSession,
    *,
    q: str | None = None,
    status: str | None = None,
    os: str | None = None,
    tag: str | None = None,
    page: int = 1,
    page_size: int = 50,
    sort: str | None = None,
) -> tuple[list[dict[str, object]], int]:
    now = datetime.now(tz=UTC)
    conditions: list[ColumnElement[bool]] = [Machine.deleted_at.is_(None)]
    if q:
        # Match the friendly name + owner too, so admins can find a box by who
        # uses it ("Pranee") not just its raw hostname ("DESKTOP-AB12").
        like = f"%{q}%"
        conditions.append(
            or_(
                Machine.hostname.ilike(like),
                Machine.display_name.ilike(like),
                Machine.owner.ilike(like),
            )
        )
    if os:
        conditions.append(Machine.os == os)
    if tag:
        # Portable JSON-array membership: match the quoted tag in the serialized
        # list. Works on SQLite (JSON-as-text) and Postgres (JSONB cast to text).
        conditions.append(cast(Machine.tags, String).ilike(f'%"{tag}"%'))
    if status:
        cond = _status_condition(status, now)
        if cond is not None:
            conditions.append(cond)

    total = (
        await session.execute(select(func.count()).select_from(Machine).where(*conditions))
    ).scalar_one()

    column = _SORTABLE.get((sort or "").lstrip("-"), Machine.last_seen_at)
    order = column.desc() if not sort or sort.startswith("-") else column.asc()

    rows = (
        (
            await session.execute(
                select(Machine)
                .where(*conditions)
                .order_by(order, Machine.id.desc())
                .offset((page - 1) * page_size)
                .limit(page_size)
            )
        )
        .scalars()
        .all()
    )

    counts = await _active_software_counts(session, [m.id for m in rows])
    items = [_to_item(m, now, counts.get(m.id, 0)) for m in rows]
    return items, total


def _to_item(machine: Machine, now: datetime, software_count: int) -> dict[str, object]:
    return {
        "uuid": machine.uuid,
        "hostname": machine.hostname,
        "display_name": machine.display_name,
        "owner": machine.owner,
        "os": machine.os,
        "os_version": machine.os_version,
        "arch": machine.arch,
        "agent_version": machine.agent_version,
        "reported_server_url": machine.reported_server_url,
        "status": compute_status(machine.last_seen_at, now),
        "last_seen_at": machine.last_seen_at,
        "last_scan_at": machine.last_scan_at,
        "enrolled_at": machine.enrolled_at,
        "tags": machine.tags,
        "software_count": software_count,
        "risk_score": machine.risk_score,
        "scan_progress": _scan_progress(machine),
    }


def _scan_progress(machine: Machine) -> dict[str, object] | None:
    """Build the scan-progress payload, or None when the machine is idle.

    Only an actively-scanning machine (phase set and not "idle") shows a bar.
    """
    phase = machine.scan_phase
    if not phase or phase == "idle":
        return None
    return {
        "phase": phase,
        "done": machine.scan_done or 0,
        "total": machine.scan_total or 0,
        "current_path": machine.scan_current_path,
        "updated_at": machine.scan_progress_at,
    }


async def get_machine(session: AsyncSession, machine_uuid: uuid_lib.UUID) -> Machine | None:
    return (
        await session.execute(
            select(Machine).where(Machine.uuid == machine_uuid, Machine.deleted_at.is_(None))
        )
    ).scalar_one_or_none()


async def machine_detail(session: AsyncSession, machine: Machine) -> dict[str, object]:
    now = datetime.now(tz=UTC)
    counts = await _active_software_counts(session, [machine.id])
    item = _to_item(machine, now, counts.get(machine.id, 0))
    breakdown = await risk_service.risk_breakdown(session, machine)
    item["vulnerability_count"] = {
        "critical": breakdown["cve_critical"],
        "high": breakdown["cve_high"],
        "medium": breakdown["cve_medium"],
        "low": breakdown["cve_low"],
    }
    return item


async def request_scan(session: AsyncSession, machine: Machine) -> None:
    cfg = await session.get(AgentConfig, machine.id)
    if cfg is None:
        cfg = AgentConfig(machine_id=machine.id)
        session.add(cfg)
    cfg.manual_scan_requested = True
