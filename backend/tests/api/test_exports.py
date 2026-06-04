"""Module 8.3 — inline CSV export endpoints (auth, content, filters)."""

from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest
from app.core.security import hash_password
from app.models.cve import CveRecord
from app.models.user import User
from app.services import csv_export

_BOM = csv_export.BOM.encode("utf-8")


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


def _hdr(access: str) -> dict:
    return {"Authorization": f"Bearer {access}"}


async def _enroll_agent(client, access: str, hostname: str) -> str:
    create = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers=_hdr(access),
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


def _scan(*software: dict) -> dict:
    now = datetime.now(tz=UTC)
    return {
        "started_at": (now - timedelta(seconds=5)).isoformat(),
        "completed_at": now.isoformat(),
        "scan_type": "auto",
        "trigger": "schedule",
        "software": list(software),
    }


def _sw(name: str, version: str, *, publisher: str = "Google LLC") -> dict:
    return {
        "name": name,
        "version": version,
        "publisher": publisher,
        "source": "registry",
        "arch": "x64",
    }


async def _post_scan(client, token: str, payload: dict):
    return await client.post("/api/v1/agents/scans", headers=_hdr(token), json=payload)


_CHROME_AFFECTED = [{"product": "chrome", "vendor": "google", "version_end_excluding": "101.0"}]


@pytest.mark.asyncio
async def test_machines_export_requires_auth(client, session) -> None:
    r = await client.get("/api/v1/machines/export")
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_machines_export_csv(client, session) -> None:
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access, "host-1")
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))

    r = await client.get("/api/v1/machines/export", headers=_hdr(access))
    assert r.status_code == 200, r.text
    assert r.headers["content-type"].startswith("text/csv")
    assert r.content.startswith(_BOM)
    text = r.content.decode("utf-8-sig")
    assert text.splitlines()[0].startswith("uuid,hostname,os")
    assert "host-1" in text


@pytest.mark.asyncio
async def test_machines_export_respects_filter(client, session) -> None:
    access = await _admin_access(client, session)
    a1 = await _enroll_agent(client, access, "alpha")
    a2 = await _enroll_agent(client, access, "beta")
    await _post_scan(client, a1, _scan(_sw("Slack", "5.0")))
    await _post_scan(client, a2, _scan(_sw("Slack", "5.0")))

    text = (
        await client.get("/api/v1/machines/export", headers=_hdr(access), params={"q": "alpha"})
    ).content.decode("utf-8-sig")
    assert "alpha" in text
    assert "beta" not in text


@pytest.mark.asyncio
async def test_software_export_csv(client, session) -> None:
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access, "host-1")
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))

    text = (
        await client.get("/api/v1/software/export", headers=_hdr(access))
    ).content.decode("utf-8-sig")
    assert text.splitlines()[0].startswith("name,version")
    assert "Google Chrome" in text


@pytest.mark.asyncio
async def test_vulnerabilities_export_csv(client, session) -> None:
    access = await _admin_access(client, session)
    agent = await _enroll_agent(client, access, "host-1")
    now = datetime.now(tz=UTC)
    session.add(
        CveRecord(
            cve_id="CVE-2024-CRIT",
            source="nvd",
            published_at=now,
            modified_at=now,
            severity="critical",
            cvss_score=9.8,
            description="Test crit",
            cpe_matches=[],
            affected=_CHROME_AFFECTED,
            references=[],
        )
    )
    await session.commit()
    await _post_scan(client, agent, _scan(_sw("Google Chrome", "100.0")))

    text = (
        await client.get("/api/v1/vulnerabilities/export", headers=_hdr(access))
    ).content.decode("utf-8-sig")
    assert text.splitlines()[0].startswith("cve_id,severity")
    assert "CVE-2024-CRIT" in text


@pytest.mark.asyncio
async def test_licenses_export_csv(client, session) -> None:
    access = await _admin_access(client, session)
    await client.post(
        "/api/v1/licenses",
        headers=_hdr(access),
        json={"software_name": "Adobe Photoshop", "purchased_count": 10},
    )
    text = (
        await client.get("/api/v1/licenses/export", headers=_hdr(access))
    ).content.decode("utf-8-sig")
    assert text.splitlines()[0].startswith("software_name,publisher")
    assert "Adobe Photoshop" in text


@pytest.mark.asyncio
async def test_alerts_export_header_present_when_empty(client, session) -> None:
    access = await _admin_access(client, session)
    r = await client.get("/api/v1/alerts/export", headers=_hdr(access))
    assert r.status_code == 200
    text = r.content.decode("utf-8-sig")
    assert text.splitlines()[0].startswith("type,severity,status")
