"""Inventory API tests (Phase 2 A3 + A4).

A3 — per-machine software / history / scans.
A4 — cross-machine /software, /software/top, /software/compare.
"""

from __future__ import annotations

import pytest
from app.core.security import hash_password
from app.models.user import User


async def _make_user(session, *, email: str, role: str) -> str:
    password = "TestPass!2026Aa"
    session.add(
        User(
            email=email,
            password_hash=hash_password(password),
            full_name=role.title(),
            role=role,
            is_active=True,
        )
    )
    await session.commit()
    return password


async def _login(client, email: str, password: str) -> str:
    r = await client.post("/api/v1/auth/login", json={"email": email, "password": password})
    return r.json()["access_token"]


async def _admin_token(client, session) -> str:
    pw = await _make_user(session, email="admin@local", role="dev")
    return await _login(client, "admin@local", pw)


def _auth(token: str) -> dict:
    return {"Authorization": f"Bearer {token}"}


async def _enroll(client, admin_token: str, hostname: str) -> str:
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers=_auth(admin_token),
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


async def _scan(client, agent_token: str, software: list[dict]) -> None:
    from datetime import UTC, datetime

    now = datetime.now(tz=UTC).isoformat()
    r = await client.post(
        "/api/v1/agents/scans",
        headers=_auth(agent_token),
        json={
            "started_at": now,
            "completed_at": now,
            "scan_type": "auto",
            "software": software,
        },
    )
    assert r.status_code == 202


async def _machine_uuid(client, admin_token: str, hostname: str) -> str:
    body = (await client.get("/api/v1/machines", headers=_auth(admin_token))).json()
    return next(m["uuid"] for m in body["items"] if m["hostname"] == hostname)


# ── A3 ──────────────────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_machine_software_lists_active_with_signature(client, session):
    admin = await _admin_token(client, session)
    agent = await _enroll(client, admin, "inv-a")
    await _scan(
        client,
        agent,
        [
            {
                "name": "Google Chrome",
                "version": "120.0",
                "publisher": "Google",
                "source": "registry",
                "signature": {"status": "valid", "signer": "Google LLC"},
            },
            {"name": "7-Zip", "version": "19.00", "source": "registry"},
        ],
    )
    uuid = await _machine_uuid(client, admin, "inv-a")

    r = await client.get(f"/api/v1/machines/{uuid}/software", headers=_auth(admin))
    assert r.status_code == 200
    body = r.json()
    assert body["total"] == 2
    chrome = next(i for i in body["items"] if i["name"] == "Google Chrome")
    assert chrome["signature_status"] == "valid"
    assert chrome["is_active"] is True


@pytest.mark.asyncio
async def test_machine_software_filter_by_query(client, session):
    admin = await _admin_token(client, session)
    agent = await _enroll(client, admin, "inv-q")
    await _scan(
        client,
        agent,
        [
            {"name": "Google Chrome", "version": "120.0", "source": "registry"},
            {"name": "Mozilla Firefox", "version": "115.0", "source": "registry"},
        ],
    )
    uuid = await _machine_uuid(client, admin, "inv-q")

    r = await client.get(
        f"/api/v1/machines/{uuid}/software", headers=_auth(admin), params={"q": "chrome"}
    )
    assert r.json()["total"] == 1
    assert r.json()["items"][0]["name"] == "Google Chrome"


@pytest.mark.asyncio
async def test_machine_history_records_install_and_update(client, session):
    admin = await _admin_token(client, session)
    agent = await _enroll(client, admin, "inv-hist")
    await _scan(client, agent, [{"name": "App", "version": "1.0", "source": "registry"}])
    # second scan upgrades App 1.0 -> 2.0 (one updated event)
    await _scan(client, agent, [{"name": "App", "version": "2.0", "source": "registry"}])
    uuid = await _machine_uuid(client, admin, "inv-hist")

    r = await client.get(f"/api/v1/machines/{uuid}/history", headers=_auth(admin))
    assert r.status_code == 200
    events = {e["event"] for e in r.json()["items"]}
    assert "installed" in events
    assert "updated" in events


