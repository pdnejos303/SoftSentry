"""Telemetry — business-KPI snapshot collection + metric helpers."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.core import metrics
from app.models.alert import Alert
from app.models.license import License
from app.models.machine import Machine
from app.services import metrics_service
from app.services.metrics_service import BusinessSnapshot
from prometheus_client import REGISTRY

pytestmark = pytest.mark.asyncio


def _machine(hostname: str, last_seen: datetime | None) -> Machine:
    now = datetime.now(tz=UTC)
    return Machine(
        hostname=hostname,
        os="windows",
        os_version="11",
        arch="amd64",
        agent_version="0.1.0",
        agent_token_hash=f"hash-{hostname}",
        enrolled_at=now,
        last_seen_at=last_seen,
    )


async def test_collect_business_snapshot(session) -> None:
    now = datetime.now(tz=UTC)
    session.add_all(
        [
            _machine("online-1", now - timedelta(minutes=1)),
            _machine("online-2", now - timedelta(minutes=4)),
            _machine("stale-1", now - timedelta(minutes=30)),
            _machine("offline-1", now - timedelta(hours=3)),
            _machine("never-seen", None),  # offline absorbs never-seen
            Alert(type="cve_critical", severity="critical", status="active", title="x"),
            Alert(type="unauthorized", severity="high", status="acknowledged", title="y"),
            Alert(type="unauthorized", severity="high", status="resolved", title="z"),  # closed
            License(software_name="Acme", purchased_count=10),
        ]
    )
    await session.commit()

    snap = await metrics_service.collect_business_snapshot(session)

    assert snap.machines == {"online": 2, "stale": 1, "offline": 2}
    assert snap.alerts_open["critical"] == 1
    assert snap.alerts_open["high"] == 1  # resolved one excluded
    assert snap.licenses["compliant"] == 1  # 0 installed of 10, perpetual
    assert snap.software_unique == 0


async def test_refresh_applies_to_registry(session) -> None:
    session.add(_machine("solo", datetime.now(tz=UTC)))
    await session.commit()

    await metrics_service.refresh(session)

    assert REGISTRY.get_sample_value("softsentry_machines", {"status": "online"}) == 1.0
    assert (
        REGISTRY.get_sample_value("softsentry_business_metrics_refreshed_timestamp_seconds")
        is not None
    )


def test_apply_snapshot_sets_all_gauges() -> None:
    snap = BusinessSnapshot(
        machines={"online": 5, "stale": 2, "offline": 3},
        software_unique=42,
        alerts_open={"critical": 4, "high": 3, "medium": 2, "low": 1},
        vulnerabilities_open={"critical": 1, "high": 0, "medium": 0, "low": 0},
        licenses={"compliant": 7, "over_used": 1, "expiring_soon": 2, "expired": 0},
    )
    metrics_service.apply_snapshot(snap)

    assert REGISTRY.get_sample_value("softsentry_software_unique") == 42.0
    assert REGISTRY.get_sample_value("softsentry_machines", {"status": "stale"}) == 2.0
    assert REGISTRY.get_sample_value("softsentry_alerts_open", {"severity": "critical"}) == 4.0
    assert (
        REGISTRY.get_sample_value("softsentry_licenses", {"status": "expiring_soon"}) == 2.0
    )


def test_observe_scan_records_agent_metrics() -> None:
    before = REGISTRY.get_sample_value("softsentry_agent_scans_total", {"scan_type": "auto"}) or 0.0
    before_count = REGISTRY.get_sample_value("softsentry_agent_scan_duration_seconds_count") or 0.0

    metrics.observe_scan(scan_type="auto", duration_seconds=3.5, software_count=120)
    # Negative durations (clock skew) are clamped, not dropped.
    metrics.observe_scan(scan_type="auto", duration_seconds=-2.0, software_count=0)

    after = REGISTRY.get_sample_value("softsentry_agent_scans_total", {"scan_type": "auto"})
    after_count = REGISTRY.get_sample_value("softsentry_agent_scan_duration_seconds_count")
    assert after == before + 2.0
    assert after_count == before_count + 2.0
