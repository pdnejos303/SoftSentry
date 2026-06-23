"""Digital-signature alerting (Module 3.4).

Evaluated inline after every scan ingest (same transaction as the unauthorized
engine). For each active software record whose signature is invalid, expired, or
unsigned it raises a deduped alert; alerts whose signature was since fixed — or
whose software was uninstalled — are auto-resolved. Mirrors the reconcile shape
of `unauthorized_service` but keyed on signature status instead of policy.
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, date, datetime
from typing import TYPE_CHECKING

from sqlalchemy import ColumnElement, func, select, or_

from app.models.alert import OPEN_STATUSES, STATUS_RESOLVED, Alert
from app.models.machine import Machine
from app.models.policy import PolicySettings
from app.models.signature import SignatureRecord
from app.models.software import SoftwareRecord

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

TYPE_INVALID = "signature_invalid"
TYPE_UNSIGNED = "signature_unsigned"
_DETECTION_TYPES = (TYPE_INVALID, TYPE_UNSIGNED)


def _expired_detail(valid_to: date | None, now: date) -> str:
    if valid_to is None:
        return "Certificate has expired"
    days = (now - valid_to).days
    if days > 0:
        return f"Certificate expired {days} day{'s' if days != 1 else ''} ago"
    return "Certificate has expired"


def _classify(
    status: str | None,
    *,
    valid_to: date | None,
    flag_unsigned: bool,
    today: date,
) -> tuple[str, str, str] | None:
    """Map a signature status to (alert_type, severity, detail), or None if no
    alert is warranted."""
    if status == "invalid":
        return TYPE_INVALID, "high", "Signature is invalid (untrusted or tampered)"
    if status == "expired":
        return TYPE_INVALID, "medium", _expired_detail(valid_to, today)
    if status == "unsigned" and flag_unsigned:
        return TYPE_UNSIGNED, "medium", "Software is not digitally signed"
    return None


async def evaluate_machine(session: AsyncSession, machine: Machine) -> None:
    """Re-evaluate this machine's active inventory signatures and reconcile
    alerts. Runs in the caller's transaction (caller commits)."""
    rows = (
        await session.execute(
            select(SoftwareRecord, SignatureRecord)
            .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
            .where(
                SoftwareRecord.machine_id == machine.id,
                SoftwareRecord.uninstalled_at.is_(None),
            )
        )
    ).all()

    policy = await session.get(PolicySettings, 1)
    flag_unsigned = policy is None or policy.flag_unsigned

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
    today = now.date()

    for sw, sig in rows:
        status = sig.status if sig is not None else None
        result = _classify(
            status,
            valid_to=sig.cert_valid_to if sig is not None else None,
            flag_unsigned=flag_unsigned,
            today=today,
        )
        if result is None:
            continue
        alert_type, severity, detail = result
        _ensure_alert(
            session,
            by_key,
            machine_id=machine.id,
            software=sw,
            alert_type=alert_type,
            severity=severity,
            title=f"{_title_prefix(alert_type)}: {sw.name}",
            detail=detail,
        )

    # Any open signature alert left in by_key is no longer offending (signature
    # fixed or software uninstalled) → auto-resolve.
    for alert in by_key.values():
        alert.status = STATUS_RESOLVED
        alert.resolved_at = now

    await session.flush()


