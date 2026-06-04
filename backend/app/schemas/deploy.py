"""Pydantic schemas for the one-click deployment endpoints (admin)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime

from pydantic import BaseModel, ConfigDict, Field


class DeploymentTokenCreateRequest(BaseModel):
    label: str | None = Field(default=None, max_length=255)
    # None → never expires. Bounded to a year to avoid forgotten eternal tokens.
    expires_in_days: int | None = Field(default=None, ge=1, le=365)
    # None → unlimited machines may enrol with this one link.
    max_uses: int | None = Field(default=None, ge=1, le=100_000)


class DeploymentTokenCreated(BaseModel):
    uuid: uuid_lib.UUID
    token: str = Field(..., description="plaintext — show once, used to build the install link")
    label: str | None
    expires_at: datetime
    max_uses: int | None


class DeploymentTokenOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    uuid: uuid_lib.UUID
    label: str | None
    expires_at: datetime
    max_uses: int | None
    use_count: int
    revoked_at: datetime | None
    created_at: datetime
