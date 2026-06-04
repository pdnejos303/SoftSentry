"""deployment tokens (reusable enrollment tokens — one-click installer)

Adds reuse + revocation columns to `enrollment_tokens` so a single
"deployment token" can enrol many machines from one installer download.
Existing rows are backfilled to classic one-time semantics (max_uses=1).

Revision ID: 0009_deployment_tokens
Revises: 0008_audit_logs
Create Date: 2026-06-04
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0009_deployment_tokens"
down_revision: str | None = "0008_audit_logs"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column(
        "enrollment_tokens",
        sa.Column("max_uses", sa.Integer(), nullable=True),
    )
    op.add_column(
        "enrollment_tokens",
        sa.Column("use_count", sa.Integer(), nullable=False, server_default="0"),
    )
    op.add_column(
        "enrollment_tokens",
        sa.Column("revoked_at", sa.DateTime(timezone=True), nullable=True),
    )

    # Backfill: every pre-existing token was one-time. Mark max_uses=1 and set
    # use_count=1 for tokens that were already consumed (used_at not null).
    op.execute("UPDATE enrollment_tokens SET max_uses = 1 WHERE max_uses IS NULL")
    op.execute("UPDATE enrollment_tokens SET use_count = 1 WHERE used_at IS NOT NULL")

    # Drop the server_default now that existing rows are backfilled; the model
    # supplies the default (0) for new rows.
    op.alter_column("enrollment_tokens", "use_count", server_default=None)


def downgrade() -> None:
    op.drop_column("enrollment_tokens", "revoked_at")
    op.drop_column("enrollment_tokens", "use_count")
    op.drop_column("enrollment_tokens", "max_uses")
