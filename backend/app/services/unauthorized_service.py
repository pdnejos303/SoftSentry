"""Unauthorized / blacklisted software detection (Module 4.4-4.5).

Evaluated after every scan ingest. For each active software record on the
machine it raises (deduped) alerts for blacklist hits and — when whitelist
enforcement is on — for software that matches no whitelist entry. Alerts whose
software has since been uninstalled are auto-resolved; alerts are NOT
auto-resolved merely because policy changed (audit trail, spec edge case 4).
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import TYPE_CHECKING

from sqlalchemy import select

from app.models.alert import OPEN_STATUSES, STATUS_RESOLVED, Alert
from app.models.policy import BlacklistEntry, PolicySettings, WhitelistEntry
from app.models.software import SoftwareRecord
from app.services.policy_matching import matches

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

    from app.models.machine import Machine

TYPE_BLACKLISTED = "blacklisted_software"
TYPE_UNAUTHORIZED = "unauthorized_software"
_DETECTION_TYPES = (TYPE_BLACKLISTED, TYPE_UNAUTHORIZED)


def _entry_matches(sw: SoftwareRecord, entry: BlacklistEntry | WhitelistEntry) -> bool:
    return matches(
        sw.name,
        sw.publisher,
        sw.version,
        name_pattern=entry.name_pattern,
        publisher_pattern=entry.publisher_pattern,
        version_constraint=entry.version_constraint,
    )


async def evaluate_machine(session: AsyncSession, machine: Machine) -> None:
    """Re-evaluate this machine's active inventory against policy and reconcile
    alerts. Runs in the caller's transaction (caller commits)."""
    active = (
        (
            await session.execute(
                select(SoftwareRecord).where(
                    SoftwareRecord.machine_id == machine.id,
                    SoftwareRecord.uninstalled_at.is_(None),
                )
            )
        )
        .scalars()
        .all()
    )
    blacklist = (await session.execute(select(BlacklistEntry))).scalars().all()
    whitelist = (await session.execute(select(WhitelistEntry))).scalars().all()
    policy = await session.get(PolicySettings, 1)
    whitelist_mode = bool(policy and policy.whitelist_mode)

    # Existing open alerts for this machine, keyed by (software_id, type).
    open_alerts = (
        (
            await session.execute(
                select(Alert).where(
                    Alert.machine_id == machine.id,
                    Alert.type.in_(_DETECTION_TYPES),
                    Alert.status.in_(OPEN_STATUSES),
                )
            )
        )
        .scalars()
        .all()
    )
    by_key: dict[tuple[int | None, str], Alert] = {(a.software_id, a.type): a for a in open_alerts}

    now = datetime.now(tz=UTC)
    active_ids = {sw.id for sw in active}

    for sw in active:
        # Blacklist wins over whitelist enforcement (spec edge case 2).
        hit = next((e for e in blacklist if _entry_matches(sw, e)), None)
        if hit is not None:
            _ensure_alert(
                session,
                by_key,
                machine_id=machine.id,
                software=sw,
                alert_type=TYPE_BLACKLISTED,
                severity=hit.severity,
                title=f"Blacklisted software: {sw.name}",
                detail=hit.reason,
            )
            continue
        if whitelist_mode and not any(_entry_matches(sw, e) for e in whitelist):
            _ensure_alert(
                session,
                by_key,
                machine_id=machine.id,
                software=sw,
                alert_type=TYPE_UNAUTHORIZED,
                severity="medium",
                title=f"Unauthorized software: {sw.name}",
                detail="Not in whitelist (whitelist enforcement is on)",
            )

    # Auto-resolve open alerts whose software is gone (uninstalled / removed).
    for (software_id, _type), alert in by_key.items():
        if software_id is None or software_id not in active_ids:
            alert.status = STATUS_RESOLVED
            alert.resolved_at = now

    await session.flush()


def _ensure_alert(
    session: AsyncSession,
    by_key: dict[tuple[int | None, str], Alert],
    *,
    machine_id: int,
    software: SoftwareRecord,
    alert_type: str,
    severity: str,
    title: str,
    detail: str | None,
) -> None:
    """Create the alert if no open one exists for (machine, software, type);
    otherwise refresh its severity/detail. Marks the alert as still-offending so
    the auto-resolve sweep leaves it open."""
    key = (software.id, alert_type)
    existing = by_key.get(key)
    if existing is not None:
        existing.severity = severity
        existing.detail = detail
        # Keep it out of the auto-resolve sweep: it still matches.
        del by_key[key]
        return
    session.add(
        Alert(
            machine_id=machine_id,
            software_id=software.id,
            type=alert_type,
            severity=severity,
            title=title,
            detail=detail,
        )
    )
