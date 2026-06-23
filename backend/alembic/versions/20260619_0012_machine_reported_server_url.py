"""machine reported_server_url

Adds ``reported_server_url`` to ``machines``: the backend URL the agent says it
phones home to (its ``server_url`` from config.yaml, sent on every heartbeat).
Lets the dashboard show *where each agent actually reports* — the real source of
truth — instead of an admin having to open config.yaml on the endpoint.

Revision ID: 0012_machine_reported_server_url
Revises: 0011_machine_identity
Create Date: 2026-06-19
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0012_machine_reported_server_url"
down_revision: str | None = "0011_machine_identity"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column(
        "machines",
        sa.Column("reported_server_url", sa.String(length=512), nullable=True),
    )


def downgrade() -> None:
    op.drop_column("machines", "reported_server_url")
