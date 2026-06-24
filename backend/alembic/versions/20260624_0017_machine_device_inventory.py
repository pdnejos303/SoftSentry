"""machine device inventory + windows update

Adds two JSONB blobs and a few denormalized scalar columns to ``machines`` so the
agent can report hardware specs (model/CPU/RAM/disks/GPU/firmware/security/...) and
Windows Update posture, and the dashboard can list/filter on the high-value fields
without parsing the blob.

- ``device_info``  — full hardware inventory blob.
- ``update_status`` — full Windows Update blob (status, pending KB list, ...).
- scalars for list/summary: ``model``, ``manufacturer``, ``cpu_model``,
  ``ram_total_mb``, ``wu_status``, ``wu_pending_count``, ``wu_checked_at``.

All nullable: machines scanned before this feature (or whose agent can't collect,
e.g. macOS this round) simply leave them NULL until the next compatible scan.

Revision ID: 0017_machine_device_inventory
Revises: 0016_signature_status_reason
Create Date: 2026-06-24
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects.postgresql import JSONB

revision: str = "0017_machine_device_inventory"
down_revision: str | None = "0016_signature_status_reason"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

# JSONB on Postgres; plain JSON elsewhere (mirrors app.models.machine.JsonList).
_JSON = sa.JSON().with_variant(JSONB, "postgresql")


def upgrade() -> None:
    op.add_column("machines", sa.Column("device_info", _JSON, nullable=True))
    op.add_column("machines", sa.Column("update_status", _JSON, nullable=True))
    op.add_column("machines", sa.Column("model", sa.String(length=255), nullable=True))
    op.add_column("machines", sa.Column("manufacturer", sa.String(length=255), nullable=True))
    op.add_column("machines", sa.Column("cpu_model", sa.String(length=255), nullable=True))
    op.add_column("machines", sa.Column("ram_total_mb", sa.Integer(), nullable=True))
    op.add_column("machines", sa.Column("wu_status", sa.String(length=20), nullable=True))
    op.add_column("machines", sa.Column("wu_pending_count", sa.Integer(), nullable=True))
    op.add_column(
        "machines",
        sa.Column("wu_checked_at", sa.DateTime(timezone=True), nullable=True),
    )


def downgrade() -> None:
    op.drop_column("machines", "wu_checked_at")
    op.drop_column("machines", "wu_pending_count")
    op.drop_column("machines", "wu_status")
    op.drop_column("machines", "ram_total_mb")
    op.drop_column("machines", "cpu_model")
    op.drop_column("machines", "manufacturer")
    op.drop_column("machines", "model")
    op.drop_column("machines", "update_status")
    op.drop_column("machines", "device_info")
