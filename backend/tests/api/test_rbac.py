"""Role-based access matrix (dev / admin / viewer).

Locks the contract that mirrors dashboard/lib/permissions.ts:
  dev    — everything
  admin  — Overview + Machines + Deploy only
  viewer — broad read-only (everything except Deploy / Users / Audit Log)

Each case asserts on the *gate* (200/201 vs 403), not endpoint payloads, so it
stays decoupled from per-endpoint query requirements.
"""

from __future__ import annotations

import pytest
from app.core.security import hash_password
from app.models.user import User

_PW = "TestPass!2026Aa"

# GET endpoints grouped by who may *view* them.
_DEV_VIEWER_VIEWS = [
    "/api/v1/vulnerabilities",
    "/api/v1/licenses",
    "/api/v1/software",
    "/api/v1/alerts",
    "/api/v1/whitelist",
    "/api/v1/blacklist",
    "/api/v1/signatures",
]
_DEV_ONLY_VIEWS = ["/api/v1/users", "/api/v1/audit-logs"]
_ALL_ROLE_VIEWS = ["/api/v1/machines", "/api/v1/dashboard/vuln-summary"]


async def _token(client, session, role: str) -> str:
    email = f"{role}@local"
    session.add(
        User(
            email=email,
            password_hash=hash_password(_PW),
            full_name=role.title(),
            role=role,
            is_active=True,
        )
    )
    await session.commit()
    r = await client.post("/api/v1/auth/login", json={"email": email, "password": _PW})
    return r.json()["access_token"]


def _hdr(token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


@pytest.mark.asyncio
async def test_dev_sees_every_module(client, session):
    token = await _token(client, session, "dev")
    for path in _DEV_VIEWER_VIEWS + _DEV_ONLY_VIEWS + _ALL_ROLE_VIEWS:
        r = await client.get(path, headers=_hdr(token))
        assert r.status_code != 403, f"dev unexpectedly denied {path}"


@pytest.mark.asyncio
async def test_admin_sees_only_overview_machines_deploy(client, session):
    token = await _token(client, session, "admin")

    # Allowed: Overview + Machines.
    for path in _ALL_ROLE_VIEWS:
        r = await client.get(path, headers=_hdr(token))
        assert r.status_code != 403, f"admin should see {path}"

    # Allowed: Deploy (mint an enrollment token).
    mint = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers=_hdr(token),
        json={"expires_in_seconds": 3600},
    )
    assert mint.status_code == 201, mint.text

    # Denied: every analyst/dev-only module.
    for path in _DEV_VIEWER_VIEWS + _DEV_ONLY_VIEWS:
        r = await client.get(path, headers=_hdr(token))
        assert r.status_code == 403, f"admin should NOT see {path} (got {r.status_code})"


@pytest.mark.asyncio
async def test_viewer_is_read_only_no_deploy_users_audit(client, session):
    token = await _token(client, session, "viewer")

    # Allowed: read-only views including the analyst modules.
    for path in _DEV_VIEWER_VIEWS + _ALL_ROLE_VIEWS:
        r = await client.get(path, headers=_hdr(token))
        assert r.status_code != 403, f"viewer should read {path}"

    # Denied: Users + Audit Log.
    for path in _DEV_ONLY_VIEWS:
        r = await client.get(path, headers=_hdr(token))
        assert r.status_code == 403, f"viewer should NOT see {path}"

    # Denied: Deploy (minting enrollment tokens).
    mint = await client.post(
        "/api/v1/agents/enrollment-tokens",
        headers=_hdr(token),
        json={"expires_in_seconds": 3600},
    )
    assert mint.status_code == 403, mint.text
