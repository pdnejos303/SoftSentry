"""Module 6 — license CRUD, live install counts, status, drill-down, compliance
summary, encryption-at-rest, and expiry-alert reconciliation, end-to-end."""

from __future__ import annotations

from datetime import UTC, date, datetime, timedelta

import pytest
from app.core.security import hash_password
from app.models.user import User
from sqlalchemy import text


async def _admin_access(client, session) -> str:
    password = "TestPass!2026Aa"
    session.add(
        User(
            email="admin@local",
            password_hash=hash_password(password),
            full_name="Admin",
            role="dev",
            is_active=True,
        )
    )
    await session.commit()
    r = await client.post("/api/v1/auth/login", json={"email": "admin@local", "password": password})
    return r.json()["access_token"]


async def _viewer_access(client, session) -> str:
    password = "ViewPass!2026Aa"
    session.add(
        User(
            email="viewer@local",
            password_hash=hash_password(password),
            full_name="Viewer",
            role="viewer",
            is_active=True,
        )
    )
    await session.commit()
    r = await client.post(
        "/api/v1/auth/login", json={"email": "viewer@local", "password": password}
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


def _scan(name: str, version: str) -> dict:
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
                "publisher": "ACME",
                "source": "registry",
                "arch": "x64",
            }
        ],
    }


async def _post_scan(client, token: str, name: str, version: str):
    return await client.post(
        "/api/v1/agents/scans",
        headers={"Authorization": f"Bearer {token}"},
        json=_scan(name, version),
    )


def _hdr(access: str) -> dict:
    return {"Authorization": f"Bearer {access}"}


async def _create_license(client, access: str, **fields):
    body = {"software_name": "Google Chrome", "purchased_count": 20}
    body.update(fields)
    return await client.post("/api/v1/licenses", headers=_hdr(access), json=body)


# ── CRUD + auth ───────────────────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_admin_creates_license_viewer_cannot(client, session):
    admin = await _admin_access(client, session)
    viewer = await _viewer_access(client, session)

    r = await _create_license(client, admin, software_name="Microsoft Office", purchased_count=20)
    assert r.status_code == 201, r.text
    assert r.json()["installed_count"] == 0
    assert r.json()["status"] == "compliant"

    denied = await _create_license(client, viewer, software_name="X", purchased_count=1)
    assert denied.status_code == 403


@pytest.mark.asyncio
async def test_viewer_cannot_delete(client, session):
    admin = await _admin_access(client, session)
    viewer = await _viewer_access(client, session)
    lic = (await _create_license(client, admin)).json()
    r = await client.delete(f"/api/v1/licenses/{lic['uuid']}", headers=_hdr(viewer))
    assert r.status_code == 403


# ── Live install count ────────────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_installed_count_tracks_inventory(client, session):
    admin = await _admin_access(client, session)
    agent = await _enroll_agent(client, admin, "pc-1")
    await _post_scan(client, agent, "Google Chrome", "120.0")

    r = await _create_license(client, admin, software_name="Google Chrome", purchased_count=5)
    lic = r.json()
    detail = (await client.get(f"/api/v1/licenses/{lic['uuid']}", headers=_hdr(admin))).json()
    assert detail["installed_count"] == 1
    assert detail["status"] == "compliant"

    # Uninstall (scan without Chrome) → count drops to 0.
    await _post_scan(client, agent, "Other App", "1.0")
    detail = (await client.get(f"/api/v1/licenses/{lic['uuid']}", headers=_hdr(admin))).json()
    assert detail["installed_count"] == 0


@pytest.mark.asyncio
async def test_over_used_when_installs_exceed_purchased(client, session):
    admin = await _admin_access(client, session)
    a1 = await _enroll_agent(client, admin, "pc-1")
    a2 = await _enroll_agent(client, admin, "pc-2")
    await _post_scan(client, a1, "Google Chrome", "120.0")
    await _post_scan(client, a2, "Google Chrome", "120.0")

    r = await _create_license(client, admin, software_name="Google Chrome", purchased_count=1)
    lic = r.json()
    detail = (await client.get(f"/api/v1/licenses/{lic['uuid']}", headers=_hdr(admin))).json()
    assert detail["installed_count"] == 2
    assert detail["status"] == "over_used"


