"""reports + report_schedules (Module 8)

Adds the reporting tables: `report_schedules` (recurring definitions) first,
then `reports` which carries a nullable FK back to it.

Revision ID: 0007_reports
Revises: 0006_machine_risk_score
Create Date: 2026-06-03
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "0007_reports"
down_revision: str | None = "0006_machine_risk_score"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

# JSONB on Postgres, plain JSON on other backends — mirrors the models.
_JSON = sa.JSON().with_variant(postgresql.JSONB(), "postgresql")


def upgrade() -> None:
    op.create_table(
        "report_schedules",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column("uuid", sa.Uuid(), nullable=False),
        sa.Column("name", sa.String(length=255), nullable=False),
        sa.Column("type", sa.String(length=50), nullable=False),
        sa.Column("format", sa.String(length=10), nullable=False),
        sa.Column("params", _JSON, nullable=False),
        sa.Column("cron", sa.String(length=50), nullable=False),
        sa.Column("recipients", _JSON, nullable=False),
        sa.Column("is_active", sa.Boolean(), nullable=False),
        sa.Column("last_run_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("next_run_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("created_by_user_id", sa.BigInteger(), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
        sa.ForeignKeyConstraint(["created_by_user_id"], ["users.id"], ondelete="SET NULL"),
    )
    op.create_index("ix_report_schedules_uuid", "report_schedules", ["uuid"], unique=True)
    op.create_index(
        "ix_report_schedules_due", "report_schedules", ["is_active", "next_run_at"]
    )

    op.create_table(
        "reports",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column("uuid", sa.Uuid(), nullable=False),
        sa.Column("type", sa.String(length=50), nullable=False),
        sa.Column("format", sa.String(length=10), nullable=False),
        sa.Column("params", _JSON, nullable=False),
        sa.Column("status", sa.String(length=20), nullable=False),
        sa.Column("error_message", sa.Text(), nullable=True),
        sa.Column("file_path", sa.Text(), nullable=True),
        sa.Column("file_size", sa.BigInteger(), nullable=True),
        sa.Column("generated_by_user_id", sa.BigInteger(), nullable=True),
        sa.Column("schedule_id", sa.BigInteger(), nullable=True),
        sa.Column("completed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
        sa.ForeignKeyConstraint(["generated_by_user_id"], ["users.id"], ondelete="SET NULL"),
        sa.ForeignKeyConstraint(["schedule_id"], ["report_schedules.id"], ondelete="SET NULL"),
    )
    op.create_index("ix_reports_uuid", "reports", ["uuid"], unique=True)
    op.create_index("ix_reports_status", "reports", ["status"])
    op.create_index("ix_reports_type", "reports", ["type"])
    op.create_index("ix_reports_expires_at", "reports", ["expires_at"])


def downgrade() -> None:
    op.drop_index("ix_reports_expires_at", table_name="reports")
    op.drop_index("ix_reports_type", table_name="reports")
    op.drop_index("ix_reports_status", table_name="reports")
    op.drop_index("ix_reports_uuid", table_name="reports")
    op.drop_table("reports")
    op.drop_index("ix_report_schedules_due", table_name="report_schedules")
    op.drop_index("ix_report_schedules_uuid", table_name="report_schedules")
    op.drop_table("report_schedules")
