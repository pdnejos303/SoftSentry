"""add 'dev' superuser role to users.role check constraint

Introduces a third dashboard role, `dev` (superuser), alongside the existing
`admin` (now limited to Overview + Machines + Deploy) and `viewer` roles.
Only the CHECK constraint changes; no data migration is needed (existing rows
stay `admin`/`viewer`).

Revision ID: 0010_user_role_dev
Revises: 0009_deployment_tokens
Create Date: 2026-06-08
"""

from __future__ import annotations

from collections.abc import Sequence

from alembic import op

revision: str = "0010_user_role_dev"
down_revision: str | None = "0009_deployment_tokens"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

_CONSTRAINT = "ck_users_role"


def upgrade() -> None:
    # batch_alter_table so SQLite (which can't ALTER a CHECK in place) rebuilds
    # the table, while Postgres issues a plain DROP/ADD CONSTRAINT.
    with op.batch_alter_table("users") as batch:
        batch.drop_constraint(_CONSTRAINT, type_="check")
        batch.create_check_constraint(
            _CONSTRAINT, "role IN ('dev', 'admin', 'viewer')"
        )


def downgrade() -> None:
    with op.batch_alter_table("users") as batch:
        batch.drop_constraint(_CONSTRAINT, type_="check")
        batch.create_check_constraint(_CONSTRAINT, "role IN ('admin', 'viewer')")
