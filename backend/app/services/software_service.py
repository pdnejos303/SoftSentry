"""Inventory query logic — per-machine (A3) and cross-machine (A4).

Aggregation for the cross-machine view is done in Python after a filtered fetch.
That keeps the (name, version) grouping + capped machine list portable across
SQLite (tests/local) and Postgres (prod) without tuple-IN gymnastics; fine at
the fleet scale we target. Revisit with SQL-side grouping if it gets large.
"""

from __future__ import annotations

import fnmatch
from datetime import UTC, date, datetime, time, timedelta
from typing import TYPE_CHECKING, Any

from sqlalchemy import ColumnElement, and_, case, func, or_, select

from app.models.license import License
from app.models.machine import Machine
from app.models.policy import BlacklistEntry
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

    raw = [
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
    items = await _enrich_license_status(session, raw)
    return items, int(total)


async def machine_history(
    session: AsyncSession, machine_id: int, *, page: int = 1, page_size: int = 50
) -> tuple[list[dict[str, object]], int]:
    total = (
        await session.execute(
            select(func.count())
            .select_from(SoftwareHistory)
            .where(SoftwareHistory.machine_id == machine_id)
        )
    ).scalar_one()

    rows = (
        (await session.execute(
            select(SoftwareHistory)
            .where(SoftwareHistory.machine_id == machine_id)
            .order_by(SoftwareHistory.occurred_at.desc(), SoftwareHistory.id.desc())
            .offset((page - 1) * page_size)
            .limit(page_size)
        )).scalars().all()
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
    session: AsyncSession, machine_id: int, *, page: int = 1, page_size: int = 50
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
                .offset((page - 1) * page_size)   
                .limit(page_size)                 
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


# ── License enrichment ──────────────────────────────────────────────────────

_EXPIRING_WINDOW = 90


async def _enrich_license_status(
    session: AsyncSession,
    items: list[dict[str, object]],
) -> list[dict[str, object]]:
    """Attach license_status to each item — None when no license covers it."""
    licenses = list((await session.execute(select(License))).scalars().all())
    if not licenses:
        return [{**item, "license_status": None} for item in items]

    # Count distinct live machines per license (reuse same query as license_service).
    from sqlalchemy import and_

    count_rows = (
        await session.execute(
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
            .where(License.id.in_([lic.id for lic in licenses]))
            .group_by(License.id)
        )
    ).all()
    installed_map: dict[int, int] = {lic_id: int(cnt) for lic_id, cnt in count_rows}

    today = date.today()

    def _status(lic: License, installed: int) -> str:
        d = (lic.expires_at - today).days if lic.expires_at else None
        if d is not None and d < 0:
            return "expired"
        if installed > lic.purchased_count:
            return "over_used"
        if d is not None and d <= _EXPIRING_WINDOW:
            return "expiring_soon"
        return "compliant"

    result: list[dict[str, object]] = []
    for item in items:
        sw_name = str(item["name"])
        matched_status: str | None = None
        for lic in licenses:
            pattern = lic.software_name.replace("%", "*").replace("_", "?")
            if fnmatch.fnmatch(sw_name.lower(), pattern.lower()):
                matched_status = _status(lic, installed_map.get(lic.id, 0))
                break
        result.append({**item, "license_status": matched_status})
    return result


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
        like = f"%{q}%"
        conditions.append(
            or_(SoftwareRecord.name.ilike(like), SoftwareRecord.publisher.ilike(like),)
        )
    if signature_status:
        conditions.append(_sig_condition(signature_status))

    stmt = (
        select(
            SoftwareRecord.name,
            SoftwareRecord.version,
            SoftwareRecord.publisher,
            Machine.uuid,
            Machine.hostname,
            Machine.display_name,
            SignatureRecord.status,
        )
        .join(Machine, Machine.id == SoftwareRecord.machine_id)
        .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
        .where(*conditions)
    )
    return (await session.execute(stmt)).all()


def _sig_condition(signature_status: str) -> ColumnElement[bool]:
    """Translate a UI signature filter to a SQL predicate. ``unsigned`` folds in
    rows that have no signature record at all (NULL status), so "show me
    everything unsigned" really means everything we couldn't trust by signature.
    """
    if signature_status == "unsigned":
        return (SignatureRecord.status == "unsigned") | (SignatureRecord.status.is_(None))
    return SignatureRecord.status == signature_status


# Sort keys accepted by the cross-machine list. Default mirrors the original
# behaviour: most-installed first, then name/version for a stable order.
_SORT_KEYS: dict[str, Any] = {
    "installs": lambda g: (-len(g["machines"]), g["name"], g["version"]),
    "-installs": lambda g: (len(g["machines"]), g["name"], g["version"]),
    "name": lambda g: (g["name"].lower(), g["version"]),
    "-name": lambda g: _ReverseStr(g["name"].lower()),
}


class _ReverseStr:
    """Wrap a string so a plain ``sorted`` produces descending order — avoids a
    second pass and keeps the sort stable for ties (handled by Python's sort)."""

    __slots__ = ("s",)

    def __init__(self, s: str) -> None:
        self.s = s

    def __lt__(self, other: "_ReverseStr") -> bool:
        return self.s > other.s


async def cross_software(
    session: AsyncSession,
    *,
    q: str | None = None,
    publisher: str | None = None,
    signature_status: str | None = None,
    sort: str = "installs",
    page: int = 1,
    page_size: int = 50,
) -> tuple[list[dict[str, object]], int]:
    rows = await _active_rows(session, q=q, publisher=publisher, signature_status=signature_status)

    grouped: dict[tuple[str, str], dict[str, object]] = {}
    for name, version, pub, machine_uuid, hostname, display_name, sig_status in rows:
        key = (name, version)
        g = grouped.setdefault(
            key,
            {"name": name, "version": version, "publisher": pub, "machines": {}, "sig": None},
        )
        # uuid → readable name (display name wins, else hostname) for the drill-down.
        g["machines"][machine_uuid] = display_name or hostname  # type: ignore[index]
        if pub and not g["publisher"]:
            g["publisher"] = pub
        if sig_status and not g["sig"]:
            g["sig"] = sig_status

    key_fn = _SORT_KEYS.get(sort, _SORT_KEYS["installs"])
    ordered = sorted(grouped.values(), key=key_fn)  # type: ignore[arg-type]
    total = len(ordered)
    page_slice = ordered[(page - 1) * page_size : (page - 1) * page_size + page_size]

    items = [
        {
            "name": g["name"],
            "version": g["version"],
            "publisher": g["publisher"],
            "installed_count": len(g["machines"]),  # type: ignore[arg-type]
            "machines": [
                {"uuid": uuid, "name": name}
                for uuid, name in list(g["machines"].items())[:_MACHINES_CAP]  # type: ignore[union-attr]
            ],
            "signature_status": g["sig"],
        }
        for g in page_slice
    ]
    return items, total


async def software_stats(
    session: AsyncSession,
    *,
    q: str | None = None,
    publisher: str | None = None,
) -> dict[str, int]:
    """Fleet-wide posture summary for the inventory header strip. Counts are over
    active install instances (one app on one machine = one install), honouring
    the name/publisher search so the strip tracks what the table shows.

    - ``unsigned`` folds in installs with no signature at all (NULL), matching
      the ``unsigned`` filter — that's the number an admin actually cares about.
    """
    conditions: list[ColumnElement[bool]] = [
        SoftwareRecord.uninstalled_at.is_(None),
        Machine.deleted_at.is_(None),
    ]
    if q:
        conditions.append(SoftwareRecord.name.ilike(f"%{q}%"))
    if publisher:
        conditions.append(SoftwareRecord.publisher.ilike(f"%{publisher}%"))

    status = SignatureRecord.status
    # case(...) instead of count(...).filter(...) so the conditional counts stay
    # portable to older SQLite (no FILTER clause) as well as Postgres.
    row = (
        await session.execute(
            select(
                func.count(func.distinct(SoftwareRecord.name)),
                func.count(),
                func.sum(case((status == "valid", 1), else_=0)),
                func.sum(case(((status == "unsigned") | (status.is_(None)), 1), else_=0)),
                func.sum(case((status == "invalid", 1), else_=0)),
            )
            .select_from(SoftwareRecord)
            .join(Machine, Machine.id == SoftwareRecord.machine_id)
            .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
            .where(*conditions)
        )
    ).one()

    unique_apps, total_installs, valid, unsigned, invalid = row
    return {
        "unique_apps": int(unique_apps or 0),
        "total_installs": int(total_installs or 0),
        "valid": int(valid or 0),
        "unsigned": int(unsigned or 0),
        "invalid": int(invalid or 0),
    }


async def top_software(
    session: AsyncSession, *, limit: int = 10, risk: bool = False
) -> list[dict[str, object]]:
    """Top-N software by fleet spread. With ``risk=True`` the ranking is limited
    to apps that look untrustworthy by signature (unsigned / no signature /
    invalid) — the security-posture cut of "what's spreading that we can't
    vouch for", rather than the plain most-installed list.
    """
    install_count = func.count(func.distinct(SoftwareRecord.machine_id))
    conditions: list[ColumnElement[bool]] = [
        SoftwareRecord.uninstalled_at.is_(None),
        Machine.deleted_at.is_(None),
    ]
    stmt = (
        select(SoftwareRecord.name, func.max(SoftwareRecord.publisher), install_count)
        .join(Machine, Machine.id == SoftwareRecord.machine_id)
        .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
    )
    if risk:
        status = SignatureRecord.status
        conditions.append(
            (status == "unsigned") | (status == "invalid") | (status.is_(None))
        )
    rows = (
        await session.execute(
            stmt.where(*conditions)
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

# --- For Software detail page -----------------------------------------------------
async def software_detail(session: AsyncSession, *, name: str) -> dict[str, object] | None:
    rows = (
        await session.execute(
            select(
                SoftwareRecord.version,
                SoftwareRecord.publisher,
                Machine.uuid,
                SignatureRecord.status,
            )
            .join(Machine, Machine.id == SoftwareRecord.machine_id)
            .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
            .where(
                SoftwareRecord.name == name,           # exact match จากชื่อใน list
                SoftwareRecord.uninstalled_at.is_(None),
                Machine.deleted_at.is_(None),
            )
        )
    ).all()
    if not rows:
        return None

    machines: set = set()
    by_version: dict[str, set] = {}
    publisher = None
    sig = None
    for version, pub, machine_uuid, sig_status in rows:
        machines.add(machine_uuid)
        by_version.setdefault(version, set()).add(machine_uuid)
        publisher = publisher or pub
        sig = sig or sig_status

    detail = {
        "name": name,
        "publisher": publisher,
        "installed_count": len(machines),
        "versions": [
            {"version": v, "installed_count": len(s)}
            for v, s in sorted(by_version.items(), key=lambda kv: -len(kv[1]))
        ],
        "signature_status": sig,
    }
    # ใช้ helper เดิมเติม license_status (รับ list ของ dict ที่มี key "name")
    return (await _enrich_license_status(session, [detail]))[0]

async def software_machines(
    session: AsyncSession, *, name: str, page: int = 1, page_size: int = 50
) -> tuple[list[dict[str, object]], int]:
    #create query but didn't run (not session.execute()) 
    base = (
        select(SoftwareRecord, Machine, SignatureRecord.status)
        .join(Machine, Machine.id == SoftwareRecord.machine_id)
        .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
        .where(
            SoftwareRecord.name == name,
            SoftwareRecord.uninstalled_at.is_(None),
            Machine.deleted_at.is_(None),
        )
    )
    # base.subquery() ใช้คำสั่งใน base เป็น subquery
    total = (await session.execute(select(func.count()).select_from(base.subquery()))).scalar_one()
    rows = (
        await session.execute(
            base.order_by(Machine.hostname.asc())
            .offset((page - 1) * page_size)
            .limit(page_size)
        )
    ).all()
    items = [
        {
            "machine_uuid": m.uuid,
            "hostname": m.hostname,
            "display_name": m.display_name,
            "owner": m.owner,
            "status": m.status,                
            "version": rec.version,
            "install_date": rec.install_date,
            "signature_status": sig_status,
        }
        for rec, m, sig_status in rows
    ]
    return items, int(total)

# ── Recently-installed feed + trust verdict (security posture) ───────────────


def trust_verdict(signature_status: str | None, *, blacklisted: bool = False) -> str:
    """Map existing safety signals to a 3-level trust label for the
    "recently installed apps" feed. Pure — unit-tested without a DB.

    - ``risky``      — name matches a blacklist entry (explicitly disallowed)
    - ``trusted``    — signed by a valid, current certificate
    - ``suspicious`` — anything else: unsigned / invalid / expired / unknown /
      no signature at all

    Blacklist wins over a valid signature: a disallowed app is risky even if
    it is correctly signed.
    """
    if blacklisted:
        return "risky"
    if signature_status == "valid":
        return "trusted"
    return "suspicious"


def _glob(pattern: str) -> str:
    """SQL LIKE wildcard → fnmatch wildcard (same convention as license match)."""
    return pattern.replace("%", "*").replace("_", "?").lower()


def _is_blacklisted(name: str, publisher: str | None, blacklist: list[BlacklistEntry]) -> bool:
    """True if (name, publisher) matches any blacklist entry. A publisher
    pattern, when present, must also match; otherwise the name match suffices.
    """
    n = name.lower()
    pub = (publisher or "").lower()
    for entry in blacklist:
        if not fnmatch.fnmatch(n, _glob(entry.name_pattern)):
            continue
        if entry.publisher_pattern and not fnmatch.fnmatch(pub, _glob(entry.publisher_pattern)):
            continue
        return True
    return False


async def recent_installs(
    session: AsyncSession,
    *,
    machine_id: int | None = None,
    days: int | None = 30,
    from_date: date | None = None,
    to_date: date | None = None,
    limit: int = 100,
) -> list[dict[str, object]]:
    """Feed of apps that newly appeared (or were updated) on the fleet, newest
    first, each enriched with provenance (accurate install time) + a trust
    verdict. Built from the existing software_history diff (event installed/
    updated) joined to the current software_record for signature + timing.

    Time window: an explicit ``from_date``/``to_date`` calendar range (inclusive,
    interpreted in UTC) takes precedence; otherwise fall back to the rolling
    ``days`` lookback. Filtering is on ``occurred_at`` — when the agent observed
    the install/update event — to keep the feed consistent with the ``days``
    preset.
    """
    conditions: list[ColumnElement[bool]] = [
        SoftwareHistory.event.in_(("installed", "updated")),
        Machine.deleted_at.is_(None),
    ]
    if machine_id is not None:
        conditions.append(SoftwareHistory.machine_id == machine_id)

    if from_date is not None or to_date is not None:
        # Explicit calendar range overrides the rolling `days` window.
        if from_date is not None:
            start = datetime.combine(from_date, time.min, tzinfo=UTC)
            conditions.append(SoftwareHistory.occurred_at >= start)
        if to_date is not None:
            # Inclusive of the whole `to_date` day → strictly before the next day.
            end = datetime.combine(to_date + timedelta(days=1), time.min, tzinfo=UTC)
            conditions.append(SoftwareHistory.occurred_at < end)
    elif days is not None:
        conditions.append(SoftwareHistory.occurred_at >= datetime.now(tz=UTC) - timedelta(days=days))

    rows = (
        await session.execute(
            select(
                SoftwareHistory,
                Machine.uuid,
                Machine.hostname,
                Machine.display_name,
                SoftwareRecord.publisher,
                SoftwareRecord.install_date,
                SoftwareRecord.install_datetime,
                SignatureRecord.status,
            )
            .join(Machine, Machine.id == SoftwareHistory.machine_id)
            .outerjoin(
                SoftwareRecord,
                and_(
                    SoftwareRecord.machine_id == SoftwareHistory.machine_id,
                    SoftwareRecord.name == SoftwareHistory.software_name,
                    SoftwareRecord.version == SoftwareHistory.software_version,
                ),
            )
            .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
            .where(*conditions)
            .order_by(SoftwareHistory.occurred_at.desc(), SoftwareHistory.id.desc())
            .limit(limit)
        )
    ).all()

    blacklist = list((await session.execute(select(BlacklistEntry))).scalars().all())

    items: list[dict[str, object]] = []
    for hist, m_uuid, hostname, display_name, publisher, inst_date, inst_dt, sig_status in rows:
        blacklisted = _is_blacklisted(hist.software_name, publisher, blacklist)
        items.append(
            {
                "machine_uuid": m_uuid,
                "machine_name": display_name or hostname,
                "name": hist.software_name,
                "version": hist.software_version,
                "event": hist.event,  # installed | updated
                "publisher": publisher,
                "detected_at": hist.occurred_at,  # when our agent first saw it
                "installed_at": inst_dt,  # accurate install moment (best-effort)
                "install_date": inst_date,  # registry date-only fallback
                "signature_status": sig_status,
                "trust": trust_verdict(sig_status, blacklisted=blacklisted),
            }
        )
    return items
