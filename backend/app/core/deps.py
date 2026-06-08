"""FastAPI dependencies — current user, current agent, role gates."""

from __future__ import annotations

import uuid as uuid_lib
from collections.abc import Awaitable, Callable
from typing import Annotated

import jwt
from fastapi import Depends, Header, HTTPException, Request, status
from redis.asyncio import Redis
from sqlalchemy import select

# Re-exported (explicit alias) so routers can import DBSession from this module.
from app.core.db import DBSession as DBSession
from app.core.redis import get_redis
from app.core.security import decode_token, verify_password
from app.models.machine import Machine
from app.models.user import User


def _bearer(authorization: str | None) -> str:
    if not authorization or not authorization.lower().startswith("bearer "):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Missing or malformed Authorization header",
            headers={"WWW-Authenticate": "Bearer"},
        )
    return authorization.split(" ", 1)[1].strip()


async def get_current_user(
    session: DBSession,
    authorization: Annotated[str | None, Header()] = None,
) -> User:
    token = _bearer(authorization)
    try:
        payload = decode_token(token, expected_type="access")
    except jwt.ExpiredSignatureError as exc:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "Token expired") from exc
    except jwt.PyJWTError as exc:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "Invalid token") from exc

    # JWT subjects are strings; the uuid column expects a UUID object.
    try:
        subject_uuid = uuid_lib.UUID(payload["sub"])
    except (ValueError, TypeError, KeyError) as exc:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "Invalid token subject") from exc

    user = (
        await session.execute(
            select(User).where(
                User.uuid == subject_uuid,
                User.deleted_at.is_(None),
                User.is_active.is_(True),
            )
        )
    ).scalar_one_or_none()
    if user is None:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "User not found or inactive")
    return user


CurrentUser = Annotated[User, Depends(get_current_user)]


# Role groups — keep these in sync with dashboard/lib/permissions.ts.
#   dev    — superuser, may access everything
#   admin  — Overview + Machines + Deploy only, full actions within that scope
#   viewer — broad read-only (everything except Deploy / Users / Audit Log)
ALL_ROLES = ("dev", "admin", "viewer")
DEV = ("dev",)
DEV_ADMIN = ("dev", "admin")
DEV_VIEWER = ("dev", "viewer")


def require_role(*allowed: str) -> Callable[..., Awaitable[User]]:
    async def dep(current: CurrentUser) -> User:
        if current.role not in allowed:
            raise HTTPException(status.HTTP_403_FORBIDDEN, "Insufficient permissions")
        return current

    return dep


async def get_current_agent(
    session: DBSession,
    authorization: Annotated[str | None, Header()] = None,
) -> Machine:
    """Match agent bearer token against bcrypt hash of stored token.

    Linear scan is acceptable at fleet scale we target; for >10k machines move
    to keyed lookup (e.g. token prefix index).
    """
    token = _bearer(authorization)

    candidates = (
        (await session.execute(select(Machine).where(Machine.deleted_at.is_(None)))).scalars().all()
    )

    machine = next(
        (m for m in candidates if verify_password(token, m.agent_token_hash)),
        None,
    )
    if machine is None:
        raise HTTPException(status.HTTP_401_UNAUTHORIZED, "Invalid agent token")
    return machine


CurrentAgent = Annotated[Machine, Depends(get_current_agent)]
RedisDep = Annotated[Redis, Depends(get_redis)]


def client_ip(request: Request) -> str:
    forwarded = request.headers.get("x-forwarded-for")
    if forwarded:
        return forwarded.split(",")[0].strip()
    return request.client.host if request.client else "0.0.0.0"  # noqa: S104  (fallback IP string, not a socket bind)
