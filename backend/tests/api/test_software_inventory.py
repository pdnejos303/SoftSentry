"""API test: cross-machine inventory endpoints (security-posture cuts).

Covers GET /api/v1/software (sort + machine drill-down shape), /software/stats,
and /software/top?risk=true.
"""

from __future__ import annotations

from datetime import UTC, datetime

import pytest
from app.core.security import hash_password
from app.models.user import User


async def _admin_token(client, session) -> str:
    password = "TestPass!2026Aa"
    session.add(
        User(
            email="admin@local",
            password_hash=hash_password(password),
            full_name="Dev",
            role="dev",
            is_active=True,
        )
    )
    await session.commit()
    r = await client.post(
        "/api/v1/auth/login", json={"email": "admin@local", "password": password}
    )
    return r.json()["access_token"]


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
    now = datetime.now(tz=UTC).isoformat()
    r = await client.post(
        "/api/v1/agents/scans",
        headers=_auth(agent_token),
        json={"started_at": now, "completed_at": now, "scan_type": "auto", "software": software},
    )
    assert r.status_code == 202


async def _seed(client, admin) -> None:
    a = await _enroll(client, admin, "PC-A")
    b = await _enroll(client, admin, "PC-B")
    await _scan(
        client,
        a,
        [
            {"name": "Chrome", "version": "120", "source": "registry",
             "signature": {"status": "valid"}},
            {"name": "7-Zip", "version": "23", "source": "registry",
             "signature": {"status": "unsigned"}},
        ],
    )
    await _scan(
        client,
        b,
        [
            {"name": "Chrome", "version": "120", "source": "registry",
             "signature": {"status": "valid"}},
            {"name": "Updater", "version": "1.0", "source": "registry",
             "signature": {"status": "invalid"}},
        ],
    )


@pytest.mark.asyncio
async def test_stats_endpoint(client, session):
    admin = await _admin_token(client, session)
    await _seed(client, admin)
    r = await client.get("/api/v1/software/stats", headers=_auth(admin))
    assert r.status_code == 200
    body = r.json()
    assert body == {
        "unique_apps": 3,
        "total_installs": 4,
        "valid": 2,
        "unsigned": 1,
        "invalid": 1,
    }


@pytest.mark.asyncio
async def test_list_includes_machine_refs(client, session):
    admin = await _admin_token(client, session)
    await _seed(client, admin)
    r = await client.get("/api/v1/software?q=chrome", headers=_auth(admin))
    assert r.status_code == 200
    chrome = next(i for i in r.json()["items"] if i["name"] == "Chrome")
    assert chrome["installed_count"] == 2
    labels = {ref["name"] for ref in chrome["machines"]}
    assert labels == {"PC-A", "PC-B"}
    assert all("uuid" in ref for ref in chrome["machines"])


@pytest.mark.asyncio
async def test_list_sort_by_name(client, session):
    admin = await _admin_token(client, session)
    await _seed(client, admin)
    r = await client.get("/api/v1/software?sort=-name", headers=_auth(admin))
    names = [i["name"] for i in r.json()["items"]]
    assert names == sorted(names, key=str.lower, reverse=True)


@pytest.mark.asyncio
async def test_top_risk_excludes_signed(client, session):
    admin = await _admin_token(client, session)
    await _seed(client, admin)
    r = await client.get("/api/v1/software/top?risk=true", headers=_auth(admin))
    names = {i["name"] for i in r.json()}
    assert "Chrome" not in names
    assert {"7-Zip", "Updater"} <= names


@pytest.mark.asyncio
async def test_software_requires_auth(client, session):
    r = await client.get("/api/v1/software/stats")
    assert r.status_code == 401
