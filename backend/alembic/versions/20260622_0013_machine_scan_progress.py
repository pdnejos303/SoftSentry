"""machine scan progress

Adds live scan-progress columns to ``machines`` so the dashboard can render a
progress bar that updates as agents heartbeat mid-scan. All nullable: a machine
that is idle (or has never scanned) simply has no progress to show.

Revision ID: 0013_machine_scan_progress
Revises: 0012_machine_reported_server_url
Create Date: 2026-06-22
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0013_machine_scan_progress"
down_revision: str | None = "0012_machine_reported_server_url"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column("machines", sa.Column("scan_phase", sa.String(length=20), nullable=True))
    op.add_column("machines", sa.Column("scan_done", sa.Integer(), nullable=True))
    op.add_column("machines", sa.Column("scan_total", sa.Integer(), nullable=True))
    op.add_column(
        "machines", sa.Column("scan_current_path", sa.String(length=512), nullable=True)
    )
    op.add_column(
        "machines",
        sa.Column("scan_progress_at", sa.DateTime(timezone=True), nullable=True),
    )


def downgrade() -> None:
    op.drop_column("machines", "scan_progress_at")
    op.drop_column("machines", "scan_current_path")
    op.drop_column("machines", "scan_total")
    op.drop_column("machines", "scan_done")
    op.drop_column("machines", "scan_phase")
