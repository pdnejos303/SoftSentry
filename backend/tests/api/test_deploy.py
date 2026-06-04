"""One-click deployment: reusable token + installer download."""

from __future__ import annotations

import json

import pytest
from app.core.security import hash_password
from app.models.user import User
from app.services import installer_service


async def _admin_token(client, session) -> str:
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


def _enroll_payload(hostname: str, token: str) -> dict:
    return {
        "enrollment_token": token,
        "hostname": hostname,
        "os": "windows",
        "os_version": "11.0.26200",
        "arch": "amd64",
        "agent_version": "0.1.0",
    }


@pytest.mark.asyncio
async def test_create_requires_admin(client, session):
    r = await client.post("/api/v1/deploy/tokens", json={})
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_deployment_token_enrolls_many_machines(client, session):
    access = await _admin_token(client, session)
    create = await client.post(
        "/api/v1/deploy/tokens",
        headers={"Authorization": f"Bearer {access}"},
        json={"label": "org-wide", "max_uses": None, "expires_in_days": None},
    )
    assert create.status_code == 201
    token = create.json()["token"]

    # Same token enrolls several machines (unlike one-time enrollment tokens).
    for host in ("pc-1", "pc-2", "pc-3"):
        r = await client.post("/api/v1/agents/enroll", json=_enroll_payload(host, token))
        assert r.status_code == 201, host

    listed = await client.get(
        "/api/v1/deploy/tokens", headers={"Authorization": f"Bearer {access}"}
    )
    assert listed.status_code == 200
    entry = next(t for t in listed.json() if t["label"] == "org-wide")
    assert entry["use_count"] == 3
    assert entry["max_uses"] is None


@pytest.mark.asyncio
async def test_max_uses_enforced(client, session):
    access = await _admin_token(client, session)
    token = (
        await client.post(
            "/api/v1/deploy/tokens",
            headers={"Authorization": f"Bearer {access}"},
            json={"max_uses": 2},
        )
    ).json()["token"]

    assert (await client.post("/api/v1/agents/enroll", json=_enroll_payload("a", token))).status_code == 201
    assert (await client.post("/api/v1/agents/enroll", json=_enroll_payload("b", token))).status_code == 201
    # Third enrolment exceeds max_uses.
    assert (await client.post("/api/v1/agents/enroll", json=_enroll_payload("c", token))).status_code == 401


@pytest.mark.asyncio
async def test_revoke_blocks_enrollment(client, session):
    access = await _admin_token(client, session)
    created = (
        await client.post(
            "/api/v1/deploy/tokens",
            headers={"Authorization": f"Bearer {access}"},
            json={},
        )
    ).json()
    token, token_uuid = created["token"], created["uuid"]

    rev = await client.post(
        f"/api/v1/deploy/tokens/{token_uuid}/revoke",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert rev.status_code == 200
    assert rev.json()["revoked_at"] is not None

    after = await client.post("/api/v1/agents/enroll", json=_enroll_payload("x", token))
    assert after.status_code == 401


@pytest.mark.asyncio
async def test_installer_download_without_binary_is_503(client, session):
    access = await _admin_token(client, session)
    token = (
        await client.post(
            "/api/v1/deploy/tokens",
            headers={"Authorization": f"Bearer {access}"},
            json={},
        )
    ).json()["token"]
    # agent_binary_dir is empty by default in tests.
    r = await client.get(f"/api/v1/deploy/installer?token={token}")
    assert r.status_code == 503


@pytest.mark.asyncio
async def test_installer_bad_token_is_403(client, session):
    r = await client.get("/api/v1/deploy/installer?token=not-real")
    assert r.status_code == 403


@pytest.mark.asyncio
async def test_installer_streams_binary_with_trailer(client, session, tmp_path, monkeypatch):
    from app.core.config import settings

    # Stage a fake agent binary + manifest in a temp agent_binary_dir.
    fake = tmp_path / "softsentry-agent-1.0.0-windows-amd64.exe"
    fake.write_bytes(b"MZ" + b"agent-binary" * 50)
    (tmp_path / "manifest.json").write_text(
        json.dumps(
            {
                "binaries": [
                    {
                        "version": "1.0.0",
                        "os": "windows",
                        "arch": "amd64",
                        "filename": fake.name,
                        "sha256": "0" * 64,
                    }
                ]
            }
        )
    )
    monkeypatch.setattr(settings, "agent_binary_dir", str(tmp_path))

    access = await _admin_token(client, session)
    token = (
        await client.post(
            "/api/v1/deploy/tokens",
            headers={"Authorization": f"Bearer {access}"},
            json={},
        )
    ).json()["token"]

    r = await client.get(
        f"/api/v1/deploy/installer?token={token}&server=http://192.168.1.50:8001"
    )
    assert r.status_code == 200
    assert "SoftSentry-Setup.exe" in r.headers["content-disposition"]
    parsed = installer_service.parse_trailer(r.content)
    assert parsed == ("http://192.168.1.50:8001", token)
    assert r.content.startswith(fake.read_bytes())