def _title_prefix(alert_type: str) -> str:
    return "Unsigned software" if alert_type == TYPE_UNSIGNED else "Invalid signature"


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
    otherwise refresh its severity/detail and keep it out of the resolve sweep."""
    key = (software.id, alert_type)
    existing = by_key.get(key)
    if existing is not None:
        existing.severity = severity
        existing.detail = detail
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


# ── Query endpoints (Module 3.2/3.3/3.5) ─────────────────────────────────────


async def list_signatures(
    session: AsyncSession,
    *,
    statuses: list[str] | None = None,
    publisher: str | None = None,
    q: str | None = None,
    page: int = 1,
    page_size: int = 50,
) -> tuple[list[dict[str, object]], int]:
    """List active software with a signature record. `statuses` is a union
    filter (spec 3.3 multi-select)."""
    conds: list[ColumnElement[bool]] = [
        SoftwareRecord.uninstalled_at.is_(None),
        Machine.deleted_at.is_(None),
    ]
    if statuses:
        real = [s for s in statuses if s != "unknown"]
        status_conds: list[ColumnElement[bool]] = []
        if real:
            status_conds.append(SignatureRecord.status.in_(real))
        if "unknown" in statuses:
            status_conds.append(SignatureRecord.id.is_(None))
            status_conds.append(SignatureRecord.status == "unknown")
        conds.append(or_(*status_conds))
    if publisher:
        conds.append(SoftwareRecord.publisher.ilike(f"%{publisher}%"))
    if q:
        conds.append(SoftwareRecord.name.ilike(f"%{q}%"))

    base = (
        select(SoftwareRecord, SignatureRecord, Machine.uuid, Machine.hostname)
        .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
        .join(Machine, Machine.id == SoftwareRecord.machine_id)
        .where(*conds)
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
            "software_uuid": sw.uuid,
            "name": sw.name,
            "version": sw.version,
            "publisher": sw.publisher,
            "machine_uuid": m_uuid,
            "machine_hostname": hostname,
            "status": sig.status if sig is not None else "unknown",
            "signer": sig.signer if sig is not None else None,
            "cert_valid_to": sig.cert_valid_to if sig is not None else None,
        }
        for sw, sig, m_uuid, hostname in rows
    ]
    return items, int(total)


def _detail_issues(sig: SignatureRecord, today: date) -> list[str]:
    """Human-readable problems for the detail drawer (spec 3.2)."""
    issues: list[str] = []
    if sig.status == "invalid":
        issues.append("Signature is invalid (untrusted or tampered)")
    elif sig.status == "expired":
        issues.append(_expired_detail(sig.cert_valid_to, today))
    elif sig.status == "unsigned":
        issues.append("Software is not digitally signed")
    # Self-signed leaf (subject == issuer) → untrusted root.
    chain = sig.chain or []
    if chain and isinstance(chain[0], dict) and chain[0].get("subject") == chain[0].get("issuer"):
        issues.append("Self-signed certificate (untrusted root)")
    return issues


async def signature_detail(
    session: AsyncSession, software_uuid: uuid_lib.UUID
) -> dict[str, object] | None:
    row = (
        await session.execute(
            select(SoftwareRecord, SignatureRecord, Machine.uuid, Machine.hostname)
            .join(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
            .join(Machine, Machine.id == SoftwareRecord.machine_id)
            .where(SoftwareRecord.uuid == software_uuid)
        )
    ).first()
    if row is None:
        return None
    sw, sig, m_uuid, hostname = row
    return {
        "software_uuid": sw.uuid,
        "software_name": sw.name,
        "software_version": sw.version,
        "machine_uuid": m_uuid,
        "machine_hostname": hostname,
        "status": sig.status,
        "signer": sig.signer,
        "issuer": sig.issuer,
        "cert_thumbprint": sig.cert_thumbprint,
        "cert_valid_from": sig.cert_valid_from,
        "cert_valid_to": sig.cert_valid_to,
        "signature_algorithm": sig.signature_algorithm,
        "chain": sig.chain,
        "verified_at": sig.verified_at,
        "issues": _detail_issues(sig, datetime.now(tz=UTC).date()),
    }


async def signature_stats(session: AsyncSession) -> dict[str, object]:
    """Signature-status distribution across the active fleet (software with no
    signature record counts as `unknown`)."""
    rows = (
        await session.execute(
            select(SignatureRecord.status, func.count())
            .select_from(SoftwareRecord)
            .outerjoin(SignatureRecord, SignatureRecord.id == SoftwareRecord.signature_id)
            .join(Machine, Machine.id == SoftwareRecord.machine_id)
            .where(SoftwareRecord.uninstalled_at.is_(None), Machine.deleted_at.is_(None))
            .group_by(SignatureRecord.status)
        )
    ).all()

    counts: dict[str, int] = {}
    for status_value, count in rows:
        counts[status_value or "unknown"] = counts.get(status_value or "unknown", 0) + int(count)

    distribution = [{"status": s, "count": c} for s, c in sorted(counts.items())]
    return {"total": sum(counts.values()), "distribution": distribution}
