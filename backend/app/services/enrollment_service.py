"""Enrollment token + agent enrollment logic.

Tokens come in two flavours that share one table/codepath:

* one-time **enrollment token** (``max_uses == 1``) — admin mints one per machine
* reusable **deployment token** (``max_uses`` NULL/`>1`) — one link for the whole
  org, backing the one-click installer download
"""

from __future__ import annotations

import uuid as uuid_lib
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING

from sqlalchemy import select

from app.core.security import (
    generate_opaque_token,
    hash_password,
    verify_password,
)
from app.models.agent_config import AgentConfig
from app.models.enrollment_token import EnrollmentToken
from app.models.machine import Machine

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

# A deployment token that never expires still needs a concrete column value; use
# a far-future date so the same "expires_at > now" check works for both flavours.
_NO_EXPIRY = datetime(2999, 1, 1, tzinfo=UTC)


class EnrollmentError(Exception):
    pass


class InvalidEnrollmentToken(EnrollmentError):
    pass


class TokenNotFound(EnrollmentError):
    pass


async def create_enrollment_token(
    *,
    session: AsyncSession,
    label: str | None,
    expires_in_seconds: int,
    created_by_user_id: int | None,
) -> tuple[EnrollmentToken, str]:
    """Mint a classic one-time enrollment token (``max_uses == 1``)."""
    plaintext = generate_opaque_token(num_bytes=32)
    expires_at = datetime.now(tz=UTC) + timedelta(seconds=expires_in_seconds)

    token = EnrollmentToken(
        token_hash=hash_password(plaintext),
        label=label,
        expires_at=expires_at,
        max_uses=1,
        created_by_user_id=created_by_user_id,
    )
    session.add(token)
    await session.flush()
    await session.refresh(token)
    return token, plaintext


async def create_deployment_token(
    *,
    session: AsyncSession,
    label: str | None,
    expires_in_days: int | None,
    max_uses: int | None,
    created_by_user_id: int | None,
) -> tuple[EnrollmentToken, str]:
    """Mint a reusable deployment token.

    ``expires_in_days`` None → never expires; ``max_uses`` None → unlimited.
    """
    plaintext = generate_opaque_token(num_bytes=32)
    expires_at = (
        _NO_EXPIRY
        if expires_in_days is None
        else datetime.now(tz=UTC) + timedelta(days=expires_in_days)
    )

    token = EnrollmentToken(
        token_hash=hash_password(plaintext),
        label=label,
        expires_at=expires_at,
        max_uses=max_uses,
        created_by_user_id=created_by_user_id,
    )
    session.add(token)
    await session.flush()
    await session.refresh(token)
    return token, plaintext


def _aware(dt: datetime) -> datetime:
    """Coerce a possibly-naive datetime to UTC.

    SQLite (used in tests) drops tzinfo on ``DateTime(timezone=True)`` columns,
    so values read back are naive even though we store aware ones. Postgres
    returns aware values, where this is a no-op.
    """
    return dt if dt.tzinfo is not None else dt.replace(tzinfo=UTC)


def _is_valid(token: EnrollmentToken, now: datetime) -> bool:
    """Whether a token may still enrol a machine right now."""
    if token.revoked_at is not None:
        return False
    if _aware(token.expires_at) <= now:
        return False
    if token.max_uses is not None and token.use_count >= token.max_uses:
        return False
    return True


async def find_valid_token(
    *, session: AsyncSession, plaintext: str
) -> EnrollmentToken | None:
    """Return the live token matching ``plaintext`` (no side effects)."""
    now = datetime.now(tz=UTC)
    candidates = (
        (
            await session.execute(
                select(EnrollmentToken).where(
                    EnrollmentToken.revoked_at.is_(None),
                    EnrollmentToken.expires_at > now,
                )
            )
        )
        .scalars()
        .all()
    )
    for t in candidates:
        if _is_valid(t, now) and verify_password(plaintext, t.token_hash):
            return t
    return None


async def list_tokens(
    *, session: AsyncSession, include_revoked: bool = False
) -> list[EnrollmentToken]:
    """Deployment + enrollment tokens, newest first (admin listing)."""
    stmt = select(EnrollmentToken).order_by(EnrollmentToken.created_at.desc())
    if not include_revoked:
        stmt = stmt.where(EnrollmentToken.revoked_at.is_(None))
    return list((await session.execute(stmt)).scalars().all())


async def revoke_token(*, session: AsyncSession, token_uuid: uuid_lib.UUID) -> EnrollmentToken:
    """Revoke a token by UUID so it can no longer enrol. Idempotent."""
    token = (
        await session.execute(
            select(EnrollmentToken).where(EnrollmentToken.uuid == token_uuid)
        )
    ).scalar_one_or_none()
    if token is None:
        raise TokenNotFound()
    if token.revoked_at is None:
        token.revoked_at = datetime.now(tz=UTC)
        await session.flush()
    return token


async def enroll_agent(
    *,
    session: AsyncSession,
    enrollment_token_plaintext: str,
    hostname: str,
    os: str,
    os_version: str,
    arch: str,
    agent_version: str,
) -> tuple[Machine, str]:
    now = datetime.now(tz=UTC)

    token = await find_valid_token(session=session, plaintext=enrollment_token_plaintext)
    if token is None:
        raise InvalidEnrollmentToken()

    agent_token_plain = generate_opaque_token(num_bytes=32)
    machine = Machine(
        hostname=hostname,
        os=os,
        os_version=os_version,
        arch=arch,
        agent_version=agent_version,
        agent_token_hash=hash_password(agent_token_plain),
        enrolled_at=now,
        last_seen_at=now,
        status="online",
        tags=[],
    )
    session.add(machine)
    await session.flush()
    await session.refresh(machine)

    # Give every machine a config row up front so heartbeat/config reads and the
    # admin trigger-scan toggle (manual_scan_requested) have a row to act on.
    session.add(AgentConfig(machine_id=machine.id))

    token.use_count += 1
    token.used_at = now
    token.used_by_machine_id = machine.id
    await session.flush()

    return machine, agent_token_plain
