"""Cross-machine inventory: posture stats, risky top-N, sort + machine drill-down.

Exercises the new security-posture cuts of the software page:
- software_stats: fleet-wide signed/unsigned/invalid + unique/total counts.
- top_software(risk=True): ranks only apps we can't vouch for by signature.
- cross_software: sort modes + the {uuid, name} machine list for drill-down.
"""

from __future__ import annotations

from datetime import UTC, datetime

import pytest
from app.models.machine import Machine
from app.schemas.scans import ScanIn, SignatureIn, SoftwareIn
from app.services import scan_service, software_service


async def _make_machine(session, hostname: str, display_name: str | None = None) -> Machine:
    m = Machine(
        hostname=hostname,
        display_name=display_name,
        os="windows",
        os_version="10.0.26200",
        arch="amd64",
        agent_version="1.0.0",
        agent_token_hash=hostname,  # unique per machine
        enrolled_at=datetime.now(tz=UTC),
        status="online",
        tags=[],
    )
    session.add(m)
    await session.flush()
    return m


def _sw(name: str, version: str, **kw) -> SoftwareIn:
    return SoftwareIn(name=name, version=version, source="registry", **kw)


def _scan(*software: SoftwareIn) -> ScanIn:
    now = datetime.now(tz=UTC)
    return ScanIn(started_at=now, completed_at=now, scan_type="auto", software=list(software))


async def _seed_fleet(session) -> None:
    """Two machines. Chrome (valid) on both; 7-Zip (unsigned) on one; a fake
    "Updater" (invalid) on one. Gives a known signed/unsigned/invalid mix."""
    a = await _make_machine(session, "PC-A", display_name="Finance laptop")
    b = await _make_machine(session, "PC-B")
    await scan_service.process_scan(
        session=session,
        machine=a,
        payload=_scan(
            _sw("Chrome", "120", signature=SignatureIn(status="valid")),
            _sw("7-Zip", "23", signature=SignatureIn(status="unsigned")),
        ),
    )
    await scan_service.process_scan(
        session=session,
        machine=b,
        payload=_scan(
            _sw("Chrome", "120", signature=SignatureIn(status="valid")),
            _sw("Updater", "1.0", signature=SignatureIn(status="invalid")),
        ),
    )


@pytest.mark.asyncio
async def test_software_stats_counts(session):
    await _seed_fleet(session)
    stats = await software_service.software_stats(session)
    assert stats["unique_apps"] == 3  # Chrome, 7-Zip, Updater
    assert stats["total_installs"] == 4  # Chrome×2 + 7-Zip + Updater
    assert stats["valid"] == 2  # both Chromes
    assert stats["unsigned"] == 1  # 7-Zip
    assert stats["invalid"] == 1  # Updater


@pytest.mark.asyncio
async def test_software_stats_unsigned_includes_no_signature(session):
    """An app with no signature record at all counts as unsigned."""
    m = await _make_machine(session, "PC-C")
    await scan_service.process_scan(session=session, machine=m, payload=_scan(_sw("NoSig", "1")))
    stats = await software_service.software_stats(session)
    assert stats["unsigned"] == 1
    assert stats["valid"] == 0


@pytest.mark.asyncio
async def test_software_stats_honours_search(session):
    await _seed_fleet(session)
    stats = await software_service.software_stats(session, q="chrome")
    assert stats["unique_apps"] == 1
    assert stats["total_installs"] == 2
    assert stats["valid"] == 2


@pytest.mark.asyncio
async def test_top_software_risk_mode_excludes_trusted(session):
    await _seed_fleet(session)
    risky = await software_service.top_software(session, risk=True)
    names = {r["name"] for r in risky}
    assert "Chrome" not in names  # validly signed → not risky
    assert {"7-Zip", "Updater"} <= names


@pytest.mark.asyncio
async def test_top_software_plain_ranks_by_spread(session):
    await _seed_fleet(session)
    top = await software_service.top_software(session)
    assert top[0]["name"] == "Chrome"  # on 2 machines, the rest on 1
    assert top[0]["installed_count"] == 2


@pytest.mark.asyncio
async def test_cross_software_machines_carry_readable_names(session):
    await _seed_fleet(session)
    items, _ = await software_service.cross_software(session)
    chrome = next(i for i in items if i["name"] == "Chrome")
    assert chrome["installed_count"] == 2
    labels = {ref["name"] for ref in chrome["machines"]}
    assert labels == {"Finance laptop", "PC-B"}  # display_name wins, else hostname
    assert all("uuid" in ref and "name" in ref for ref in chrome["machines"])


@pytest.mark.asyncio
async def test_cross_software_sort_by_name(session):
    await _seed_fleet(session)
    asc, _ = await software_service.cross_software(session, sort="name")
    names_asc = [i["name"] for i in asc]
    assert names_asc == sorted(names_asc, key=str.lower)

    desc, _ = await software_service.cross_software(session, sort="-name")
    names_desc = [i["name"] for i in desc]
    assert names_desc == sorted(names_desc, key=str.lower, reverse=True)


@pytest.mark.asyncio
async def test_cross_software_filter_unsigned_includes_no_signature(session):
    await _seed_fleet(session)
    m = await _make_machine(session, "PC-D")
    await scan_service.process_scan(session=session, machine=m, payload=_scan(_sw("Bare", "1")))
    items, _ = await software_service.cross_software(session, signature_status="unsigned")
    names = {i["name"] for i in items}
    assert "7-Zip" in names  # explicit unsigned
    assert "Bare" in names  # no signature row at all
    assert "Chrome" not in names
