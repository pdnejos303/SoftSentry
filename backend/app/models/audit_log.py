"""Audit log model — records every admin mutation (Module 9 — spec 9.6)."""

from __future__ import annotations

from datetime import datetime
from typing import Any

from sqlalchemy import (
    JSON,
    DateTime,
    ForeignKey,
    Index,
    String,
    Text,
    func,
)
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base

# JSONB on Postgres, plain JSON elsewhere (SQLite in tests).
Json = JSON().with_variant(JSONB, "postgresql")


class AuditLog(Base):
    """One row per audited mutation. ``user_id`` NULL means a system action.

    ``changes`` carries ``{"before": ..., "after": ...}`` — ``before`` is null
    for creates, ``after`` is null for deletes (spec convention).
    """

    __tablename__ = "audit_logs"
    __table_args__ = (
        Index("ix_audit_logs_user_created", "user_id", "created_at"),
        Index("ix_audit_logs_entity", "entity_type", "entity_id"),
        Index("ix_audit_logs_created_at", "created_at"),
        Index("ix_audit_logs_action", "action"),
    )

    # Plain Integer PK in the model so SQLite (tests) autoincrements via rowid;
    # the migration declares BigInteger for Postgres.
    id: Mapped[int] = mapped_column(primary_key=True)
    # NULL = system / automated action.
    user_id: Mapped[int | None] = mapped_column(ForeignKey("users.id", ondelete="SET NULL"))
    action: Mapped[str] = mapped_column(String(50), nullable=False)
    entity_type: Mapped[str] = mapped_column(String(50), nullable=False)
    entity_id: Mapped[str | None] = mapped_column(String(50))
    changes: Mapped[dict[str, Any] | None] = mapped_column(Json)
    ip_address: Mapped[str | None] = mapped_column(String(64))
    user_agent: Mapped[str | None] = mapped_column(Text)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        server_default=func.now(),
        nullable=False,
    )
