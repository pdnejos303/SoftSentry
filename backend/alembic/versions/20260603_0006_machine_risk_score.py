"""machine risk_score (Module 7)

Adds the stored `machines.risk_score` column (indexed) so the dashboard's
"top-N risky machines" query is a cheap indexed sort. The value is recomputed
inline after every scan ingest.

Revision ID: 0006_machine_risk_score
Revises: 0005_licenses
Create Date: 2026-06-03
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0006_machine_risk_score"
down_revision: str | None = "0005_licenses"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column(
        "machines",
        sa.Column("risk_score", sa.Float(), nullable=False, server_default="0"),
    )
    op.create_index("ix_machines_risk_score", "machines", ["risk_score"])


def downgrade() -> None:
    op.drop_index("ix_machines_risk_score", table_name="machines")
    op.drop_column("machines", "risk_score")
