"""Settings behaviour tests — cookie security resolution."""

from __future__ import annotations

from app.core.config import Settings

_BASE = {"database_url": "sqlite+aiosqlite:///:memory:", "jwt_secret": "x"}


def test_refresh_cookie_secure_defaults_to_production():
    # No explicit override → follow the environment.
    assert Settings(**_BASE, env="production").refresh_cookie_secure is True
    assert Settings(**_BASE, env="staging").refresh_cookie_secure is False
    assert Settings(**_BASE, env="development").refresh_cookie_secure is False


def test_refresh_cookie_secure_explicit_override_wins():
    # HTTP-only intranet prod: keep ENV=production but serve over plain HTTP,
    # where browsers drop Secure cookies. The override must let us disable it.
    assert (
        Settings(**_BASE, env="production", cookie_secure=False).refresh_cookie_secure
        is False
    )
    # And the opposite override is honoured too.
    assert (
        Settings(**_BASE, env="development", cookie_secure=True).refresh_cookie_secure
        is True
    )
