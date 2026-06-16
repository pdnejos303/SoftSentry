"""Pydantic schemas for dashboard machine endpoints."""

from __future__ import annotations

import uuid as uuid_lib
from datetime import datetime

from pydantic import BaseModel, Field


class VulnerabilityCount(BaseModel):
    """Per-severity vuln tally. Populated in Phase 3 (CVE matching); zeros for now."""

    critical: int = 0
    high: int = 0
    medium: int = 0
    low: int = 0


class MachineListItem(BaseModel):
    uuid: uuid_lib.UUID
    hostname: str
    display_name: str | None = None
    owner: str | None = None
    os: str
    os_version: str
    agent_version: str
    status: str
    last_seen_at: datetime | None
    last_scan_at: datetime | None
    tags: list[str]
    software_count: int
    vulnerability_count: VulnerabilityCount = Field(default_factory=VulnerabilityCount)
    risk_score: float = 0


class MachineDetail(MachineListItem):
    arch: str
    enrolled_at: datetime


class MachineList(BaseModel):
    items: list[MachineListItem]
    total: int
    page: int
    page_size: int
    total_pages: int


class MachineUpdate(BaseModel):
    """Admin edit. All fields optional — only those present in the request body
    are applied (partial update), so renaming a machine doesn't clobber its tags
    and vice versa. ``None`` is a valid value to clear display_name/owner."""

    tags: list[str] | None = Field(default=None, max_length=50)
    display_name: str | None = Field(default=None, max_length=255)
    owner: str | None = Field(default=None, max_length=255)


class TriggerScanResponse(BaseModel):
    manual_scan_requested: bool = True
