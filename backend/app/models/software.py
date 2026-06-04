"""Software inventory records + install/uninstall history."""

from __future__ import annotations

from datetime import date, datetime

from sqlalchemy import (
    BigInteger,
    Date,
    DateTime,
    ForeignKey,
    Index,
    String,
    Text,
    UniqueConstraint,
)
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, TimestampMixin, UuidMixin


class SoftwareRecord(Base, UuidMixin, TimestampMixin):
    __tablename__ = "software_records"
    __table_args__ = (
        UniqueConstraint("machine_id", "name", "version", name="uq_software_machine_name_version"),
        Index("ix_software_name_version", "name", "version"),
        Index("ix_software_machine_uninstalled", "machine_id", "uninstalled_at"),
    )

    id: Mapped[int] = mapped_column(primary_key=True)
    machine_id: Mapped[int] = mapped_column(
        ForeignKey("machines.id", ondelete="CASCADE"),
        nullable=False,
    )
    name: Mapped[str] = mapped_column(String(500), nullable=False)
    version: Mapped[str] = mapped_column(String(100), nullable=False)
    publisher: Mapped[str | None] = mapped_column(String(255))
    install_date: Mapped[date | None] = mapped_column(Date)
    install_path: Mapped[str | None] = mapped_column(Text)
    install_size_kb: Mapped[int | None] = mapped_column(BigInteger)
    arch: Mapped[str | None] = mapped_column(String(10))
    source: Mapped[str] = mapped_column(String(20), nullable=False)  # registry | appstore | plist
    # 1:1 link to the signature seen in the latest scan. use_alter resolves the
    # circular FK with signature_records (which also references this table).
    signature_id: Mapped[int | None] = mapped_column(
        ForeignKey(
            "signature_records.id",
            ondelete="SET NULL",
            use_alter=True,
            name="fk_software_signature",
        ),
    )
    first_seen_scan_id: Mapped[int | None] = mapped_column(
        ForeignKey("scans.id", ondelete="SET NULL"),
    )
    last_seen_scan_id: Mapped[int | None] = mapped_column(
        ForeignKey("scans.id", ondelete="SET NULL"),
    )
    uninstalled_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))


class SoftwareHistory(Base):
    __tablename__ = "software_history"
    __table_args__ = (Index("ix_software_history_machine_occurred", "machine_id", "occurred_at"),)

    id: Mapped[int] = mapped_column(primary_key=True)
    machine_id: Mapped[int] = mapped_column(
        ForeignKey("machines.id", ondelete="CASCADE"),
        nullable=False,
    )
    software_name: Mapped[str] = mapped_column(String(500), nullable=False)
    software_version: Mapped[str] = mapped_column(String(100), nullable=False)
    # installed | uninstalled | updated
    event: Mapped[str] = mapped_column(String(20), nullable=False)
    previous_version: Mapped[str | None] = mapped_column(String(100))
    occurred_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    scan_id: Mapped[int | None] = mapped_column(ForeignKey("scans.id", ondelete="SET NULL"))
