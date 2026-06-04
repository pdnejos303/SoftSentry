"""Audit logging — record + query admin mutations (Module 9 — spec 9.6).

``record`` flushes (does not commit); the calling request handler owns the
transaction so the audit row lands atomically with the mutation it describes.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from sqlalchemy import ColumnElement, func, select

from app.models.audit_log import AuditLog
from app.models.user import User

if TYPE_CHECKING:
    from datetime import datetime

    from sqlalchemy.ext.asyncio import AsyncSession


async def record(
    session: AsyncSession,
    *,
    action: str,
    entity_type: str,
    entity_id: str | None = None,
    user_id: int | None = None,
    before: dict[str, Any] | None = None,
    after: dict[str, Any] | None = None,
    ip: str | None = None,
    user_agent: str | None = None,
) -> AuditLog:
    """Append an audit row. ``user_id`` None == system action.

    ``changes`` is only populated when a before/after is supplied, following the
    spec convention (``before=None`` for creates, ``after=None`` for deletes).
    """
    changes: dict[str, Any] | None = None
    if before is not None or after is not None:
        changes = {"before": before, "after": after}
    entry = AuditLog(
        user_id=user_id,
        action=action,
        entity_type=entity_type,
        entity_id=entity_id,
        changes=changes,
        ip_address=ip,
        # Audit log column is TEXT but keep the stored UA bounded.
        user_agent=user_agent[:1000] if user_agent else None,
    )
    session.add(entry)
    await session.flush()
    return entry


async def list_audit_logs(
    session: AsyncSession,
    *,
    user_uuid: str | None = None,
    action: str | None = None,
    entity_type: str | None = None,
    date_from: datetime | None = None,
    date_to: datetime | None = None,
    page: int = 1,
    page_size: int = 50,
) -> tuple[list[dict[str, object]], int]:
    """Newest-first, filtered, paginated audit log listing."""
    stmt = select(AuditLog, User.uuid, User.email, User.full_name).outerjoin(
        User, AuditLog.user_id == User.id
    )
    count_stmt = select(func.count()).select_from(AuditLog)

    conds: list[ColumnElement[bool]] = []
    if user_uuid:
        sub = select(User.id).where(User.uuid == user_uuid).scalar_subquery()
        conds.append(AuditLog.user_id == sub)
    if action:
        conds.append(AuditLog.action == action)
    if entity_type:
        conds.append(AuditLog.entity_type == entity_type)
    if date_from is not None:
        conds.append(AuditLog.created_at >= date_from)
    if date_to is not None:
        conds.append(AuditLog.created_at <= date_to)
    for c in conds:
        stmt = stmt.where(c)
        count_stmt = count_stmt.where(c)

    total = (await session.execute(count_stmt)).scalar_one()
    stmt = (
        stmt.order_by(AuditLog.created_at.desc(), AuditLog.id.desc())
        .offset((page - 1) * page_size)
        .limit(page_size)
    )
    rows = (await session.execute(stmt)).all()

    items = [
        {
            "id": log.id,
            "actor_uuid": actor_uuid,
            "actor_email": actor_email,
            "actor_name": actor_name,
            "action": log.action,
            "entity_type": log.entity_type,
            "entity_id": log.entity_id,
            "changes": log.changes,
            "ip_address": log.ip_address,
            "user_agent": log.user_agent,
            "created_at": log.created_at,
        }
        for log, actor_uuid, actor_email, actor_name in rows
    ]
    return items, total


async def distinct_actions(session: AsyncSession) -> list[str]:
    """Distinct action values — powers the audit-log filter dropdown."""
    rows = (
        await session.execute(select(AuditLog.action).distinct().order_by(AuditLog.action))
    ).scalars().all()
    return list(rows)