@pytest.mark.asyncio
async def test_machine_scans_history(client, session):
    admin = await _admin_token(client, session)
    agent = await _enroll(client, admin, "inv-scan")
    await _scan(client, agent, [{"name": "App", "version": "1.0", "source": "registry"}])
    uuid = await _machine_uuid(client, admin, "inv-scan")

    r = await client.get(f"/api/v1/machines/{uuid}/scans", headers=_auth(admin))
    assert r.status_code == 200
    assert r.json()["total"] == 1
    assert r.json()["items"][0]["software_count"] == 1


# ── A4 ──────────────────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_cross_software_aggregates_installed_count(client, session):
    admin = await _admin_token(client, session)
    a = await _enroll(client, admin, "cross-a")
    b = await _enroll(client, admin, "cross-b")
    await _scan(client, a, [{"name": "Google Chrome", "version": "120.0", "source": "registry"}])
    await _scan(client, b, [{"name": "Google Chrome", "version": "120.0", "source": "registry"}])

    r = await client.get("/api/v1/software", headers=_auth(admin))
    assert r.status_code == 200
    chrome = next(i for i in r.json()["items"] if i["name"] == "Google Chrome")
    assert chrome["installed_count"] == 2
    assert len(chrome["machines"]) == 2


@pytest.mark.asyncio
async def test_top_software(client, session):
    admin = await _admin_token(client, session)
    a = await _enroll(client, admin, "top-a")
    b = await _enroll(client, admin, "top-b")
    await _scan(
        client,
        a,
        [
            {"name": "Common", "version": "1", "source": "registry"},
            {"name": "OnlyA", "version": "1", "source": "registry"},
        ],
    )
    await _scan(client, b, [{"name": "Common", "version": "1", "source": "registry"}])

    r = await client.get("/api/v1/software/top", headers=_auth(admin), params={"limit": 5})
    assert r.status_code == 200
    items = r.json()
    assert items[0]["name"] == "Common"
    assert items[0]["installed_count"] == 2


@pytest.mark.asyncio
async def test_compare_two_machines(client, session):
    admin = await _admin_token(client, session)
    a = await _enroll(client, admin, "cmp-a")
    b = await _enroll(client, admin, "cmp-b")
    await _scan(
        client,
        a,
        [
            {"name": "Shared", "version": "1.0", "source": "registry"},
            {"name": "Chrome", "version": "120", "source": "registry"},
            {"name": "OnlyA", "version": "9", "source": "registry"},
        ],
    )
    await _scan(
        client,
        b,
        [
            {"name": "Shared", "version": "1.0", "source": "registry"},
            {"name": "Chrome", "version": "121", "source": "registry"},
            {"name": "OnlyB", "version": "9", "source": "registry"},
        ],
    )
    ua = await _machine_uuid(client, admin, "cmp-a")
    ub = await _machine_uuid(client, admin, "cmp-b")

    r = await client.post(
        "/api/v1/software/compare",
        headers=_auth(admin),
        json={"machine_uuids": [ua, ub]},
    )
    assert r.status_code == 200
    body = r.json()
    assert {"name": "Shared", "version": "1.0"} in body["common"]
    assert {"name": "OnlyA", "version": "9"} in body["only_in_a"]
    assert {"name": "OnlyB", "version": "9"} in body["only_in_b"]
    diff = next(d for d in body["version_diff"] if d["name"] == "Chrome")
    assert diff["version_a"] == "120"
    assert diff["version_b"] == "121"


@pytest.mark.asyncio
async def test_inventory_endpoints_require_auth(client, session):
    assert (await client.get("/api/v1/software")).status_code == 401
    assert (await client.get("/api/v1/software/top")).status_code == 401
