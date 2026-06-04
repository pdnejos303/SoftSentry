"""inventory + signatures + agent config/heartbeat (Phase 2) and policy + alerts (Phase 3)

Backfills every model added after the Phase 1 initial schema so
`alembic upgrade head` matches the ORM metadata.

Revision ID: 0002_inventory_policy_alerts
Revises: 0001_initial
Create Date: 2026-05-31
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "0002_inventory_policy_alerts"
down_revision: str | None = "0001_initial"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def _uuid_col() -> sa.Column:
    return sa.Column("uuid", postgresql.UUID(as_uuid=True), nullable=False, unique=True)


def _timestamps() -> list[sa.Column]:
    return [
        sa.Column(
            "created_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
        sa.Column(
            "updated_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
    ]


# JSONB on Postgres; plain JSON elsewhere (mirrors the model variants).
_JSON = sa.JSON().with_variant(postgresql.JSONB(), "postgresql")


def upgrade() -> None:
    # ── scans ────────────────────────────────────────────────────────────────
    op.create_table(
        "scans",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        _uuid_col(),
        sa.Column(
            "machine_id",
            sa.BigInteger(),
            sa.ForeignKey("machines.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column("started_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("completed_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column(
            "received_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
        sa.Column("software_count", sa.Integer(), nullable=False),
        sa.Column("scan_type", sa.String(20), nullable=False),
        sa.Column("trigger", sa.String(20)),
        sa.Column("idempotency_key", sa.String(64), unique=True),
    )
    op.create_index("ix_scans_uuid", "scans", ["uuid"], unique=True)
    op.create_index("ix_scans_machine_received", "scans", ["machine_id", "received_at"])

    # ── software_records (signature_id FK added after signature_records) ───────
    op.create_table(
        "software_records",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        _uuid_col(),
        sa.Column(
            "machine_id",
            sa.BigInteger(),
            sa.ForeignKey("machines.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column("name", sa.String(500), nullable=False),
        sa.Column("version", sa.String(100), nullable=False),
        sa.Column("publisher", sa.String(255)),
        sa.Column("install_date", sa.Date()),
        sa.Column("install_path", sa.Text()),
        sa.Column("install_size_kb", sa.BigInteger()),
        sa.Column("arch", sa.String(10)),
        sa.Column("source", sa.String(20), nullable=False),
        sa.Column("signature_id", sa.BigInteger()),  # FK added later (circular)
        sa.Column(
            "first_seen_scan_id",
            sa.BigInteger(),
            sa.ForeignKey("scans.id", ondelete="SET NULL"),
        ),
        sa.Column(
            "last_seen_scan_id",
            sa.BigInteger(),
            sa.ForeignKey("scans.id", ondelete="SET NULL"),
        ),
        sa.Column("uninstalled_at", sa.DateTime(timezone=True)),
        *_timestamps(),
        sa.UniqueConstraint(
            "machine_id", "name", "version", name="uq_software_machine_name_version"
        ),
    )
    op.create_index("ix_software_records_uuid", "software_records", ["uuid"], unique=True)
    op.create_index("ix_software_name_version", "software_records", ["name", "version"])
    op.create_index(
        "ix_software_machine_uninstalled", "software_records", ["machine_id", "uninstalled_at"]
    )

    # ── signature_records ──────────────────────────────────────────────────────
    op.create_table(
        "signature_records",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        _uuid_col(),
        sa.Column(
            "software_id",
            sa.BigInteger(),
            sa.ForeignKey("software_records.id", ondelete="CASCADE"),
            nullable=False,
            unique=True,
        ),
        sa.Column("status", sa.String(20), nullable=False),
        sa.Column("signer", sa.String(500)),
        sa.Column("issuer", sa.String(500)),
        sa.Column("cert_thumbprint", sa.String(64)),
        sa.Column("cert_valid_from", sa.Date()),
        sa.Column("cert_valid_to", sa.Date()),
        sa.Column("signature_algorithm", sa.String(50)),
        sa.Column("chain", _JSON),
        sa.Column(
            "verified_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
    )
    op.create_index("ix_signature_records_uuid", "signature_records", ["uuid"], unique=True)
    op.create_index("ix_signature_records_status", "signature_records", ["status"])

    # Resolve the circular FK now that both tables exist.
    op.create_foreign_key(
        "fk_software_signature",
        "software_records",
        "signature_records",
        ["signature_id"],
        ["id"],
        ondelete="SET NULL",
    )

    # ── software_history ────────────────────────────────────────────────────────
    op.create_table(
        "software_history",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column(
            "machine_id",
            sa.BigInteger(),
            sa.ForeignKey("machines.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column("software_name", sa.String(500), nullable=False),
        sa.Column("software_version", sa.String(100), nullable=False),
        sa.Column("event", sa.String(20), nullable=False),
        sa.Column("previous_version", sa.String(100)),
        sa.Column("occurred_at", sa.DateTime(timezone=True), nullable=False),
        sa.Column("scan_id", sa.BigInteger(), sa.ForeignKey("scans.id", ondelete="SET NULL")),
    )
    op.create_index(
        "ix_software_history_machine_occurred", "software_history", ["machine_id", "occurred_at"]
    )

    # ── agent_configs ──────────────────────────────────────────────────────────
    op.create_table(
        "agent_configs",
        sa.Column(
            "machine_id",
            sa.BigInteger(),
            sa.ForeignKey("machines.id", ondelete="CASCADE"),
            primary_key=True,
        ),
        sa.Column("scan_interval_hours", sa.Integer(), nullable=False, server_default="6"),
        sa.Column(
            "auto_update_enabled", sa.Boolean(), nullable=False, server_default=sa.true()
        ),
        sa.Column(
            "manual_scan_requested", sa.Boolean(), nullable=False, server_default=sa.false()
        ),
        sa.Column(
            "updated_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
    )

    # ── heartbeats ──────────────────────────────────────────────────────────────
    op.create_table(
        "heartbeats",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column(
            "machine_id",
            sa.BigInteger(),
            sa.ForeignKey("machines.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column(
            "received_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False
        ),
        sa.Column("agent_version", sa.String(20)),
    )
    op.create_index("ix_heartbeats_machine_received", "heartbeats", ["machine_id", "received_at"])

    # ── whitelist_entries ────────────────────────────────────────────────────────
    op.create_table(
        "whitelist_entries",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        _uuid_col(),
        sa.Column("name_pattern", sa.String(500), nullable=False),
        sa.Column("publisher_pattern", sa.String(255)),
        sa.Column("version_constraint", sa.String(100)),
        sa.Column("notes", sa.Text()),
        *_timestamps(),
    )
    op.create_index("ix_whitelist_entries_uuid", "whitelist_entries", ["uuid"], unique=True)
    op.create_index("ix_whitelist_name_pattern", "whitelist_entries", ["name_pattern"])

    # ── blacklist_entries ────────────────────────────────────────────────────────
    op.create_table(
        "blacklist_entries",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        _uuid_col(),
        sa.Column("name_pattern", sa.String(500), nullable=False),
        sa.Column("publisher_pattern", sa.String(255)),
        sa.Column("version_constraint", sa.String(100)),
        sa.Column("notes", sa.Text()),
        sa.Column("severity", sa.String(10), nullable=False, server_default="high"),
        sa.Column("reason", sa.Text(), nullable=False),
        *_timestamps(),
    )
    op.create_index("ix_blacklist_entries_uuid", "blacklist_entries", ["uuid"], unique=True)
    op.create_index("ix_blacklist_name_pattern", "blacklist_entries", ["name_pattern"])

    # ── policy_settings ────────────────────────────────────────────────────────
    op.create_table(
        "policy_settings",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        sa.Column("whitelist_mode", sa.Boolean(), nullable=False, server_default=sa.false()),
        *_timestamps(),
    )

    # ── alerts ──────────────────────────────────────────────────────────────────
    op.create_table(
        "alerts",
        sa.Column("id", sa.BigInteger(), primary_key=True),
        _uuid_col(),
        sa.Column(
            "machine_id",
            sa.BigInteger(),
            sa.ForeignKey("machines.id", ondelete="CASCADE"),
            nullable=False,
        ),
        sa.Column(
            "software_id",
            sa.BigInteger(),
            sa.ForeignKey("software_records.id", ondelete="SET NULL"),
        ),
        sa.Column("type", sa.String(50), nullable=False),
        sa.Column("severity", sa.String(10), nullable=False),
        sa.Column("status", sa.String(20), nullable=False, server_default="active"),
        sa.Column("title", sa.String(500), nullable=False),
        sa.Column("detail", sa.Text()),
        sa.Column("acknowledged_at", sa.DateTime(timezone=True)),
        sa.Column("resolved_at", sa.DateTime(timezone=True)),
        *_timestamps(),
    )
    op.create_index("ix_alerts_uuid", "alerts", ["uuid"], unique=True)
    op.create_index("ix_alerts_status_severity", "alerts", ["status", "severity"])
    op.create_index("ix_alerts_dedup", "alerts", ["machine_id", "software_id", "type"])
    op.create_index("ix_alerts_type", "alerts", ["type"])


def downgrade() -> None:
    op.drop_table("alerts")
    op.drop_table("policy_settings")
    op.drop_index("ix_blacklist_name_pattern", table_name="blacklist_entries")
    op.drop_index("ix_blacklist_entries_uuid", table_name="blacklist_entries")
    op.drop_table("blacklist_entries")
    op.drop_index("ix_whitelist_name_pattern", table_name="whitelist_entries")
    op.drop_index("ix_whitelist_entries_uuid", table_name="whitelist_entries")
    op.drop_table("whitelist_entries")
    op.drop_index("ix_heartbeats_machine_received", table_name="heartbeats")
    op.drop_table("heartbeats")
    op.drop_table("agent_configs")
    op.drop_index("ix_software_history_machine_occurred", table_name="software_history")
    op.drop_table("software_history")
    op.drop_constraint("fk_software_signature", "software_records", type_="foreignkey")
    op.drop_index("ix_signature_records_status", table_name="signature_records")
    op.drop_index("ix_signature_records_uuid", table_name="signature_records")
    op.drop_table("signature_records")
    op.drop_index("ix_software_machine_uninstalled", table_name="software_records")
    op.drop_index("ix_software_name_version", table_name="software_records")
    op.drop_index("ix_software_records_uuid", table_name="software_records")
    op.drop_table("software_records")
    op.drop_index("ix_scans_machine_received", table_name="scans")
    op.drop_index("ix_scans_uuid", table_name="scans")
    op.drop_table("scans")
