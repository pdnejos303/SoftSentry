"""machine identity: display_name, owner, fingerprint

Adds admin-set naming (`display_name`, `owner`) and a stable hardware
`fingerprint` (indexed) to `machines`. The fingerprint lets the enroll path
dedup a re-installed machine back onto its existing row instead of creating a
duplicate; display_name/owner make "whose machine is this?" answerable.

Revision ID: 0011_machine_identity
Revises: 0010_user_role_dev
Create Date: 2026-06-15
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0011_machine_identity"
down_revision: str | None = "0010_user_role_dev"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column("machines", sa.Column("display_name", sa.String(length=255), nullable=True))
    op.add_column("machines", sa.Column("owner", sa.String(length=255), nullable=True))
    op.add_column("machines", sa.Column("fingerprint", sa.String(length=255), nullable=True))
    op.create_index("ix_machines_fingerprint", "machines", ["fingerprint"])


def downgrade() -> None:
    op.drop_index("ix_machines_fingerprint", table_name="machines")
    op.drop_column("machines", "fingerprint")
    op.drop_column("machines", "owner")
    op.drop_column("machines", "display_name")
