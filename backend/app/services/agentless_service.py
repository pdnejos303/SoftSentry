from __future__ import annotations

from datetime import UTC, datetime
from typing import TYPE_CHECKING

from sqlalchemy import select

from app.core.security import generate_opaque_token, hash_password
from app.models.agent_config import AgentConfig
from app.models.machine import Machine

if TYPE_CHECKING:
    from sqlalchemy.ext.asyncio import AsyncSession

# Marks a machine whose data came from MongoDB rather than a live agent.
AGENTLESS_VERSION = "mongo-import"


async def get_or_create_machine(
    *,
    session: AsyncSession,
    hostname: str,
    os: str,
    os_version: str,
    arch: str,
) -> Machine:
    """Return the existing agentless machine for `hostname`, or create one.

    Matching is on hostname (case-sensitive, as stored). If you import from
    multiple orgs with colliding hostnames, extend the match key accordingly.
    """
    existing = (
        await session.execute(
            select(Machine).where(
                Machine.hostname == hostname,
                Machine.deleted_at.is_(None),
            )
        )
    ).scalar_one_or_none()

    now = datetime.now(tz=UTC)
    if existing is not None:
        existing.os = os
        existing.os_version = os_version
        existing.arch = arch
        existing.last_seen_at = now
        existing.status = "online"
        return existing

    machine = Machine(
        hostname=hostname,
        os=os,
        os_version=os_version,
        arch=arch,
        agent_version=AGENTLESS_VERSION,
        # Random, hashed, never returned to anyone — the agent_token_hash column
        # is NOT NULL + unique, but agentless machines never authenticate with it.
        agent_token_hash=hash_password(generate_opaque_token(num_bytes=32)),
        enrolled_at=now,
        last_seen_at=now,
        status="online",
        tags=["agentless", "mongodb"],
    )
    session.add(machine)
    await session.flush()
    # Every machine needs a config row (heartbeat/scan toggles read it).
    session.add(AgentConfig(machine_id=machine.id))
    await session.flush()
    return machine