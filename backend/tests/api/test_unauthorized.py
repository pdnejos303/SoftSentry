"""Module 4 — unauthorized/blacklisted detection end-to-end.

Drives the real endpoints: policy CRUD + scan ingest + alert listing, asserting
the auto-detect, dedup, and auto-resolve behaviour from spec 4.4/4.5.
"""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.core.security import hash_password
from app.models.user import User


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


async def _enroll_agent(client, access: str, hostname: str = "host-1") -> str:
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


def _scan(*names_versions: tuple[str, str]) -> dict:
    now = datetime.now(tz=UTC)
    return {
        "started_at": (now - timedelta(seconds=5)).isoformat(),
        "completed_at": now.isoformat(),
        "scan_type": "auto",
        "trigger": "schedule",
        "software": [
            {"name": n, "version": v, "publisher": "ACME", "source": "registry", "arch": "x64"}
            for n, v in names_versions
        ],
    }


async def _post_scan(client, token: str, payload: dict):
    return await client.post(
        "/api/v1/agents/scans",
        headers={"Authorization": f"Bearer {token}"},
        json=payload,
    )


async def _alerts(client, access: str, **params) -> list[dict]:
    r = await client.get(
        "/api/v1/alerts", headers={"Authorization": f"Bearer {access}"}, params=params
    )
    assert r.status_code == 200, r.text
    return r.json()["items"]


@pytest.mark.asyncio
async def test_blacklist_hit_creates_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)

    add = await client.post(
        "/api/v1/blacklist",
        headers={"Authorization": f"Bearer {access}"},
        json={"name_pattern": "uTorrent%", "severity": "high", "reason": "P2P prohibited"},
    )
    assert add.status_code == 201, add.text

    await _post_scan(client, agent, _scan(("uTorrent", "7.10"), ("Google Chrome", "120.0")))

    alerts = await _alerts(client, access)
    blk = [a for a in alerts if a["type"] == "blacklisted_software"]
    assert len(blk) == 1
    assert blk[0]["severity"] == "high"
    assert blk[0]["software_name"] == "uTorrent"
    assert blk[0]["status"] == "active"


@pytest.mark.asyncio
async def test_rescan_does_not_duplicate_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await client.post(
        "/api/v1/blacklist",
        headers={"Authorization": f"Bearer {access}"},
        json={"name_pattern": "uTorrent%", "reason": "P2P"},
    )

    await _post_scan(client, agent, _scan(("uTorrent", "7.10")))
    await _post_scan(client, agent, _scan(("uTorrent", "7.10")))

    alerts = await _alerts(client, access, type="blacklisted_software")
    assert len(alerts) == 1


@pytest.mark.asyncio
async def test_uninstall_auto_resolves_alert(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await client.post(
        "/api/v1/blacklist",
        headers={"Authorization": f"Bearer {access}"},
        json={"name_pattern": "uTorrent%", "reason": "P2P"},
    )

    await _post_scan(client, agent, _scan(("uTorrent", "7.10"), ("Chrome", "120")))
    # uninstall uTorrent: next scan omits it
    await _post_scan(client, agent, _scan(("Chrome", "120")))

    active = await _alerts(client, access, status="active")
    assert [a for a in active if a["type"] == "blacklisted_software"] == []
    resolved = await _alerts(client, access, status="resolved")
    assert len(resolved) == 1
    assert resolved[0]["resolved_at"] is not None


@pytest.mark.asyncio
async def test_whitelist_mode_flags_unlisted_software(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    # Enable whitelist enforcement, allow only Chrome.
    await client.patch(
        "/api/v1/policy",
        headers={"Authorization": f"Bearer {access}"},
        json={"whitelist_mode": True},
    )
    await client.post(
        "/api/v1/whitelist",
        headers={"Authorization": f"Bearer {access}"},
        json={"name_pattern": "Google Chrome%"},
    )

    await _post_scan(client, agent, _scan(("Google Chrome", "120.0"), ("Sketchy Tool", "1.0")))

    alerts = await _alerts(client, access, type="unauthorized_software")
    assert len(alerts) == 1
    assert alerts[0]["software_name"] == "Sketchy Tool"
    assert alerts[0]["severity"] == "medium"


@pytest.mark.asyncio
async def test_acknowledge_and_resolve_endpoints(client, session):
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access)
    await client.post(
        "/api/v1/blacklist",
        headers={"Authorization": f"Bearer {access}"},
        json={"name_pattern": "uTorrent%", "reason": "P2P"},
    )
    await _post_scan(client, agent, _scan(("uTorrent", "7.10")))

    alert_uuid = (await _alerts(client, access))[0]["uuid"]
    hdr = {"Authorization": f"Bearer {access}"}

    ack = await client.post(f"/api/v1/alerts/{alert_uuid}/acknowledge", headers=hdr)
    assert ack.status_code == 200
    assert ack.json()["status"] == "acknowledged"

    res = await client.post(f"/api/v1/alerts/{alert_uuid}/resolve", headers=hdr)
    assert res.status_code == 200
    assert res.json()["status"] == "resolved"
    assert res.json()["resolved_at"] is not None
