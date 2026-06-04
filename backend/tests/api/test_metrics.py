"""Telemetry — the /metrics scrape endpoint, populated through the real
request path (HTTP middleware) and a real agent scan ingest."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.core.security import hash_password
from app.models.user import User

pytestmark = pytest.mark.asyncio


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
        "/api/v1/auth/login", json={"email": "admin@local", "password": password}
    )
    return r.json()["access_token"]


def _hdr(access: str) -> dict:
    return {"Authorization": f"Bearer {access}"}


async def _enroll(client, access: str) -> str:
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers=_hdr(access),
        json={"expires_in_seconds": 3600},
    )
    enroll = await client.post(
        "/api/v1/agents/enroll",
        json={
            "enrollment_token": create.json()["token"],
            "hostname": "host-metrics",
            "os": "windows",
            "os_version": "11.0.26200",
            "arch": "amd64",
            "agent_version": "0.1.0",
        },
    )
    return enroll.json()["agent_token"]


async def test_metrics_endpoint_exposes_families(client, session) -> None:
    access = await _admin_access(client, session)
    agent_token = await _enroll(client, access)

    now = datetime.now(tz=UTC)
    scan = await client.post(
        "/api/v1/agents/scans",
        headers={"Authorization": f"Bearer {agent_token}"},
        json={
            "started_at": (now - timedelta(seconds=4)).isoformat(),
            "completed_at": now.isoformat(),
            "scan_type": "auto",
            "software": [
                {
                    "name": "Google Chrome",
                    "version": "120.0",
                    "publisher": "Google LLC",
                    "source": "registry",
                    "arch": "x64",
                }
            ],
        },
    )
    assert scan.status_code == 202

    resp = await client.get("/metrics")
    assert resp.status_code == 200
    assert resp.headers["content-type"].startswith("text/plain")
    body = resp.text

    # Agent family — driven by the scan we just pushed.
    assert 'softsentry_agent_scans_total{scan_type="auto"}' in body
    assert "softsentry_agent_scan_duration_seconds_count" in body
    assert "softsentry_agent_scan_software_count_count" in body
    # HTTP family — the middleware recorded the scan POST on its route template.
    assert 'path="/api/v1/agents/scans"' in body
    # /metrics itself is excluded from the HTTP counters.
    assert 'path="/metrics"' not in body
