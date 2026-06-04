"""Heartbeat — liveness ping from an agent (retained ~7 days in production)."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Index, String, func
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base


class Heartbeat(Base):
    __tablename__ = "heartbeats"
    __table_args__ = (Index("ix_heartbeats_machine_received", "machine_id", "received_at"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    machine_id: Mapped[int] = mapped_column(
        ForeignKey("machines.id", ondelete="CASCADE"),
        nullable=False,
    )
    received_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        server_default=func.now(),
        nullable=False,
    )
    agent_version: Mapped[str | None] = mapped_column(String(20))
