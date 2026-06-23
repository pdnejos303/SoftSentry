"""software accurate install datetime

Adds ``install_datetime`` to ``software_records`` — an accurate, best-effort
install timestamp reported by the agent (e.g. the install-dir / main-exe
creation time). The existing ``install_date`` keeps the registry's date-only
value for back-compat; this carries the time component when the agent can
determine it. Nullable: older agents (and apps with no derivable timestamp)
simply leave it empty.

Revision ID: 0014_software_install_datetime
Revises: 0013_machine_scan_progress
Create Date: 2026-06-22
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0014_software_install_datetime"
down_revision: str | None = "0013_machine_scan_progress"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column(
        "software_records",
        sa.Column("install_datetime", sa.DateTime(timezone=True), nullable=True),
    )


def downgrade() -> None:
    op.drop_column("software_records", "install_datetime")
