"""policy_settings.flag_unsigned toggle (Module 3.4)

Adds the admin toggle controlling whether unsigned software raises alerts.

Revision ID: 0003_policy_flag_unsigned
Revises: 0002_inventory_policy_alerts
Create Date: 2026-06-01
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0003_policy_flag_unsigned"
down_revision: str | None = "0002_inventory_policy_alerts"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column(
        "policy_settings",
        sa.Column(
            "flag_unsigned",
            sa.Boolean(),
            nullable=False,
            server_default=sa.true(),
        ),
    )
    # Drop the server default once existing rows are backfilled; the ORM supplies
    # the default on insert going forward (matches whitelist_mode's pattern).
    op.alter_column("policy_settings", "flag_unsigned", server_default=None)


def downgrade() -> None:
    op.drop_column("policy_settings", "flag_unsigned")
