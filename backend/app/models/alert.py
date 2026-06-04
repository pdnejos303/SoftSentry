"""Alerts raised by detection modules (unauthorized, blacklisted, later: unsigned, vulnerable)."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Index, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, TimestampMixin, UuidMixin

# Alert lifecycle statuses.
STATUS_ACTIVE = "active"
STATUS_ACKNOWLEDGED = "acknowledged"
STATUS_RESOLVED = "resolved"

# Statuses considered "open" for dedup: one open alert per (machine, software, type).
OPEN_STATUSES = (STATUS_ACTIVE, STATUS_ACKNOWLEDGED)


class Alert(Base, UuidMixin, TimestampMixin):
    __tablename__ = "alerts"
    __table_args__ = (
        Index("ix_alerts_status_severity", "status", "severity"),
        Index("ix_alerts_dedup", "machine_id", "software_id", "type"),
        Index("ix_alerts_license", "license_id", "type"),
        Index("ix_alerts_type", "type"),
    )

    id: Mapped[int] = mapped_column(primary_key=True)
    # Null for org-level alerts not tied to an endpoint (e.g. license expiry).
    machine_id: Mapped[int | None] = mapped_column(
        ForeignKey("machines.id", ondelete="CASCADE")
    )
    # Null when the offending software has since been removed.
    software_id: Mapped[int | None] = mapped_column(
        ForeignKey("software_records.id", ondelete="SET NULL")
    )
    # Set for license_* alerts (Module 6); null otherwise.
    license_id: Mapped[int | None] = mapped_column(
        ForeignKey("licenses.id", ondelete="CASCADE")
    )
    # blacklisted_software | unauthorized_software | unsigned_software |
    # cve_critical | license_expiring | license_expired | license_overused
    type: Mapped[str] = mapped_column(String(50), nullable=False)
    severity: Mapped[str] = mapped_column(String(10), nullable=False)  # critical|high|medium|low
    status: Mapped[str] = mapped_column(String(20), nullable=False, default=STATUS_ACTIVE)
    title: Mapped[str] = mapped_column(String(500), nullable=False)
    detail: Mapped[str | None] = mapped_column(Text)
    # Extra discriminator for dedup when (machine, software, type) is insufficient
    # — e.g. license milestone band ("expiring_30"). Null for machine alerts.
    dedup_key: Mapped[str | None] = mapped_column(String(100))
    acknowledged_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    resolved_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
