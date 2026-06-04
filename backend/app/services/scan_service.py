"""Scan ingestion — upsert software inventory + compute install/uninstall/update diff.

Called from POST /api/v1/agents/scans. Runs inline within the request transaction;
the caller commits. CVE/alert matching jobs are enqueued separately (Phase 3).
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import TYPE_CHECKING

from sqlalchemy import select

from app.models.agent_config import AgentConfig
from app.models.scan import Scan
from app.models.signature import SignatureRecord
from app.models.software import SoftwareHistory, SoftwareRecord

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

    from app.models.machine import Machine
    from app.schemas.scans import ScanIn, SignatureIn, SoftwareIn


def _norm(value: str | None) -> str | None:
    """Registry/plist values often carry stray whitespace — trim it."""
    return value.strip() if value else value


async def _upsert_signature(
    session: AsyncSession,
    record: SoftwareRecord,
    sig_in: SignatureIn | None,
    now: datetime,
) -> None:
    if sig_in is None:
        return

    sig: SignatureRecord | None = None
    if record.signature_id is not None:
        sig = await session.get(SignatureRecord, record.signature_id)
    if sig is None:
        sig = (
            await session.execute(
                select(SignatureRecord).where(SignatureRecord.software_id == record.id)
            )
        ).scalar_one_or_none()

    if sig is None:
        sig = SignatureRecord(software_id=record.id)
        session.add(sig)

    sig.status = sig_in.status
    sig.signer = _norm(sig_in.signer)
    sig.issuer = _norm(sig_in.issuer)
    sig.cert_thumbprint = sig_in.cert_thumbprint
    sig.cert_valid_from = sig_in.cert_valid_from
    sig.cert_valid_to = sig_in.cert_valid_to
    sig.signature_algorithm = sig_in.signature_algorithm
    sig.chain = sig_in.chain
    sig.verified_at = now

    await session.flush()
    record.signature_id = sig.id


async def process_scan(
    *,
    session: AsyncSession,
    machine: Machine,
    payload: ScanIn,
    idempotency_key: str | None = None,
) -> Scan:
    """Ingest one scan: create scan row, upsert software_records + signatures,
    mark missing software uninstalled, and emit software_history events.

    Returns the created (or, on idempotent retry, the existing) Scan.
    """
    if idempotency_key:
        existing = (
            await session.execute(select(Scan).where(Scan.idempotency_key == idempotency_key))
        ).scalar_one_or_none()
        if existing is not None:
            return existing

    now = datetime.now(tz=UTC)

    # De-dupe incoming by (name, version); a restarted agent may report the same
    # entry twice. Last one wins.
    incoming: dict[tuple[str, str], SoftwareIn] = {}
    for sw in payload.software:
        incoming[(sw.name.strip(), sw.version.strip())] = sw

    scan = Scan(
        machine_id=machine.id,
        started_at=payload.started_at,
        completed_at=payload.completed_at,
        received_at=now,
        software_count=len(incoming),
        scan_type=payload.scan_type,
        trigger=payload.trigger,
        idempotency_key=idempotency_key,
    )
    session.add(scan)
    await session.flush()

    # All prior records for this machine (active + previously uninstalled) so a
    # reinstall reactivates the existing row instead of violating the unique key.
    prior = (
        (
            await session.execute(
                select(SoftwareRecord).where(SoftwareRecord.machine_id == machine.id)
            )
        )
        .scalars()
        .all()
    )
    by_key: dict[tuple[str, str], SoftwareRecord] = {(r.name, r.version): r for r in prior}
    prev_active = {(r.name, r.version) for r in prior if r.uninstalled_at is None}
    curr_set = set(incoming)

    for key, sw in incoming.items():
        name, version = key
        record = by_key.get(key)
        if record is None:
            record = SoftwareRecord(
                machine_id=machine.id,
                name=name,
                version=version,
                source=sw.source,
                first_seen_scan_id=scan.id,
                last_seen_scan_id=scan.id,
            )
            session.add(record)
            by_key[key] = record
        else:
            record.last_seen_scan_id = scan.id
            record.uninstalled_at = None
            if record.first_seen_scan_id is None:
                record.first_seen_scan_id = scan.id

        record.publisher = _norm(sw.publisher)
        record.install_date = sw.install_date
        record.install_path = sw.install_path
        record.install_size_kb = sw.install_size_kb
        record.arch = sw.arch
        record.source = sw.source
        await session.flush()  # ensure record.id for signature FK

        await _upsert_signature(session, record, sw.signature, now)

    # Mark software that was active but is absent from this scan as uninstalled.
    for key, record in by_key.items():
        if record.uninstalled_at is None and key not in curr_set:
            record.uninstalled_at = now

    _emit_history(session, machine.id, scan.id, prev_active, curr_set, now)

    machine.last_scan_at = payload.completed_at
    machine.last_seen_at = now
    machine.status = "online"
    cfg = await session.get(AgentConfig, machine.id)
    if cfg is not None:
        cfg.manual_scan_requested = False

    await session.flush()
    return scan


def _emit_history(
    session: AsyncSession,
    machine_id: int,
    scan_id: int,
    prev_active: set[tuple[str, str]],
    curr_set: set[tuple[str, str]],
    now: datetime,
) -> None:
    """Classify per-name version changes into installed/uninstalled/updated events."""
    prev_by_name: dict[str, set[str]] = {}
    for name, version in prev_active:
        prev_by_name.setdefault(name, set()).add(version)
    curr_by_name: dict[str, set[str]] = {}
    for name, version in curr_set:
        curr_by_name.setdefault(name, set()).add(version)

    events: list[SoftwareHistory] = []
    for name in set(prev_by_name) | set(curr_by_name):
        added = sorted(curr_by_name.get(name, set()) - prev_by_name.get(name, set()))
        removed = sorted(prev_by_name.get(name, set()) - curr_by_name.get(name, set()))

        # Pair an added with a removed version → treat as an in-place update.
        while added and removed:
            new_version = added.pop(0)
            old_version = removed.pop(0)
            events.append(
                SoftwareHistory(
                    machine_id=machine_id,
                    software_name=name,
                    software_version=new_version,
                    event="updated",
                    previous_version=old_version,
                    occurred_at=now,
                    scan_id=scan_id,
                )
            )
        for version in added:
            events.append(
                SoftwareHistory(
                    machine_id=machine_id,
                    software_name=name,
                    software_version=version,
                    event="installed",
                    occurred_at=now,
                    scan_id=scan_id,
                )
            )
        for version in removed:
            events.append(
                SoftwareHistory(
                    machine_id=machine_id,
                    software_name=name,
                    software_version=version,
                    event="uninstalled",
                    occurred_at=now,
                    scan_id=scan_id,
                )
            )

    session.add_all(events)
