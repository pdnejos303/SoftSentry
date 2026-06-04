"""User management + audit log endpoint tests (Module 9)."""

from __future__ import annotations

import pytest
from app.core.security import hash_password
from app.models.user import User

ADMIN_PW = "TestPass!2026Aa"


async def _make_user(
    session,
    *,
    email: str,
    role: str = "viewer",
    password: str = ADMIN_PW,
    is_active: bool = True,
) -> User:
    user = User(
        email=email,
        password_hash=hash_password(password),
        full_name=f"User {email}",
        role=role,
        is_active=is_active,
    )
    session.add(user)
    await session.commit()
    await session.refresh(user)
    return user


async def _token(client, email: str, password: str = ADMIN_PW) -> str:
    r = await client.post("/api/v1/auth/login", json={"email": email, "password": password})
    assert r.status_code == 200, r.text
    return r.json()["access_token"]


def _auth(token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


@pytest.mark.asyncio
async def test_admin_can_list_users(client, session):
    await _make_user(session, email="admin@local", role="admin")
    token = await _token(client, "admin@local")
    r = await client.get("/api/v1/users", headers=_auth(token))
    assert r.status_code == 200
    body = r.json()
    assert body["total"] >= 1
    assert any(u["email"] == "admin@local" for u in body["items"])


@pytest.mark.asyncio
async def test_viewer_cannot_create_user(client, session):
    await _make_user(session, email="viewer@local", role="viewer")
    token = await _token(client, "viewer@local")
    r = await client.post(
        "/api/v1/users",
        headers=_auth(token),
        json={"email": "new@example.com", "full_name": "New", "role": "viewer"},
    )
    assert r.status_code == 403


@pytest.mark.asyncio
async def test_create_user_generates_password_and_can_login(client, session):
    await _make_user(session, email="admin@local", role="admin")
    token = await _token(client, "admin@local")
    r = await client.post(
        "/api/v1/users",
        headers=_auth(token),
        json={"email": "new@example.com", "full_name": "New Person", "role": "viewer"},
    )
    assert r.status_code == 201, r.text
    body = r.json()
    pw = body["initial_password"]
    assert pw and len(pw) >= 12
    assert body["email"] == "new@example.com"

    # The new user can log in with the generated password.
    login = await client.post(
        "/api/v1/auth/login", json={"email": "new@example.com", "password": pw}
    )
    assert login.status_code == 200


@pytest.mark.asyncio
async def test_create_duplicate_email_conflicts(client, session):
    await _make_user(session, email="admin@local", role="admin")
    token = await _token(client, "admin@local")
    first = await client.post(
        "/api/v1/users",
        headers=_auth(token),
        json={"email": "dup@example.com", "full_name": "First", "role": "viewer"},
    )
    assert first.status_code == 201
    r = await client.post(
        "/api/v1/users",
        headers=_auth(token),
        json={"email": "dup@example.com", "full_name": "Dup", "role": "viewer"},
    )
    assert r.status_code == 409


@pytest.mark.asyncio
async def test_create_weak_password_rejected(client, session):
    await _make_user(session, email="admin@local", role="admin")
    token = await _token(client, "admin@local")
    r = await client.post(
        "/api/v1/users",
        headers=_auth(token),
        json={
            "email": "weak@example.com",
            "full_name": "Weak",
            "role": "viewer",
            "password": "short",
        },
    )
    assert r.status_code == 422


@pytest.mark.asyncio
async def test_update_user_role(client, session):
    await _make_user(session, email="admin@local", role="admin")
    target = await _make_user(session, email="bob@local", role="viewer")
    token = await _token(client, "admin@local")
    r = await client.patch(
        f"/api/v1/users/{target.uuid}",
        headers=_auth(token),
        json={"role": "admin"},
    )
    assert r.status_code == 200
    assert r.json()["role"] == "admin"


@pytest.mark.asyncio
async def test_cannot_demote_last_admin(client, session):
    admin = await _make_user(session, email="admin@local", role="admin")
    token = await _token(client, "admin@local")
    r = await client.patch(
        f"/api/v1/users/{admin.uuid}",
        headers=_auth(token),
        json={"role": "viewer"},
    )
    assert r.status_code == 409


@pytest.mark.asyncio
async def test_cannot_delete_self(client, session):
    admin = await _make_user(session, email="admin@local", role="admin")
    token = await _token(client, "admin@local")
    r = await client.delete(f"/api/v1/users/{admin.uuid}", headers=_auth(token))
    assert r.status_code == 409


@pytest.mark.asyncio
async def test_soft_delete_user_then_login_fails(client, session):
    await _make_user(session, email="admin@local", role="admin")
    target = await _make_user(session, email="leaver@example.com", role="viewer")
    token = await _token(client, "admin@local")
    r = await client.delete(f"/api/v1/users/{target.uuid}", headers=_auth(token))
    assert r.status_code == 204

    login = await client.post(
        "/api/v1/auth/login", json={"email": "leaver@example.com", "password": ADMIN_PW}
    )
    assert login.status_code == 401

    # Email becomes reusable after soft-delete.
    again = await client.post(
        "/api/v1/users",
        headers=_auth(token),
        json={"email": "leaver@example.com", "full_name": "Replacement", "role": "viewer"},
    )
    assert again.status_code == 201


@pytest.mark.asyncio
async def test_reset_password(client, session):
    await _make_user(session, email="admin@local", role="admin")
    target = await _make_user(session, email="carol@local", role="viewer")
    token = await _token(client, "admin@local")
    r = await client.post(
        f"/api/v1/users/{target.uuid}/reset-password", headers=_auth(token)
    )
    assert r.status_code == 200
    new_pw = r.json()["new_password"]

    login = await client.post(
        "/api/v1/auth/login", json={"email": "carol@local", "password": new_pw}
    )
    assert login.status_code == 200


@pytest.mark.asyncio
async def test_audit_log_records_login_and_user_create(client, session):
    await _make_user(session, email="admin@local", role="admin")
    token = await _token(client, "admin@local")  # records auth.login_success
    await client.post(
        "/api/v1/users",
        headers=_auth(token),
        json={"email": "audited@example.com", "full_name": "Aud", "role": "viewer"},
    )

    r = await client.get("/api/v1/audit-logs", headers=_auth(token))
    assert r.status_code == 200
    actions = {item["action"] for item in r.json()["items"]}
    assert "auth.login_success" in actions
    assert "user.create" in actions


@pytest.mark.asyncio
async def test_audit_log_records_login_failure(client, session):
    await _make_user(session, email="admin@local", role="admin")
    await client.post(
        "/api/v1/auth/login", json={"email": "admin@local", "password": "WrongPass!1A"}
    )
    token = await _token(client, "admin@local")
    r = await client.get(
        "/api/v1/audit-logs", headers=_auth(token), params={"action": "auth.login_failure"}
    )
    assert r.status_code == 200
    assert r.json()["total"] >= 1


@pytest.mark.asyncio
async def test_audit_log_admin_only(client, session):
    await _make_user(session, email="viewer@local", role="viewer")
    token = await _token(client, "viewer@local")
    r = await client.get("/api/v1/audit-logs", headers=_auth(token))
    assert r.status_code == 403
