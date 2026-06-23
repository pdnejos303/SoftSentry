"""Recently-installed feed + trust verdict (security posture).

- trust_verdict: pure mapping of signature/blacklist signals → 3-level label.
- recent_installs: software_history (installed/updated) enriched with the
  current record's signature, accurate install time, and a trust verdict.
"""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.models.machine import Machine
from app.models.policy import BlacklistEntry
from app.schemas.scans import ScanIn, SignatureIn, SoftwareIn
from app.services import scan_service, software_service


# ── trust_verdict (pure) ─────────────────────────────────────────────────────


@pytest.mark.parametrize(
    ("status", "blacklisted", "expected"),
    [
        ("valid", False, "trusted"),
        ("unsigned", False, "suspicious"),
        ("invalid", False, "suspicious"),
        ("expired", False, "suspicious"),
        ("unknown", False, "suspicious"),
        (None, False, "suspicious"),
        ("valid", True, "risky"),  # blacklist overrides a valid signature
        ("unsigned", True, "risky"),
        (None, True, "risky"),
    ],
)
def test_trust_verdict(status: str | None, blacklisted: bool, expected: str) -> None:
    assert software_service.trust_verdict(status, blacklisted=blacklisted) == expected


# ── recent_installs (service + DB) ───────────────────────────────────────────


async def _make_machine(session, hostname: str = "PC-007") -> Machine:
    m = Machine(
        hostname=hostname,
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


@pytest.mark.asyncio
async def test_recent_installs_surfaces_new_app_with_trust(session):
    """At first there's no LINE; after a later scan LINE appears → feed shows it."""
    m = await _make_machine(session)
    # scan 1: only Chrome
    await scan_service.process_scan(session=session, machine=m, payload=_scan(_sw("Chrome", "120")))
    # scan 2: LINE shows up, validly signed
    await scan_service.process_scan(
        session=session,
        machine=m,
        payload=_scan(
            _sw("Chrome", "120"),
            _sw(
                "LINE",
                "8.5",
                publisher="LINE Corporation",
                signature=SignatureIn(status="valid", signer="LINE Corporation"),
                installed_at=datetime(2026, 6, 22, 9, 30, tzinfo=UTC),
            ),
        ),
    )

    items = await software_service.recent_installs(session)
    line = next(i for i in items if i["name"] == "LINE")
    assert line["event"] == "installed"
    assert line["machine_name"] == "PC-007"
    assert line["signature_status"] == "valid"
    assert line["trust"] == "trusted"
    # SQLite drops tz on round-trip; assert the wall-clock components instead.
    installed_at = line["installed_at"]
    assert (installed_at.year, installed_at.month, installed_at.day) == (2026, 6, 22)
    assert (installed_at.hour, installed_at.minute) == (9, 30)
    # newest first → LINE (scan 2) before Chrome (scan 1)
    assert items[0]["name"] == "LINE"


@pytest.mark.asyncio
async def test_recent_installs_unsigned_is_suspicious(session):
    m = await _make_machine(session, "PC-008")
    await scan_service.process_scan(
        session=session,
        machine=m,
        payload=_scan(_sw("RandomTool", "1.0", signature=SignatureIn(status="unsigned"))),
    )
    items = await software_service.recent_installs(session)
    assert items[0]["trust"] == "suspicious"


@pytest.mark.asyncio
async def test_recent_installs_blacklisted_is_risky(session):
    m = await _make_machine(session, "PC-009")
    session.add(BlacklistEntry(name_pattern="%torrent%", reason="P2P client banned"))
    await session.flush()
    await scan_service.process_scan(
        session=session,
        machine=m,
        payload=_scan(_sw("uTorrent", "3.6", signature=SignatureIn(status="valid"))),
    )
    items = await software_service.recent_installs(session)
    tor = next(i for i in items if i["name"] == "uTorrent")
    assert tor["trust"] == "risky"  # blacklist wins over valid signature


@pytest.mark.asyncio
async def test_recent_installs_explicit_date_range(session):
    """from_date/to_date select an exact calendar window (on the event's
    occurred_at) and override the rolling `days` lookback."""
    m = await _make_machine(session, "PC-010")
    await scan_service.process_scan(
        session=session, machine=m, payload=_scan(_sw("Slack", "5.0"))
    )

    today = datetime.now(tz=UTC).date()
    tomorrow = today + timedelta(days=1)
    yesterday = today - timedelta(days=1)

    # Window covering today → app is present.
    in_window = await software_service.recent_installs(
        session, from_date=today, to_date=today
    )
    assert any(i["name"] == "Slack" for i in in_window)

    # Window entirely in the future → excluded.
    future = await software_service.recent_installs(
        session, from_date=tomorrow, to_date=tomorrow
    )
    assert all(i["name"] != "Slack" for i in future)

    # Window entirely in the past → excluded.
    past = await software_service.recent_installs(
        session, from_date=yesterday, to_date=yesterday
    )
    assert all(i["name"] != "Slack" for i in past)
