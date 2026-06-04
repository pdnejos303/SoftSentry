"""Whitelist / blacklist policy entries + the single-row policy settings (Module 4)."""

from __future__ import annotations

from sqlalchemy import Boolean, Index, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, TimestampMixin, UuidMixin


class WhitelistEntry(Base, UuidMixin, TimestampMixin):
    """Allowed-software pattern. In whitelist-enforcement mode, software that
    matches no entry is flagged."""

    __tablename__ = "whitelist_entries"
    __table_args__ = (Index("ix_whitelist_name_pattern", "name_pattern"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    name_pattern: Mapped[str] = mapped_column(String(500), nullable=False)
    publisher_pattern: Mapped[str | None] = mapped_column(String(255))
    version_constraint: Mapped[str | None] = mapped_column(String(100))
    notes: Mapped[str | None] = mapped_column(Text)


class BlacklistEntry(Base, UuidMixin, TimestampMixin):
    """Prohibited-software pattern. A match always raises an alert."""

    __tablename__ = "blacklist_entries"
    __table_args__ = (Index("ix_blacklist_name_pattern", "name_pattern"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    name_pattern: Mapped[str] = mapped_column(String(500), nullable=False)
    publisher_pattern: Mapped[str | None] = mapped_column(String(255))
    version_constraint: Mapped[str | None] = mapped_column(String(100))
    notes: Mapped[str | None] = mapped_column(Text)
    severity: Mapped[str] = mapped_column(String(10), nullable=False, default="high")
    reason: Mapped[str] = mapped_column(Text, nullable=False)


class PolicySettings(Base, TimestampMixin):
    """Single-row org policy. `id` is pinned to 1 by the service layer."""

    __tablename__ = "policy_settings"

    id: Mapped[int] = mapped_column(primary_key=True)
    whitelist_mode: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)
    # Raise alerts for unsigned software (spec 3.4 — admin may disable if noisy).
    flag_unsigned: Mapped[bool] = mapped_column(Boolean, nullable=False, default=True)
