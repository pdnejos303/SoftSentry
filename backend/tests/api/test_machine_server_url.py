"""Agent-reported server_url: stored on heartbeat, surfaced in the machine list.

This is the "single source of truth" loop — the dashboard shows where each agent
*actually* reports (its config.yaml server_url, echoed on every heartbeat), so an
admin never has to open config.yaml on the endpoint to know which backend a
machine talks to.
"""

from __future__ import annotations

import pytest
from app.core.security import hash_password
from app.models.user import User


async def _admin_access(client, session) -> str:
    session.add(
        User(
            email="admin@local",
            password_hash=hash_password("TestPass!2026Aa"),
            full_name="Admin",
            role="admin",
            is_active=True,
        )
    )
    await session.commit()
    return (
        await client.post(
            "/api/v1/auth/login",
            json={"email": "admin@local", "password": "TestPass!2026Aa"},
        )
    ).json()["access_token"]


async def _enroll(client, access: str) -> tuple[str, str]:
    """Mint an enrollment token, enroll an agent; return (agent_token, machine_uuid)."""
    token = (
        await client.post(
            "/api/v1/agents/enrollment-tokens",
            headers={"Authorization": f"Bearer {access}"},
            json={"expires_in_seconds": 3600},
        )
    ).json()["token"]
    enroll = (
        await client.post(
            "/api/v1/agents/enroll",
            json={
                "enrollment_token": token,
                "hostname": "h",
                "os": "windows",
                "os_version": "10.0.26200",
                "arch": "amd64",
                "agent_version": "0.1.0",
            },
        )
    ).json()
    return enroll["agent_token"], enroll["machine_uuid"]


@pytest.mark.asyncio
async def test_heartbeat_stores_reported_server_url(client, session):
    access = await _admin_access(client, session)
    agent_token, machine_uuid = await _enroll(client, access)

    hb = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {agent_token}"},
        json={
            "agent_version": "0.1.0",
            "uptime_seconds": 1,
            "server_url": "http://localhost:47800",
        },
    )
    assert hb.status_code == 200

    # Surfaced in the dashboard machine list...
    listing = await client.get(
        "/api/v1/machines",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert listing.status_code == 200
    item = next(m for m in listing.json()["items"] if m["uuid"] == machine_uuid)
    assert item["reported_server_url"] == "http://localhost:47800"

    # ...and in machine detail.
    detail = await client.get(
        f"/api/v1/machines/{machine_uuid}",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert detail.status_code == 200
    assert detail.json()["reported_server_url"] == "http://localhost:47800"


@pytest.mark.asyncio
async def test_machine_server_url_null_before_first_heartbeat(client, session):
    """An agent that never heartbeats (or an older agent) reports null, not an error."""
    access = await _admin_access(client, session)
    _, machine_uuid = await _enroll(client, access)

    listing = await client.get(
        "/api/v1/machines",
        headers={"Authorization": f"Bearer {access}"},
    )
    item = next(m for m in listing.json()["items"] if m["uuid"] == machine_uuid)
    assert item["reported_server_url"] is None
