"""Password hashing + JWT encode/decode + agent token utilities."""

from __future__ import annotations

import base64
import secrets
from datetime import UTC, datetime, timedelta
from typing import Any, Literal

import bcrypt
import jwt

from app.core.config import settings

# ── Password (bcrypt) ──────────────────────────────────────────

BCRYPT_COST = 12


def _peppered(password: str) -> bytes:
    return (password + settings.bcrypt_pepper).encode("utf-8")


def hash_password(password: str) -> str:
    return bcrypt.hashpw(_peppered(password), bcrypt.gensalt(rounds=BCRYPT_COST)).decode()


def verify_password(password: str, password_hash: str) -> bool:
    try:
        return bcrypt.checkpw(_peppered(password), password_hash.encode())
    except (ValueError, TypeError):
        return False


# ── JWT ─────────────────────────────────────────────────────────

TokenType = Literal["access", "refresh"]


def _now() -> datetime:
    return datetime.now(tz=UTC)


def create_token(
    *,
    subject: str,
    token_type: TokenType,
    extra: dict[str, Any] | None = None,
) -> tuple[str, datetime]:
    ttl = (
        settings.jwt_access_ttl_seconds
        if token_type == "access"
        else settings.jwt_refresh_ttl_seconds
    )
    # Truncate to whole seconds so the returned datetimes match the integer
    # NumericDate values encoded into the JWT (iat/exp).
    issued_at = _now().replace(microsecond=0)
    expires_at = issued_at + timedelta(seconds=ttl)
    payload: dict[str, Any] = {
        "sub": subject,
        "iat": int(issued_at.timestamp()),
        "exp": int(expires_at.timestamp()),
        "type": token_type,
        "jti": secrets.token_urlsafe(16),
    }
    if extra:
        payload.update(extra)
    token = jwt.encode(payload, settings.jwt_secret, algorithm=settings.jwt_algorithm)
    return token, expires_at


def decode_token(token: str, *, expected_type: TokenType | None = None) -> dict[str, Any]:
    payload = jwt.decode(
        token,
        settings.jwt_secret,
        algorithms=[settings.jwt_algorithm],
        options={"require": ["exp", "iat", "sub", "type"]},
    )
    if expected_type is not None and payload.get("type") != expected_type:
        raise jwt.InvalidTokenError(f"Wrong token type: expected {expected_type}")
    return payload


# ── Random tokens (agent + enrollment) ─────────────────────────


def generate_opaque_token(num_bytes: int = 32) -> str:
    """Return base64url-encoded random token (no padding)."""
    raw = secrets.token_bytes(num_bytes)
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode("ascii")
