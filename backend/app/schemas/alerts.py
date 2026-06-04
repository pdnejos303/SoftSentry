"""Pydantic schemas for alerts (shared across detection modules)."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime

from pydantic import BaseModel


class AlertOut(BaseModel):
    uuid: uuid_lib.UUID
    # Null for org-level alerts (e.g. license expiry) not tied to an endpoint.
    machine_uuid: uuid_lib.UUID | None = None
    machine_hostname: str | None = None
    software_name: str | None = None
    software_version: str | None = None
    license_uuid: uuid_lib.UUID | None = None
    license_name: str | None = None
    type: str
    severity: str
    status: str
    title: str
    detail: str | None
    created_at: datetime
    acknowledged_at: datetime | None
    resolved_at: datetime | None


class AlertList(BaseModel):
    items: list[AlertOut]
    total: int
    page: int
    page_size: int
    total_pages: int
