"""CVE cache (from NVD/OSV) + software↔CVE match join (Module 5)."""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import (
    JSON,
    Boolean,
    DateTime,
    ForeignKey,
    Index,
    Numeric,
    String,
    Text,
    UniqueConstraint,
    func,
)
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, TimestampMixin, UuidMixin

# JSONB on Postgres (GIN-indexable), plain JSON on other backends (SQLite in tests).
JsonValue = JSON().with_variant(JSONB, "postgresql")

# Severity buckets, ordered most → least severe (used for sorting).
SEVERITY_ORDER = {"critical": 0, "high": 1, "medium": 2, "low": 3, "unknown": 4}


class CveRecord(Base, TimestampMixin):
    """One CVE as cached from an upstream feed (NVD primary, OSV future)."""

    __tablename__ = "cve_records"
    __table_args__ = (
        Index("ix_cve_records_severity", "severity"),
        Index("ix_cve_records_modified", "modified_at"),
    )

    id: Mapped[int] = mapped_column(primary_key=True)
    cve_id: Mapped[str] = mapped_column(String(20), unique=True, nullable=False, index=True)
    source: Mapped[str] = mapped_column(String(10), nullable=False)  # nvd | osv
    published_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    modified_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    severity: Mapped[str] = mapped_column(String(10), nullable=False)  # critical|high|medium|low
    cvss_score: Mapped[float | None] = mapped_column(Numeric(3, 1))
    description: Mapped[str] = mapped_column(Text, nullable=False)
    # CPE 2.3 strings for the GIN `@>` pre-filter on Postgres.
    cpe_matches: Mapped[list[str]] = mapped_column(JsonValue, nullable=False, default=list)
    # Normalized [{product, vendor, version_start_including, version_end_excluding, ...}].
    affected: Mapped[list[dict[str, object]] | None] = mapped_column(JsonValue)
    references: Mapped[list[str] | None] = mapped_column(JsonValue)


class Vulnerability(Base, UuidMixin):
    """A confirmed match between an installed software record and a CVE."""

    __tablename__ = "vulnerabilities"
    __table_args__ = (
        UniqueConstraint("software_id", "cve_id", name="uq_vuln_software_cve"),
        Index("ix_vuln_cve_id", "cve_id"),
        Index("ix_vuln_dismissed_matched", "is_dismissed", "matched_at"),
    )

    id: Mapped[int] = mapped_column(primary_key=True)
    software_id: Mapped[int] = mapped_column(
        ForeignKey("software_records.id", ondelete="CASCADE"),
        nullable=False,
    )
    cve_id: Mapped[str] = mapped_column(
        ForeignKey("cve_records.cve_id", ondelete="CASCADE"),
        nullable=False,
    )
    matched_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        server_default=func.now(),
        nullable=False,
    )
    recommended_version: Mapped[str | None] = mapped_column(String(100))
    # high | medium | low — name+vendor+version exact = high (spec 5.2).
    match_confidence: Mapped[str] = mapped_column(String(10), nullable=False, default="medium")
    is_dismissed: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)
    dismissed_by_user_id: Mapped[int | None] = mapped_column(
        ForeignKey("users.id", ondelete="SET NULL"),
    )
    dismissed_reason: Mapped[str | None] = mapped_column(Text)
    dismissed_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
