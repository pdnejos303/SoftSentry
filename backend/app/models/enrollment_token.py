"""Enrollment / deployment token used by agents to obtain a permanent agent token.

A token with ``max_uses == 1`` is a classic one-time enrollment token. A token
with ``max_uses`` NULL (unlimited) or > 1 is a reusable *deployment token* — one
link an admin can hand to the whole organisation so every endpoint enrols from
the same installer download (one-click installer design).
"""

from __future__ import annotations

from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Integer, String
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base, TimestampMixin, UuidMixin


class EnrollmentToken(Base, UuidMixin, TimestampMixin):
    __tablename__ = "enrollment_tokens"

    id: Mapped[int] = mapped_column(primary_key=True)
    token_hash: Mapped[str] = mapped_column(String(255), unique=True, nullable=False)
    label: Mapped[str | None] = mapped_column(String(255))
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), nullable=False)
    # NULL = unlimited uses (deployment token); 1 = classic one-time token.
    max_uses: Mapped[int | None] = mapped_column(Integer)
    # How many machines have enrolled with this token so far.
    use_count: Mapped[int] = mapped_column(Integer, nullable=False, default=0)
    # Set when an admin revokes the token; a revoked token can never enrol again.
    revoked_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    # Last enrolment time + the most recent machine (kept for one-time history).
    used_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    used_by_machine_id: Mapped[int | None] = mapped_column(
        ForeignKey("machines.id", ondelete="SET NULL"),
    )
    created_by_user_id: Mapped[int | None] = mapped_column(
        ForeignKey("users.id", ondelete="SET NULL"),
    )

    @property
    def is_revoked(self) -> bool:
        return self.revoked_at is not None

    def uses_remaining(self) -> int | None:
        """Remaining enrolments, or None when unlimited."""
        if self.max_uses is None:
            return None
        return max(self.max_uses - self.use_count, 0)
