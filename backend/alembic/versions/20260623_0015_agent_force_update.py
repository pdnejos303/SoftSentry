"""agent force update flag

Adds ``force_update_requested`` to ``agent_configs`` — a one-shot flag set by the
admin "Update agent now" trigger. On the next heartbeat the backend offers the
latest binary as a *forced* update (bypassing version-compare / auto-update /
loop-breaker) and clears the flag in the same transaction. Defaults to false so
existing rows are unaffected.

Revision ID: 0015_agent_force_update
Revises: 0014_software_install_datetime
Create Date: 2026-06-23
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0015_agent_force_update"
down_revision: str | None = "0014_software_install_datetime"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column(
        "agent_configs",
        sa.Column(
            "force_update_requested",
            sa.Boolean(),
            nullable=False,
            server_default=sa.false(),
        ),
    )


def downgrade() -> None:
    op.drop_column("agent_configs", "force_update_requested")
