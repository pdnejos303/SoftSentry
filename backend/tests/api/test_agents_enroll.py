"""Enrollment flow tests — token issue + agent enroll + reuse rejection."""

from __future__ import annotations

import pytest
from app.core.security import hash_password
from app.models.user import User


async def _make_admin(session) -> tuple[User, str]:
    password = "TestPass!2026Aa"
    user = User(
        email="admin@local",
        password_hash=hash_password(password),
        full_name="Admin",
        role="admin",
        is_active=True,
    )
    session.add(user)
    await session.commit()
    await session.refresh(user)
    return user, password


async def _login(client, password: str) -> str:
    r = await client.post(
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": password},
    )
    return r.json()["access_token"]


@pytest.mark.asyncio
async def test_enrollment_token_creation_requires_admin(client, session):
    r = await client.post("/api/v1/agents/enrollment-tokens", json={})
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_full_enroll_flow(client, session):
    _, password = await _make_admin(session)
    access = await _login(client, password)

    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers={"Authorization": f"Bearer {access}"},
        json={"label": "lab-01", "expires_in_seconds": 3600},
    )
    assert create.status_code == 201
    body = create.json()
    enrollment_token = body["token"]
    assert enrollment_token
    assert body["label"] == "lab-01"

    enroll = await client.post(
        "/api/v1/agents/enroll",
        json={
            "enrollment_token": enrollment_token,
            "hostname": "lab-01-pc",
            "os": "windows",
            "os_version": "11.0.26200",
            "arch": "amd64",
            "agent_version": "0.1.0",
        },
    )
    assert enroll.status_code == 201
    enroll_body = enroll.json()
    agent_token = enroll_body["agent_token"]
    assert agent_token
    assert enroll_body["machine_uuid"]

    # Agent can call its own /me with the bearer
    me = await client.get(
        "/api/v1/agents/me",
        headers={"Authorization": f"Bearer {agent_token}"},
    )
    assert me.status_code == 200
    assert me.json()["hostname"] == "lab-01-pc"


@pytest.mark.asyncio
async def test_enrollment_token_single_use(client, session):
    _, password = await _make_admin(session)
    access = await _login(client, password)
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers={"Authorization": f"Bearer {access}"},
        json={"expires_in_seconds": 3600},
    )
    enrollment_token = create.json()["token"]

    payload = {
        "enrollment_token": enrollment_token,
        "hostname": "host-a",
        "os": "macos",
        "os_version": "14.5",
        "arch": "arm64",
        "agent_version": "0.1.0",
    }
    first = await client.post("/api/v1/agents/enroll", json=payload)
    assert first.status_code == 201

    second = await client.post("/api/v1/agents/enroll", json={**payload, "hostname": "host-b"})
    assert second.status_code == 401


@pytest.mark.asyncio
async def test_enroll_with_garbage_token_rejected(client, session):
    r = await client.post(
        "/api/v1/agents/enroll",
        json={
            "enrollment_token": "this-is-not-a-real-token",
            "hostname": "x",
            "os": "windows",
            "os_version": "11",
            "arch": "amd64",
            "agent_version": "0.1.0",
        },
    )
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_agent_heartbeat_updates_last_seen(client, session):
    _, password = await _make_admin(session)
    access = await _login(client, password)
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers={"Authorization": f"Bearer {access}"},
        json={"expires_in_seconds": 3600},
    )
    enroll = await client.post(
        "/api/v1/agents/enroll",
        json={
            "enrollment_token": create.json()["token"],
            "hostname": "h",
            "os": "windows",
            "os_version": "11",
            "arch": "amd64",
            "agent_version": "0.1.0",
        },
    )
    token = enroll.json()["agent_token"]

    hb = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {token}"},
    )
    assert hb.status_code == 200
    assert hb.json()["manual_scan_requested"] is False
