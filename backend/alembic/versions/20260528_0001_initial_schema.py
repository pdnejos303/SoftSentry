"""initial schema — users, machines, enrollment_tokens

Revision ID: 0001_initial
Revises:
Create Date: 2026-05-28
"""
from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "0001_initial"
down_revision: str | None = None
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.create_table(
        "users",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column(
            "uuid",
            postgresql.UUID(as_uuid=True),
            nullable=False,
            unique=True,
        ),
        sa.Column("email", sa.String(255), nullable=False, unique=True),
        sa.Column("password_hash", sa.String(255), nullable=False),
        sa.Column("full_name", sa.String(255), nullable=False),
        sa.Column("role", sa.String(20), nullable=False),
        sa.Column("is_active", sa.Boolean(), nullable=False, server_default=sa.true()),
        sa.Column("last_login_at", sa.DateTime(timezone=True)),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
        sa.Column(
            "updated_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
        sa.Column("deleted_at", sa.DateTime(timezone=True)),
        sa.CheckConstraint("role IN ('admin', 'viewer')", name="ck_users_role"),
    )
    op.create_index("ix_users_email", "users", ["email"], unique=True)
    op.create_index("ix_users_uuid", "users", ["uuid"], unique=True)

    op.create_table(
        "machines",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column(
            "uuid",
            postgresql.UUID(as_uuid=True),
            nullable=False,
            unique=True,
        ),
        sa.Column("hostname", sa.String(255), nullable=False),
        sa.Column("os", sa.String(20), nullable=False),
        sa.Column("os_version", sa.String(50), nullable=False),
        sa.Column("arch", sa.String(10), nullable=False),
        sa.Column("agent_version", sa.String(20), nullable=False),
        sa.Column("agent_token_hash", sa.String(255), nullable=False, unique=True),
        sa.Column("enrolled_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("last_seen_at", sa.DateTime(timezone=True)),
        sa.Column("last_scan_at", sa.DateTime(timezone=True)),
        sa.Column(
            "status",
            sa.String(20),
            nullable=False,
            server_default=sa.text("'online'"),
        ),
        sa.Column(
            "tags",
            postgresql.JSONB(astext_type=sa.Text()),
            nullable=False,
            server_default=sa.text("'[]'::jsonb"),
        ),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
        sa.Column(
            "updated_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
        sa.Column("deleted_at", sa.DateTime(timezone=True)),
    )
    op.create_index("ix_machines_uuid", "machines", ["uuid"], unique=True)
    op.create_index("ix_machines_hostname", "machines", ["hostname"])
    op.create_index(
        "ix_machines_status_last_seen", "machines", ["status", "last_seen_at"]
    )

    op.create_table(
        "enrollment_tokens",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column(
            "uuid",
            postgresql.UUID(as_uuid=True),
            nullable=False,
            unique=True,
        ),
        sa.Column("token_hash", sa.String(255), nullable=False, unique=True),
        sa.Column("label", sa.String(255)),
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("used_at", sa.DateTime(timezone=True)),
        sa.Column(
            "used_by_machine_id",
            sa.BigInteger(),
            sa.ForeignKey("machines.id", ondelete="SET NULL"),
        ),
        sa.Column(
            "created_by_user_id",
            sa.BigInteger(),
            sa.ForeignKey("users.id", ondelete="SET NULL"),
        ),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
        sa.Column(
            "updated_at",
            sa.DateTime(timezone=True),
            server_default=sa.func.now(),
            nullable=False,
        ),
    )
    op.create_index(
        "ix_enrollment_tokens_uuid", "enrollment_tokens", ["uuid"], unique=True
    )
    op.create_index(
        "ix_enrollment_tokens_expires_at", "enrollment_tokens", ["expires_at"]
    )


def downgrade() -> None:
    op.drop_index("ix_enrollment_tokens_expires_at", table_name="enrollment_tokens")
    op.drop_index("ix_enrollment_tokens_uuid", table_name="enrollment_tokens")
    op.drop_table("enrollment_tokens")
    op.drop_index("ix_machines_status_last_seen", table_name="machines")
    op.drop_index("ix_machines_hostname", table_name="machines")
    op.drop_index("ix_machines_uuid", table_name="machines")
    op.drop_table("machines")
    op.drop_index("ix_users_uuid", table_name="users")
    op.drop_index("ix_users_email", table_name="users")
    op.drop_table("users")
