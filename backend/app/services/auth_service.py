"""Auth business logic — login, lockout, refresh token rotation."""

from __future__ import annotations

import hashlib
import uuid as uuid_lib
from datetime import UTC, datetime
from typing import TYPE_CHECKING, Final

from sqlalchemy import select

from app.core.config import settings
from app.core.security import (
    create_token,
    decode_token,
    hash_password,
    verify_password,
)
from app.models.user import User

if TYPE_CHECKING:
    from redis.asyncio import Redis
    from sqlalchemy.ext.asyncio import AsyncSession

LOCKOUT_PREFIX: Final = "auth:lockout:"
ATTEMPT_PREFIX: Final = "auth:attempt:"
REFRESH_PREFIX: Final = "auth:refresh:"
MAX_FAILED_ATTEMPTS: Final = 5
ATTEMPT_WINDOW_SECONDS: Final = 300
LOCKOUT_SECONDS: Final = 900


class AuthError(Exception):
    pass


class InvalidCredentials(AuthError):
    pass


class AccountLocked(AuthError):
    def __init__(self, retry_after_seconds: int):
        super().__init__("Account locked")
        self.retry_after_seconds = retry_after_seconds


class AccountInactive(AuthError):
    pass


def _attempt_key(email: str, ip: str) -> str:
    # SHA-256 normalizes the composite into a fixed-length key, preventing
    # unusually long emails or special characters from injecting into Redis keys.
    safe = hashlib.sha256(f"{email.lower()}|{ip}".encode()).hexdigest()[:32]
    return f"{ATTEMPT_PREFIX}{safe}"


def _lockout_key(email: str, ip: str) -> str:
    safe = hashlib.sha256(f"{email.lower()}|{ip}".encode()).hexdigest()[:32]
    return f"{LOCKOUT_PREFIX}{safe}"


async def authenticate(
    *,
    session: AsyncSession,
    redis: Redis,
    email: str,
    password: str,
    ip: str,
) -> User:
    lockout_key = _lockout_key(email, ip)
    ttl = await redis.ttl(lockout_key)
    if ttl > 0:
        raise AccountLocked(retry_after_seconds=ttl)

    stmt = select(User).where(User.email == email.lower(), User.deleted_at.is_(None))
    user = (await session.execute(stmt)).scalar_one_or_none()

    if user is None or not verify_password(password, user.password_hash):
        await _record_failure(redis, email, ip)
        raise InvalidCredentials()

    if not user.is_active:
        raise AccountInactive()

    await redis.delete(_attempt_key(email, ip))
    user.last_login_at = datetime.now(tz=UTC)
    await session.flush()
    return user


async def _record_failure(redis: Redis, email: str, ip: str) -> None:
    attempt_key = _attempt_key(email, ip)
    count = await redis.incr(attempt_key)
    # Set the TTL only on the first increment. Setting it on every call would
    # allow a persistent attacker to slide the window forward indefinitely.
    if count == 1:
        await redis.expire(attempt_key, ATTEMPT_WINDOW_SECONDS)
    if count >= MAX_FAILED_ATTEMPTS:
        await redis.set(_lockout_key(email, ip), "1", ex=LOCKOUT_SECONDS)
        await redis.delete(attempt_key)


async def issue_token_pair(
    *,
    redis: Redis,
    user: User,
    remember_me: bool = False,
) -> tuple[str, str, int]:
    access, _ = create_token(
        subject=str(user.uuid),
        token_type="access",
        extra={"role": user.role},
    )
    refresh, _refresh_exp = create_token(
        subject=str(user.uuid),
        token_type="refresh",
    )
    payload = decode_token(refresh, expected_type="refresh")
    jti = payload["jti"]
    # edit jwt_reresh_ttl_seconds, jwt_refresh_ttl_no_remember_seconds on backend/app/core/config.py 
    # can set in .env to overwrite default value in config.py
    ttl = (
        settings.jwt_refresh_ttl_seconds
        if remember_me
        else settings.jwt_refresh_ttl_no_remember_seconds
    )
    await redis.set(f"{REFRESH_PREFIX}{jti}", f"{user.uuid}|{int(remember_me)}", ex=ttl)
    return access, refresh, settings.jwt_access_ttl_seconds


async def rotate_refresh(
    *,
    session: AsyncSession,
    redis: Redis,
    refresh_token: str,
) -> tuple[str, str, int, bool]:
    payload = decode_token(refresh_token, expected_type="refresh")
    jti = payload["jti"]
    key = f"{REFRESH_PREFIX}{jti}"
    stored = await redis.get(key)
    if stored is None:
        raise InvalidCredentials()

    # Delete the old token before the DB lookup — one-time-use enforcement.
    # If the DB query fails after this, the token is gone and the user must
    # re-authenticate (security-over-UX trade-off).
    await redis.delete(key)

    # value = "<uuid>|<0|1>"; รองรับ token เก่าที่ยังเป็น "<uuid>" ล้วน (remember=False)
    raw_uuid, _, remember_flag = stored.partition("|")
    remember_me = remember_flag == "1"

    # Stored in Redis as a string; the uuid column expects a UUID object.
    try:
        user_uuid_obj = uuid_lib.UUID(raw_uuid)
    except (ValueError, TypeError) as exc:
        raise InvalidCredentials() from exc

    stmt = select(User).where(User.uuid == user_uuid_obj, User.deleted_at.is_(None))
    user = (await session.execute(stmt)).scalar_one_or_none()
    if user is None or not user.is_active:
        raise InvalidCredentials()
    
    access, new_refresh, ttl = await issue_token_pair(
        redis=redis, user=user, remember_me=remember_me
    )
    return access, new_refresh, ttl, remember_me



async def revoke_refresh(*, redis: Redis, refresh_token: str) -> None:
    try:
        payload = decode_token(refresh_token, expected_type="refresh")
    except Exception:
        return
    await redis.delete(f"{REFRESH_PREFIX}{payload['jti']}")


async def change_password(
    *,
    session: AsyncSession,
    user: User,
    current_password: str,
    new_password: str,
) -> None:
    if not verify_password(current_password, user.password_hash):
        raise InvalidCredentials()
    user.password_hash = hash_password(new_password)
    await session.flush()
