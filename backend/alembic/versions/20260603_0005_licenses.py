"""licenses + license alerts (Module 6)

Adds the `licenses` table and extends `alerts` to carry org-level (machine-less)
license alerts: `machine_id` becomes nullable and a `license_id` FK + `dedup_key`
discriminator are added.

Revision ID: 0005_licenses
Revises: 0004_cve_vulnerabilities
Create Date: 2026-06-03
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0005_licenses"
down_revision: str | None = "0004_cve_vulnerabilities"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    # ── licenses ──────────────────────────────────────────────────────────────
    op.create_table(
        "licenses",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column("uuid", sa.Uuid(as_uuid=True), nullable=False, unique=True),
        sa.Column("software_name", sa.String(500), nullable=False),
        sa.Column("publisher", sa.String(255)),
        sa.Column("license_key", sa.LargeBinary()),
        sa.Column("purchased_count", sa.Integer(), nullable=False),
        sa.Column("purchased_at", sa.Date()),
        sa.Column("expires_at", sa.Date()),
        sa.Column("cost_total", sa.Numeric(14, 2)),
        sa.Column("currency", sa.String(3), nullable=False, server_default="THB"),
        sa.Column("vendor_contact", sa.String(255)),
        sa.Column("notes", sa.Text()),
        sa.Column(
            "created_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
        sa.Column(
            "updated_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
    )
    op.create_index("ix_licenses_uuid", "licenses", ["uuid"], unique=True)
    op.create_index("ix_licenses_software_name", "licenses", ["software_name"])

    # ── alerts: org-level (license) support ───────────────────────────────────
    op.alter_column("alerts", "machine_id", existing_type=sa.BigInteger(), nullable=True)
    op.add_column(
        "alerts",
        sa.Column(
            "license_id",
            sa.BigInteger(),
            sa.ForeignKey("licenses.id", ondelete="CASCADE"),
        ),
    )
    op.add_column("alerts", sa.Column("dedup_key", sa.String(100)))
    op.create_index("ix_alerts_license", "alerts", ["license_id", "type"])


def downgrade() -> None:
    op.drop_index("ix_alerts_license", table_name="alerts")
    op.drop_column("alerts", "dedup_key")
    op.drop_column("alerts", "license_id")
    op.alter_column("alerts", "machine_id", existing_type=sa.BigInteger(), nullable=False)
    op.drop_index("ix_licenses_software_name", table_name="licenses")
    op.drop_index("ix_licenses_uuid", table_name="licenses")
    op.drop_table("licenses")
