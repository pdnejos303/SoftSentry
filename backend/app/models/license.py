"""Software licenses purchased by the org (Module 6 — License Compliance)."""

from __future__ import annotations

from datetime import date
from decimal import Decimal

from sqlalchemy import Date, Index, Integer, Numeric, String, Text
from sqlalchemy.orm import Mapped, mapped_column

from app.core.crypto import EncryptedString
from app.models.base import Base, TimestampMixin, UuidMixin


class License(Base, UuidMixin, TimestampMixin):
    """A license entitlement. `software_name` is matched (SQL ILIKE) against the
    software inventory to derive the installed count; admins may use % wildcards."""

    __tablename__ = "licenses"
    __table_args__ = (Index("ix_licenses_software_name", "software_name"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    software_name: Mapped[str] = mapped_column(String(500), nullable=False)
    publisher: Mapped[str | None] = mapped_column(String(255))
    # AES-encrypted at rest (column-level). Never returned in list responses.
    license_key: Mapped[str | None] = mapped_column(EncryptedString())
    purchased_count: Mapped[int] = mapped_column(Integer, nullable=False)
    purchased_at: Mapped[date | None] = mapped_column(Date)
    # NULL = perpetual license (never expires).
    expires_at: Mapped[date | None] = mapped_column(Date)
    cost_total: Mapped[Decimal | None] = mapped_column(Numeric(14, 2))
    currency: Mapped[str] = mapped_column(String(3), nullable=False, default="THB")
    vendor_contact: Mapped[str | None] = mapped_column(String(255))
    notes: Mapped[str | None] = mapped_column(Text)
