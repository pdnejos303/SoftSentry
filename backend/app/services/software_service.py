"""Inventory query logic — per-machine (A3) and cross-machine (A4).

Aggregation for the cross-machine view is done in Python after a filtered fetch.
That keeps the (name, version) grouping + capped machine list portable across
SQLite (tests/local) and Postgres (prod) without tuple-IN gymnastics; fine at
the fleet scale we target. Revisit with SQL-side grouping if it gets large.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from sqlalchemy import ColumnElement, func, select

from app.models.machine import Machine
from app.models.scan import Scan
from app.models.signature import SignatureRecord
from app.models.software import SoftwareHistory, SoftwareRecord

if TYPE_CHECKING:
    from collections.abc import Sequence

    from sqlalchemy import Row
    from sqlalchemy.ext.asyncio import AsyncSession

_MACHINES_CAP = 10


# ── A3: per-machine software ────────────────────────────────────────────────


async def machine_software(
    session: AsyncSession,
    machine: Machine,
    *,
    q: str | None = None,
    signature_status: str | None = None,
    include_inactive: bool = False,
    page: int = 1,
    page_size: int = 50,
) -> tuple[list[dict[str, object]], int]:
    conditions = [SoftwareRecord.machine_id == machine.id]
    if not include_inactive:
        conditions.append(SoftwareRecord.uninstalled_at.is_(None))
    if q:
        conditions.append(SoftwareRecord.name.ilike(f"%{q}%"))
    if signature_status:
        conditions.append(SignatureRecord.status == signature_status)

    base = (
        select(SoftwareRecord, SignatureRecord.status)
        .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
        .where(*conditions)
    )

    total = (await session.execute(select(func.count()).select_from(base.subquery()))).scalar_one()

    rows = (
        await session.execute(
            base.order_by(SoftwareRecord.name.asc(), SoftwareRecord.version.asc())
            .offset((page - 1) * page_size)
            .limit(page_size)
        )
    ).all()

    items = [
        {
            "uuid": rec.uuid,
            "name": rec.name,
            "version": rec.version,
            "publisher": rec.publisher,
            "install_date": rec.install_date,
            "install_path": rec.install_path,
            "install_size_kb": rec.install_size_kb,
            "arch": rec.arch,
            "source": rec.source,
            "signature_status": sig_status,
            "is_active": rec.uninstalled_at is None,
        }
        for rec, sig_status in rows
    ]
    return items, int(total)


async def machine_history(
    session: AsyncSession, machine_id: int, *, limit: int = 100
) -> tuple[list[dict[str, object]], int]:
    total = (
        await session.execute(
            select(func.count())
            .select_from(SoftwareHistory)
            .where(SoftwareHistory.machine_id == machine_id)
        )
    ).scalar_one()

    rows = (
        (
            await session.execute(
                select(SoftwareHistory)
                .where(SoftwareHistory.machine_id == machine_id)
                .order_by(SoftwareHistory.occurred_at.desc(), SoftwareHistory.id.desc())
                .limit(limit)
            )
        )
        .scalars()
        .all()
    )
    items: list[dict[str, object]] = [
        {
            "software_name": h.software_name,
            "software_version": h.software_version,
            "event": h.event,
            "previous_version": h.previous_version,
            "occurred_at": h.occurred_at,
        }
        for h in rows
    ]
    return items, int(total)


async def machine_scans(
    session: AsyncSession, machine_id: int, *, limit: int = 50
) -> tuple[list[dict[str, object]], int]:
    total = (
        await session.execute(
            select(func.count()).select_from(Scan).where(Scan.machine_id == machine_id)
        )
    ).scalar_one()

    rows = (
        (
            await session.execute(
                select(Scan)
                .where(Scan.machine_id == machine_id)
                .order_by(Scan.received_at.desc(), Scan.id.desc())
                .limit(limit)
            )
        )
        .scalars()
        .all()
    )
    items: list[dict[str, object]] = [
        {
            "uuid": s.uuid,
            "started_at": s.started_at,
            "completed_at": s.completed_at,
            "received_at": s.received_at,
            "software_count": s.software_count,
            "scan_type": s.scan_type,
            "trigger": s.trigger,
        }
        for s in rows
    ]
    return items, int(total)


# ── A4: cross-machine ───────────────────────────────────────────────────────


async def _active_rows(
    session: AsyncSession,
    *,
    q: str | None = None,
    publisher: str | None = None,
    signature_status: str | None = None,
) -> Sequence[Row[Any]]:
    conditions: list[ColumnElement[bool]] = [
        SoftwareRecord.uninstalled_at.is_(None),
        Machine.deleted_at.is_(None),
    ]
    if q:
        conditions.append(SoftwareRecord.name.ilike(f"%{q}%"))
    if publisher:
        conditions.append(SoftwareRecord.publisher.ilike(f"%{publisher}%"))
    if signature_status:
        conditions.append(SignatureRecord.status == signature_status)

    stmt = (
        select(
            SoftwareRecord.name,
            SoftwareRecord.version,
            SoftwareRecord.publisher,
            Machine.uuid,
            SignatureRecord.status,
        )
        .join(Machine, Machine.id == SoftwareRecord.machine_id)
        .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
        .where(*conditions)
    )
    return (await session.execute(stmt)).all()


async def cross_software(
    session: AsyncSession,
    *,
    q: str | None = None,
    publisher: str | None = None,
    signature_status: str | None = None,
    page: int = 1,
    page_size: int = 50,
) -> tuple[list[dict[str, object]], int]:
    rows = await _active_rows(session, q=q, publisher=publisher, signature_status=signature_status)

    grouped: dict[tuple[str, str], dict[str, object]] = {}
    for name, version, pub, machine_uuid, sig_status in rows:
        key = (name, version)
        g = grouped.setdefault(
            key,
            {"name": name, "version": version, "publisher": pub, "machines": set(), "sig": None},
        )
        g["machines"].add(machine_uuid)  # type: ignore[attr-defined]
        if pub and not g["publisher"]:
            g["publisher"] = pub
        if sig_status and not g["sig"]:
            g["sig"] = sig_status

    ordered = sorted(
        grouped.values(),
        key=lambda g: (-len(g["machines"]), g["name"], g["version"]),  # type: ignore[arg-type]
    )
    total = len(ordered)
    page_slice = ordered[(page - 1) * page_size : (page - 1) * page_size + page_size]

    items = [
        {
            "name": g["name"],
            "version": g["version"],
            "publisher": g["publisher"],
            "installed_count": len(g["machines"]),  # type: ignore[arg-type]
            "machines": list(g["machines"])[:_MACHINES_CAP],  # type: ignore[call-overload]
            "signature_status": g["sig"],
        }
        for g in page_slice
    ]
    return items, total


async def top_software(session: AsyncSession, *, limit: int = 10) -> list[dict[str, object]]:
    install_count = func.count(func.distinct(SoftwareRecord.machine_id))
    rows = (
        await session.execute(
            select(SoftwareRecord.name, func.max(SoftwareRecord.publisher), install_count)
            .where(SoftwareRecord.uninstalled_at.is_(None))
            .group_by(SoftwareRecord.name)
            .order_by(install_count.desc(), SoftwareRecord.name)
            .limit(limit)
        )
    ).all()
    return [
        {"name": name, "publisher": pub, "installed_count": int(count)} for name, pub, count in rows
    ]


async def _active_pairs(session: AsyncSession, machine_id: int) -> dict[str, set[str]]:
    rows = (
        await session.execute(
            select(SoftwareRecord.name, SoftwareRecord.version).where(
                SoftwareRecord.machine_id == machine_id,
                SoftwareRecord.uninstalled_at.is_(None),
            )
        )
    ).all()
    by_name: dict[str, set[str]] = {}
    for name, version in rows:
        by_name.setdefault(name, set()).add(version)
    return by_name


async def compare(
    session: AsyncSession, machine_a: Machine, machine_b: Machine
) -> dict[str, list[dict[str, str]]]:
    a = await _active_pairs(session, machine_a.id)
    b = await _active_pairs(session, machine_b.id)

    common: list[dict[str, str]] = []
    only_in_a: list[dict[str, str]] = []
    only_in_b: list[dict[str, str]] = []
    version_diff: list[dict[str, str]] = []

    for name in sorted(set(a) | set(b)):
        av = sorted(a.get(name, set()))
        bv = sorted(b.get(name, set()))
        if av and not bv:
            only_in_a.extend({"name": name, "version": v} for v in av)
        elif bv and not av:
            only_in_b.extend({"name": name, "version": v} for v in bv)
        elif set(av) == set(bv):
            common.extend({"name": name, "version": v} for v in av)
        else:
            version_diff.append({"name": name, "version_a": av[0], "version_b": bv[0]})

    return {
        "common": common,
        "only_in_a": only_in_a,
        "only_in_b": only_in_b,
        "version_diff": version_diff,
    }
