"""Per-machine agent configuration, pulled by the agent on heartbeat signal."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import Boolean, DateTime, ForeignKey, Integer, func
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base


class AgentConfig(Base):
    __tablename__ = "agent_configs"

    machine_id: Mapped[int] = mapped_column(
        ForeignKey("machines.id", ondelete="CASCADE"),
        primary_key=True,
    )
    scan_interval_hours: Mapped[int] = mapped_column(Integer, nullable=False, default=6)
    auto_update_enabled: Mapped[bool] = mapped_column(Boolean, nullable=False, default=True)
    manual_scan_requested: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        server_default=func.now(),
        onupdate=func.now(),
        nullable=False,
    )
