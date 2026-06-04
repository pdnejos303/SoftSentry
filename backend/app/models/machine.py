"""Machine model — enrolled endpoint running an agent."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import JSON, DateTime, Float, String
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, TimestampMixin, UuidMixin

# JSONB on Postgres (GIN-indexable), plain JSON on other backends (e.g. SQLite in tests).
JsonList = JSON().with_variant(JSONB, "postgresql")


class Machine(Base, UuidMixin, TimestampMixin):
    __tablename__ = "machines"

    id: Mapped[int] = mapped_column(primary_key=True)
    hostname: Mapped[str] = mapped_column(String(255), nullable=False, index=True)
    os: Mapped[str] = mapped_column(String(20), nullable=False)
    os_version: Mapped[str] = mapped_column(String(50), nullable=False)
    arch: Mapped[str] = mapped_column(String(10), nullable=False)
    agent_version: Mapped[str] = mapped_column(String(20), nullable=False)
    agent_token_hash: Mapped[str] = mapped_column(String(255), unique=True, nullable=False)
    enrolled_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    last_seen_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    last_scan_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    status: Mapped[str] = mapped_column(String(20), nullable=False, default="online")
    tags: Mapped[list[str]] = mapped_column(JsonList, nullable=False, default=list)
    # Risk score recomputed inline after each scan (Module 7); stored so the
    # "top-N risky machines" query stays a cheap indexed sort.
    risk_score: Mapped[float] = mapped_column(Float, nullable=False, default=0, index=True)
    deleted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
