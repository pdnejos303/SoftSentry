"""Scan — one row per software inventory submission from an agent."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Index, Integer, String, func
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, UuidMixin


class Scan(Base, UuidMixin):
    __tablename__ = "scans"
    __table_args__ = (Index("ix_scans_machine_received", "machine_id", "received_at"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    machine_id: Mapped[int] = mapped_column(
        ForeignKey("machines.id", ondelete="CASCADE"),
        nullable=False,
    )
    started_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    completed_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    received_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        server_default=func.now(),
        nullable=False,
    )
    software_count: Mapped[int] = mapped_column(Integer, nullable=False)
    scan_type: Mapped[str] = mapped_column(String(20), nullable=False)  # auto | manual
    trigger: Mapped[str | None] = mapped_column(String(20))  # schedule | user_request | enroll
    # Agent-supplied Idempotency-Key; lets a retried POST /scans de-dup (protocol §Idempotency).
    idempotency_key: Mapped[str | None] = mapped_column(String(64), unique=True)
