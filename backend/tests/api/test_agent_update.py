"""Auto-update distribution: heartbeat offer + binary latest/download endpoints."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

import pytest
from app.core import config as config_module
from app.core.security import hash_password
from app.models.user import User

NEW_VERSION = "1.0.0"
BINARY_BYTES = b"PRETEND-AGENT-BINARY"


async def _enroll(client, session, *, agent_version: str) -> str:
    """Create admin, mint an enrollment token, enroll an agent, return its token."""
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
    access = (
        await client.post(
            "/api/v1/auth/login",
            json={"email": "admin@local", "password": "TestPass!2026Aa"},
        )
    ).json()["access_token"]
    token = (
        await client.post(
            "/api/v1/agents/enrollment-tokens",
            headers={"Authorization": f"Bearer {access}"},
            json={"expires_in_seconds": 3600},
        )
    ).json()["token"]
    enroll = await client.post(
        "/api/v1/agents/enroll",
        json={
            "enrollment_token": token,
            "hostname": "h",
            "os": "windows",
            "os_version": "10.0.26200",
            "arch": "amd64",
            "agent_version": agent_version,
        },
    )
    return enroll.json()["agent_token"]


def _setup_binary_dir(tmp_path: Path, monkeypatch) -> str:
    binary = tmp_path / "softsentry-agent-1.0.0-windows-amd64.exe"
    binary.write_bytes(BINARY_BYTES)
    (tmp_path / "manifest.json").write_text(
        json.dumps(
            {
                "binaries": [
                    {
                        "version": NEW_VERSION,
                        "os": "windows",
                        "arch": "amd64",
                        "filename": binary.name,
                        "sha256": hashlib.sha256(BINARY_BYTES).hexdigest(),
                    }
                ]
            }
        )
    )
    monkeypatch.setattr(config_module.settings, "agent_binary_dir", str(tmp_path))
    return str(tmp_path)


@pytest.mark.asyncio
async def test_heartbeat_offers_update_when_newer_binary_exists(
    client, session, tmp_path, monkeypatch
):
    _setup_binary_dir(tmp_path, monkeypatch)
    agent_token = await _enroll(client, session, agent_version="0.1.0")

    hb = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {agent_token}"},
        json={"agent_version": "0.1.0", "uptime_seconds": 1},
    )
    assert hb.status_code == 200
    offer = hb.json()["agent_update_available"]
    assert offer is not None
    assert offer["version"] == NEW_VERSION
    assert offer["sha256"] == hashlib.sha256(BINARY_BYTES).hexdigest()
    assert offer["download_url"] == "/api/v1/agents/binary/download/1.0.0/windows/amd64"


@pytest.mark.asyncio
async def test_heartbeat_no_offer_when_already_latest(client, session, tmp_path, monkeypatch):
    _setup_binary_dir(tmp_path, monkeypatch)
    agent_token = await _enroll(client, session, agent_version=NEW_VERSION)

    hb = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {agent_token}"},
        json={"agent_version": NEW_VERSION, "uptime_seconds": 1},
    )
    assert hb.status_code == 200
    assert hb.json()["agent_update_available"] is None


@pytest.mark.asyncio
async def test_binary_download_serves_bytes(client, session, tmp_path, monkeypatch):
    _setup_binary_dir(tmp_path, monkeypatch)
    agent_token = await _enroll(client, session, agent_version="0.1.0")

    resp = await client.get(
        "/api/v1/agents/binary/download/1.0.0/windows/amd64",
        headers={"Authorization": f"Bearer {agent_token}"},
    )
    assert resp.status_code == 200
    assert resp.content == BINARY_BYTES


@pytest.mark.asyncio
async def test_binary_download_unknown_version_404(client, session, tmp_path, monkeypatch):
    _setup_binary_dir(tmp_path, monkeypatch)
    agent_token = await _enroll(client, session, agent_version="0.1.0")

    resp = await client.get(
        "/api/v1/agents/binary/download/9.9.9/windows/amd64",
        headers={"Authorization": f"Bearer {agent_token}"},
    )
    assert resp.status_code == 404


@pytest.mark.asyncio
async def test_no_offer_when_binary_dir_unconfigured(client, session, monkeypatch):
    monkeypatch.setattr(config_module.settings, "agent_binary_dir", "")
    agent_token = await _enroll(client, session, agent_version="0.1.0")

    hb = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {agent_token}"},
        json={"agent_version": "0.1.0", "uptime_seconds": 1},
    )
    assert hb.status_code == 200
    assert hb.json()["agent_update_available"] is None
