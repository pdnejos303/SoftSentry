"""Live scan progress: stored on heartbeat, surfaced on the machine list/detail,
and cleared back to idle when the agent stops reporting it.

Lets the dashboard render a progress bar that follows the agent's scan without a
separate endpoint — progress simply rides along on the existing heartbeat.
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


async def _heartbeat(client, agent_token: str, **extra) -> None:
    body = {"agent_version": "0.1.0", "uptime_seconds": 1, **extra}
    resp = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {agent_token}"},
        json=body,
    )
    assert resp.status_code == 200


async def _list_item(client, access: str, machine_uuid: str) -> dict:
    listing = await client.get(
        "/api/v1/machines", headers={"Authorization": f"Bearer {access}"}
    )
    assert listing.status_code == 200
    return next(m for m in listing.json()["items"] if m["uuid"] == machine_uuid)


@pytest.mark.asyncio
async def test_heartbeat_with_progress_is_surfaced(client, session):
    access = await _admin_access(client, session)
    agent_token, machine_uuid = await _enroll(client, access)

    await _heartbeat(
        client,
        agent_token,
        scan_progress={
            "phase": "scanning",
            "done": 120,
            "total": 300,
            "current_path": r"C:\Program Files\App\app.exe",
        },
    )

    item = await _list_item(client, access, machine_uuid)
    assert item["scan_progress"] is not None
    assert item["scan_progress"]["phase"] == "scanning"
    assert item["scan_progress"]["done"] == 120
    assert item["scan_progress"]["total"] == 300

    detail = await client.get(
        f"/api/v1/machines/{machine_uuid}",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert detail.status_code == 200
    assert detail.json()["scan_progress"]["phase"] == "scanning"


@pytest.mark.asyncio
async def test_idle_heartbeat_clears_progress(client, session):
    """A heartbeat without scan_progress (idle) resets the bar to None."""
    access = await _admin_access(client, session)
    agent_token, machine_uuid = await _enroll(client, access)

    await _heartbeat(
        client,
        agent_token,
        scan_progress={"phase": "scanning", "done": 5, "total": 10},
    )
    assert (await _list_item(client, access, machine_uuid))["scan_progress"] is not None

    # Next heartbeat omits scan_progress → machine is idle again.
    await _heartbeat(client, agent_token)
    assert (await _list_item(client, access, machine_uuid))["scan_progress"] is None


@pytest.mark.asyncio
async def test_progress_null_before_first_scan(client, session):
    access = await _admin_access(client, session)
    _, machine_uuid = await _enroll(client, access)

    item = await _list_item(client, access, machine_uuid)
    assert item["scan_progress"] is None
