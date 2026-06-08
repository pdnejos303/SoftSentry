"""User model — dashboard accounts (admin/viewer)."""

from __future__ import annotations

from datetime import datetime
from typing import Literal

from sqlalchemy import Boolean, CheckConstraint, DateTime, Index, String, text
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, TimestampMixin, UuidMixin

UserRole = Literal["dev", "admin", "viewer"]


class User(Base, UuidMixin, TimestampMixin):
    __tablename__ = "users"
    __table_args__ = (
        CheckConstraint("role IN ('dev', 'admin', 'viewer')", name="ck_users_role"),
        # Non-unique index for lookups…
        Index("ix_users_email", "email"),
        # …plus a PARTIAL unique index so an email is unique only among live
        # accounts — soft-deleting a user frees the address for reuse (spec 9.4).
        Index(
            "uq_users_email_active",
            "email",
            unique=True,
            postgresql_where=text("deleted_at IS NULL"),
            sqlite_where=text("deleted_at IS NULL"),
        ),
    )

    id: Mapped[int] = mapped_column(primary_key=True)
    email: Mapped[str] = mapped_column(String(255), nullable=False)
    password_hash: Mapped[str] = mapped_column(String(255), nullable=False)
    full_name: Mapped[str] = mapped_column(String(255), nullable=False)
    role: Mapped[str] = mapped_column(String(20), nullable=False, default="viewer")
    is_active: Mapped[bool] = mapped_column(Boolean, nullable=False, default=True)
    last_login_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    deleted_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
