"""Machine model — enrolled endpoint running an agent."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import JSON, DateTime, Float, Integer, String
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, TimestampMixin, UuidMixin

# JSONB on Postgres (GIN-indexable), plain JSON on other backends (e.g. SQLite in tests).
JsonList = JSON().with_variant(JSONB, "postgresql")


class Machine(Base, UuidMixin, TimestampMixin):
    __tablename__ = "machines"

    id: Mapped[int] = mapped_column(primary_key=True)
    hostname: Mapped[str] = mapped_column(String(255), nullable=False, index=True)
    # Admin-set friendly name + owner so "whose machine is this?" is answerable
    # from the dashboard (hostnames like DESKTOP-AB12 don't say). Both optional;
    # the UI falls back to hostname when display_name is unset.
    display_name: Mapped[str | None] = mapped_column(String(255))
    owner: Mapped[str | None] = mapped_column(String(255))
    # Stable hardware fingerprint reported by the agent (Windows MachineGuid /
    # macOS IOPlatformUUID). Lets re-enrollment (e.g. re-running the one-click
    # installer) map back to the same machine instead of creating a duplicate.
    fingerprint: Mapped[str | None] = mapped_column(String(255), index=True)
    os: Mapped[str] = mapped_column(String(20), nullable=False)
    os_version: Mapped[str] = mapped_column(String(50), nullable=False)
    arch: Mapped[str] = mapped_column(String(10), nullable=False)
    agent_version: Mapped[str] = mapped_column(String(20), nullable=False)
    # Backend URL the agent reports phoning home to (its config.yaml server_url),
    # sent on every heartbeat. The dashboard shows this so admins can tell which
    # backend a machine talks to (local vs LAN) without opening config.yaml — and
    # it's the *reported* value, the real destination, not what deploy assumed.
    reported_server_url: Mapped[str | None] = mapped_column(String(512))
    agent_token_hash: Mapped[str] = mapped_column(String(255), unique=True, nullable=False)
    enrolled_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    last_seen_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    last_scan_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    status: Mapped[str] = mapped_column(String(20), nullable=False, default="online")
    tags: Mapped[list[str]] = mapped_column(JsonList, nullable=False, default=list)
    # Risk score recomputed inline after each scan (Module 7); stored so the
    # "top-N risky machines" query stays a cheap indexed sort.
    risk_score: Mapped[float] = mapped_column(Float, nullable=False, default=0, index=True)
    # Live scan progress, refreshed from each heartbeat while a scan runs (and
    # reset to "idle" when it ends). Lets the dashboard show a live progress bar
    # without a separate endpoint. All nullable: a machine that has never scanned
    # (or is idle) simply has no progress to show.
    scan_phase: Mapped[str | None] = mapped_column(String(20))
    scan_done: Mapped[int | None] = mapped_column(Integer)
    scan_total: Mapped[int | None] = mapped_column(Integer)
    scan_current_path: Mapped[str | None] = mapped_column(String(512))
    scan_progress_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    deleted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
