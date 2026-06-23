"""Auto-update distribution: heartbeat offer + binary latest/download endpoints."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

import pytest
from app.core import config as config_module
from app.core.security import hash_password
from app.models.agent_config import AgentConfig
from app.models.machine import Machine
from app.models.user import User
from sqlalchemy import select

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


# ── forced update (admin "Update agent now") ──────────────────────────────────


async def _admin_access(client) -> str:
    return (
        await client.post(
            "/api/v1/auth/login",
            json={"email": "admin@local", "password": "TestPass!2026Aa"},
        )
    ).json()["access_token"]


async def _machine_uuid(session) -> str:
    machine = (await session.execute(select(Machine))).scalars().first()
    return str(machine.uuid)


@pytest.mark.asyncio
async def test_trigger_update_forces_offer_at_latest_then_clears(
    client, session, tmp_path, monkeypatch
):
    """Admin forces an update even when the agent is already on the latest version;
    the offer is marked forced and fires exactly once (flag cleared)."""
    _setup_binary_dir(tmp_path, monkeypatch)
    # Agent already runs NEW_VERSION, so the normal path would never offer.
    agent_token = await _enroll(client, session, agent_version=NEW_VERSION)
    access = await _admin_access(client)
    uuid = await _machine_uuid(session)

    trig = await client.post(
        f"/api/v1/machines/{uuid}/trigger-update",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert trig.status_code == 202
    assert trig.json()["force_update_requested"] is True

    hb = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {agent_token}"},
        json={"agent_version": NEW_VERSION, "uptime_seconds": 1},
    )
    offer = hb.json()["agent_update_available"]
    assert offer is not None
    assert offer["version"] == NEW_VERSION
    assert offer["forced"] is True

    # Second heartbeat: flag was cleared at offer time → no repeat (loop-safe).
    hb2 = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {agent_token}"},
        json={"agent_version": NEW_VERSION, "uptime_seconds": 2},
    )
    assert hb2.json()["agent_update_available"] is None


@pytest.mark.asyncio
async def test_forced_update_overrides_auto_update_disabled(
    client, session, tmp_path, monkeypatch
):
    _setup_binary_dir(tmp_path, monkeypatch)
    agent_token = await _enroll(client, session, agent_version="0.1.0")
    machine = (await session.execute(select(Machine))).scalars().first()
    # Auto-update off + force flag set: forced must still win.
    cfg = await session.get(AgentConfig, machine.id)
    if cfg is None:
        cfg = AgentConfig(machine_id=machine.id)
        session.add(cfg)
    cfg.auto_update_enabled = False
    cfg.force_update_requested = True
    await session.commit()

    hb = await client.post(
        "/api/v1/agents/heartbeat",
        headers={"Authorization": f"Bearer {agent_token}"},
        json={"agent_version": "0.1.0", "uptime_seconds": 1},
    )
    offer = hb.json()["agent_update_available"]
    assert offer is not None
    assert offer["forced"] is True


@pytest.mark.asyncio
async def test_trigger_update_409_when_no_binary(client, session, monkeypatch):
    monkeypatch.setattr(config_module.settings, "agent_binary_dir", "")
    await _enroll(client, session, agent_version="0.1.0")
    access = await _admin_access(client)
    uuid = await _machine_uuid(session)

    trig = await client.post(
        f"/api/v1/machines/{uuid}/trigger-update",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert trig.status_code == 409
