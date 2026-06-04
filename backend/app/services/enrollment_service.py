"""Enrollment token + agent enrollment logic."""

from __future__ import annotations

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


class EnrollmentError(Exception):
    pass


class InvalidEnrollmentToken(EnrollmentError):
    pass


async def create_enrollment_token(
    *,
    session: AsyncSession,
    label: str | None,
    expires_in_seconds: int,
    created_by_user_id: int | None,
) -> tuple[EnrollmentToken, str]:
    plaintext = generate_opaque_token(num_bytes=32)
    expires_at = datetime.now(tz=UTC) + timedelta(seconds=expires_in_seconds)

    token = EnrollmentToken(
        token_hash=hash_password(plaintext),
        label=label,
        expires_at=expires_at,
        created_by_user_id=created_by_user_id,
    )
    session.add(token)
    await session.flush()
    await session.refresh(token)
    return token, plaintext


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

    candidates = (
        (
            await session.execute(
                select(EnrollmentToken).where(
                    EnrollmentToken.used_at.is_(None),
                    EnrollmentToken.expires_at > now,
                )
            )
        )
        .scalars()
        .all()
    )

    token: EnrollmentToken | None = next(
        (t for t in candidates if verify_password(enrollment_token_plaintext, t.token_hash)),
        None,
    )
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

    token.used_at = now
    token.used_by_machine_id = machine.id
    await session.flush()

    return machine, agent_token_plain
