"""Auth endpoint tests — login, lockout, refresh rotation, me."""

from __future__ import annotations

import pytest
from app.core.security import hash_password
from app.models.user import User


async def _make_user(
    session,
    *,
    email: str = "admin@local",
    password: str = "TestPass!2026Aa",
    role: str = "admin",
    is_active: bool = True,
) -> User:
    user = User(
        email=email,
        password_hash=hash_password(password),
        full_name="Test Admin",
        role=role,
        is_active=is_active,
    )
    session.add(user)
    await session.commit()
    await session.refresh(user)
    return user


@pytest.mark.asyncio
async def test_login_success(client, session):
    await _make_user(session)

    r = await client.post(
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": "TestPass!2026Aa"},
    )
    assert r.status_code == 200
    body = r.json()
    assert body["token_type"] == "bearer"
    assert body["access_token"]
    assert body["expires_in"] > 0
    assert "softsentry_refresh" in r.cookies


@pytest.mark.asyncio
async def test_login_wrong_password(client, session):
    await _make_user(session)
    r = await client.post(
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": "WrongPass!"},
    )
    assert r.status_code == 401
    assert "Incorrect" in r.json()["detail"]


@pytest.mark.asyncio
async def test_login_unknown_email_same_message(client, session):
    r = await client.post(
        "/api/v1/auth/login",
        json={"email": "ghost@local", "password": "Whatever!1A"},
    )
    assert r.status_code == 401
    assert "Incorrect" in r.json()["detail"]


@pytest.mark.asyncio
async def test_login_inactive_user(client, session):
    await _make_user(session, is_active=False)
    r = await client.post(
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": "TestPass!2026Aa"},
    )
    assert r.status_code == 403


@pytest.mark.asyncio
async def test_lockout_after_five_failed_attempts(client, session):
    await _make_user(session)
    for _ in range(5):
        await client.post(
            "/api/v1/auth/login",
            json={"email": "admin@local", "password": "Wrong!1A"},
        )
    r = await client.post(
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": "TestPass!2026Aa"},
    )
    assert r.status_code == 429
    assert r.headers.get("retry-after")


@pytest.mark.asyncio
async def test_me_requires_auth(client, session):
    r = await client.get("/api/v1/auth/me")
    assert r.status_code == 401


@pytest.mark.asyncio
async def test_me_with_valid_token(client, session):
    await _make_user(session)
    login = await client.post(
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": "TestPass!2026Aa"},
    )
    access = login.json()["access_token"]

    r = await client.get(
        "/api/v1/auth/me",
        headers={"Authorization": f"Bearer {access}"},
    )
    assert r.status_code == 200
    body = r.json()
    assert body["email"] == "admin@local"
    assert body["role"] == "admin"


@pytest.mark.asyncio
async def test_refresh_rotates_token(client, session):
    await _make_user(session)
    login = await client.post(
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": "TestPass!2026Aa"},
    )
    first_refresh = login.cookies["softsentry_refresh"]

    r = await client.post(
        "/api/v1/auth/refresh",
        cookies={"softsentry_refresh": first_refresh},
    )
    assert r.status_code == 200
    assert r.json()["access_token"]
    new_refresh = r.cookies.get("softsentry_refresh")
    assert new_refresh and new_refresh != first_refresh

    # Old refresh should no longer work
    again = await client.post(
        "/api/v1/auth/refresh",
        cookies={"softsentry_refresh": first_refresh},
    )
    assert again.status_code == 401


@pytest.mark.asyncio
async def test_logout_clears_cookie_and_revokes(client, session):
    await _make_user(session)
    login = await client.post(
        "/api/v1/auth/login",
        json={"email": "admin@local", "password": "TestPass!2026Aa"},
    )
    refresh = login.cookies["softsentry_refresh"]

    r = await client.post(
        "/api/v1/auth/logout",
        cookies={"softsentry_refresh": refresh},
    )
    assert r.status_code == 204

    again = await client.post(
        "/api/v1/auth/refresh",
        cookies={"softsentry_refresh": refresh},
    )
    assert again.status_code == 401
