"""Machines dashboard API tests (Phase 2 A2).

Covers list (filter/paginate/computed status/software_count), detail, admin-only
tag edit + soft delete, and the trigger-scan toggle the agent reads via heartbeat.
"""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.core.security import hash_password
from app.models.machine import Machine
from app.models.user import User
from sqlalchemy import select


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
    pw = await _make_user(session, email="admin@local", role="admin")
    return await _login(client, "admin@local", pw)


async def _enroll(client, admin_token: str, hostname: str) -> str:
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers={"Authorization": f"Bearer {admin_token}"},
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


def _auth(token: str) -> dict:
    return {"Authorization": f"Bearer {token}"}


@pytest.mark.asyncio
async def test_list_machines_requires_auth(client, session):
    r = await client.get("/api/v1/machines")
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_list_machines_returns_enrolled_with_phase2_stub_fields(client, session):
    admin = await _admin_token(client, session)
    await _enroll(client, admin, "desktop-a")
    await _enroll(client, admin, "desktop-b")

    r = await client.get("/api/v1/machines", headers=_auth(admin))
    assert r.status_code == 200
    body = r.json()
    assert body["total"] == 2
    assert body["page"] == 1
    assert len(body["items"]) == 2

    item = body["items"][0]
    assert {"uuid", "hostname", "os", "status", "software_count"} <= item.keys()
    assert item["software_count"] == 0
    # CVE scoring lands in Phase 3 — stubbed to zero for now.
    assert item["vulnerability_count"] == {"critical": 0, "high": 0, "medium": 0, "low": 0}
    assert item["risk_score"] == 0


@pytest.mark.asyncio
async def test_list_machines_filter_by_query(client, session):
    admin = await _admin_token(client, session)
    await _enroll(client, admin, "finance-pc")
    await _enroll(client, admin, "hr-laptop")

    r = await client.get("/api/v1/machines", headers=_auth(admin), params={"q": "finance"})
    assert r.status_code == 200
    body = r.json()
    assert body["total"] == 1
    assert body["items"][0]["hostname"] == "finance-pc"


@pytest.mark.asyncio
async def test_list_machines_status_computed_from_last_seen(client, session):
    admin = await _admin_token(client, session)
    await _enroll(client, admin, "stale-pc")

    machine = (await session.execute(select(Machine))).scalar_one()
    machine.last_seen_at = datetime.now(tz=UTC) - timedelta(hours=2)
    await session.commit()

    r = await client.get("/api/v1/machines", headers=_auth(admin))
    assert r.json()["items"][0]["status"] == "offline"


@pytest.mark.asyncio
async def test_list_machines_software_count_reflects_active_records(client, session):
    admin = await _admin_token(client, session)
    agent = await _enroll(client, admin, "with-software")

    now = datetime.now(tz=UTC)
    await client.post(
        "/api/v1/agents/scans",
        headers=_auth(agent),
        json={
            "started_at": now.isoformat(),
            "completed_at": now.isoformat(),
            "scan_type": "auto",
            "software": [
                {"name": "Chrome", "version": "120", "source": "registry"},
                {"name": "Firefox", "version": "121", "source": "registry"},
            ],
        },
    )

    r = await client.get("/api/v1/machines", headers=_auth(admin))
    assert r.json()["items"][0]["software_count"] == 2


@pytest.mark.asyncio
async def test_machine_detail_returns_full_record(client, session):
    admin = await _admin_token(client, session)
    await _enroll(client, admin, "detail-pc")
    uuid = (await client.get("/api/v1/machines", headers=_auth(admin))).json()["items"][0]["uuid"]

    r = await client.get(f"/api/v1/machines/{uuid}", headers=_auth(admin))
    assert r.status_code == 200
    body = r.json()
    assert body["hostname"] == "detail-pc"
    assert body["arch"] == "amd64"
    assert "enrolled_at" in body


@pytest.mark.asyncio
async def test_patch_tags_requires_admin(client, session):
    admin = await _admin_token(client, session)
    await _enroll(client, admin, "tag-pc")
    uuid = (await client.get("/api/v1/machines", headers=_auth(admin))).json()["items"][0]["uuid"]

    viewer_pw = await _make_user(session, email="viewer@local", role="viewer")
    viewer = await _login(client, "viewer@local", viewer_pw)

    denied = await client.patch(
        f"/api/v1/machines/{uuid}", headers=_auth(viewer), json={"tags": ["x"]}
    )
    assert denied.status_code == 403

    ok = await client.patch(
        f"/api/v1/machines/{uuid}", headers=_auth(admin), json={"tags": ["finance", "vip"]}
    )
    assert ok.status_code == 200
    assert ok.json()["tags"] == ["finance", "vip"]


@pytest.mark.asyncio
async def test_delete_soft_deletes_and_rejects_agent_token(client, session):
    admin = await _admin_token(client, session)
    agent = await _enroll(client, admin, "doomed-pc")
    uuid = (await client.get("/api/v1/machines", headers=_auth(admin))).json()["items"][0]["uuid"]

    d = await client.delete(f"/api/v1/machines/{uuid}", headers=_auth(admin))
    assert d.status_code == 204

    # No longer listed
    assert (await client.get("/api/v1/machines", headers=_auth(admin))).json()["total"] == 0
    # Agent token now rejected (machine soft-deleted)
    hb = await client.post("/api/v1/agents/heartbeat", headers=_auth(agent))
    assert hb.status_code == 401


@pytest.mark.asyncio
async def test_trigger_scan_sets_flag_seen_by_agent(client, session):
    admin = await _admin_token(client, session)
    agent = await _enroll(client, admin, "trigger-pc")
    uuid = (await client.get("/api/v1/machines", headers=_auth(admin))).json()["items"][0]["uuid"]

    t = await client.post(f"/api/v1/machines/{uuid}/trigger-scan", headers=_auth(admin))
    assert t.status_code == 202

    hb = await client.post("/api/v1/agents/heartbeat", headers=_auth(agent))
    assert hb.json()["manual_scan_requested"] is True
