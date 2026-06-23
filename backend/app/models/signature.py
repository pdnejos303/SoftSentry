"""Digital signature metadata for a scanned software's executable (1:1)."""

from __future__ import annotations

from datetime import date, datetime

from sqlalchemy import JSON, Date, DateTime, ForeignKey, String, func
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, UuidMixin

# JSONB on Postgres (GIN-indexable), plain JSON on other backends (e.g. SQLite in tests).
JsonValue = JSON().with_variant(JSONB, "postgresql")


class SignatureRecord(Base, UuidMixin):
    __tablename__ = "signature_records"

    id: Mapped[int] = mapped_column(primary_key=True)
    software_id: Mapped[int] = mapped_column(
        ForeignKey("software_records.id", ondelete="CASCADE"),
        unique=True,
        nullable=False,
    )
    status: Mapped[str] = mapped_column(String(20), nullable=False, index=True)
    # Why verification failed (status == "invalid"): reason code or raw HRESULT
    # hex. NULL for valid/unsigned/expired/unknown and for pre-feature rows.
    status_reason: Mapped[str | None] = mapped_column(String(40))
    signer: Mapped[str | None] = mapped_column(String(500))
    issuer: Mapped[str | None] = mapped_column(String(500))
    cert_thumbprint: Mapped[str | None] = mapped_column(String(64))
    cert_valid_from: Mapped[date | None] = mapped_column(Date)
    cert_valid_to: Mapped[date | None] = mapped_column(Date)
    signature_algorithm: Mapped[str | None] = mapped_column(String(50))
    chain: Mapped[list[dict[str, object]] | None] = mapped_column(JsonValue)
    verified_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        server_default=func.now(),
        nullable=False,
    )
