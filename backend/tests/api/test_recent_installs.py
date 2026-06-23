"""API test: GET /api/v1/software/recent-installs.

End-to-end of the security-posture "what got newly installed + is it
trustworthy" feed: enroll an agent, scan a baseline, then scan again with a
new app and confirm it surfaces with a trust verdict.
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
        json={
            "started_at": now,
            "completed_at": now,
            "scan_type": "auto",
            "software": software,
        },
    )
    assert r.status_code == 202


@pytest.mark.asyncio
async def test_recent_installs_endpoint_shows_new_app(client, session):
    admin = await _admin_token(client, session)
    agent = await _enroll(client, admin, "PC-007")

    # baseline: no LINE
    await _scan(client, agent, [{"name": "Chrome", "version": "120", "source": "registry"}])
    # later: LINE appears, downloaded + validly signed, with accurate install time
    await _scan(
        client,
        agent,
        [
            {"name": "Chrome", "version": "120", "source": "registry"},
            {
                "name": "LINE",
                "version": "8.5",
                "publisher": "LINE Corporation",
                "source": "registry",
                "installed_at": "2026-06-22T09:30:00+00:00",
                "signature": {"status": "valid", "signer": "LINE Corporation"},
            },
        ],
    )

    r = await client.get("/api/v1/software/recent-installs", headers=_auth(admin))
    assert r.status_code == 200
    body = r.json()
    line = next(i for i in body["items"] if i["name"] == "LINE")
    assert line["event"] == "installed"
    assert line["trust"] == "trusted"
    assert line["machine_name"] == "PC-007"
    assert line["installed_at"].startswith("2026-06-22T09:30:00")


@pytest.mark.asyncio
async def test_recent_installs_explicit_date_range(client, session):
    admin = await _admin_token(client, session)
    agent = await _enroll(client, admin, "PC-007")
    await _scan(client, agent, [{"name": "Slack", "version": "5.0", "source": "registry"}])

    today = datetime.now(tz=UTC).date().isoformat()
    r = await client.get(
        f"/api/v1/software/recent-installs?from_date={today}&to_date={today}",
        headers=_auth(admin),
    )
    assert r.status_code == 200
    assert any(i["name"] == "Slack" for i in r.json()["items"])


@pytest.mark.asyncio
async def test_recent_installs_rejects_inverted_range(client, session):
    admin = await _admin_token(client, session)
    r = await client.get(
        "/api/v1/software/recent-installs?from_date=2026-06-22&to_date=2026-06-01",
        headers=_auth(admin),
    )
    assert r.status_code == 400


@pytest.mark.asyncio
async def test_recent_installs_requires_auth(client, session):
    r = await client.get("/api/v1/software/recent-installs")
    assert r.status_code == 401
