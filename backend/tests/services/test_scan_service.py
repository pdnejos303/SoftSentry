"""Unit tests for scan ingestion diff logic (installed/uninstalled/updated)."""

from __future__ import annotations

from datetime import UTC, datetime

import pytest
from app.models.machine import Machine
from app.models.signature import SignatureRecord
from app.models.software import SoftwareHistory, SoftwareRecord
from app.schemas.scans import ScanIn, SignatureIn, SoftwareIn
from app.services import scan_service
from sqlalchemy import func, select


async def _make_machine(session) -> Machine:
    m = Machine(
        hostname="HOST-1",
        os="windows",
        os_version="10.0.26200",
        arch="amd64",
        agent_version="1.0.0",
        agent_token_hash="x",
        enrolled_at=datetime.now(tz=UTC),
        status="online",
        tags=[],
    )
    session.add(m)
    await session.flush()
    return m


def _sw(name: str, version: str, **kw) -> SoftwareIn:
    return SoftwareIn(name=name, version=version, source="registry", **kw)


def _scan(*software: SoftwareIn, scan_type: str = "auto") -> ScanIn:
    now = datetime.now(tz=UTC)
    return ScanIn(
        started_at=now,
        completed_at=now,
        scan_type=scan_type,
        software=list(software),
    )


async def _events(session, machine_id: int) -> list[tuple[str, str, str, str | None]]:
    rows = (
        (
            await session.execute(
                select(SoftwareHistory)
                .where(SoftwareHistory.machine_id == machine_id)
                .order_by(SoftwareHistory.id)
            )
        )
        .scalars()
        .all()
    )
    return [(r.event, r.software_name, r.software_version, r.previous_version) for r in rows]


@pytest.mark.asyncio
async def test_first_scan_inserts_records_and_installed_history(session):
    m = await _make_machine(session)
    payload = _scan(
        _sw("Google Chrome", "120.0", publisher=" Google LLC "),
        _sw("7-Zip", "23.01"),
    )

    scan = await scan_service.process_scan(session=session, machine=m, payload=payload)

    assert scan.software_count == 2
    records = (
        (await session.execute(select(SoftwareRecord).where(SoftwareRecord.machine_id == m.id)))
        .scalars()
        .all()
    )
    assert {r.name for r in records} == {"Google Chrome", "7-Zip"}
    chrome = next(r for r in records if r.name == "Google Chrome")
    assert chrome.publisher == "Google LLC"  # trimmed
    assert chrome.first_seen_scan_id == scan.id
    assert chrome.last_seen_scan_id == scan.id
    assert chrome.uninstalled_at is None

    events = await _events(session, m.id)
    assert ("installed", "Google Chrome", "120.0", None) in events
    assert ("installed", "7-Zip", "23.01", None) in events
    assert len(events) == 2


@pytest.mark.asyncio
async def test_missing_software_marked_uninstalled(session):
    m = await _make_machine(session)
    await scan_service.process_scan(
        session=session, machine=m, payload=_scan(_sw("Chrome", "120"), _sw("7-Zip", "23.01"))
    )
    # Second scan drops 7-Zip
    await scan_service.process_scan(session=session, machine=m, payload=_scan(_sw("Chrome", "120")))

    seven = (
        await session.execute(
            select(SoftwareRecord).where(
                SoftwareRecord.machine_id == m.id, SoftwareRecord.name == "7-Zip"
            )
        )
    ).scalar_one()
    assert seven.uninstalled_at is not None

    events = await _events(session, m.id)
    assert ("uninstalled", "7-Zip", "23.01", None) in events


@pytest.mark.asyncio
async def test_version_update_emits_updated_event(session):
    m = await _make_machine(session)
    await scan_service.process_scan(session=session, machine=m, payload=_scan(_sw("Chrome", "119")))
    await scan_service.process_scan(session=session, machine=m, payload=_scan(_sw("Chrome", "120")))

    events = await _events(session, m.id)
    assert ("installed", "Chrome", "119", None) in events
    assert ("updated", "Chrome", "120", "119") in events

    # Old version record is uninstalled, new version is active.
    records = (
        (await session.execute(select(SoftwareRecord).where(SoftwareRecord.machine_id == m.id)))
        .scalars()
        .all()
    )
    v119 = next(r for r in records if r.version == "119")
    v120 = next(r for r in records if r.version == "120")
    assert v119.uninstalled_at is not None
    assert v120.uninstalled_at is None


@pytest.mark.asyncio
async def test_reinstall_clears_uninstalled_at(session):
    m = await _make_machine(session)
    await scan_service.process_scan(session=session, machine=m, payload=_scan(_sw("Chrome", "120")))
    await scan_service.process_scan(session=session, machine=m, payload=_scan())  # uninstalled
    await scan_service.process_scan(session=session, machine=m, payload=_scan(_sw("Chrome", "120")))

    rec = (
        await session.execute(
            select(SoftwareRecord).where(
                SoftwareRecord.machine_id == m.id, SoftwareRecord.name == "Chrome"
            )
        )
    ).scalar_one()
    assert rec.uninstalled_at is None
    # installed, uninstalled, installed again
    events = [e[0] for e in await _events(session, m.id)]
    assert events.count("installed") == 2
    assert events.count("uninstalled") == 1


@pytest.mark.asyncio
async def test_signature_upserted_and_linked(session):
    m = await _make_machine(session)
    await scan_service.process_scan(
        session=session,
        machine=m,
        payload=_scan(
            _sw("Chrome", "120", signature=SignatureIn(status="valid", signer="Google LLC"))
        ),
    )
    rec = (
        await session.execute(select(SoftwareRecord).where(SoftwareRecord.machine_id == m.id))
    ).scalar_one()
    sig = (
        await session.execute(select(SignatureRecord).where(SignatureRecord.software_id == rec.id))
    ).scalar_one()
    assert sig.status == "valid"
    assert rec.signature_id == sig.id

    # Re-scan with changed signature status → same single signature row, updated.
    await scan_service.process_scan(
        session=session,
        machine=m,
        payload=_scan(_sw("Chrome", "120", signature=SignatureIn(status="expired"))),
    )
    sig_count = (
        await session.execute(
            select(func.count())
            .select_from(SignatureRecord)
            .where(SignatureRecord.software_id == rec.id)
        )
    ).scalar_one()
    assert sig_count == 1
    await session.refresh(sig)
    assert sig.status == "expired"


@pytest.mark.asyncio
async def test_idempotent_retry_returns_same_scan(session):
    m = await _make_machine(session)
    p = _scan(_sw("Chrome", "120"))
    first = await scan_service.process_scan(
        session=session, machine=m, payload=p, idempotency_key="key-abc"
    )
    second = await scan_service.process_scan(
        session=session, machine=m, payload=p, idempotency_key="key-abc"
    )
    assert first.id == second.id

    scan_count = (
        await session.execute(select(func.count()).select_from(scan_service.Scan))
    ).scalar_one()
    assert scan_count == 1
    # No duplicate history from the retry.
    events = await _events(session, m.id)
    assert events.count(("installed", "Chrome", "120", None)) == 1
