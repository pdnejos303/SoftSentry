"""signature status_reason

Adds ``status_reason`` to ``signature_records`` — explains *why* a signature
verification failed when ``status == "invalid"``. Holds a reason code reported by
the agent (tampered / untrusted_root / broken_chain / revoked / distrusted) or a
raw HRESULT hex (e.g. "0x800B0004") for codes the agent couldn't map. Nullable:
NULL for valid/unsigned/expired/unknown rows and for rows scanned before this
feature (they fill in on the next scan).

Revision ID: 0016_signature_status_reason
Revises: 0015_agent_force_update
Create Date: 2026-06-23
"""

from __future__ import annotations

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "0016_signature_status_reason"
down_revision: str | None = "0015_agent_force_update"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None


def upgrade() -> None:
    op.add_column(
        "signature_records",
        sa.Column("status_reason", sa.String(length=40), nullable=True),
    )


def downgrade() -> None:
    op.drop_column("signature_records", "status_reason")
