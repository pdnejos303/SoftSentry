"""Mutations across the detection/compliance surface must land in the audit log
(Module 9 spec 9.6 — "log ทุก mutation"; ROADMAP Phase 5 acceptance "edit
license → เห็นใน log"). Each test drives a real HTTP mutation then reads it back
through the audit-log API."""

from __future__ import annotations

import pytest
from app.core.security import hash_password
from app.models.user import User


@pytest.fixture
def _stub_enqueue(monkeypatch) -> None:
    """No live arq/redis in tests — stub the report enqueue so /generate returns."""

    async def _noop(_report_uuid) -> bool:
        return True

    monkeypatch.setattr("app.api.v1.reports.enqueue_report", _noop)


def _hdr(access: str) -> dict:
    return {"Authorization": f"Bearer {access}"}


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


async def _audit_actions(client, access: str, **params) -> list[dict]:
    r = await client.get("/api/v1/audit-logs", headers=_hdr(access), params=params)
    assert r.status_code == 200, r.text
    return r.json()["items"]


@pytest.mark.asyncio
async def test_license_lifecycle_is_audited(client, session):
    admin = await _admin_access(client, session)

    created = await client.post(
        "/api/v1/licenses",
        headers=_hdr(admin),
        json={"software_name": "Google Chrome", "purchased_count": 20},
    )
    assert created.status_code == 201, created.text
    lic_uuid = created.json()["uuid"]

    upd = await client.patch(
        f"/api/v1/licenses/{lic_uuid}",
        headers=_hdr(admin),
        json={"purchased_count": 50},
    )
    assert upd.status_code == 200, upd.text

    deleted = await client.delete(f"/api/v1/licenses/{lic_uuid}", headers=_hdr(admin))
    assert deleted.status_code == 204

    # create + update + delete all recorded, attributed to the admin, scoped to uuid.
    items = await _audit_actions(client, admin, entity_type="license")
    by_action = {i["action"]: i for i in items}
    assert {"license.create", "license.update", "license.delete"} <= set(by_action)
    assert by_action["license.update"]["entity_id"] == lic_uuid
    # before/after captured on edit; secret key never present.
    changes = by_action["license.update"]["changes"]
    assert changes["before"]["purchased_count"] == "20"
    assert changes["after"]["purchased_count"] == "50"
    assert "license_key" not in changes["after"]
    assert by_action["license.create"]["actor_email"] == "admin@local"


@pytest.mark.asyncio
async def test_policy_and_blacklist_mutations_are_audited(client, session):
    admin = await _admin_access(client, session)

    bl = await client.post(
        "/api/v1/blacklist",
        headers=_hdr(admin),
        json={"name_pattern": "uTorrent%", "severity": "high", "reason": "P2P"},
    )
    assert bl.status_code == 201, bl.text

    pol = await client.patch(
        "/api/v1/policy", headers=_hdr(admin), json={"whitelist_mode": True}
    )
    assert pol.status_code == 200, pol.text

    actions = {i["action"] for i in await _audit_actions(client, admin)}
    assert "blacklist.create" in actions
    assert "policy.update" in actions


@pytest.mark.asyncio
async def test_report_generate_is_audited(client, session, _stub_enqueue):
    admin = await _admin_access(client, session)
    gen = await client.post(
        "/api/v1/reports/generate",
        headers=_hdr(admin),
        json={"type": "org_summary", "format": "pdf"},
    )
    assert gen.status_code == 202, gen.text
    items = await _audit_actions(client, admin, action="report.generate")
    assert items and items[0]["entity_id"] == str(gen.json()["uuid"])
