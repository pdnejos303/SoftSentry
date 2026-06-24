"""Device inventory + Windows Update ingestion (Phase 6).

Covers the scan path writing hardware + WU posture onto the machine, the detail
endpoint returning the blobs/scalars, and back-compat for agents that send no
``device`` block.
"""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.core.security import hash_password
from app.models.machine import Machine
from app.models.user import User
from sqlalchemy import select


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
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": password},
    )
    return r.json()["access_token"]


async def _enroll(client, access: str) -> tuple[str, str]:
    """Returns (agent_token, machine_uuid)."""
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers={"Authorization": f"Bearer {access}"},
        json={"expires_in_seconds": 3600},
    )
    enroll = await client.post(
        "/api/v1/agents/enroll",
        json={
            "enrollment_token": create.json()["token"],
            "hostname": "host-1",
            "os": "windows",
            "os_version": "11.0.26200",
            "arch": "amd64",
            "agent_version": "0.1.0",
        },
    )
    body = enroll.json()
    return body["agent_token"], body["machine_uuid"]


def _device_block() -> dict:
    return {
        "system": {
            "manufacturer": "ASUSTeK COMPUTER INC.",
            "model": "ROG Strix G15",
            "serial_number": "SN12345",
            "system_type": "x64-based PC",
            "total_ram_mb": 32768,
        },
        "cpu": {"model": "AMD Ryzen 7 6800H", "cores": 8, "logical_count": 16, "clock_mhz": 3200},
        "memory": {
            "total_mb": 32768,
            "modules": [{"capacity_mb": 16384, "speed_mhz": 4800, "slot": "DIMM 0"}],
        },
        "disks": [{"model": "Samsung SSD 980", "size_gb": 953, "media_type": "ssd", "interface_type": "NVMe"}],
        "gpus": [{"name": "NVIDIA GeForce RTX 3060", "vram_mb": 6144}],
        "network": [{"name": "Wi-Fi", "mac": "AA:BB:CC:DD:EE:FF", "type": "Wireless"}],
        "firmware": {"bios_vendor": "American Megatrends", "bios_version": "G513QR.318"},
        "security": {"secure_boot": "enabled", "tpm_present": True, "tpm_enabled": True, "tpm_version": "2.0"},
        "monitors": [{"name": "Generic PnP Monitor", "width": 1920, "height": 1080}],
        "windows_update": {
            "status": "updates_pending",
            "pending_count": 2,
            "security_pending": 1,
            "reboot_pending": False,
            "last_installed_kb": "KB5034123",
            "last_checked_at": datetime.now(tz=UTC).isoformat(),
            "source": "online",
            "pending": [
                {"kb": "KB5040000", "title": "Cumulative Update", "security": True, "severity": "Critical"},
                {"kb": "KB5040001", "title": "Defender update", "security": False},
            ],
        },
    }


def _scan_payload(device: dict | None) -> dict:
    now = datetime.now(tz=UTC)
    payload: dict = {
        "started_at": (now - timedelta(seconds=5)).isoformat(),
        "completed_at": now.isoformat(),
        "scan_type": "auto",
        "trigger": "schedule",
        "software": [{"name": "Google Chrome", "version": "120.0.0", "source": "registry"}],
    }
    if device is not None:
        payload["device"] = device
    return payload


@pytest.mark.asyncio
async def test_scan_with_device_populates_machine(client, session):
    access = await _admin_access(client, session)
    token, uuid = await _enroll(client, access)

    r = await client.post(
        "/api/v1/agents/scans",
        headers={"Authorization": f"Bearer {token}"},
        json=_scan_payload(_device_block()),
    )
    assert r.status_code == 202

    machine = (
        await session.execute(select(Machine).where(Machine.hostname == "host-1"))
    ).scalar_one()
    # scalars denormalized from the blobs
    assert machine.model == "ROG Strix G15"
    assert machine.manufacturer == "ASUSTeK COMPUTER INC."
    assert machine.cpu_model == "AMD Ryzen 7 6800H"
    assert machine.ram_total_mb == 32768
    assert machine.wu_status == "updates_pending"
    assert machine.wu_pending_count == 2
    assert machine.wu_checked_at is not None
    # blobs split: hardware in device_info, WU in update_status
    assert machine.device_info["system"]["serial_number"] == "SN12345"
    assert machine.device_info["security"]["tpm_present"] is True
    assert "windows_update" not in machine.device_info
    assert machine.update_status["security_pending"] == 1
    assert len(machine.update_status["pending"]) == 2


@pytest.mark.asyncio
async def test_machine_detail_returns_device(client, session):
    access = await _admin_access(client, session)
    token, uuid = await _enroll(client, access)
    await client.post(
        "/api/v1/agents/scans",
        headers={"Authorization": f"Bearer {token}"},
        json=_scan_payload(_device_block()),
    )

    r = await client.get(
        f"/api/v1/machines/{uuid}", headers={"Authorization": f"Bearer {access}"}
    )
    assert r.status_code == 200
    body = r.json()
    assert body["model"] == "ROG Strix G15"
    assert body["cpu_model"] == "AMD Ryzen 7 6800H"
    assert body["ram_total_mb"] == 32768
    assert body["wu_status"] == "updates_pending"
    assert body["wu_pending_count"] == 2
    assert body["device_info"]["gpus"][0]["name"] == "NVIDIA GeForce RTX 3060"
    assert body["update_status"]["status"] == "updates_pending"


@pytest.mark.asyncio
async def test_scan_without_device_is_backcompat(client, session):
    access = await _admin_access(client, session)
    token, uuid = await _enroll(client, access)

    r = await client.post(
        "/api/v1/agents/scans",
        headers={"Authorization": f"Bearer {token}"},
        json=_scan_payload(None),
    )
    assert r.status_code == 202

    machine = (
        await session.execute(select(Machine).where(Machine.hostname == "host-1"))
    ).scalar_one()
    assert machine.device_info is None
    assert machine.update_status is None
    assert machine.model is None
    assert machine.wu_status is None
