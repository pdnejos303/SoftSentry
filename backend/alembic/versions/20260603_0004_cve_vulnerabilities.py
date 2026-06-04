"""cve_records + vulnerabilities (Module 5)

Adds the CVE cache and the software↔CVE match join.

Revision ID: 0004_cve_vulnerabilities
Revises: 0003_policy_flag_unsigned
Create Date: 2026-06-03
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "0004_cve_vulnerabilities"
down_revision: str | None = "0003_policy_flag_unsigned"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

# JSONB on Postgres; plain JSON elsewhere (mirrors the model variants).
_JSON = sa.JSON().with_variant(postgresql.JSONB(), "postgresql")


def _timestamps() -> list[sa.Column]:
    return [
        sa.Column(
            "created_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
        sa.Column(
            "updated_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
    ]


def upgrade() -> None:
    # ── cve_records ───────────────────────────────────────────────────────────
    op.create_table(
        "cve_records",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column("cve_id", sa.String(20), nullable=False, unique=True),
        sa.Column("source", sa.String(10), nullable=False),
        sa.Column("published_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("modified_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("severity", sa.String(10), nullable=False),
        sa.Column("cvss_score", sa.Numeric(3, 1)),
        sa.Column("description", sa.Text(), nullable=False),
        sa.Column("cpe_matches", _JSON, nullable=False),
        sa.Column("affected", _JSON),
        sa.Column("references", _JSON),
        *_timestamps(),
    )
    op.create_index("ix_cve_records_cve_id", "cve_records", ["cve_id"], unique=True)
    op.create_index("ix_cve_records_severity", "cve_records", ["severity"])
    op.create_index("ix_cve_records_modified", "cve_records", ["modified_at"])
    # GIN index for the `cpe_matches @>` candidate pre-filter (Postgres only).
    op.create_index(
        "ix_cve_records_cpe_gin",
        "cve_records",
        ["cpe_matches"],
        postgresql_using="gin",
    )

    # ── vulnerabilities ───────────────────────────────────────────────────────
    op.create_table(
        "vulnerabilities",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column("uuid", postgresql.UUID(as_uuid=True), nullable=False, unique=True),
        sa.Column(
            "software_id",
            sa.BigInteger(),
            sa.ForeignKey("software_records.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column(
            "cve_id",
            sa.String(20),
            sa.ForeignKey("cve_records.cve_id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column(
            "matched_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
        sa.Column("recommended_version", sa.String(100)),
        sa.Column("match_confidence", sa.String(10), nullable=False, server_default="medium"),
        sa.Column("is_dismissed", sa.Boolean(), nullable=False, server_default=sa.false()),
        sa.Column(
            "dismissed_by_user_id",
            sa.BigInteger(),
            sa.ForeignKey("users.id", ondelete="SET NULL"),
        ),
        sa.Column("dismissed_reason", sa.Text()),
        sa.Column("dismissed_at", sa.DateTime(timezone=True)),
        sa.UniqueConstraint("software_id", "cve_id", name="uq_vuln_software_cve"),
    )
    op.create_index("ix_vulnerabilities_uuid", "vulnerabilities", ["uuid"], unique=True)
    op.create_index("ix_vuln_cve_id", "vulnerabilities", ["cve_id"])
    op.create_index(
        "ix_vuln_dismissed_matched", "vulnerabilities", ["is_dismissed", "matched_at"]
    )


def downgrade() -> None:
    op.drop_index("ix_vuln_dismissed_matched", table_name="vulnerabilities")
    op.drop_index("ix_vuln_cve_id", table_name="vulnerabilities")
    op.drop_index("ix_vulnerabilities_uuid", table_name="vulnerabilities")
    op.drop_table("vulnerabilities")
    op.drop_index("ix_cve_records_cpe_gin", table_name="cve_records")
    op.drop_index("ix_cve_records_modified", table_name="cve_records")
    op.drop_index("ix_cve_records_severity", table_name="cve_records")
    op.drop_index("ix_cve_records_cve_id", table_name="cve_records")
    op.drop_table("cve_records")
