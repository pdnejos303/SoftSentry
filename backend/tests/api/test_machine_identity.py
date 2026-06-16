"""Machine identity: fingerprint dedup on re-enroll + admin rename.

Two operator concerns when rolling out the one-click installer to a fleet:

1. Re-running the installer on the *same* box must NOT spawn a duplicate machine
   row — the server dedups by a stable hardware fingerprint the agent reports.
2. Admins can give each machine a friendly `display_name` + `owner` so "whose
   machine is this?" is answerable (hostnames like ``DESKTOP-AB12`` don't say).
"""

from __future__ import annotations

import pytest
from app.core.security import hash_password
from app.models.machine import Machine
from app.models.user import User
from sqlalchemy import func, select


async def _make_admin(session) -> str:
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
    return password


async def _login(client, password: str) -> str:
    r = await client.post(
        "/api/v1/auth/login", json={"email": "admin@local", "password": password}
    )
    return r.json()["access_token"]


def _auth(token: str) -> dict:
    return {"Authorization": f"Bearer {token}"}


async def _admin_token(client, session) -> str:
    return await _login(client, await _make_admin(session))


async def _deployment_token(client, admin: str) -> str:
    """A reusable token (max_uses unlimited) so we can enroll the same box twice."""
    r = await client.post("/api/v1/deploy/tokens", headers=_auth(admin), json={"label": "fleet"})
    return r.json()["token"]


def _enroll_body(token: str, *, hostname: str, fingerprint: str | None = None) -> dict:
    body = {
        "enrollment_token": token,
        "hostname": hostname,
        "os": "windows",
        "os_version": "11.0.26200",
        "arch": "amd64",
        "agent_version": "0.1.0",
    }
    if fingerprint is not None:
        body["fingerprint"] = fingerprint
    return body


async def _machine_count(session) -> int:
    return (await session.execute(select(func.count()).select_from(Machine))).scalar_one()


@pytest.mark.asyncio
async def test_reenroll_same_fingerprint_reuses_machine(client, session):
    admin = await _admin_token(client, session)
    dt = await _deployment_token(client, admin)

    first = await client.post(
        "/api/v1/agents/enroll", json=_enroll_body(dt, hostname="pc-01", fingerprint="fp-1")
    )
    second = await client.post(
        "/api/v1/agents/enroll", json=_enroll_body(dt, hostname="pc-01", fingerprint="fp-1")
    )
    assert first.status_code == 201
    assert second.status_code == 201
    # Same identity — not a duplicate row.
    assert first.json()["machine_uuid"] == second.json()["machine_uuid"]
    assert await _machine_count(session) == 1
    # Freshly issued token authenticates against the same machine.
    me = await client.get("/api/v1/agents/me", headers=_auth(second.json()["agent_token"]))
    assert me.status_code == 200
    assert me.json()["hostname"] == "pc-01"


@pytest.mark.asyncio
async def test_reenroll_preserves_admin_naming(client, session):
    admin = await _admin_token(client, session)
    dt = await _deployment_token(client, admin)
    enroll = await client.post(
        "/api/v1/agents/enroll", json=_enroll_body(dt, hostname="pc-02", fingerprint="fp-2")
    )
    uuid = enroll.json()["machine_uuid"]

    await client.patch(
        f"/api/v1/machines/{uuid}",
        headers=_auth(admin),
        json={"display_name": "Somchai's Laptop", "owner": "somchai"},
    )
    # Re-run the installer (same fingerprint) — must not clobber the admin's naming.
    await client.post(
        "/api/v1/agents/enroll", json=_enroll_body(dt, hostname="pc-02", fingerprint="fp-2")
    )

    detail = (await client.get(f"/api/v1/machines/{uuid}", headers=_auth(admin))).json()
    assert detail["display_name"] == "Somchai's Laptop"
    assert detail["owner"] == "somchai"


@pytest.mark.asyncio
async def test_different_fingerprints_create_separate_machines(client, session):
    admin = await _admin_token(client, session)
    dt = await _deployment_token(client, admin)
    await client.post(
        "/api/v1/agents/enroll", json=_enroll_body(dt, hostname="pc-a", fingerprint="fp-a")
    )
    await client.post(
        "/api/v1/agents/enroll", json=_enroll_body(dt, hostname="pc-b", fingerprint="fp-b")
    )
    assert await _machine_count(session) == 2


@pytest.mark.asyncio
async def test_enroll_without_fingerprint_always_creates(client, session):
    # Back-compat: an older agent that sends no fingerprint can't be deduped, so
    # each enroll stays a distinct machine (today's behaviour, preserved).
    admin = await _admin_token(client, session)
    dt = await _deployment_token(client, admin)
    await client.post("/api/v1/agents/enroll", json=_enroll_body(dt, hostname="legacy"))
    await client.post("/api/v1/agents/enroll", json=_enroll_body(dt, hostname="legacy"))
    assert await _machine_count(session) == 2


@pytest.mark.asyncio
async def test_patch_display_name_and_owner(client, session):
    admin = await _admin_token(client, session)
    dt = await _deployment_token(client, admin)
    enroll = await client.post(
        "/api/v1/agents/enroll", json=_enroll_body(dt, hostname="pc-x", fingerprint="fp-x")
    )
    uuid = enroll.json()["machine_uuid"]

    r = await client.patch(
        f"/api/v1/machines/{uuid}",
        headers=_auth(admin),
        json={"display_name": "Reception Desk", "owner": "front-office"},
    )
    assert r.status_code == 200
    assert r.json()["display_name"] == "Reception Desk"
    assert r.json()["owner"] == "front-office"


@pytest.mark.asyncio
async def test_patch_partial_keeps_other_fields(client, session):
    admin = await _admin_token(client, session)
    dt = await _deployment_token(client, admin)
    enroll = await client.post(
        "/api/v1/agents/enroll", json=_enroll_body(dt, hostname="pc-y", fingerprint="fp-y")
    )
    uuid = enroll.json()["machine_uuid"]
    await client.patch(
        f"/api/v1/machines/{uuid}",
        headers=_auth(admin),
        json={"display_name": "Lab PC", "tags": ["lab"]},
    )
    # Patching tags alone must not wipe the previously-set display_name.
    r = await client.patch(
        f"/api/v1/machines/{uuid}", headers=_auth(admin), json={"tags": ["lab", "vip"]}
    )
    assert r.json()["display_name"] == "Lab PC"
    assert r.json()["tags"] == ["lab", "vip"]


@pytest.mark.asyncio
async def test_list_search_matches_display_name_and_owner(client, session):
    admin = await _admin_token(client, session)
    dt = await _deployment_token(client, admin)
    enroll = await client.post(
        "/api/v1/agents/enroll", json=_enroll_body(dt, hostname="DESKTOP-AB12", fingerprint="fp-z")
    )
    uuid = enroll.json()["machine_uuid"]
    await client.patch(
        f"/api/v1/machines/{uuid}",
        headers=_auth(admin),
        json={"display_name": "CEO Laptop", "owner": "Pranee"},
    )

    by_name = await client.get("/api/v1/machines", headers=_auth(admin), params={"q": "CEO"})
    assert by_name.json()["total"] == 1
    by_owner = await client.get("/api/v1/machines", headers=_auth(admin), params={"q": "Pranee"})
    assert by_owner.json()["total"] == 1
