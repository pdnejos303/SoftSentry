"""Scan ingestion + agent config/heartbeat contract tests (Phase 2 A1).

Covers POST /agents/scans (202 + idempotency), GET /agents/config, and the
heartbeat response body the agent relies on for manual-scan + update signals.
"""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.core.security import hash_password
from app.models.user import User
from sqlalchemy import func, select


async def _admin_access(client, session) -> str:
    password = "TestPass!2026Aa"
    session.add(
        User(
            email="admin@local",
            password_hash=hash_password(password),
            full_name="Admin",
            role="admin",
            is_active=True,
        )
    )
    await session.commit()
    r = await client.post(
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": password},
    )
    return r.json()["access_token"]


async def _enroll_agent(client, session, hostname: str = "host-1") -> str:
    access = await _admin_access(client, session)
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers={"Authorization": f"Bearer {access}"},
        json={"expires_in_seconds": 3600},
    )
    enroll = await client.post(
        "/api/v1/agents/enroll",
        json={
            "enrollment_token": create.json()["token"],
            "hostname": hostname,
            "os": "windows",
            "os_version": "11.0.26200",
            "arch": "amd64",
            "agent_version": "0.1.0",
        },
    )
    return enroll.json()["agent_token"]


def _scan_payload(*, name: str = "Google Chrome", version: str = "120.0.0") -> dict:
    now = datetime.now(tz=UTC)
    return {
        "started_at": (now - timedelta(seconds=5)).isoformat(),
        "completed_at": now.isoformat(),
        "scan_type": "auto",
        "trigger": "schedule",
        "software": [
            {
                "name": name,
                "version": version,
                "publisher": "Google LLC",
                "source": "registry",
                "arch": "x64",
                "signature": {"status": "valid", "signer": "Google LLC"},
            }
        ],
    }


@pytest.mark.asyncio
async def test_post_scan_requires_agent_token(client, session):
    r = await client.post("/api/v1/agents/scans", json=_scan_payload())
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_post_scan_accepted_and_ingests_software(client, session):
    from app.models.software import SoftwareRecord

    token = await _enroll_agent(client, session)
    r = await client.post(
        "/api/v1/agents/scans",
        headers={"Authorization": f"Bearer {token}"},
        json=_scan_payload(),
    )
    assert r.status_code == 202
    body = r.json()
    assert body["scan_uuid"]
    assert body["queued_for_analysis"] is True

    rec = (
        await session.execute(select(SoftwareRecord).where(SoftwareRecord.name == "Google Chrome"))
    ).scalar_one()
    assert rec.version == "120.0.0"
    assert rec.uninstalled_at is None


@pytest.mark.asyncio
async def test_post_scan_idempotent_on_repeated_key(client, session):
    from app.models.scan import Scan

    token = await _enroll_agent(client, session)
    headers = {"Authorization": f"Bearer {token}", "Idempotency-Key": "scan-abc-123"}
    payload = _scan_payload()

    first = await client.post("/api/v1/agents/scans", headers=headers, json=payload)
    second = await client.post("/api/v1/agents/scans", headers=headers, json=payload)

    assert first.status_code == 202
    assert second.status_code == 202
    assert first.json()["scan_uuid"] == second.json()["scan_uuid"]

    count = (await session.execute(select(func.count()).select_from(Scan))).scalar_one()
    assert count == 1


@pytest.mark.asyncio
async def test_get_agent_config_returns_defaults(client, session):
    token = await _enroll_agent(client, session)
    r = await client.get(
        "/api/v1/agents/config",
        headers={"Authorization": f"Bearer {token}"},
    )
    assert r.status_code == 200
    body = r.json()
    assert body["scan_interval_hours"] == 6
    assert body["auto_update_enabled"] is True
    assert body["manual_scan_requested"] is False


@pytest.mark.asyncio
async def test_heartbeat_returns_signal_flags(client, session):
    token = await _enroll_agent(client, session)
    r = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {token}"},
        json={"agent_version": "0.1.0", "uptime_seconds": 120},
    )
    assert r.status_code == 200
    body = r.json()
    assert body["config_changed"] is False
    assert body["manual_scan_requested"] is False
    assert body["agent_update_available"] is None
