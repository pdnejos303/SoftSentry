"""Attribute an audit record to the current HTTP request.

Thin wrapper over ``audit_service.record`` that fills ``ip``/``user_agent`` from
the FastAPI ``Request`` so mutation routers stay terse. Like ``record`` it
flushes (not commits) — the calling handler still owns the transaction, so the
audit row lands atomically with the mutation it describes.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from fastapi import Request
    from sqlalchemy.ext.asyncio import AsyncSession

    from app.models.audit_log import AuditLog
    from app.models.user import User

from app.core.deps import client_ip
from app.services import audit_service


async def audit(
    session: AsyncSession,
    request: Request,
    actor: User,
    *,
    action: str,
    entity_type: str,
    entity_id: str | None = None,
    before: dict[str, Any] | None = None,
    after: dict[str, Any] | None = None,
) -> AuditLog:
    """Record a request-attributed audit row for ``actor``."""
    return await audit_service.record(
        session,
        user_id=actor.id,
        action=action,
        entity_type=entity_type,
        entity_id=entity_id,
        before=before,
        after=after,
        ip=client_ip(request),
        user_agent=request.headers.get("user-agent"),
    )