# ── Expiry status ─────────────────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_expiring_soon_and_expired_status(client, session):
    admin = await _admin_access(client, session)
    soon = (date.today() + timedelta(days=25)).isoformat()
    past = (date.today() - timedelta(days=1)).isoformat()

    a = (await _create_license(client, admin, software_name="A", expires_at=soon)).json()
    b = (await _create_license(client, admin, software_name="B", expires_at=past)).json()
    assert a["status"] == "expiring_soon"
    assert a["days_until_expiry"] == 25
    assert b["status"] == "expired"


# ── Compliance summary ────────────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_compliance_summary(client, session):
    admin = await _admin_access(client, session)
    await _create_license(client, admin, software_name="Good", purchased_count=20)
    await _create_license(
        client, admin, software_name="Soon", purchased_count=20,
        expires_at=(date.today() + timedelta(days=25)).isoformat(),
    )
    await _create_license(
        client, admin, software_name="Dead", purchased_count=20,
        expires_at=(date.today() - timedelta(days=1)).isoformat(),
    )

    s = (await client.get("/api/v1/licenses/compliance-summary", headers=_hdr(admin))).json()
    assert s["total"] == 3
    assert s["compliant"] == 1
    assert s["expiring_30"] == 1
    assert s["expired"] == 1
    assert s["compliance_rate"] == 33


# ── Drill-down ────────────────────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_installations_drilldown(client, session):
    admin = await _admin_access(client, session)
    a1 = await _enroll_agent(client, admin, "pc-1")
    a2 = await _enroll_agent(client, admin, "pc-2")
    await _post_scan(client, a1, "Google Chrome", "120.0")
    await _post_scan(client, a2, "Google Chrome", "121.0")
    lic = (await _create_license(client, admin, software_name="Google Chrome")).json()

    r = await client.get(f"/api/v1/licenses/{lic['uuid']}/installations", headers=_hdr(admin))
    assert r.status_code == 200, r.text
    body = r.json()
    assert body["total"] == 2
    assert {i["hostname"] for i in body["items"]} == {"pc-1", "pc-2"}


# ── Encryption at rest ────────────────────────────────────────────────────────
@pytest.mark.asyncio
async def test_license_key_encrypted_and_not_leaked_in_list(client, session):
    admin = await _admin_access(client, session)
    created = (await _create_license(client, admin, license_key="SUPER-SECRET-KEY")).json()
    assert created["has_license_key"] is True
    assert created["license_key"] == "SUPER-SECRET-KEY"  # detail response decrypts

    # List response must not carry the raw key.
    listed = (await client.get("/api/v1/licenses", headers=_hdr(admin))).json()
    assert "license_key" not in listed["items"][0]
    assert listed["items"][0]["has_license_key"] is True

    # Stored column is ciphertext, not plaintext.
    raw = (await session.execute(text("SELECT license_key FROM licenses"))).scalar_one()
    assert raw is not None
    assert b"SUPER-SECRET-KEY" not in bytes(raw)


# ── Expiry alert reconciliation ───────────────────────────────────────────────
@pytest.mark.asyncio
async def test_refresh_raises_expiry_alert_and_renewal_resolves(client, session):
    admin = await _admin_access(client, session)
    soon = (date.today() + timedelta(days=5)).isoformat()
    lic = (await _create_license(client, admin, software_name="Soon", expires_at=soon)).json()

    refreshed = await client.post("/api/v1/licenses/refresh-counts", headers=_hdr(admin))
    assert refreshed.status_code == 200, refreshed.text
    assert refreshed.json()["alerts_raised"] == 1

    alerts = (
        await client.get(
            "/api/v1/alerts", headers=_hdr(admin), params={"type": "license_expiring"}
        )
    ).json()["items"]
    assert len(alerts) == 1
    assert alerts[0]["severity"] == "high"  # 5 days out
    assert alerts[0]["license_name"] == "Soon"
    assert alerts[0]["machine_uuid"] is None

    # Renew far into the future → reconcile resolves the open alert.
    far = (date.today() + timedelta(days=400)).isoformat()
    await client.patch(
        f"/api/v1/licenses/{lic['uuid']}", headers=_hdr(admin), json={"expires_at": far}
    )
    await client.post("/api/v1/licenses/refresh-counts", headers=_hdr(admin))

    active = (
        await client.get(
            "/api/v1/alerts",
            headers=_hdr(admin),
            params={"type": "license_expiring", "status": "active"},
        )
    ).json()["items"]
    assert active == []
